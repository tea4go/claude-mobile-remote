import { describe, expect, test } from 'vitest'
import { decodeServerMessage } from './decode'
import type { Envelope } from './types'

// 构造合法 envelope 的工厂；各用例按需覆盖字段。
function baseEnvelope(overrides: Partial<Envelope> = {}): Envelope {
  return {
    version: 1,
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
        status: 'idle' as const,
        requestId: '',
        source: 'mobile' as const,
        message: '',
        error: null as { code: string; message: string } | null,
        pendingAction: null as Record<string, never> | null,
      },
      queueDepth: 0,
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

  test('session snapshot 的 pendingAction 为空对象', () => {
    const payload = snapshotPayload()
    payload.snapshot.turn.pendingAction = {}
    const message = decodeServerMessage(
      JSON.stringify(baseEnvelope({ type: 'snapshot', seq: 7, payload })),
    )
    if (message.type !== 'snapshot') throw new Error('unexpected type')
    expect(message.payload.snapshot.turn.pendingAction).toEqual({})
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

  test('version = 2 拒绝（未知版本）', () => {
    const envelope = baseEnvelope({ version: 2 })
    expect(() => decodeServerMessage(JSON.stringify(envelope))).toThrow()
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
      decodeServerMessage('{"version":1,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":0,"payload":"x"}'),
    ).toThrow()
  })

  test('payload 为 null 拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":1,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":0,"payload":null}'),
    ).toThrow()
  })

  test('seq 缺失拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":1,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","payload":{}}'),
    ).toThrow()
  })

  test('seq 为负数拒绝', () => {
    expect(() =>
      decodeServerMessage('{"version":1,"type":"hello_ack","sessionId":"","commandId":"","requestId":"","seq":-1,"payload":{}}'),
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
})
