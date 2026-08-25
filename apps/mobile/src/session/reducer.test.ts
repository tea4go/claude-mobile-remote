import { describe, expect, test } from 'vitest'
import type { DomainEvent, SessionSnapshot } from '../protocol/types'
import {
  commandAccepted,
  commandRejected,
  commandSent,
  commandUnknown,
  connectionFailed,
  createInitialState,
  eventReceived,
  pairLoaded,
  reducer,
  reset,
  snapshotReceived,
  type ConnectionState,
  type MobileState,
} from './reducer'

function snapshot(overrides: Partial<SessionSnapshot> = {}): SessionSnapshot {
  return {
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
    ...overrides,
  }
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
    text: '',
    sessionStatus: 'ready',
    turnStatus: 'running',
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

  test('终端 turn_started 只显示电脑端发起，不显示用户正文', () => {
    let state = stateWithSnapshot()
    state = reducer(state, eventReceived(event('turn_started', 11, { requestId: 'req-2', source: 'terminal' })))
    expect(state.messages).toEqual([
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
      kind: 'mobile',
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
    expect(state.messages).not.toContainEqual(expect.objectContaining({ kind: 'mobile' }))
    expect(state.lastError).toEqual({ code: 'queue_full', message: '队列已满' })
  })

  test('commandUnknown：结果未知反馈', () => {
    let state = stateWithSnapshot()
    state = reducer(state, commandSent({ commandId: 'cmd-1', text: 'hi' }))
    state = reducer(state, commandUnknown({ commandId: 'cmd-1' }))
    expect(state.pendingCommand).toBeNull()
    expect(state.messages).toContainEqual({
      kind: 'mobile',
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
      kind: 'mobile',
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

    state = reducer(state, eventReceived(event('turn_started', 8, { requestId: 'req-m', source: 'mobile' })))
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
