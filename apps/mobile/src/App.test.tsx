import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import App from './App'
import { PROTOCOL_VERSION } from './protocol/types'

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
  // 配对负载版本独立于传输层协议版本，固定为 1。
  version: 1,
  wsUrl: 'ws://127.0.0.1:8765/ws',
  sessionId: 'session-1',
  pairToken: 'test-token-placeholder',
  expiresAt: '2099-01-01T00:00:00Z',
}

const SNAPSHOT = {
  version: PROTOCOL_VERSION,
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
      queuedRequests: [],
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
    version: PROTOCOL_VERSION, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
  })
  socket.serverMessage({
    version: PROTOCOL_VERSION, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {},
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
      version: PROTOCOL_VERSION, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
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

  test('粘贴完整 Fragment 后连接成功', async () => {
    window.location.hash = ''
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: fragmentOf(PAIRING) } })
    fireEvent.click(screen.getByRole('button', { name: '连接' }))

    const socket = FakeWebSocket.instances[0]!
    expect(socket.url).toBe(PAIRING.wsUrl)
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
      version: PROTOCOL_VERSION,
      type: 'command_result',
      sessionId: 'session-1',
      commandId: command.commandId,
      requestId: 'req-9',
      seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    await waitFor(() => {
      expect(screen.getByText(/已接收/)).toBeInTheDocument()
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
  test('活动 Session 注册页面可见性监听且不自动重连', async () => {
    const addEventListener = vi.spyOn(document, 'addEventListener')
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    expect(addEventListener).toHaveBeenCalledWith('visibilitychange', expect.any(Function))

    expect(FakeWebSocket.instances).toHaveLength(1)
    document.dispatchEvent(new Event('visibilitychange'))
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
    addEventListener.mockRestore()
  })

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

describe('App：返回列表', () => {
  test('清除 Fragment、关闭连接并保留列表记录', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('返回'))

    expect(screen.getByText('会话列表')).toBeInTheDocument()
    expect(screen.getByTestId('session-list-item')).toBeInTheDocument()
    expect(window.location.hash).toBe('')
    expect(socket.closed).toBe(true)
  })

  test('从列表再进入会重新连接同一 Session', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    completeHandshake(FakeWebSocket.instances[0]!)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('返回'))
    fireEvent.click(screen.getByRole('button', { name: '进入' }))

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(2)
    })
    completeHandshake(FakeWebSocket.instances[1]!)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })
  })
})

describe('App：单连接生命周期', () => {
  test('切换到第二个 Session 先关闭第一个再连接', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const first = FakeWebSocket.instances[0]!
    completeHandshake(first)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    // 返回列表后新增第二个配对。
    fireEvent.click(screen.getByText('返回'))
    expect(first.closed).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: '新增配对' }))
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: JSON.stringify({ ...PAIRING, sessionId: 'session-2' }) },
    })
    fireEvent.click(screen.getByRole('button', { name: '连接' }))

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(2)
    })
    // 任意时刻只有一个未关闭连接。
    expect(FakeWebSocket.instances.filter((s) => !s.closed)).toHaveLength(1)

    fireEvent.click(screen.getByText('返回'))

    // 两个 Session 都在列表中，且都已断开。
    expect(screen.getAllByTestId('session-list-item')).toHaveLength(2)
    expect(FakeWebSocket.instances.filter((s) => !s.closed)).toHaveLength(0)
  })

  test('同一 sessionId 重新配对后列表不重复', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    completeHandshake(FakeWebSocket.instances[0]!)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('返回'))
    fireEvent.click(screen.getByRole('button', { name: '新增配对' }))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: JSON.stringify(PAIRING) } })
    fireEvent.click(screen.getByRole('button', { name: '连接' }))
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(2)
    })
    fireEvent.click(screen.getByText('返回'))

    expect(screen.getAllByTestId('session-list-item')).toHaveLength(1)
  })

  test('删除列表记录后回到配对页', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    completeHandshake(FakeWebSocket.instances[0]!)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('返回'))
    fireEvent.click(screen.getByRole('button', { name: '删除' }))

    // 列表为空时直接回到配对输入页。
    expect(screen.getByText('连接 Claude 会话')).toBeInTheDocument()
  })
})

describe('App：刷新边界', () => {
  test('无 Fragment 时列表为空，不恢复任何记录', () => {
    window.location.hash = ''
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)

    expect(screen.getByText('连接 Claude 会话')).toBeInTheDocument()
    expect(screen.queryByTestId('session-list-item')).not.toBeInTheDocument()
    expect(FakeWebSocket.instances).toHaveLength(0)
  })

  test('详情页 Fragment 只恢复当前 Session，不恢复其他记录与旧消息', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    completeHandshake(FakeWebSocket.instances[0]!)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    // 只有 Fragment 中的这一个会话，且消息流为空（不做 replay）。
    expect(screen.getByTestId('message-feed').textContent).toBe('')
    fireEvent.click(screen.getByText('返回'))
    expect(screen.getAllByTestId('session-list-item')).toHaveLength(1)
  })
})

describe('App：unauthorized 配对失效', () => {
  test('服务端拒绝认证后引导更换配对，禁用重连', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({
      version: PROTOCOL_VERSION,
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
    // 配对失效后通过返回列表重新配对。
    expect(screen.getByText('返回')).toBeInTheDocument()
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
      version: PROTOCOL_VERSION,
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

// ---- A5.1：PC 输入实时同步与手机输入去重 ----

describe('App：A5.1 PC 输入与去重', () => {
  test('PC 端 turn_started 显示电脑端正文并紧随 Claude 回复', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-pc', seq: 8,
      payload: { event: { seq: 8, kind: 'turn_started', sessionId: 'session-1', requestId: 'req-pc', source: 'terminal', text: '来自电脑的问题', sessionStatus: '', turnStatus: 'running', error: null } },
    })
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-pc', seq: 9,
      payload: { event: { seq: 9, kind: 'text_delta', sessionId: 'session-1', requestId: 'req-pc', source: 'terminal', text: '电脑的回答', sessionStatus: '', turnStatus: '', error: null } },
    })

    await waitFor(() => {
      expect(screen.getByText('来自电脑的问题')).toBeInTheDocument()
    })
    expect(screen.getByText(/电脑端/)).toBeInTheDocument()
    expect(screen.getByText('电脑的回答')).toBeInTheDocument()
  })

  test('手机发送后正文只出现一次（command_result 与 turn_started 去重）', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    completeHandshake(socket)
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '手机提问' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    const command = socket.sent.map((s) => JSON.parse(s)).find((m) => m.type === 'command')

    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'command_result', sessionId: 'session-1',
      commandId: command.commandId, requestId: 'req-m', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-m', seq: 8,
      payload: { event: { seq: 8, kind: 'turn_started', sessionId: 'session-1', requestId: 'req-m', source: 'mobile', text: '手机提问', sessionStatus: '', turnStatus: 'running', error: null } },
    })
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-m', seq: 9,
      payload: { event: { seq: 9, kind: 'text_delta', sessionId: 'session-1', requestId: 'req-m', source: 'mobile', text: '答', sessionStatus: '', turnStatus: '', error: null } },
    })

    await waitFor(() => {
      expect(screen.getByText('答')).toBeInTheDocument()
    })
    expect(screen.getAllByText('手机提问')).toHaveLength(1)
    expect(screen.getByText(/已接收/)).toBeInTheDocument()
  })
})

// ---- B2：结构化内容、待处理事项与取消 ----

const APPROVAL = {
  actionId: 'act-1',
  requestId: 'req-pc',
  kind: 'tool_approval',
  toolName: 'Bash',
  preview: 'go test ./...',
  allowSessionAvailable: true,
}

function turnStartedAt(seq: number) {
  return {
    version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-pc', seq,
    payload: { event: { seq, kind: 'turn_started', sessionId: 'session-1', requestId: 'req-pc', source: 'terminal', text: '跑一次测试', sessionStatus: '', turnStatus: 'running', error: null } },
  }
}

function actionRequiredAt(seq: number) {
  return {
    version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-pc', seq,
    payload: { event: { seq, kind: 'action_required', sessionId: 'session-1', requestId: 'req-pc', source: '', text: '', sessionStatus: '', turnStatus: 'waiting', error: null, pendingAction: APPROVAL } },
  }
}

function lastCommand(socket: FakeWebSocket) {
  const commands = socket.sent.map((s) => JSON.parse(s)).filter((m) => m.type === 'command')
  return commands[commands.length - 1]
}

async function connected() {
  window.location.hash = fragmentOf(PAIRING)
  render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
  const socket = FakeWebSocket.instances[0]!
  completeHandshake(socket)
  await waitFor(() => {
    expect(screen.getByText('已连接')).toBeInTheDocument()
  })
  return socket
}

describe('App：B2 结构化内容块', () => {
  test('content_block 事件渲染为摘要行，不再显示只读占位', async () => {
    const socket = await connected()
    socket.serverMessage(turnStartedAt(8))
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-pc', seq: 9,
      payload: { event: { seq: 9, kind: 'content_block', sessionId: 'session-1', requestId: 'req-pc', source: '', text: '', sessionStatus: '', turnStatus: '', error: null, block: { id: 'b1', kind: 'tool_use', toolUseId: 'tu-1', toolClass: 'builtin', toolName: 'Bash', mcpServer: '' } } },
    })

    await waitFor(() => {
      expect(screen.getByText('查看 1 个步骤，运行 1 个工具')).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId('activity-group-summary'))
    expect(screen.getByText('工具：Bash')).toBeInTheDocument()
    expect(screen.queryByText('只读占位')).not.toBeInTheDocument()
  })
})

describe('App：B2 工具审批', () => {
  test('action_required 展示审批卡片、禁用文本输入，决议发出 resolve_action', async () => {
    const socket = await connected()
    socket.serverMessage(turnStartedAt(8))
    socket.serverMessage(actionRequiredAt(9))

    await waitFor(() => {
      expect(screen.getByTestId('pending-action-card')).toBeInTheDocument()
    })
    expect(screen.getByTestId('action-preview').textContent).toBe('go test ./...')
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
    // 自由文本输入框本身也禁用（B2 §6），不只禁用发送按钮。
    expect(screen.getByRole('textbox')).toBeDisabled()
    expect(screen.getByText(/Turn：等待中/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '本次允许' }))
    expect(lastCommand(socket)).toMatchObject({
      sessionId: 'session-1',
      payload: { kind: 'resolve_action', actionId: 'act-1', decision: 'allow_once' },
    })

    // 决议被接受后卡片消失，文本输入恢复。
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'command_result', sessionId: 'session-1',
      commandId: lastCommand(socket).commandId, requestId: 'req-pc', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    await waitFor(() => {
      expect(screen.queryByTestId('pending-action-card')).not.toBeInTheDocument()
    })
  })

  test('竞态失败时提示已由其他客户端处理并清除卡片', async () => {
    const socket = await connected()
    socket.serverMessage(turnStartedAt(8))
    socket.serverMessage(actionRequiredAt(9))
    await waitFor(() => {
      expect(screen.getByTestId('pending-action-card')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: '拒绝' }))
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'command_result', sessionId: 'session-1',
      commandId: lastCommand(socket).commandId, requestId: '', seq: 0,
      payload: { accepted: false, code: 'action_not_pending', message: 'already resolved' },
    })

    await waitFor(() => {
      expect(screen.getByText(/已由其他客户端处理/)).toBeInTheDocument()
    })
    expect(screen.queryByTestId('pending-action-card')).not.toBeInTheDocument()
  })

  test('重连后从 snapshot 恢复审批卡片，不伪造历史内容块', async () => {
    const socket = await connected()
    socket.serverClose()
    await waitFor(() => {
      expect(screen.getByText('已断开')).toBeInTheDocument()
    })
    fireEvent.click(screen.getByRole('button', { name: '重新连接' }))
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(2)
    })

    const second = FakeWebSocket.instances[1]!
    second.serverOpen()
    second.serverMessage({
      version: PROTOCOL_VERSION, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
    })
    second.serverMessage({
      version: PROTOCOL_VERSION, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {},
    })
    second.serverMessage({
      ...SNAPSHOT,
      payload: {
        connectionStatus: 'subscribed',
        snapshot: {
          ...SNAPSHOT.payload.snapshot,
          turn: {
            status: 'waiting',
            requestId: 'req-pc',
            source: 'terminal',
            message: '',
            error: null,
            pendingAction: APPROVAL,
          },
        },
      },
    })

    await waitFor(() => {
      expect(screen.getByTestId('pending-action-card')).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: '本会话允许' })).toBeInTheDocument()
    expect(screen.getByTestId('message-feed').textContent).not.toContain('工具')
  })
})

describe('App：B2 取消与排队', () => {
  test('运行中 Turn 可取消，取消生效后显示已取消', async () => {
    const socket = await connected()
    socket.serverMessage(turnStartedAt(8))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '取消当前 Turn' })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: '取消当前 Turn' }))
    expect(lastCommand(socket)).toMatchObject({
      payload: { kind: 'cancel_turn', requestId: 'req-pc' },
    })
    // 发出取消请求与已取消是两个状态（§7）。
    expect(screen.getByRole('button', { name: '取消当前 Turn' })).toHaveTextContent('停止中')

    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'command_result', sessionId: 'session-1',
      commandId: lastCommand(socket).commandId, requestId: 'req-pc', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: 'req-pc', seq: 9,
      payload: { event: { seq: 9, kind: 'turn_completed', sessionId: 'session-1', requestId: 'req-pc', source: 'terminal', text: '', sessionStatus: '', turnStatus: 'cancelled', error: null } },
    })

    await waitFor(() => {
      expect(screen.getByText(/Turn：已取消/)).toBeInTheDocument()
    })
    expect(screen.queryByRole('button', { name: '取消当前 Turn' })).not.toBeInTheDocument()
  })

  test('waiting 期间仍可取消当前 Turn', async () => {
    const socket = await connected()
    socket.serverMessage(turnStartedAt(8))
    socket.serverMessage(actionRequiredAt(9))
    await waitFor(() => {
      expect(screen.getByTestId('pending-action-card')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: '取消当前 Turn' }))
    expect(lastCommand(socket)).toMatchObject({
      payload: { kind: 'cancel_turn', requestId: 'req-pc' },
    })
  })

  test('排队列表按 snapshot 渲染，逐项取消后本地移除', async () => {
    window.location.hash = fragmentOf(PAIRING)
    render(<App socketFactory={(url) => new FakeWebSocket(url)} />)
    const socket = FakeWebSocket.instances[0]!
    socket.serverOpen()
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'hello_ack', sessionId: '', commandId: '', requestId: '', seq: 0, payload: {},
    })
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'subscribed', sessionId: 'session-1', commandId: '', requestId: '', seq: 0, payload: {},
    })
    socket.serverMessage({
      ...SNAPSHOT,
      payload: {
        connectionStatus: 'subscribed',
        snapshot: {
          ...SNAPSHOT.payload.snapshot,
          turn: { status: 'running', requestId: 'req-pc', source: 'terminal', message: '', error: null, pendingAction: null },
          queueDepth: 1,
          queuedRequests: [{ requestId: 'req-q1', source: 'mobile', preview: '排队中的输入' }],
        },
      },
    })

    await waitFor(() => {
      expect(screen.getByText('排队中的输入')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: '取消' }))
    expect(lastCommand(socket)).toMatchObject({
      payload: { kind: 'cancel_queued', requestId: 'req-q1' },
    })

    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'command_result', sessionId: 'session-1',
      commandId: lastCommand(socket).commandId, requestId: 'req-q1', seq: 0,
      payload: { accepted: true, code: '', message: '' },
    })
    await waitFor(() => {
      expect(screen.queryByText('排队中的输入')).not.toBeInTheDocument()
    })
  })
})

describe('App：B2 终态与不支持通知', () => {
  test('session_stopped 显示终态横幅、禁用输入并保留已收内容', async () => {
    const socket = await connected()
    socket.serverMessage(turnStartedAt(8))
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'event', sessionId: 'session-1', commandId: '', requestId: '', seq: 9,
      payload: { event: { seq: 9, kind: 'session_stopped', sessionId: 'session-1', requestId: '', source: '', text: '', sessionStatus: 'stopped', turnStatus: '', error: null } },
    })

    await waitFor(() => {
      expect(screen.getByTestId('session-terminal-banner').textContent).toContain('会话已结束')
    })
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
    // 已收到的内容保留。
    expect(screen.getByText('跑一次测试')).toBeInTheDocument()
  })

  test('unsupported 通知转为可读提示且不断开连接', async () => {
    const socket = await connected()
    socket.serverMessage({
      version: PROTOCOL_VERSION, type: 'unsupported', sessionId: 'session-1', commandId: '', requestId: '', seq: 8,
      payload: { code: 'unknown_block' },
    })

    await waitFor(() => {
      expect(screen.getByTestId('session-notice').textContent).toContain('电脑端')
    })
    expect(screen.getByText('已连接')).toBeInTheDocument()
    expect(socket.closed).toBe(false)
  })
})
