import { describe, expect, test } from 'vitest'
import type {
  DomainEvent,
  PendingAction,
  QueuedRequest,
  SessionSnapshot,
} from '../protocol/types'
import {
  actionResolutionSent,
  commandAccepted,
  commandRejected,
  commandSent,
  commandUnknown,
  connectionFailed,
  createInitialState,
  eventReceived,
  noticeReceived,
  pairLoaded,
  queuedCancelSent,
  reducer,
  reset,
  snapshotReceived,
  turnCancelSent,
  type ConnectionState,
  type MobileState,
} from './reducer'

// 队列夹具：保证 queueDepth 与 queuedRequests 长度一致（B2 §4.3）。
function queuedFixture(count: number): QueuedRequest[] {
  return Array.from({ length: count }, (_, index) => ({
    requestId: `req-q${index + 1}`,
    source: 'mobile' as const,
    preview: `排队输入 ${index + 1}`,
  }))
}

function approvalAction(overrides: Partial<PendingAction> = {}): PendingAction {
  return {
    actionId: 'act-1',
    requestId: 'req-9',
    kind: 'tool_approval',
    toolName: 'Bash',
    preview: 'ls -al',
    allowSessionAvailable: true,
    ...overrides,
  } as PendingAction
}

function questionAction(overrides: Partial<PendingAction> = {}): PendingAction {
  return {
    actionId: 'act-2',
    requestId: 'req-9',
    kind: 'ask_user_question',
    toolName: 'AskUserQuestion',
    allowSessionAvailable: false,
    questions: [
      {
        questionId: 'q-1',
        header: '范围',
        question: '选一个目录',
        inputType: 'choice',
        multiSelect: false,
        options: [{ label: 'src', description: '' }],
      },
    ],
    ...overrides,
  } as PendingAction
}

function snapshot(overrides: Partial<SessionSnapshot> = {}): SessionSnapshot {
  const merged: SessionSnapshot = {
    sessionId: 'session-1',
    sessionStatus: 'ready',
    sessionError: null,
    seq: 10,
    turn: {
      status: 'idle',
      requestId: '',
      source: 'mobile',
      message: '',
      error: null,
      pendingAction: null,
    },
    queueDepth: 0,
    queuedRequests: [],
    ...overrides,
  }
  // 只覆盖 queueDepth 时自动补齐列表，避免夹具自身违反协议不变量。
  if (merged.queuedRequests.length !== merged.queueDepth) {
    merged.queuedRequests = queuedFixture(merged.queueDepth)
  }
  return merged
}

function stateWithSnapshot(overrides: Partial<SessionSnapshot> = {}): MobileState {
  let state = createInitialState()
  state = reducer(state, pairLoaded())
  state = reducer(state, snapshotReceived(snapshot(overrides)))
  return state
}

function event(
  kind: DomainEvent['kind'],
  seq: number,
  fields: Partial<DomainEvent> = {},
): DomainEvent {
  return {
    seq,
    kind,
    sessionId: 'session-1',
    requestId: 'req-1',
    source: 'terminal',
    // turn_started 按协议必须携带非空正文；其余 kind 默认空。
    text: kind === 'turn_started' ? 'input text' : '',
    sessionStatus: 'ready',
    turnStatus: kind === 'turn_started' || kind === 'text_delta' ? 'running' : '',
    error: null,
    ...fields,
  }
}

describe('初始与配对', () => {
  test('初始状态未配对、无消息', () => {
    const state = createInitialState()
    expect(state.phase).toBe('unpaired')
    expect(state.messages).toEqual([])
    expect(state.canSend).toBe(false)
  })

  test('pairLoaded 进入连接中并保留消息为空', () => {
    const state = reducer(createInitialState(), pairLoaded())
    expect(state.phase).toBe('connecting')
    expect(state.messages).toEqual([])
  })
})

describe('snapshot 归并', () => {
  test('初始化 session 状态、seq 基准与 queueDepth', () => {
    const state = stateWithSnapshot({ seq: 10, queueDepth: 3 })
    expect(state.sessionStatus).toBe('ready')
    expect(state.seqBase).toBe(10)
    expect(state.queueDepthHint).toBe(3)
  })

  test('queueDepth > 0 时提示连接时有等待任务，= 0 无提示', () => {
    expect(stateWithSnapshot({ queueDepth: 3 }).queueDepthHint).toBe(3)
    expect(stateWithSnapshot({ queueDepth: 0 }).queueDepthHint).toBe(0)
  })

  test('snapshot 含运行中 Turn 时以 message 初始化 Claude 文本', () => {
    const state = stateWithSnapshot({
      seq: 5,
      turn: {
        status: 'running',
        requestId: 'req-9',
        source: 'terminal',
        message: '已有输出',
        error: null,
        pendingAction: null,
      },
    })
    expect(state.messages).toEqual([
      {
        kind: 'claude',
        requestId: 'req-9',
        source: 'terminal',
        text: '已有输出',
        status: 'streaming',
        error: null,
      },
    ])
  })

  test('snapshot 已连接后可发送', () => {
    expect(stateWithSnapshot().canSend).toBe(true)
  })

  test('Session stopped 的 snapshot 禁止发送', () => {
    expect(stateWithSnapshot({ sessionStatus: 'stopped' }).canSend).toBe(false)
  })

  test('Session failed 的 snapshot 禁止发送', () => {
    expect(stateWithSnapshot({ sessionStatus: 'failed' }).canSend).toBe(false)
  })
})

describe('event 严格连续 seq', () => {
  test('seq 基准 + 1 的 event 正常归并', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2' })))
    expect(state.messages.find((m) => m.kind === 'claude')?.requestId).toBe('req-2')
  })

  test('重复 seq → 协议错误', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11)))
    state = reducer(state, eventReceived(event('text_delta', 11, { text: 'x' })))
    expect(state.phase).toBe('protocolError')
  })

  test('倒退 seq → 协议错误', () => {
    const state = stateWithSnapshot()
    const next = reducer(state, eventReceived(event('turn_started', 9)))
    expect(next.phase).toBe('protocolError')
  })

  test('缺口 seq → 协议错误', () => {
    const state = stateWithSnapshot()
    const next = reducer(state, eventReceived(event('turn_started', 12)))
    expect(next.phase).toBe('protocolError')
  })
})

describe('Turn 与消息流', () => {
  test('turn_started → text_delta → turn_completed 按 requestId 累积', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2', source: 'mobile' })))
    state = reducer(state, eventReceived(event('text_delta', 12, { requestId: 'req-2', text: '你好' })))
    state = reducer(state, eventReceived(event('text_delta', 13, { requestId: 'req-2', text: '，世界' })))
    state = reducer(state, eventReceived(event('turn_completed', 14, { requestId: 'req-2', turnStatus: 'completed' })))

    const claude = state.messages.find((m) => m.kind === 'claude')
    expect(claude).toMatchObject({ requestId: 'req-2', text: '你好，世界', status: 'done' })
  })

  test('终端 turn_started 创建输入与空 Claude 回复', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2', source: 'terminal' })))
    expect(state.messages).toEqual([
      { kind: 'input', source: 'terminal', requestId: 'req-2', text: 'input text', status: null },
      { kind: 'claude', requestId: 'req-2', source: 'terminal', text: '', status: 'streaming', error: null },
    ])
  })

  test('turn_failed 记录错误并结束 Turn', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2' })))
    state = reducer(state, eventReceived(event('turn_failed', 12, {
      requestId: 'req-2',
      turnStatus: 'failed',
      error: { code: 'internal', message: 'boom' },
    })))
    const claude = state.messages.find((m) => m.kind === 'claude')
    expect(claude).toMatchObject({ status: 'failed', error: { code: 'internal' } })
    expect(state.sessionStatus).toBe('ready')
  })

  test('session_failed 更新 Session 状态并禁止发送', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('session_failed', 11, {
      sessionStatus: 'failed',
      error: { code: 'internal', message: 'boom' },
    })))
    expect(state.sessionStatus).toBe('failed')
    expect(state.canSend).toBe(false)
  })

  test('session_stopped 更新 Session 状态并禁止发送', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('session_stopped', 11, {
      sessionStatus: 'stopped',
    })))
    expect(state.sessionStatus).toBe('stopped')
    expect(state.canSend).toBe(false)
  })
})

describe('单飞命令', () => {
  test('pending 状态下 canSend 为 false', () => {
    let state = stateWithSnapshot()
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: 'hi' }))
    expect(state.canSend).toBe(false)
  })

  test('commandAccepted：手机正文入消息流并解除 pending', () => {
    let state = stateWithSnapshot()
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: 'hi' }))
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-5' }))
    expect(state.pendingCommand).toBeNull()
    expect(state.messages).toContainEqual({
      kind: 'input',
      source: 'mobile',
      requestId: 'req-5',
      text: 'hi',
      status: 'accepted',
    })
    expect(state.canSend).toBe(true)
  })

  test('commandRejected：正文不入流，保留输入', () => {
    let state = stateWithSnapshot()
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: 'hi' }))
    state = reducer(state, commandRejected({ commandId: 'cmd-1', code: 'queue_full', message: '队列已满' }))
    expect(state.pendingCommand).toBeNull()
    expect(state.messages).not.toContainEqual(expect.objectContaining({ kind: 'input' }))
    expect(state.lastError).toEqual({ code: 'queue_full', message: '队列已满' })
  })

  test('commandUnknown：结果未知反馈', () => {
    let state = stateWithSnapshot()
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: 'hi' }))
    state = reducer(state, commandUnknown({ commandId: 'cmd-1' }))
    expect(state.pendingCommand).toBeNull()
    expect(state.messages).toContainEqual({
      kind: 'input',
      source: 'mobile',
      requestId: null,
      text: 'hi',
      status: 'unknown',
    })
  })
})

describe('断线与重置', () => {
  test('connectionFailed：已断开、pending 标记结果未知', () => {
    let state = stateWithSnapshot()
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: 'hi' }))
    state = reducer(state, connectionFailed({ kind: 'disconnected' }))
    expect(state.connection).toBe('disconnected')
    expect(state.messages).toContainEqual({
      kind: 'input',
      source: 'mobile',
      requestId: null,
      text: 'hi',
      status: 'unknown',
    })
  })

  test('reset：清空一切回到未配对', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2' })))
    state = reducer(state, reset())
    expect(state).toEqual(createInitialState())
  })

  test('重连（pairLoaded 后新 snapshot）清空旧消息', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2' })))
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 20 })))
    expect(state.messages).toEqual([])
    expect(state.seqBase).toBe(20)
  })
})

describe('connectionState 字段被忽略', () => {
  test('不同 connectionStatus 值产生相同状态（仅校验类型）', () => {
    const a = stateWithSnapshot()
    const b = stateWithSnapshot()
    expect(a).toEqual(b)
  })
})

test('类型完备性：导出的连接阶段覆盖设计 §5.2', () => {
  const phases: ConnectionState[] = [
    'connecting',
    'authenticating',
    'subscribing',
    'connected',
    'disconnected',
  ]
  expect(phases).toHaveLength(5)
})

// ---- A5 闭环回归：真实浏览器联调中发现的问题 ----

describe('A5 闭环回归：命令后事件序列', () => {
  function loopSnapshot(seq: number): SessionSnapshot {
    return snapshot({ seq })
  }

  test('手机发送 → 命令接受 → turn_started → text_delta 应出现在消息流', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(loopSnapshot(7)))
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: '你好' }))
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-m' }))

    state = reducer(state, eventReceived(event('turn_started', 8, { requestId: 'req-m', source: 'mobile', text: '你好' })))
    state = reducer(state, eventReceived(event('text_delta', 9, { requestId: 'req-m', text: '回复' })))

    const claude = state.messages.find((m) => m.kind === 'claude')
    expect(claude).toMatchObject({ requestId: 'req-m', text: '回复', status: 'streaming' })
  })

  test('PC 端对话：turn_started(source=terminal) → text_delta 应渲染', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(loopSnapshot(7)))

    state = reducer(state, eventReceived(event('turn_started', 8, { source: 'terminal', requestId: 'req-pc' })))
    state = reducer(state, eventReceived(event('text_delta', 9, { source: 'terminal', requestId: 'req-pc', text: 'PC 回复' })))

    const claude = state.messages.find((m) => m.kind === 'claude')
    expect(claude).toMatchObject({ requestId: 'req-pc', text: 'PC 回复' })
  })

  test('连续多轮 Turn 均渲染', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(loopSnapshot(7)))

    state = reducer(state, eventReceived(event('turn_started', 8, { source: 'terminal', requestId: 'req-1' })))
    state = reducer(state, eventReceived(event('text_delta', 9, { requestId: 'req-1', text: '一' })))
    state = reducer(state, eventReceived(event('turn_completed', 10, { requestId: 'req-1', turnStatus: 'completed' })))
    state = reducer(state, eventReceived(event('turn_started', 11, { source: 'mobile', requestId: 'req-2' })))
    state = reducer(state, eventReceived(event('text_delta', 12, { requestId: 'req-2', text: '二' })))
    state = reducer(state, eventReceived(event('turn_completed', 13, { requestId: 'req-2', turnStatus: 'completed' })))

    const claudeMessages = state.messages.filter((m) => m.kind === 'claude')
    expect(claudeMessages).toHaveLength(2)
    expect(claudeMessages[0]).toMatchObject({ requestId: 'req-1', text: '一', status: 'done' })
    expect(claudeMessages[1]).toMatchObject({ requestId: 'req-2', text: '二', status: 'done' })
  })

  test('text_delta 无对应 turn_started 时不崩溃（静默忽略，非协议错误）', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(loopSnapshot(7)))
    state = reducer(state, eventReceived(event('text_delta', 8, { text: '孤儿' })))
    expect(state.phase).toBe('connected')
    expect(state.messages.filter((m) => m.kind === 'claude')).toHaveLength(0)
  })
})

// ---- A5.1：输入消息归并（D37）----

describe('A5.1 输入消息归并', () => {
  test('PC turn_started 先创建电脑端输入，再创建 Claude 回复', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, eventReceived(event('turn_started', 8, {
      source: 'terminal', requestId: 'req-pc', text: '来自电脑的问题',
    })))
    state = reducer(state, eventReceived(event('text_delta', 9, { requestId: 'req-pc', text: '回复' })))

    const input = state.messages.find((m) => m.kind === 'input')
    expect(input).toEqual({
      kind: 'input',
      source: 'terminal',
      requestId: 'req-pc',
      text: '来自电脑的问题',
      status: null,
    })
    const claude = state.messages.find((m) => m.kind === 'claude')
    expect(claude).toMatchObject({ requestId: 'req-pc', text: '回复' })
    expect(state.messages.indexOf(input!)).toBeLessThan(state.messages.indexOf(claude!))
  })

  test('本页面手机输入：commandAccepted 创建 accepted 输入，匹配 turn_started 只补 Claude 回复', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: '手机提问' }))
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-m' }))
    state = reducer(state, eventReceived(event('turn_started', 8, {
      source: 'mobile', requestId: 'req-m', text: '手机提问',
    })))
    state = reducer(state, eventReceived(event('text_delta', 9, { requestId: 'req-m', text: '答' })))

    const inputs = state.messages.filter((m) => m.kind === 'input')
    expect(inputs).toEqual([
      { kind: 'input', source: 'mobile', requestId: 'req-m', text: '手机提问', status: 'accepted' },
    ])
    expect(state.messages.filter((m) => m.kind === 'claude')).toHaveLength(1)
  })

  test('其他手机连接的输入：turn_started 创建无确认状态的输入', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, eventReceived(event('turn_started', 8, {
      source: 'mobile', requestId: 'req-other', text: '另一台手机',
    })))
    const input = state.messages.find((m) => m.kind === 'input')
    expect(input).toEqual({
      kind: 'input',
      source: 'mobile',
      requestId: 'req-other',
      text: '另一台手机',
      status: null,
    })
  })

  test('匹配 request ID 但正文冲突 → 协议错误', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: '手机提问' }))
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-m' }))
    state = reducer(state, eventReceived(event('turn_started', 8, {
      source: 'mobile', requestId: 'req-m', text: '被篡改的正文',
    })))
    expect(state.phase).toBe('protocolError')
  })

  test('匹配 request ID 但来源冲突 → 协议错误', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: '手机提问' }))
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-m' }))
    state = reducer(state, eventReceived(event('turn_started', 8, {
      source: 'terminal', requestId: 'req-m', text: '手机提问',
    })))
    expect(state.phase).toBe('protocolError')
  })

  test('同一 requestId 已有 Claude 回复后再次 turn_started → 协议错误（重复 Turn）', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, eventReceived(event('turn_started', 8, { requestId: 'req-1' })))
    state = reducer(state, eventReceived(event('turn_completed', 9, { requestId: 'req-1', turnStatus: 'completed' })))
    state = reducer(state, eventReceived(event('turn_started', 10, { requestId: 'req-1' })))
    expect(state.phase).toBe('protocolError')
  })

  test('断线未确认的手机输入保留 unknown 状态', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 7 })))
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: '悬而未决' }))
    state = reducer(state, connectionFailed({ kind: 'disconnected' }))
    const input = state.messages.find((m) => m.kind === 'input')
    expect(input).toEqual({
      kind: 'input',
      source: 'mobile',
      requestId: null,
      text: '悬而未决',
      status: 'unknown',
    })
  })

  test('snapshot 仍只创建 Claude 消息，不恢复输入正文', () => {
    const state = stateWithSnapshot({
      seq: 5,
      turn: {
        status: 'running',
        requestId: 'req-9',
        source: 'terminal',
        message: '已有输出',
        error: null,
        pendingAction: null,
      },
    })
    expect(state.messages.filter((m) => m.kind === 'input')).toHaveLength(0)
    expect(state.messages.filter((m) => m.kind === 'claude')).toHaveLength(1)
  })
})

// ---- 问题2回归：Turn 运行中订阅，text_delta 必须可归并 ----

describe('Turn 运行中订阅的 snapshot 归并（A5.1 验收回归）', () => {
  test('snapshot Turn 运行中且 message 为空时仍创建 Claude 占位，后续 text_delta 可累积', () => {
    let state = createInitialState()
    state = reducer(state, pairLoaded())
    // 订阅时 Turn 运行中、尚无文本（用户在 AI 回复前配对的场景）
    state = reducer(state, snapshotReceived(snapshot({
      seq: 8,
      turn: {
        status: 'running',
        requestId: 'req-first',
        source: 'terminal',
        message: '',
        error: null,
        pendingAction: null,
      },
    })))
    // 后续 text_delta 到达必须累积到该 Turn
    state = reducer(state, eventReceived(event('text_delta', 9, {
      requestId: 'req-first', text: '第一段',
    })))
    state = reducer(state, eventReceived(event('text_delta', 10, {
      requestId: 'req-first', text: '第二段',
    })))
    const claude = state.messages.find((m) => m.kind === 'claude')
    expect(claude).toMatchObject({
      requestId: 'req-first',
      text: '第一段第二段',
      status: 'streaming',
    })
    state = reducer(state, eventReceived(event('turn_completed', 11, {
      requestId: 'req-first', turnStatus: 'completed',
    })))
    expect(state.messages.find((m) => m.kind === 'claude')).toMatchObject({
      status: 'done',
    })
  })

  test('snapshot Turn 已完成且 message 为空时不创建占位（无可显示内容）', () => {
    const state = stateWithSnapshot({
      seq: 8,
      turn: {
        status: 'completed',
        requestId: 'req-old',
        source: 'terminal',
        message: '',
        error: null,
        pendingAction: null,
      },
    })
    expect(state.messages).toEqual([])
  })
})

// ---- B2：结构化内容块归并（§4.1）----

describe('B2 结构化内容块', () => {
  function blockEvent(seq: number, block: Record<string, unknown>): DomainEvent {
    return event('content_block', seq, { requestId: 'req-9', block } as Partial<DomainEvent>)
  }

  test('content_block 按到达顺序追加块消息', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, eventReceived(blockEvent(12, { id: 'b1', kind: 'thinking', thinking: '先看目录' })))
    state = reducer(
      state,
      eventReceived(
        blockEvent(13, {
          id: 'b2',
          kind: 'tool_use',
          toolUseId: 'tu-1',
          toolClass: 'builtin',
          toolName: 'Bash',
          mcpServer: '',
        }),
      ),
    )
    state = reducer(
      state,
      eventReceived(blockEvent(14, { id: 'b3', kind: 'tool_result', toolUseId: 'tu-1', isError: false })),
    )

    const blocks = state.messages.filter((m) => m.kind === 'block')
    expect(blocks.map((m) => m.block.kind)).toEqual(['thinking', 'tool_use', 'tool_result'])
    expect(blocks.every((m) => m.requestId === 'req-9')).toBe(true)
    expect(state.phase).toBe('connected')
  })

  test('content_block 后的文本新建片段但不丢失同 Turn 文本', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, eventReceived(event('text_delta', 12, { requestId: 'req-9', text: '一' })))
    state = reducer(state, eventReceived(blockEvent(13, { id: 'b1', kind: 'thinking', thinking: 'x' })))
    state = reducer(state, eventReceived(event('text_delta', 14, { requestId: 'req-9', text: '二' })))

    expect(state.messages.filter((m) => m.kind === 'claude').map((m) => m.text)).toEqual(['一', '二'])
  })

  test('文本、工具、后续文本保持事件到达顺序', () => {
    let state = stateWithSnapshot({
      turn: { status: 'running', requestId: 'req-9', source: 'terminal', message: '', error: null, pendingAction: null },
    })
    state = reducer(state, eventReceived(event('text_delta', 11, { requestId: 'req-9', text: '前置文本' })))
    state = reducer(state, eventReceived(event('content_block', 12, {
      requestId: 'req-9',
      block: { id: 'b-1', kind: 'tool_use', toolUseId: 'tu-1', toolClass: 'builtin', toolName: 'Bash', mcpServer: '', preview: 'ls' },
    })))
    state = reducer(state, eventReceived(event('text_delta', 13, { requestId: 'req-9', text: '后置文本' })))
    expect(state.messages.map((message) => message.kind === 'claude' ? message.text : message.kind)).toEqual([
      '前置文本',
      'block',
      '后置文本',
    ])
  })

  test('重连 snapshot 不伪造历史内容块', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, eventReceived(blockEvent(12, { id: 'b1', kind: 'thinking', thinking: 'x' })))
    state = reducer(state, pairLoaded())
    state = reducer(
      state,
      snapshotReceived(
        snapshot({
          seq: 30,
          turn: {
            status: 'running',
            requestId: 'req-9',
            source: 'terminal',
            message: '已有输出',
            error: null,
            pendingAction: null,
          },
        }),
      ),
    )
    expect(state.messages.filter((m) => m.kind === 'block')).toHaveLength(0)
    expect(state.messages.filter((m) => m.kind === 'claude')).toHaveLength(1)
  })
})

// ---- B2：pending action 与决议（§4.2 / §6）----

describe('B2 pending action', () => {
  function actionRequired(seq: number, action: PendingAction): DomainEvent {
    return event('action_required', seq, {
      requestId: 'req-9',
      turnStatus: 'waiting',
      pendingAction: action,
    } as Partial<DomainEvent>)
  }

  function waitingState(action: PendingAction = approvalAction()): MobileState {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    return reducer(state, eventReceived(actionRequired(12, action)))
  }

  test('action_required 设置 pending action 并禁用文本输入', () => {
    const state = waitingState()
    expect(state.pendingAction).toMatchObject({ actionId: 'act-1', kind: 'tool_approval' })
    expect(state.canSend).toBe(false)
    expect(state.canResolveAction).toBe(true)
  })

  test('ask_user_question 的 pending action 保留问题列表', () => {
    const state = waitingState(questionAction())
    if (state.pendingAction === null || state.pendingAction.kind !== 'ask_user_question') {
      throw new Error('unexpected action')
    }
    expect(state.pendingAction.questions).toHaveLength(1)
  })

  test('action_resolved 清除 pending action 并恢复输入', () => {
    let state = waitingState()
    state = reducer(
      state,
      eventReceived(event('action_resolved', 13, { requestId: 'req-9', turnStatus: 'running' })),
    )
    expect(state.pendingAction).toBeNull()
    expect(state.canSend).toBe(true)
    expect(state.canResolveAction).toBe(false)
  })

  test('决议在途时禁用重复提交', () => {
    let state = waitingState()
    state = reducer(state, actionResolutionSent({ commandId: 'cmd-1', actionId: 'act-1' }))
    expect(state.canResolveAction).toBe(false)
    expect(state.canSend).toBe(false)
    expect(state.pendingCommand).toMatchObject({ commandId: 'cmd-1', kind: 'resolve_action' })
  })

  test('决议被接受后清除 pending action，且不把决议当作输入正文', () => {
    let state = waitingState()
    state = reducer(state, actionResolutionSent({ commandId: 'cmd-1', actionId: 'act-1' }))
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-9' }))
    expect(state.pendingAction).toBeNull()
    expect(state.pendingCommand).toBeNull()
    // 夹具里的终端输入来自 turn_started（A5.1 契约），决议本身不得再产生
    // 任何手机输入正文。
    expect(
      state.messages.filter((m) => m.kind === 'input' && m.source === 'mobile'),
    ).toHaveLength(0)
  })

  // 双客户端竞态：另一端已完成决议，本端必须明确提示而不是一直等。
  test('action_not_pending 拒绝显示“已由其他客户端处理”并清除 action', () => {
    let state = waitingState()
    state = reducer(state, actionResolutionSent({ commandId: 'cmd-1', actionId: 'act-1' }))
    state = reducer(
      state,
      commandRejected({ commandId: 'cmd-1', code: 'action_not_pending', message: '' }),
    )
    expect(state.pendingAction).toBeNull()
    expect(state.lastError?.code).toBe('action_not_pending')
    expect(state.lastError?.message).toContain('已由其他客户端处理')
  })

  test('invalid_answer 拒绝保留 pending action 供重填', () => {
    let state = waitingState(questionAction())
    state = reducer(state, actionResolutionSent({ commandId: 'cmd-1', actionId: 'act-2' }))
    state = reducer(
      state,
      commandRejected({ commandId: 'cmd-1', code: 'invalid_answer', message: '' }),
    )
    expect(state.pendingAction).not.toBeNull()
    expect(state.canResolveAction).toBe(true)
    expect(state.lastError?.code).toBe('invalid_answer')
  })

  test('action_timeout 拒绝清除 action', () => {
    let state = waitingState()
    state = reducer(state, actionResolutionSent({ commandId: 'cmd-1', actionId: 'act-1' }))
    state = reducer(
      state,
      commandRejected({ commandId: 'cmd-1', code: 'action_timeout', message: '' }),
    )
    expect(state.pendingAction).toBeNull()
    expect(state.lastError?.code).toBe('action_timeout')
  })

  test('waiting snapshot 恢复 pending action，且重连后仍禁用输入', () => {
    const state = stateWithSnapshot({
      seq: 12,
      turn: {
        status: 'waiting',
        requestId: 'req-9',
        source: 'terminal',
        message: '已有输出',
        error: null,
        pendingAction: approvalAction(),
      },
    })
    expect(state.pendingAction).toMatchObject({ actionId: 'act-1' })
    expect(state.canSend).toBe(false)
    expect(state.canResolveAction).toBe(true)
    expect(state.messages.find((m) => m.kind === 'claude')).toMatchObject({ status: 'streaming' })
  })

  test('turn_completed 清除未决 action', () => {
    let state = waitingState()
    state = reducer(
      state,
      eventReceived(event('turn_completed', 13, { requestId: 'req-9', turnStatus: 'completed' })),
    )
    expect(state.pendingAction).toBeNull()
  })

  test('session_failed 清除未决 action 并禁用一切输入', () => {
    let state = waitingState()
    state = reducer(
      state,
      eventReceived(
        event('session_failed', 13, {
          sessionStatus: 'failed',
          error: { code: 'internal', message: 'boom' },
        }),
      ),
    )
    expect(state.pendingAction).toBeNull()
    expect(state.canSend).toBe(false)
    expect(state.canResolveAction).toBe(false)
  })

  test('session_stopped 清除未决 action 与队列但保留已收内容', () => {
    let state = stateWithSnapshot({ queueDepth: 2 })
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, eventReceived(event('text_delta', 12, { requestId: 'req-9', text: '已收' })))
    state = reducer(
      state,
      eventReceived(event('session_stopped', 13, { sessionStatus: 'stopped' })),
    )
    expect(state.pendingAction).toBeNull()
    expect(state.queuedRequests).toEqual([])
    expect(state.queueDepthHint).toBe(0)
    expect(state.messages.find((m) => m.kind === 'claude')).toMatchObject({ text: '已收' })
  })
})

// ---- B2：取消与排队（§4.3 / §5）----

describe('B2 取消与排队', () => {
  test('snapshot 恢复排队列表与深度', () => {
    const state = stateWithSnapshot({ queueDepth: 2 })
    expect(state.queuedRequests).toHaveLength(2)
    expect(state.queueDepthHint).toBe(2)
    expect(state.queuedRequests[0]).toMatchObject({ requestId: 'req-q1', source: 'mobile' })
  })

  test('排队项转为运行中 Turn 后移出列表', () => {
    let state = stateWithSnapshot({ queueDepth: 2 })
    state = reducer(
      state,
      eventReceived(event('turn_started', 11, { requestId: 'req-q1', source: 'mobile' })),
    )
    expect(state.queuedRequests.map((q) => q.requestId)).toEqual(['req-q2'])
    expect(state.queueDepthHint).toBe(1)
  })

  test('cancel_queued 被接受后本地移除队列项（服务端不发事件）', () => {
    let state = stateWithSnapshot({ queueDepth: 2 })
    state = reducer(state, queuedCancelSent({ commandId: 'cmd-1', requestId: 'req-q1' }))
    expect(state.canSend).toBe(false)
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-q1' }))
    expect(state.queuedRequests.map((q) => q.requestId)).toEqual(['req-q2'])
    expect(state.queueDepthHint).toBe(1)
    expect(state.messages.filter((m) => m.kind === 'input')).toHaveLength(0)
    expect(state.canSend).toBe(true)
  })

  test('request_not_queued 拒绝时清除陈旧队列项', () => {
    let state = stateWithSnapshot({ queueDepth: 2 })
    state = reducer(state, queuedCancelSent({ commandId: 'cmd-1', requestId: 'req-q1' }))
    state = reducer(
      state,
      commandRejected({ commandId: 'cmd-1', code: 'request_not_queued', message: '' }),
    )
    expect(state.queuedRequests.map((q) => q.requestId)).toEqual(['req-q2'])
    expect(state.lastError?.code).toBe('request_not_queued')
  })

  test('cancel_turn 在途时禁用发送，完成后恢复', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, turnCancelSent({ commandId: 'cmd-1', requestId: 'req-9' }))
    expect(state.canSend).toBe(false)
    expect(state.pendingCommand).toMatchObject({ kind: 'cancel_turn', targetId: 'req-9' })
    state = reducer(state, commandAccepted({ commandId: 'cmd-1', requestId: 'req-9' }))
    // 取消命令不是输入正文：终端输入仍只有 turn_started 创建的那一条。
    expect(
      state.messages.filter((m) => m.kind === 'input' && m.source === 'mobile'),
    ).toHaveLength(0)
    expect(state.canSend).toBe(true)
  })

  // cancel_turn 在服务端复用 turn_completed 事件，只靠 turnStatus 区分。
  test('turn_completed + turnStatus=cancelled 显示为已取消', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, eventReceived(event('text_delta', 12, { requestId: 'req-9', text: '半句' })))
    state = reducer(
      state,
      eventReceived(event('turn_completed', 13, { requestId: 'req-9', turnStatus: 'cancelled' })),
    )
    expect(state.messages.find((m) => m.kind === 'claude')).toMatchObject({
      text: '半句',
      status: 'cancelled',
    })
  })

  test('turn_not_cancellable 拒绝给出安全文案', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-9' })))
    state = reducer(state, turnCancelSent({ commandId: 'cmd-1', requestId: 'req-9' }))
    state = reducer(
      state,
      commandRejected({ commandId: 'cmd-1', code: 'turn_not_cancellable', message: '' }),
    )
    expect(state.lastError?.code).toBe('turn_not_cancellable')
    expect(state.lastError?.message).not.toBe('')
  })
})

// ---- B2：unsupported 通知（§4.4）----

describe('B2 unsupported 通知', () => {
  test('四类 code 均转为可读提示且不改变连接阶段', () => {
    for (const code of [
      'unknown_block',
      'unknown_subtype',
      'concurrent_action',
      'text_input_disabled',
    ] as const) {
      const state = reducer(stateWithSnapshot(), noticeReceived(code, 11))
      expect(state.lastNotice).not.toBeNull()
      expect(state.lastNotice?.code).toBe(code)
      expect(state.lastNotice?.message).not.toBe('')
      expect(state.phase).toBe('connected')
      expect(state.canSend).toBe(true)
    }
  })

  // 服务端 unsupported 通知经 publish 递增并占用一个 seq：通知必须像普通
  // 事件一样推进 seqBase，否则其后事件会被误判为缺口而断连（真实 Claude
  // 复测缺陷）。
  test('通知消费 seq 并推进 seqBase，其后事件不缺口', () => {
    let state = stateWithSnapshot()
    state = reducer(state, noticeReceived('unknown_subtype', 11))
    expect(state.seqBase).toBe(11)
    expect(state.phase).toBe('connected')
    state = reducer(state, eventReceived(event('turn_started', 12, { requestId: 'req-9' })))
    expect(state.phase).toBe('connected')
    expect(state.seqBase).toBe(12)
  })

  test('通知 seq 缺口按协议错误处理', () => {
    const state = reducer(stateWithSnapshot(), noticeReceived('unknown_subtype', 13))
    expect(state.phase).toBe('protocolError')
    expect(state.canSend).toBe(false)
  })

  test('新 snapshot 清除陈旧通知', () => {
    let state = reducer(stateWithSnapshot(), noticeReceived('concurrent_action', 11))
    state = reducer(state, pairLoaded())
    state = reducer(state, snapshotReceived(snapshot({ seq: 20 })))
    expect(state.lastNotice).toBeNull()
  })
})
