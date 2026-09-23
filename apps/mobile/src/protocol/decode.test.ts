import { describe, expect, test } from 'vitest'
import { decodeServerMessage } from './decode'
import type { Envelope } from './types'

// 构造合法 envelope 的工厂；各用例按需覆盖字段。
function baseEnvelope(overrides: Partial<Envelope> = {}): Envelope {
  return {
    version: 2,
    type: 'hello_ack',
    sessionId: 'session-1',
    commandId: '',
    requestId: '',
    seq: 0,
    payload: {},
    ...overrides,
  }
}

function snapshotPayload() {
  return {
    connectionStatus: 'subscribed',
    snapshot: {
      sessionId: 'session-1',
      sessionStatus: 'ready' as const,
      sessionError: null as { code: string; message: string } | null,
      seq: 7,
      turn: {
        status: 'idle' as string,
        requestId: '',
        source: 'mobile' as const,
        message: '',
        error: null as { code: string; message: string } | null,
        pendingAction: null as Record<string, unknown> | null,
      },
      queueDepth: 0,
      queuedRequests: [] as unknown,
    },
  }
}

// B2 §4.2：tool_approval 形态的 pendingAction；preview 可缺省。
function approvalAction(overrides: Record<string, unknown> = {}) {
  return {
    actionId: 'act-1',
    requestId: 'req-1',
    kind: 'tool_approval',
    toolName: 'Bash',
    preview: 'ls -al',
    allowSessionAvailable: true,
    ...overrides,
  }
}

// B2 §4.2：ask_user_question 形态的 pendingAction；toolName 固定为
// AskUserQuestion，且不得携带 preview。
function questionAction(overrides: Record<string, unknown> = {}) {
  return {
    actionId: 'act-2',
    requestId: 'req-1',
    kind: 'ask_user_question',
    toolName: 'AskUserQuestion',
    allowSessionAvailable: false,
    questions: [
      {
        questionId: 'q-1',
        header: '范围',
        question: '选择一个目录',
        inputType: 'choice',
        multiSelect: false,
        options: [{ label: 'src', description: '源码' }, { label: 'docs' }],
      },
    ],
    ...overrides,
  }
}

// waiting 状态的 snapshot：pendingAction 非空时 turn 必为 waiting，且
// pendingAction.requestId 必须等于 turn.requestId。
function waitingSnapshotPayload(action: Record<string, unknown> = approvalAction()) {
  const payload = snapshotPayload()
  payload.snapshot.turn.status = 'waiting'
  payload.snapshot.turn.requestId = 'req-1'
  payload.snapshot.turn.message = 'run the build'
  payload.snapshot.turn.pendingAction = action
  return payload
}

// content_block 事件：三类 kind 共用 envelope 形状，block 内字段按 kind 白名单。
function blockEventPayload(block: Record<string, unknown>) {
  return {
    event: {
      seq: 8,
      kind: 'content_block',
      sessionId: 'session-1',
      requestId: 'req-1',
      source: 'mobile',
      text: '',
      sessionStatus: '',
      turnStatus: '',
      error: null,
      block,
    },
  }
}

function actionRequiredPayload(action: Record<string, unknown> = approvalAction()) {
  return {
    event: {
      seq: 8,
      kind: 'action_required',
      sessionId: 'session-1',
      requestId: 'req-1',
      source: 'mobile',
      text: '',
      sessionStatus: '',
      turnStatus: 'waiting',
      error: null,
      pendingAction: action,
    },
  }
}

function eventPayload() {
  return {
    event: {
      seq: 8,
      kind: 'text_delta' as 'text_delta' | 'turn_completed',
      sessionId: 'session-1',
      requestId: 'req-1',
      source: 'mobile' as const,
      text: 'hello',
      // 服务端按 kind 部分携带字段：text_delta 不带状态枚举（空字符串）。
      sessionStatus: '' as string,
      turnStatus: '' as string,
      error: null as { code: string; message: string } | null,
    },
  }
}

function turnStartedPayload() {
  return {
    event: {
      seq: 8,
      kind: 'turn_started' as const,
      sessionId: 'session-1',
      requestId: 'req-9',
      source: 'mobile' as 'mobile' | '',
      text: 'from phone',
      sessionStatus: '' as string,
      turnStatus: 'running' as 'running' | 'completed',
      error: null as { code: string; message: string } | null,
    },
  }
}

describe('decodeServerMessage：合法消息', () => {
  test('hello_ack：空对象 payload', () => {
    const message = decodeServerMessage(JSON.stringify(baseEnvelope()))
    expect(message.type).toBe('hello_ack')
  })

  test('subscribed：空对象 payload', () => {
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'subscribed' })),
    )
    expect(message.type).toBe('subscribed')
  })

  test('snapshot：完整 payload 结构', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({ type: 'snapshot', seq: 7, payload: snapshotPayload() }),
      ),
    )
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    expect(message.payload.snapshot.seq).toBe(7)
    expect(message.payload.snapshot.sessionStatus).toBe('ready')
  })

  test('event：完整 payload 结构', () => {
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload: eventPayload() })),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    expect(message.payload.event.kind).toBe('text_delta')
    expect(message.payload.event.text).toBe('hello')
  })

  test('command_result：accepted 结果', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'command_result',
          commandId: 'cmd-1',
          requestId: 'req-1',
          payload: { accepted: true, code: '', message: '' },
        }),
      ),
    )
    if (message.type !== 'command_result') throw new Error('unexpected type')
    expect(message.payload.accepted).toBe(true)
  })

  test('error：错误码与消息', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'error',
          payload: { code: 'unauthorized', message: 'invalid pair token' },
        }),
      ),
    )
    if (message.type !== 'error') throw new Error('unexpected type')
    expect(message.payload.code).toBe('unauthorized')
  })

  test('session snapshot 的 pendingAction 可为真实 tool_approval', () => {
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload: waitingSnapshotPayload() })),
    )
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    const action = message.payload.snapshot.turn.pendingAction
    if (action === null || action.kind !== 'tool_approval') throw new Error('unexpected action')
    expect(action.actionId).toBe('act-1')
    expect(action.toolName).toBe('Bash')
    expect(action.preview).toBe('ls -al')
    expect(action.allowSessionAvailable).toBe(true)
  })

  test('snapshot turn error 与 sessionError 可为非空对象', () => {
    const payload = snapshotPayload()
    payload.snapshot.sessionError = { code: 'internal', message: 'boom' }
    payload.snapshot.turn.error = { code: 'internal', message: 'turn boom' }
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload })),
    )
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    expect(message.payload.snapshot.sessionError?.code).toBe('internal')
    expect(message.payload.snapshot.turn.error?.message).toBe('turn boom')
  })

  // A5.2 人工验收回归：首轮前 snapshot 的 turn 尚无任何 Turn，source
  // 与 requestId 均为空串（Go 零值）。下方字面量与
  // TestWebSocketPreTurnSnapshotTurnWireShape 断言的 wire 形状逐字对应。
  test('首轮前 snapshot：turn.source 为空串时仍可解码', () => {
    const wire = JSON.stringify({
      version: 2,
      type: 'snapshot',
      sessionId: 'session-1',
      commandId: '',
      requestId: '',
      seq: 0,
      payload: {
        connectionStatus: 'subscribed',
        snapshot: {
          sessionId: 'session-1',
          sessionStatus: 'starting',
          sessionError: null,
          seq: 0,
          turn: {
            status: 'idle',
            requestId: '',
            source: '',
            message: '',
            error: null,
            pendingAction: null,
          },
          queueDepth: 0,
          queuedRequests: null,
        },
      },
    })
    const message = decodeServerMessage(wire)
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    expect(message.payload.snapshot.sessionStatus).toBe('starting')
    expect(message.payload.snapshot.turn.status).toBe('idle')
    expect(message.payload.snapshot.turn.source).toBe('')
  })

  test('snapshot：requestId 非空但 source 为空串拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.turn.requestId = 'req-1'
    payload.snapshot.turn.source = '' as unknown as 'mobile'
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow()
  })
})

describe('decodeServerMessage：非法 JSON 与基础结构', () => {
  test('非 JSON 文本拒绝', () => {
    expect(() => decodeServerMessage('not json')).toThrow()
  })

  test('非对象顶层（数组）拒绝', () => {
    expect(() => decodeServerMessage('[]')).toThrow()
  })

  test('null 拒绝', () => {
    expect(() => decodeServerMessage('null')).toThrow()
  })

  test('缺 version 拒绝', () => {
    const envelope = baseEnvelope()
    delete (envelope as Partial<Envelope>).version
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('version 为字符串拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":"1","type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":0,"payload":{}}'),
    ).toThrow()
  })

  test('version = 1（B1 旧客户端形态）拒绝', () => {
    const envelope = baseEnvelope({ version: 1 })
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('version = 4（未来版本）拒绝', () => {
    const envelope = baseEnvelope({ version: 4 })
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('v3 snapshot 解码目录与稳定标题，v2 禁止新字段', () => {
    const payload = snapshotPayload()
    const snapshot = payload.snapshot as typeof payload.snapshot & { projectCwd?: string; sessionTitle?: string; createdAt?: string; lastActivityAt?: string }
    snapshot.projectCwd = '/work/project'
    snapshot.sessionTitle = '修复登录'
    snapshot.createdAt = '2026-09-23T01:00:00Z'
    snapshot.lastActivityAt = '2026-09-23T02:00:00Z'
    const wire = baseEnvelope({ version: 3, type: 'snapshot', seq: 7, payload })
    const decoded = decodeServerMessage(JSON.stringify(wire))
    expect(decoded.type === 'snapshot' && decoded.payload.snapshot.projectCwd).toBe('/work/project')
    expect(() => decodeServerMessage(JSON.stringify({ ...wire, version: 2 }))).toThrow()
    snapshot.lastActivityAt = '2026-09-23T10:00:00+08:00'
    expect(() => decodeServerMessage(JSON.stringify(wire))).toThrow()
  })

  test('缺 type 拒绝', () => {
    const envelope = baseEnvelope()
    delete (envelope as Partial<Envelope>).type
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('未知 type 拒绝', () => {
    const envelope = baseEnvelope({ type: 'mystery' as Envelope['type'] })
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('payload 非对象（字符串）拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":2,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":0,"payload":"x"}'),
    ).toThrow()
  })

  test('payload 为 null 拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":2,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":0,"payload":null}'),
    ).toThrow()
  })

  test('seq 缺失拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":2,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","payload":{}}'),
    ).toThrow()
  })

  test('seq 为负数拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":2,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":-1,"payload":{}}'),
    ).toThrow()
  })
})

describe('decodeServerMessage：方向校验', () => {
  test('客户端方向消息 hello 拒绝', () => {
    const envelope = baseEnvelope({ type: 'hello' })
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('客户端方向消息 command 拒绝', () => {
    const envelope = baseEnvelope({ type: 'command' })
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
  })

  test('客户端方向消息 subscribe / unsubscribe 拒绝', () => {
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'subscribe' }))),
    ).toThrow()
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'unsubscribe' }))),
    ).toThrow()
  })
})

describe('decodeServerMessage：payload 结构校验', () => {
  test('snapshot 的嵌套 sessionId 与 seq 必须匹配 envelope', () => {
    const payload = snapshotPayload()
    payload.snapshot.sessionId = 'other-session'
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()

    const mismatchedSeq = snapshotPayload()
    mismatchedSeq.snapshot.seq = 8
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload: mismatchedSeq }))),
    ).toThrow()
  })

  test('event 的嵌套 sessionId、requestId 与 seq 必须匹配 envelope', () => {
    const payload = eventPayload()
    payload.event.sessionId = 'other-session'
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow()

    const mismatchedRequest = eventPayload()
    mismatchedRequest.event.requestId = 'other-request'
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload: mismatchedRequest }))),
    ).toThrow()
  })

  test('snapshot 缺 connectionStatus 拒绝', () => {
    const payload = snapshotPayload() as Record<string, unknown>
    delete payload.connectionStatus
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()
  })

  test('snapshot.snapshot 缺 sessionStatus 拒绝', () => {
    const payload = snapshotPayload() as { snapshot: Record<string, unknown> }
    delete payload.snapshot.sessionStatus
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()
  })

  test('snapshot 未知 sessionStatus 枚举拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.sessionStatus = 'exploded' as never
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()
  })

  test('snapshot 未知 turn status 枚举拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.turn.status = 'exploded' as never
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()
  })

  test('snapshot seq 为负数拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.seq = -3
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()
  })

  test('snapshot queueDepth 为负数拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.queueDepth = -1
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', payload }))),
    ).toThrow()
  })

  test('event 缺 event 对象拒绝', () => {
    const payload = {}
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow()
  })

  test('event 未知 kind 枚举拒绝', () => {
    const payload = eventPayload()
    payload.event.kind = 'exploded' as never
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow()
  })

  test('event 未知 source 枚举拒绝', () => {
    const payload = eventPayload()
    payload.event.source = 'watch' as never
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow()
  })

  test('事件枚举字段允许空字符串（服务端零值序列化，A5 联调回归）', () => {
    // 真实服务端 text_delta：sessionStatus/turnStatus 均为空字符串。
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: {
            event: {
              seq: 8,
              kind: 'text_delta',
              sessionId: 'session-1',
              requestId: 'req-1',
              source: 'terminal',
              text: 'hi',
              sessionStatus: '',
              turnStatus: '',
              error: null,
            },
          },
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    expect(message.payload.event.text).toBe('hi')
    expect(message.payload.event.sessionStatus).toBe('')
  })

  test('事件 kind 为空字符串拒绝', () => {
    const payload = eventPayload()
    payload.event.kind = '' as never
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow()
  })

  test('turn_started 合法正文可解码（A5.1）', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-9',
          seq: 8,
          payload: {
            event: {
              seq: 8,
              kind: 'turn_started',
              sessionId: 'session-1',
              requestId: 'req-9',
              source: 'terminal',
              text: 'from pc',
              sessionStatus: '',
              turnStatus: 'running',
              error: null,
            },
          },
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    expect(message.payload.event.text).toBe('from pc')
  })

  test('turn_started 空 requestId 拒绝', () => {
    const payload = turnStartedPayload()
    payload.event.requestId = ''
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', requestId: '', payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('turn_started 空正文拒绝（envelope 一致，纯语义校验）', () => {
    const payload = turnStartedPayload()
    payload.event.text = ''
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-9', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('turn_started 空 source 拒绝', () => {
    const payload = turnStartedPayload()
    payload.event.source = ''
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-9', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('turn_started 空正文拒绝', () => {
    const payload = turnStartedPayload()
    payload.event.text = ''
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('turn_started 非 running 的 turnStatus 拒绝', () => {
    const payload = turnStartedPayload()
    payload.event.turnStatus = 'completed'
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-9', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('turn_started 超过 1 MiB UTF-8 字节的正文拒绝', () => {
    const payload = turnStartedPayload()
    payload.event.text = '你'.repeat(350_000) // 1050000 字节 > 1 MiB
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-9', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('text_delta 空正文拒绝', () => {
    const payload = eventPayload()
    payload.event.text = ''
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('turn_completed 携带正文拒绝', () => {
    const payload = eventPayload()
    payload.event.kind = 'turn_completed'
    payload.event.turnStatus = 'completed'
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('event seq 为负数拒绝', () => {
    const payload = eventPayload()
    payload.event.seq = -1
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'event', payload }))),
    ).toThrow()
  })

  test('command_result 缺 accepted 拒绝', () => {
    const payload = { code: '', message: '' }
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'command_result', payload })),
      ),
    ).toThrow()
  })

  test('command_result accepted 非布尔拒绝', () => {
    const payload = { accepted: 'yes', code: '', message: '' }
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'command_result', payload })),
      ),
    ).toThrow()
  })

  test('error 缺 code 拒绝', () => {
    const payload = { message: 'x' }
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'error', payload }))),
    ).toThrow()
  })

  test('unsubscribed：空对象 payload 合法', () => {
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'unsubscribed' })),
    )
    expect(message.type).toBe('unsubscribed')
  })

  test('error payload 的 expectedVersion 可读出（版本不匹配提示升级）', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'error',
          payload: {
            code: 'unsupported_version',
            message: 'unsupported protocol version',
            expectedVersion: 2,
          },
        }),
      ),
    )
    if (message.type !== 'error') throw new Error('unexpected type')
    expect(message.payload.code).toBe('unsupported_version')
    expect(message.payload.expectedVersion).toBe(2)
  })

  test('error payload 缺 expectedVersion 仍合法（非版本错误不携带）', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({ type: 'error', payload: { code: 'unauthorized', message: 'nope' } }),
      ),
    )
    if (message.type !== 'error') throw new Error('unexpected type')
    expect(message.payload.expectedVersion).toBeUndefined()
  })
})

describe('decodeServerMessage：content_block（B2 §4.1）', () => {
  test('thinking 块可解码', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: blockEventPayload({ id: 'blk-1', kind: 'thinking', thinking: '先看目录' }),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const block = message.payload.event.block
    if (block === undefined || block.kind !== 'thinking') throw new Error('unexpected block')
    expect(block.id).toBe('blk-1')
    expect(block.thinking).toBe('先看目录')
  })

  test('builtin tool_use 块可解码且不带 mcpServer', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: blockEventPayload({
            id: 'blk-2',
            kind: 'tool_use',
            toolUseId: 'tu-1',
            toolClass: 'builtin',
            toolName: 'Bash',
          }),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const block = message.payload.event.block
    if (block === undefined || block.kind !== 'tool_use') throw new Error('unexpected block')
    expect(block.toolClass).toBe('builtin')
    expect(block.toolName).toBe('Bash')
    expect(block.mcpServer).toBe('')
  })

  test('mcp tool_use 块必须带 mcpServer', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: blockEventPayload({
            id: 'blk-3',
            kind: 'tool_use',
            toolUseId: 'tu-2',
            toolClass: 'mcp',
            toolName: 'lookup',
            mcpServer: 'docs',
          }),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const block = message.payload.event.block
    if (block === undefined || block.kind !== 'tool_use') throw new Error('unexpected block')
    expect(block.mcpServer).toBe('docs')
  })

  test('builtin tool_use 携带 mcpServer 拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'event',
            requestId: 'req-1',
            seq: 8,
            payload: blockEventPayload({
              id: 'blk-4',
              kind: 'tool_use',
              toolUseId: 'tu-3',
              toolClass: 'builtin',
              toolName: 'Bash',
              mcpServer: 'docs',
            }),
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('mcp tool_use 缺 mcpServer 拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'event',
            requestId: 'req-1',
            seq: 8,
            payload: blockEventPayload({
              id: 'blk-5',
              kind: 'tool_use',
              toolUseId: 'tu-4',
              toolClass: 'mcp',
              toolName: 'lookup',
            }),
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('tool_result 块只带 toolUseId 与布尔 isError', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: blockEventPayload({
            id: 'blk-6',
            kind: 'tool_result',
            toolUseId: 'tu-1',
            isError: false,
          }),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const block = message.payload.event.block
    if (block === undefined || block.kind !== 'tool_result') throw new Error('unexpected block')
    expect(block.toolUseId).toBe('tu-1')
    expect(block.isError).toBe(false)
  })

  test('tool_result 缺 isError 拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'event',
            requestId: 'req-1',
            seq: 8,
            payload: blockEventPayload({ id: 'blk-7', kind: 'tool_result', toolUseId: 'tu-1' }),
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('tool_result 携带 thinking 等越界字段拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'event',
            requestId: 'req-1',
            seq: 8,
            payload: blockEventPayload({
              id: 'blk-8',
              kind: 'tool_result',
              toolUseId: 'tu-1',
              isError: false,
              thinking: '不应该出现',
            }),
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('未知 block kind 拒绝（unsupported 不是第四类 block）', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'event',
            requestId: 'req-1',
            seq: 8,
            payload: blockEventPayload({ id: 'blk-9', kind: 'unsupported' }),
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('content_block 缺 block 拒绝', () => {
    const payload = blockEventPayload({ id: 'blk-10', kind: 'thinking', thinking: 'x' }) as {
      event: Record<string, unknown>
    }
    delete payload.event.block
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('content_block 同时携带 pendingAction 拒绝', () => {
    const payload = blockEventPayload({ id: 'blk-11', kind: 'thinking', thinking: 'x' }) as {
      event: Record<string, unknown>
    }
    payload.event.pendingAction = approvalAction()
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('非 content_block 事件携带 block 拒绝', () => {
    const payload = eventPayload() as unknown as { event: Record<string, unknown> }
    payload.event.block = { id: 'blk-12', kind: 'thinking', thinking: 'x' }
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('事件携带非空 code 拒绝（unsupported 只走自己的 type）', () => {
    const payload = eventPayload() as unknown as { event: Record<string, unknown> }
    payload.event.code = 'unknown_block'
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('事件 kind 为 unsupported 拒绝', () => {
    const payload = eventPayload() as unknown as { event: Record<string, unknown> }
    payload.event.kind = 'unsupported'
    payload.event.text = ''
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })
})

describe('decodeServerMessage：action_required / action_resolved（B2 §4.2）', () => {
  test('tool_approval 的 action_required 可解码', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload: actionRequiredPayload() }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const action = message.payload.event.pendingAction
    if (action === undefined || action.kind !== 'tool_approval') throw new Error('unexpected action')
    expect(action.actionId).toBe('act-1')
    expect(action.requestId).toBe('req-1')
  })

  test('tool_approval 缺省 preview 仍可解码（不自行拼接）', () => {
    const action = approvalAction() as Record<string, unknown>
    delete action.preview
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: actionRequiredPayload(action),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const decoded = message.payload.event.pendingAction
    if (decoded === undefined || decoded.kind !== 'tool_approval') throw new Error('unexpected action')
    expect(decoded.preview).toBe('')
  })

  test('ask_user_question 多问题、单选与多选可解码', () => {
    const action = questionAction({
      questions: [
        {
          questionId: 'q-1',
          header: '范围',
          question: '选一个目录',
          inputType: 'choice',
          multiSelect: false,
          options: [{ label: 'src' }, { label: 'docs' }],
        },
        {
          questionId: 'q-2',
          header: '检查项',
          question: '选择需要的检查',
          inputType: 'choice',
          multiSelect: true,
          options: [{ label: 'lint' }, { label: 'test' }, { label: 'build' }],
        },
      ],
    })
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: actionRequiredPayload(action),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const decoded = message.payload.event.pendingAction
    if (decoded === undefined || decoded.kind !== 'ask_user_question') {
      throw new Error('unexpected action')
    }
    expect(decoded.questions).toHaveLength(2)
    expect(decoded.questions[0].multiSelect).toBe(false)
    expect(decoded.questions[1].multiSelect).toBe(true)
    expect(decoded.questions[1].options).toHaveLength(3)
  })

  // B2 Task 2 Step 1 自由文本 CLI 实证已通过（docs/testing/B2-result.md），
  // 解码器必须能读 inputType=text；服务端是否下发由配置控制。
  test('inputType 为 text 的问题可解码且不带 options', () => {
    const action = questionAction({
      questions: [
        {
          questionId: 'q-1',
          header: '补充',
          question: '请输入备注',
          inputType: 'text',
          multiSelect: false,
        },
      ],
    })
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: actionRequiredPayload(action),
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    const decoded = message.payload.event.pendingAction
    if (decoded === undefined || decoded.kind !== 'ask_user_question') {
      throw new Error('unexpected action')
    }
    expect(decoded.questions[0].inputType).toBe('text')
    expect(decoded.questions[0].options).toHaveLength(0)
  })

  test('action_required 的 turnStatus 非 waiting 拒绝', () => {
    const payload = actionRequiredPayload() as { event: Record<string, unknown> }
    payload.event.turnStatus = 'running'
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('action_required 缺 pendingAction 拒绝', () => {
    const payload = actionRequiredPayload() as { event: Record<string, unknown> }
    delete payload.event.pendingAction
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('pendingAction.requestId 与事件 requestId 不一致拒绝', () => {
    const payload = actionRequiredPayload(approvalAction({ requestId: 'req-other' }))
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('tool_approval 携带 questions 拒绝', () => {
    const payload = actionRequiredPayload(
      approvalAction({ questions: [{ questionId: 'q-1', header: 'h', question: 'q', inputType: 'choice', multiSelect: false, options: [{ label: 'a' }] }] }),
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('tool_approval 缺 toolName 拒绝', () => {
    const action = approvalAction() as Record<string, unknown>
    action.toolName = ''
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'event',
            requestId: 'req-1',
            seq: 8,
            payload: actionRequiredPayload(action),
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('ask_user_question 携带 preview 拒绝', () => {
    const payload = actionRequiredPayload(questionAction({ preview: 'ls' }))
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('ask_user_question 的 toolName 不是 AskUserQuestion 拒绝', () => {
    const payload = actionRequiredPayload(questionAction({ toolName: 'Bash' }))
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('ask_user_question 的 questions 为空拒绝', () => {
    const payload = actionRequiredPayload(questionAction({ questions: [] }))
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('questionId 重复拒绝（答案无法寻址）', () => {
    const question = {
      questionId: 'q-1',
      header: 'h',
      question: 'q',
      inputType: 'choice',
      multiSelect: false,
      options: [{ label: 'a' }],
    }
    const payload = actionRequiredPayload(questionAction({ questions: [question, { ...question }] }))
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('choice 问题缺 options 拒绝', () => {
    const payload = actionRequiredPayload(
      questionAction({
        questions: [
          { questionId: 'q-1', header: 'h', question: 'q', inputType: 'choice', multiSelect: false },
        ],
      }),
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('choice 选项 label 为空拒绝', () => {
    const payload = actionRequiredPayload(
      questionAction({
        questions: [
          {
            questionId: 'q-1',
            header: 'h',
            question: 'q',
            inputType: 'choice',
            multiSelect: false,
            options: [{ label: '' }],
          },
        ],
      }),
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('text 问题携带 options 拒绝', () => {
    const payload = actionRequiredPayload(
      questionAction({
        questions: [
          {
            questionId: 'q-1',
            header: 'h',
            question: 'q',
            inputType: 'text',
            multiSelect: false,
            options: [{ label: 'a' }],
          },
        ],
      }),
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('text 问题 multiSelect 为真拒绝', () => {
    const payload = actionRequiredPayload(
      questionAction({
        questions: [
          { questionId: 'q-1', header: 'h', question: 'q', inputType: 'text', multiSelect: true },
        ],
      }),
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('未知 inputType 拒绝', () => {
    const payload = actionRequiredPayload(
      questionAction({
        questions: [
          { questionId: 'q-1', header: 'h', question: 'q', inputType: 'voice', multiSelect: false },
        ],
      }),
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('pendingAction 携带越界字段（工具原始 input）拒绝', () => {
    const payload = actionRequiredPayload(approvalAction({ input: { command: 'rm -rf /' } }))
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('action_resolved 不带 block / pendingAction', () => {
    const message = decodeServerMessage(
      JSON.stringify(
        baseEnvelope({
          type: 'event',
          requestId: 'req-1',
          seq: 8,
          payload: {
            event: {
              seq: 8,
              kind: 'action_resolved',
              sessionId: 'session-1',
              requestId: 'req-1',
              source: 'mobile',
              text: '',
              sessionStatus: '',
              turnStatus: 'running',
              error: null,
            },
          },
        }),
      ),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    expect(message.payload.event.kind).toBe('action_resolved')
    expect(message.payload.event.pendingAction).toBeUndefined()
  })

  test('action_resolved 携带 pendingAction 拒绝', () => {
    const payload = actionRequiredPayload() as { event: Record<string, unknown> }
    payload.event.kind = 'action_resolved'
    payload.event.turnStatus = 'running'
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  // D52：action_resolved 可回带服务端已校验的答案，作为界面回显。
  function resolvedPayload(answers: unknown, kind = 'action_resolved') {
    return {
      event: {
        seq: 8,
        kind,
        sessionId: 'session-1',
        requestId: 'req-1',
        source: 'mobile',
        text: '',
        sessionStatus: '',
        turnStatus: kind === 'action_resolved' ? 'running' : '',
        error: null,
        answers,
      },
    }
  }

  test('action_resolved 解码已校验答案', () => {
    const payload = resolvedPayload([
      { questionId: 'q1', header: '框架', question: '选哪个框架？', values: ['React'] },
      { questionId: 'q2', question: '选哪些端？', values: ['iOS', 'Android'] },
    ])
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
    )
    if (message.type !== 'event') throw new Error('unexpected type')
    expect(message.payload.event.answers).toEqual([
      { questionId: 'q1', header: '框架', question: '选哪个框架？', values: ['React'] },
      { questionId: 'q2', header: '', question: '选哪些端？', values: ['iOS', 'Android'] },
    ])
  })

  test.each([
    ['缺 questionId', [{ question: '选哪个？', values: ['React'] }]],
    ['缺 question', [{ questionId: 'q1', values: ['React'] }]],
    ['取值为空数组', [{ questionId: 'q1', question: '选哪个？', values: [] }]],
    ['取值空白', [{ questionId: 'q1', question: '选哪个？', values: ['  '] }]],
    [
      'questionId 重复',
      [
        { questionId: 'q1', question: '选哪个？', values: ['React'] },
        { questionId: 'q1', question: '选哪个？', values: ['Vue'] },
      ],
    ],
    ['越界字段', [{ questionId: 'q1', question: '选哪个？', values: ['React'], result: '正文' }]],
  ])('action_resolved 答案 %s 拒绝', (_name, answers) => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload: resolvedPayload(answers) }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('非 action_resolved 携带 answers 拒绝', () => {
    const payload = resolvedPayload(
      [{ questionId: 'q1', question: '选哪个？', values: ['React'] }],
      'turn_completed',
    )
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'event', requestId: 'req-1', seq: 8, payload })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('snapshot 的 pendingAction 非空但 turn 非 waiting 拒绝', () => {
    const payload = waitingSnapshotPayload()
    payload.snapshot.turn.status = 'running'
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('snapshot 的 pendingAction.requestId 与 turn.requestId 不一致拒绝', () => {
    const payload = waitingSnapshotPayload(approvalAction({ requestId: 'req-other' }))
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })
})

describe('decodeServerMessage：queuedRequests（B2 §4.3）', () => {
  test('非空队列可解码且保留来源与预览', () => {
    const payload = snapshotPayload()
    payload.snapshot.queueDepth = 2
    payload.snapshot.queuedRequests = [
      { requestId: 'req-2', source: 'mobile', preview: '跑一下测试' },
      { requestId: 'req-3', source: 'terminal', preview: '看一下日志' },
    ]
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload })),
    )
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    expect(message.payload.snapshot.queuedRequests).toHaveLength(2)
    expect(message.payload.snapshot.queuedRequests[1].source).toBe('terminal')
    expect(message.payload.snapshot.queuedRequests[0].preview).toBe('跑一下测试')
  })

  // Go 侧 Snapshot.QueuedRequests 无 omitempty，空队列在 wire 上是 null。
  test('queuedRequests 为 null 视为空队列', () => {
    const payload = snapshotPayload()
    payload.snapshot.queuedRequests = null
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload })),
    )
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    expect(message.payload.snapshot.queuedRequests).toEqual([])
  })

  test('queuedRequests 字段缺失拒绝', () => {
    const payload = snapshotPayload() as { snapshot: Record<string, unknown> }
    delete payload.snapshot.queuedRequests
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('queueDepth 与 queuedRequests 长度不一致拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.queueDepth = 3
    payload.snapshot.queuedRequests = [{ requestId: 'req-2', source: 'mobile', preview: 'x' }]
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('queuedRequests 元素未知 source 拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.queueDepth = 1
    payload.snapshot.queuedRequests = [{ requestId: 'req-2', source: 'watch', preview: 'x' }]
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('queuedRequests 元素携带越界字段拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.queueDepth = 1
    payload.snapshot.queuedRequests = [
      { requestId: 'req-2', source: 'mobile', preview: 'x', text: '完整正文' },
    ]
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })

  test('queuedRequests 非数组拒绝', () => {
    const payload = snapshotPayload()
    payload.snapshot.queuedRequests = { requestId: 'req-2' }
    expect(() =>
      decodeServerMessage(JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload }))),
    ).toThrow(/protocol decode error/)
  })
})

describe('decodeServerMessage：unsupported 独立消息类型（B2 §4.4）', () => {
  test('四个固定安全 code 均可解码', () => {
    for (const code of [
      'unknown_block',
      'unknown_subtype',
      'concurrent_action',
      'text_input_disabled',
    ]) {
      const message = decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'unsupported', payload: { code } })),
      )
      if (message.type !== 'unsupported') throw new Error('unexpected type')
      expect(message.payload.code).toBe(code)
    }
  })

  test('未知 code 拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'unsupported', payload: { code: 'mystery' } })),
      ),
    ).toThrow(/protocol decode error/)
  })

  test('空 code 拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(baseEnvelope({ type: 'unsupported', payload: { code: '' } })),
      ),
    ).toThrow(/protocol decode error/)
  })

  // unsupported 只允许固定 code，不得透传原始 SDK 数据。
  test('携带额外字段拒绝', () => {
    expect(() =>
      decodeServerMessage(
        JSON.stringify(
          baseEnvelope({
            type: 'unsupported',
            payload: { code: 'unknown_block', data: { raw: 'secret' } },
          }),
        ),
      ),
    ).toThrow(/protocol decode error/)
  })
})
