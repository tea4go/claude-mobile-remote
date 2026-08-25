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
})
