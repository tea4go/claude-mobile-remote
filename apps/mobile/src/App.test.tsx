import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test } from 'vitest'
import App from './App'

// 可编程假 WebSocket（与 client.test 相同形状）。
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
    this.sent.push(data)
  }

  close() {
    if (this.closed) return
    this.closed = true
    this.onclose?.()
  }

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

const PAIRING = {
  version: 1,
  wsUrl: 'ws://127.0.0.1:8765/ws',
  sessionId: 'session-1',
  pairToken: 'test-token-placeholder',
  expiresAt: '2099-01-01T00:00:00Z',
}

const SNAPSHOT = {
  version: 1,
  type: 'snapshot',
  sessionId: 'session-1',
  commandId: '',
  requestId: '',
  seq: 7,
  payload: {
    connectionStatus: 'subscribed',
    snapshot: {
      sessionId: 'session-1',
      sessionStatus: 'ready',
      sessionError: null,
      seq: 7,
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

function fragmentOf(payload: unknown): string {
  const json = JSON.stringify(payload)
  const base64 = btoa(json).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
  return `#pair=${base64}`
}

beforeEach(() => {
  FakeWebSocket.instances = []
})

afterEach(() => {
  window.location.hash = ''
})

function completeHandshake(socket: FakeWebSocket) {
  socket.serverOpen()
  socket.serverMessage({
    version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
  })
  socket.serverMessage({
    version: 1, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {},
  })
  socket.serverMessage(SNAPSHOT)
}

describe('App：无配对', () => {
  test('显示配对输入页', () => {
    window.location.hash = ''
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    expect(screen.getByText('连接 Claude 会话')).toBeInTheDocument()
  })

  test('无效 Fragment 显示原因并允许输入', () => {
    window.location.hash = '#pair=garbage'
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    expect(screen.getByText('配对信息无效')).toBeInTheDocument()
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  test('过期 Fragment 提示重新配对', () => {
    window.location.hash = fragmentOf({ ...PAIRING, expiresAt: '2020-01-01T00:00:00Z' })
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    expect(screen.getByText(/已过期/)).toBeInTheDocument()
  })
})

describe('App：自动连接与发送', () => {
  test('连接阶段展示认证与订阅状态', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!

    socket.serverOpen()
    await waitFor(() => {
      expect(screen.getByText('认证中')).toBeInTheDocument()
    })

    socket.serverMessage({
      version: 1, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
    })
    await waitFor(() => {
      expect(screen.getByText('订阅中')).toBeInTheDocument()
    })
  })

  test('有效 Fragment 自动连接并完成握手', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    expect(socket).toBeDefined()
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })
  })

  test('粘贴 JSON 连接成功后可发送文本', async () => {
    window.location.hash = ''
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: JSON.stringify(PAIRING) } })
    fireEvent.click(screen.getByRole('button', { name: '连接' }))
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: '你好 Claude' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    const command = socket.sent.map((s) => JSON.parse(s)).find((m) => m.type === 'command')
    expect(command).toMatchObject({
      sessionId: 'session-1',
      payload: { kind: 'send_text', text: '你好 Claude' },
    })

    // 接受结果后显示已接收
    socket.serverMessage({
      version: 1,
      type: 'command_result',
      sessionId: 'session-1',
      commandId: command.commandId,
      requestId: 'req-9',
      seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    await waitFor(() => {
      expect(screen.getByText('已接收')).toBeInTheDocument()
    })
  })

  test('等待 command result 时发送禁用', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '第一条' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
  })
})

describe('App：断线与重连', () => {
  test('运行期断线显示已断开，不自动重连，可显式重连', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const first = FakeWebSocket.instances[0]!
    completeHandshake(first)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    first.serverClose()
    await waitFor(() => {
      expect(screen.getByText('已断开')).toBeInTheDocument()
    })
    // 不自动重连：仍只有一个 WebSocket 实例
    expect(FakeWebSocket.instances).toHaveLength(1)

    fireEvent.click(screen.getByRole('button', { name: '重新连接' }))
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(2)
    })
    completeHandshake(FakeWebSocket.instances[1]!)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })
  })

  test('断线时 pending command 标记结果未知', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '悬而未决' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    socket.serverClose()
    await waitFor(() => {
      expect(screen.getByText(/结果未知/)).toBeInTheDocument()
    })
  })
})

describe('App：更换配对', () => {
  test('清除 Fragment、关闭连接并返回输入页', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: '更换配对' }))
    expect(screen.getByText('连接 Claude 会话')).toBeInTheDocument()
    expect(window.location.hash).toBe('')
    expect(socket.closed).toBe(true)
  })
})

describe('App：unauthorized 配对失效', () => {
  test('服务端拒绝认证后引导更换配对，禁用重连', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({
      version: 1,
      type: 'error',
      sessionId: '',
      commandId: '',
      requestId: '',
      seq: 0,
      payload: { code: 'unauthorized', message: 'invalid pair token' },
    })
    await waitFor(() => {
      expect(screen.getByText(/配对已失效/)).toBeInTheDocument()
    })
    expect(screen.queryByRole('button', { name: '重新连接' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '更换配对' })).toBeInTheDocument()
  })

  test('已连接后配对失效会禁用发送并标记 pending 结果未知', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '未决消息' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    socket.serverMessage({
      version: 1,
      type: 'error',
      sessionId: '',
      commandId: '',
      requestId: '',
      seq: 0,
      payload: { code: 'unauthorized', message: 'invalid pair token' },
    })

    await waitFor(() => {
      expect(screen.getByText(/配对已失效/)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
      expect(screen.getByText(/结果未知/)).toBeInTheDocument()
    })
  })
})
