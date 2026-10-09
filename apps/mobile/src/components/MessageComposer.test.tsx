import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import MessageComposer from './MessageComposer'

describe('MessageComposer', () => {
  test('enabled 时可输入并发送', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled onSend={onSend} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: '你好' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(onSend).toHaveBeenCalledWith('你好')
  })

  test('disabled 时输入与按钮禁用', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled={false} onSend={onSend} />)
    expect(screen.getByRole('textbox')).toBeDisabled()
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '输入暂不可用' }))
    expect(screen.getByRole('status')).toHaveTextContent('当前会话暂不可输入')
    expect(screen.getByRole('textbox')).not.toHaveFocus()
    expect(onSend).not.toHaveBeenCalled()
  })

  test('连接不可用时点击输入区有具体反馈，恢复后反馈消失', () => {
    const { rerender } = render(<MessageComposer enabled={false} disabledReason="连接不可用，暂时无法输入" onSend={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: '输入暂不可用' }))
    expect(screen.getByRole('status')).toHaveTextContent('连接不可用，暂时无法输入')
    rerender(<MessageComposer enabled onSend={vi.fn()} />)
    expect(screen.queryByRole('button', { name: '输入暂不可用' })).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  test('附件和权限触控保留已聚焦输入，菜单项仍可执行；外部和其他按钮不拦截', () => {
    const onPermissionModeChange = vi.fn()
    render(<MessageComposer enabled onSend={vi.fn()} onPermissionModeChange={onPermissionModeChange} />)
    const input = screen.getByRole('textbox')
    input.focus()
    const pointerDown = (target: Element) => {
      const event = new Event('pointerdown', { bubbles: true, cancelable: true })
      target.dispatchEvent(event)
      return event.defaultPrevented
    }
    expect(pointerDown(screen.getByRole('button', { name: '添加照片或文件' }))).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: '添加照片或文件' }))
    expect(pointerDown(screen.getByTestId('choose-file'))).toBe(true)
    fireEvent.click(document.body)
    expect(pointerDown(screen.getByRole('button', { name: '权限模式' }))).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: '权限模式' }))
    expect(pointerDown(screen.getByTestId('permission-option-plan'))).toBe(true)
    fireEvent.click(screen.getByTestId('permission-option-plan'))
    expect(onPermissionModeChange).toHaveBeenCalledWith('plan')
    expect(pointerDown(screen.getByRole('button', { name: '语音输入（原生能力预留）' }))).toBe(false)
    expect(pointerDown(document.body)).toBe(false)
  })

  test('空白文本不发送', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled onSend={onSend} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '   ' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(onSend).not.toHaveBeenCalled()
  })

  test('发送后清空输入', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled onSend={onSend} />)
    const input = screen.getByRole('textbox') as HTMLTextAreaElement
    fireEvent.change(input, { target: { value: '你好' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(input.value).toBe('')
  })

  test('超出 1 MiB UTF-8 字节的文本被拒绝并提示', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled onSend={onSend} />)
    // '你' UTF-8 编码 3 字节；350000 个字符 = 1050000 字节 > 1 MiB。
    const oversized = '你'.repeat(350_000)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: oversized } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(onSend).not.toHaveBeenCalled()
    expect(screen.getByText('文本超出长度上限')).toBeInTheDocument()
  })

  test('Enter 发送，Shift+Enter 保留换行', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled onSend={onSend} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: '第一行' } })
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })
    expect(onSend).not.toHaveBeenCalled()
    fireEvent.change(input, { target: { value: '第一行\n第二行' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSend).toHaveBeenCalledWith('第一行\n第二行')
  })

  test('附件、相机和语音入口明确标记为原生能力预留', () => {
    const onSend = vi.fn()
    render(<MessageComposer enabled onSend={onSend} />)
    expect(screen.getByTestId('attachment-toggle').querySelector('img')?.getAttribute('src')).toContain('document.svg')
    fireEvent.click(screen.getByTestId('attachment-toggle'))
    expect(screen.getByTestId('choose-photo')).toHaveTextContent('照片')
    expect(screen.getByTestId('choose-file')).toHaveTextContent('文件')
    expect(screen.getByTestId('open-camera')).toHaveTextContent('相机')
    fireEvent.click(document.body)
    expect(document.querySelector('.composer-popover-menu')).toHaveClass('adm-popover-hidden')
    fireEvent.click(screen.getByTestId('attachment-toggle'))
    fireEvent.click(screen.getByTestId('voice-input'))
    expect(screen.getByText('语音输入需要原生麦克风能力，当前 Web 版本暂未启用。')).toBeInTheDocument()
  })

  test('权限请求显示 PC 同步状态并保留运行中终止入口', () => {
    const onCancelTurn = vi.fn()
    render(
      <MessageComposer
        enabled
        onSend={vi.fn()}
        turnRunning
        cancelTarget="turn-1"
        cancelEnabled
        onCancelTurn={onCancelTurn}
        permissionPending
      />,
    )
    expect(screen.getByRole('button', { name: '权限请求待确认' })).toBeDisabled()
    expect(screen.getByTestId('floating-cancel-turn')).toHaveStyle({ position: 'absolute', height: '26px' })
    expect(screen.getByTestId('floating-cancel-turn')).toHaveTextContent('停止')
    fireEvent.click(screen.getByRole('button', { name: '取消当前 Turn' }))
    expect(onCancelTurn).toHaveBeenCalledWith('turn-1')
  })

  test('取消请求发出后显示简洁反馈并禁用重复点击', () => {
    render(
      <MessageComposer
        enabled
        onSend={vi.fn()}
        turnRunning
        cancelTarget="turn-1"
        cancelEnabled={false}
        onCancelTurn={vi.fn()}
      />,
    )
    const stopButton = screen.getByRole('button', { name: '取消当前 Turn' })
    expect(stopButton).toHaveTextContent('停止中')
    expect(stopButton).toBeDisabled()
  })

  test('空闲时也展示当前权限模式并可请求切换', () => {
    const onPermissionModeChange = vi.fn()
    render(
      <MessageComposer
        enabled
        onSend={vi.fn()}
        permissionMode="plan"
        onPermissionModeChange={onPermissionModeChange}
      />,
    )
    const selector = screen.getByRole('button', { name: '权限模式' })
    expect(selector).toHaveTextContent('')
    fireEvent.click(selector)
    const activeOption = screen.getByTestId('permission-option-plan')
    expect(activeOption).toHaveClass('composer-permission-active')
    expect(activeOption.querySelector('.composer-permission-check')).not.toBeNull()
    expect(screen.getByTestId('permission-option-default').querySelector('.composer-permission-check')).toBeNull()
    fireEvent.click(screen.getByTestId('permission-option-acceptEdits'))
    expect(onPermissionModeChange).toHaveBeenCalledWith('acceptEdits')
  })
})
