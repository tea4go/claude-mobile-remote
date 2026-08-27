import { describe, expect, test } from 'vitest'
import { ProtocolClient } from './client'
import type { PairingPayload } from './pairing'

// 可编程的假 WebSocket：测试手动推进 open/message/close。
class FakeWebSocket {
  static instances: FakeWebSocket[] = []

  url: string
  sent: string[] = []
  closed = false
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  send(data: string) {
    if (this.closed) throw new Error('closed')
    this.sent.push(data)
  }

  close() {
    if (this.closed) return
    this.closed = true
    this.onclose?.()
  }

  // ---- 测试驱动辅助 ----

  serverOpen() {
    this.onopen?.()
  }

  serverMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) })
  }

  serverClose() {
    if (this.closed) return
    this.closed = true
    this.onclose?.()
  }
}

function pairing(): PairingPayload {
  return {
    // 配对负载版本与传输层协议版本相互独立（internal/pairing/payload.go
    // 的 PayloadVersion 仍为 1），不得随协议升级而变。
    version: 1,
    wsUrl: 'ws://127.0.0.1:8765/ws',
    sessionId: 'session-1',
    pairToken: 'test-token-placeholder',
    expiresAt: '2099-01-01T00:00:00Z',
  }
}

function snapshotMessage(seq = 7) {
  return {
    version: 2,
    type: 'snapshot',
    sessionId: 'session-1',
    commandId: '',
    requestId: '',
    seq,
    payload: {
      connectionStatus: 'subscribed',
      snapshot: {
        sessionId: 'session-1',
        sessionStatus: 'ready',
        sessionError: null,
        seq,
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
      },
    },
  }
}

// 首轮前 snapshot（A5.2）：Session 已预留真实 ID 但 Claude init 未确认，
// 尚无任何 Turn，因此 turn.requestId 与 turn.source 均为空串。
function preTurnSnapshotMessage() {
  return {
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
  }
}

function makeClient(payload: PairingPayload = pairing()) {
  const socket = () => new FakeWebSocket(payload.wsUrl)
  const client = new ProtocolClient(payload, socket)
  return { client, payload }
}

beforeEach(() => {
  FakeWebSocket.instances = []
})

describe('握手流程', () => {
  test('open 后发送 hello（携带 sessionId 与 pairToken）', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    expect(socket.sent).toHaveLength(1)
    expect(JSON.parse(socket.sent[0]!)).toEqual({
      version: 2,
      type: 'hello',
      sessionId: 'session-1',
      commandId: '',
      requestId: '',
      seq: 0,
      payload: { pairToken: 'test-token-placeholder' },
    })
    client.close()
  })

  test('hello_ack 后发送 subscribe（携带 sessionId）', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({
      version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
    })
    expect(JSON.parse(socket.sent[1]!)).toMatchObject({
      type: 'subscribe',
      sessionId: 'session-1',
    })
    client.close()
  })

  test('subscribed 后收到 snapshot 进入已连接', () => {
    const { client } = makeClient()
    const events: string[] = []
    client.onEvent = (e) => events.push(e.kind)
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 2, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage(snapshotMessage())
    expect(events).toContain('snapshot')
    expect(client.isReady()).toBe(true)
    client.close()
  })

  // A5.2 人工验收回归：首轮前（Session starting、Turn idle、seq 0、
  // turn.source 为空串）扫码必须能直接连上，不得因解码失败断开。
  test('首轮前 snapshot 仍能进入已连接（不报协议错误）', () => {
    const { client } = makeClient()
    const events: string[] = []
    const closes: string[] = []
    client.onEvent = (e) => events.push(e.kind)
    client.onClose = (info) => closes.push(info.kind)
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 2, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage(preTurnSnapshotMessage())
    expect(closes).toEqual([])
    expect(events).toContain('snapshot')
    expect(client.isReady()).toBe(true)
    client.close()
  })

  test('snapshot 前发送文本被拒绝', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 2, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    const result = client.sendText('hi', 'cmd-1')
    expect(result.ok).toBe(false)
    expect(socket.sent.filter((m) => JSON.parse(m).type === 'command')).toHaveLength(0)
    client.close()
  })
})

describe('命令与事件', () => {
  function connectedClient() {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 2, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage(snapshotMessage())
    return { client, socket }
  }

  test('sendText 在就绪后发出 command', () => {
    const { client, socket } = connectedClient()
    const result = client.sendText('hi', 'cmd-1')
    expect(result.ok).toBe(true)
    const sent = JSON.parse(socket.sent[2]!)
    expect(sent).toMatchObject({
      type: 'command',
      commandId: 'cmd-1',
      payload: { kind: 'send_text', text: 'hi' },
    })
    client.close()
  })

  test('未收到 command_result 前第二条 sendText 被拒绝（单飞）', () => {
    const { client } = connectedClient()
    expect(client.sendText('hi', 'cmd-1').ok).toBe(true)
    expect(client.sendText('again', 'cmd-2').ok).toBe(false)
    client.close()
  })

  test('command_result accepted 通知观察者并解除单飞', () => {
    const { client, socket } = connectedClient()
    client.sendText('hi', 'cmd-1')
    const results: unknown[] = []
    client.onEvent = (e) => {
      if (e.kind === 'commandResult') results.push(e)
    }
    socket.serverMessage({
      version: 2, type: 'command_result', sessionId: 'session-1', commandId: 'cmd-1', requestId: 'req-9', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    expect(results).toEqual([{ kind: 'commandResult', commandId: 'cmd-1', requestId: 'req-9', accepted: true, code: '', message: '' }])
    expect(client.sendText('again', 'cmd-2').ok).toBe(true)
    client.close()
  })

  test('resolveAction 发出审批决议命令', () => {
    const { client, socket } = connectedClient()
    const result = client.resolveAction({ actionId: 'act-1', decision: 'allow_once' }, 'cmd-1')
    expect(result.ok).toBe(true)
    expect(JSON.parse(socket.sent[2]!)).toMatchObject({
      type: 'command',
      commandId: 'cmd-1',
      payload: { kind: 'resolve_action', actionId: 'act-1', decision: 'allow_once' },
    })
    client.close()
  })

  test('resolveAction 发出问答答案命令且不携带 decision', () => {
    const { client, socket } = connectedClient()
    const answers = [{ questionId: 'q-1', values: ['src'] }]
    expect(client.resolveAction({ actionId: 'act-2', answers }, 'cmd-1').ok).toBe(true)
    const sent = JSON.parse(socket.sent[2]!)
    expect(sent.payload).toEqual({ kind: 'resolve_action', actionId: 'act-2', answers })
    client.close()
  })

  test('cancelTurn 与 cancelQueued 按 requestId 发出命令', () => {
    const { client, socket } = connectedClient()
    expect(client.cancelTurn('req-9', 'cmd-1').ok).toBe(true)
    expect(JSON.parse(socket.sent[2]!).payload).toEqual({
      kind: 'cancel_turn',
      requestId: 'req-9',
    })
    socket.serverMessage({
      version: 2, type: 'command_result', sessionId: 'session-1', commandId: 'cmd-1', requestId: 'req-9', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    expect(client.cancelQueued('req-10', 'cmd-2').ok).toBe(true)
    expect(JSON.parse(socket.sent[3]!).payload).toEqual({
      kind: 'cancel_queued',
      requestId: 'req-10',
    })
    client.close()
  })

  // 控制命令与文本命令共用同一个单在途门控：服务端按 commandId 单次应答。
  test('控制命令与 sendText 共用单飞门控', () => {
    const { client } = connectedClient()
    expect(client.resolveAction({ actionId: 'act-1', decision: 'deny' }, 'cmd-1').ok).toBe(true)
    expect(client.sendText('hi', 'cmd-2')).toEqual({ ok: false, reason: 'busy' })
    expect(client.cancelTurn('req-9', 'cmd-3')).toEqual({ ok: false, reason: 'busy' })
    expect(client.cancelQueued('req-10', 'cmd-4')).toEqual({ ok: false, reason: 'busy' })
    client.close()
  })

  test('未就绪时控制命令返回 notReady 且不发帧', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    expect(client.resolveAction({ actionId: 'act-1', decision: 'deny' }, 'cmd-1')).toEqual({
      ok: false,
      reason: 'notReady',
    })
    expect(client.cancelTurn('req-9', 'cmd-2')).toEqual({ ok: false, reason: 'notReady' })
    expect(socket.sent.filter((m) => JSON.parse(m).type === 'command')).toHaveLength(0)
    client.close()
  })

  // unsupported 是服务端通知，不是错误：必须上报观察者且不终止连接。
  test('unsupported 通知上报且保持连接', () => {
    const { client, socket } = connectedClient()
    const events: unknown[] = []
    const closes: unknown[] = []
    client.onEvent = (e) => events.push(e)
    client.onClose = (info) => closes.push(info)
    socket.serverMessage({
      version: 2, type: 'unsupported', sessionId: 'session-1', commandId: '', requestId: '', seq: 0,
      payload: { code: 'concurrent_action' },
    })
    expect(events).toEqual([{ kind: 'unsupported', code: 'concurrent_action', seq: 0 }])
    expect(closes).toEqual([])
    expect(socket.closed).toBe(false)
    expect(client.isReady()).toBe(true)
    client.close()
  })

  test('其他 Session 的 unsupported 通知按协议错误关闭连接', () => {
    const { client, socket } = connectedClient()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    socket.serverMessage({
      version: 2, type: 'unsupported', sessionId: 'other-session', commandId: '', requestId: '', seq: 0,
      payload: { code: 'unknown_block' },
    })
    expect(closes).toEqual([{ kind: 'protocolError', pendingCommandId: null }])
    expect(socket.closed).toBe(true)
  })

  test('未知 commandId 的 command_result 按协议错误关闭连接', () => {
    const { client, socket } = connectedClient()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    client.sendText('hello', 'cmd-1')

    socket.serverMessage({
      version: 2, type: 'command_result', sessionId: 'session-1', commandId: 'other-command', requestId: 'req-9', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })

    expect(closes).toEqual([{ kind: 'protocolError', pendingCommandId: 'cmd-1' }])
    expect(socket.closed).toBe(true)
  })

  test('snapshot 与 envelope 的 sessionId 或 seq 不一致时关闭连接', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    socket.serverMessage({
      ...snapshotMessage(7),
      sessionId: 'other-session',
    })
    expect(closes).toEqual([{ kind: 'protocolError', pendingCommandId: null }])
    expect(socket.closed).toBe(true)
  })

  test('event 通知观察者', () => {
    const { client, socket } = connectedClient()
    const events: unknown[] = []
    client.onEvent = (e) => events.push(e)
    socket.serverMessage({
      version: 2, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-9', seq: 8,
      payload: { event: { seq: 8, kind: 'turn_started', sessionId: 'session-1', requestId: 'req-9', source: 'mobile', text: 'from phone', sessionStatus: '', turnStatus: 'running', error: null } },
    })
    expect(events).toEqual([{
      kind: 'event',
      event: expect.objectContaining({ seq: 8, kind: 'turn_started' }),
    }])
    client.close()
  })
})

describe('断线与失效', () => {
  function connectedClient() {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 2, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage(snapshotMessage())
    return { client, socket }
  }

  test('运行期非正常 close：已断开 + 结果未知，无 close code', () => {
    const { client } = connectedClient()
    client.sendText('hi', 'cmd-1')
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    FakeWebSocket.instances[0]!.serverClose()
    expect(closes).toEqual([{ kind: 'disconnected', pendingCommandId: 'cmd-1' }])
  })

  test('服务端 error unauthorized：配对失效', () => {
    const { client, socket } = connectedClient()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    socket.serverMessage({
      version: 2, type: 'error', sessionId: '', commandId: '', requestId: '', seq: 0,
      payload: { code: 'unauthorized', message: 'invalid pair token' },
    })
    expect(closes).toEqual([{ kind: 'pairingInvalid', pendingCommandId: null }])
    expect(socket.closed).toBe(true)
  })

  test('协议错误：终止连接', () => {
    const { client, socket } = connectedClient()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    socket.serverMessage('{"version":1,"type":"hello_ack","sessionId":"","commandId":"","requestId","seq":0,"payload":{}}'.replace('"requestId"', '"requestId": ""'))
    expect(closes).toEqual([{ kind: 'protocolError', pendingCommandId: null }])
    expect(socket.closed).toBe(true)
  })

  test('显式 close 不触发 onClose', () => {
    const { client } = connectedClient()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    client.close()
    expect(closes).toEqual([])
  })

  test('旧代次回调被忽略（重连后旧 socket 迟到消息）', () => {
    const { client } = connectedClient()
    const first = FakeWebSocket.instances[0]!
    // 显式重连
    client.reconnect()
    const second = FakeWebSocket.instances[1]!
    expect(second).toBeDefined()
    second.serverOpen()
    second.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    second.serverMessage({ version: 2, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    second.serverMessage(snapshotMessage())

    // 旧 socket 的迟到消息与关闭都不得影响当前状态
    const events: unknown[] = []
    client.onEvent = (e) => events.push(e)
    first.serverMessage(snapshotMessage(99))
    first.serverClose()
    expect(events).toEqual([])
    expect(client.isReady()).toBe(true)
    client.close()
  })
})

describe('构造与输入校验', () => {
  test('重复 connect 不新建连接', () => {
    const { client } = makeClient()
    client.connect()
    client.connect()
    expect(FakeWebSocket.instances).toHaveLength(1)
    client.close()
  })

  test('close 幂等', () => {
    const { client } = makeClient()
    client.connect()
    FakeWebSocket.instances[0]!.serverOpen()
    client.close()
    client.close()
    expect(FakeWebSocket.instances[0]!.closed).toBe(true)
  })

  test('hello 前收到其他消息按协议错误关闭', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    socket.serverMessage(snapshotMessage())
    expect(closes).toEqual([{ kind: 'protocolError', pendingCommandId: null }])
    expect(socket.closed).toBe(true)
  })

  test('onEvent 抛错不影响连接（防御）', () => {
    const { client } = makeClient()
    client.onEvent = () => {
      throw new Error('observer bug')
    }
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    expect(() =>
      socket.serverMessage({ version: 2, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} }),
    ).not.toThrow()
    client.close()
  })
})

test('构造后未连接时 isReady 为 false', () => {
  const { client } = makeClient()
  expect(client.isReady()).toBe(false)
  client.close()
})
