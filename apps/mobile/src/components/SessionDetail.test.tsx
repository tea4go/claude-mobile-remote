import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { createInitialState, type MobileState } from '../session/reducer'
import { listSessions, upsertSession, createInitialSessionStore, type SessionRecord } from '../session/store'
import SessionDetail from './SessionDetail'

function record(): SessionRecord {
  const store = upsertSession(createInitialSessionStore(), {
    version: 1,
    wsUrl: 'ws://127.0.0.1:8082/ws',
    sessionId: 'session-abcdefgh-1',
    pairToken: 'pair-token-must-not-render',
    expiresAt: '2099-01-01T00:00:00Z',
  })
  return listSessions(store)[0]!
}

function connectedState(overrides: Partial<MobileState> = {}): MobileState {
  return {
    ...createInitialState(),
    phase: 'connected',
    connection: 'connected',
    sessionId: 'session-abcdefgh-1',
    sessionStatus: 'ready',
    canSend: true,
    ...overrides,
  }
}

function noop() {}

describe('SessionDetail：基本渲染', () => {
  test('状态和重连固定在标题栏，消息使用全宽滚动区，输入栏固定在滚动区外', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState({ lastNotice: { code: 'unknown_block', message: '请到电脑端处理' } })}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )
    const scroll = screen.getByTestId('detail-scroll')
    expect(scroll).not.toContainElement(screen.getByTestId('session-status-compact'))
    expect(scroll).toContainElement(screen.getByTestId('session-notice'))
    expect(scroll).toContainElement(screen.getByTestId('message-feed'))
    expect(scroll).not.toContainElement(screen.getByRole('textbox'))
    expect(scroll.style.paddingRight).toBe('')
    expect(scroll.style.overflowY).toBe('auto')
  })

  test('CLI 配对在输入区左下角新建，终止入口隐藏', () => {
    const onCreateSession = vi.fn()
    render(
      <SessionDetail
        record={{ ...record(), capabilities: ['create_session', 'stop_session'] }}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
        onCreateSession={onCreateSession}
      />,
    )

    const create = screen.getByRole('button', { name: '新建会话' })
    const attachment = screen.getByRole('button', { name: '添加照片或文件' })
    expect(create.compareDocumentPosition(attachment) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.queryByRole('button', { name: '终止此会话' })).not.toBeInTheDocument()
    fireEvent.click(create)
    expect(onCreateSession).toHaveBeenCalledOnce()
  })

  test('终态仍保留新建入口，但不显示终止操作', () => {
    render(
      <SessionDetail
        record={{ ...record(), capabilities: ['create_session', 'stop_session'] }}
        state={connectedState({ sessionStatus: 'stopped', canSend: false })}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
        onCreateSession={noop}
      />,
    )
    expect(screen.getByRole('button', { name: '新建会话' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: '终止此会话' })).not.toBeInTheDocument()
  })

  test('创建中禁用新建入口，附件入口保持独立', () => {
    render(
      <SessionDetail
        record={{ ...record(), capabilities: ['create_session', 'stop_session'] }}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
        onCreateSession={vi.fn()}
        managementBusy
      />,
    )
    expect(screen.getByRole('button', { name: '新建会话' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '添加照片或文件' })).toBeEnabled()
  })

  test('WhaleTerm 配对无新建与终止入口，附件入口保留', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
        onCreateSession={vi.fn()}
      />,
    )
    expect(screen.queryByRole('button', { name: '新建会话' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '终止此会话' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '添加照片或文件' })).toBeInTheDocument()
  })

  test('显示会话名称、状态栏与输入区', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(screen.getByText('会话 session-')).toBeInTheDocument()
    expect(screen.getByText('已连接')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '发送' })).toBeInTheDocument()
  })

  test('渲染消息流内容', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState({
          messages: [
            { kind: 'input', source: 'mobile', requestId: 'r-1', text: '手机提问', status: 'accepted' },
          ],
        })}
        turnStatus="running"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(screen.getByText('手机提问')).toBeInTheDocument()
    expect(screen.getByText(/已接收/)).toBeInTheDocument()
  })

  test('不渲染 Pair Token', () => {
    const { container } = render(
      <SessionDetail
        record={record()}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(container.innerHTML).not.toContain('pair-token-must-not-render')
  })
})

describe('SessionDetail：交互', () => {
  test('运行中显示悬浮终止按钮并回调当前 Turn', () => {
    const onCancelTurn = vi.fn()
    render(
      <SessionDetail
        record={record()}
        state={connectedState()}
        turnStatus="running"
        pairingInvalid={false}
        cancelTarget="req-running"
        onResolveAction={noop}
        onCancelTurn={onCancelTurn}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    const button = screen.getByRole('button', { name: '取消当前 Turn' })
    expect(screen.queryByRole('button', { name: '发送' })).not.toBeInTheDocument()
    expect(screen.getByText(/正在等待 Claude 响应/)).toBeInTheDocument()
    fireEvent.click(button)
    expect(onCancelTurn).toHaveBeenCalledExactlyOnceWith('req-running')
  })

  test('返回按钮回调 onBack', () => {
    const onBack = vi.fn()
    render(
      <SessionDetail
        record={record()}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={onBack}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(screen.queryByText('返回')).not.toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('返回'))

    expect(onBack).toHaveBeenCalledOnce()
  })

  test('发送文本回调 onSend', () => {
    const onSend = vi.fn()
    render(
      <SessionDetail
        record={record()}
        state={connectedState()}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={onSend}
      />,
    )

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '你好' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(onSend).toHaveBeenCalledExactlyOnceWith('你好')
  })
})

describe('SessionDetail：错误与重连', () => {
  test('断开时显示重连按钮并回调', () => {
    const onReconnect = vi.fn()
    render(
      <SessionDetail
        record={record()}
        state={connectedState({ phase: 'disconnected', connection: 'disconnected', canSend: false })}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={onReconnect}
        onSend={noop}
      />,
    )

    expect(screen.getByTestId('detail-scroll')).not.toContainElement(screen.getByRole('button', { name: '重新连接' }))
    fireEvent.click(screen.getByRole('button', { name: '重新连接' }))

    expect(onReconnect).toHaveBeenCalledOnce()
  })

  test('配对失效时提示更换配对并隐藏重连', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState({ phase: 'disconnected', connection: 'disconnected', canSend: false })}
        turnStatus="idle"
        pairingInvalid
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(screen.getByText(/配对已失效/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '重新连接' })).not.toBeInTheDocument()
  })

  test('协议错误时提示重新连接以获取最新状态', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState({ phase: 'protocolError', canSend: false })}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(screen.getByText(/状态异常/)).toBeInTheDocument()
  })

  test('断开连接时触碰禁用输入区说明原因', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState({ phase: 'disconnected', connection: 'disconnected', canSend: false })}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: '输入暂不可用' }))
    expect(screen.getByRole('status')).toHaveTextContent('连接不可用，暂时无法输入')
  })

  test('展示命令错误信息', () => {
    render(
      <SessionDetail
        record={record()}
        state={connectedState({ lastError: { code: 'busy', message: '会话忙' } })}
        turnStatus="idle"
        pairingInvalid={false}
        cancelTarget={null}
        onResolveAction={noop}
        onCancelTurn={noop}
        onCancelQueued={noop}
        onBack={noop}
        onReconnect={noop}
        onSend={noop}
      />,
    )

    expect(screen.getByText('会话忙')).toBeInTheDocument()
  })
})
