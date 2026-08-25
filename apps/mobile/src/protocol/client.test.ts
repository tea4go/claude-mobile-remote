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
    version: 1,
    wsUrl: 'ws://127.0.0.1:8765/ws',
    sessionId: 'session-1',
    pairToken: 'test-token-placeholder',
    expiresAt: '2099-01-01T00:00:00Z',
  }
}

function snapshotMessage(seq = 7) {
  return {
    version: 1,
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
      version: 1,
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
      version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
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
    socket.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 1, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage(snapshotMessage())
    expect(events).toContain('snapshot')
    expect(client.isReady()).toBe(true)
    client.close()
  })

  test('snapshot 前发送文本被拒绝', () => {
    const { client } = makeClient()
    client.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 1, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
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
    socket.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 1, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
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
      version: 1, type: 'command_result', sessionId: 'session-1', commandId: 'cmd-1', requestId: 'req-9', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    expect(results).toEqual([{ kind: 'commandResult', commandId: 'cmd-1', requestId: 'req-9', accepted: true, code: '', message: '' }])
    expect(client.sendText('again', 'cmd-2').ok).toBe(true)
    client.close()
  })

  test('未知 commandId 的 command_result 按协议错误关闭连接', () => {
    const { client, socket } = connectedClient()
    const closes: unknown[] = []
    client.onClose = (info) => closes.push(info)
    client.sendText('hello', 'cmd-1')

    socket.serverMessage({
      version: 1, type: 'command_result', sessionId: 'session-1', commandId: 'other-command', requestId: 'req-9', seq: 0,
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
    socket.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
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
      version: 1, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-9', seq: 8,
      payload: { event: { seq: 8, kind: 'turn_started', sessionId: 'session-1', requestId: 'req-9', source: 'mobile', text: '', sessionStatus: 'ready', turnStatus: 'running', error: null } },
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
    socket.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    socket.serverMessage({ version: 1, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
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
      version: 1, type: 'error', sessionId: '', commandId: '', requestId: '', seq: 0,
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
    second.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} })
    second.serverMessage({ version: 1, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {} })
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
      socket.serverMessage({ version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {} }),
    ).not.toThrow()
    client.close()
  })
})

test('构造后未连接时 isReady 为 false', () => {
  const { client } = makeClient()
  expect(client.isReady()).toBe(false)
  client.close()
})
