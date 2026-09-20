import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import PairingForm from './PairingForm'

const VALID_JSON =
  '{"version":1,"wsUrl":"ws://127.0.0.1:8765/ws","sessionId":"s","pairToken":"t","expiresAt":"2099-01-01T00:00:00Z"}'

describe('PairingForm', () => {
  test('渲染标题、说明、输入框与连接按钮', () => {
    render(<PairingForm initialError={null} onSubmit={() => {}} />)
    expect(screen.getByText('连接 Claude 会话')).toBeInTheDocument()
    expect(screen.getByText('粘贴 Service 输出的配对 JSON 或完整 #pair Fragment')).toBeInTheDocument()
    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '连接' })).toBeInTheDocument()
  })

  test('空输入提交显示就地错误', () => {
    const onSubmit = vi.fn()
    render(<PairingForm initialError={null} onSubmit={onSubmit} />)
    fireEvent.click(screen.getByRole('button', { name: '连接' }))
    expect(screen.getByText('请输入配对 JSON 或 Fragment')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  test('有效 JSON 提交回调', () => {
    const onSubmit = vi.fn()
    render(<PairingForm initialError={null} onSubmit={onSubmit} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: VALID_JSON } })
    fireEvent.click(screen.getByRole('button', { name: '连接' }))
    expect(onSubmit).toHaveBeenCalledWith(VALID_JSON)
  })

  test('显示初始错误', () => {
    render(<PairingForm initialError="配对信息无效" onSubmit={() => {}} />)
    expect(screen.getByText('配对信息无效')).toBeInTheDocument()
  })

  test('错误文案不回显输入的 token', () => {
    render(
      <PairingForm initialError="配对信息无效" onSubmit={() => {}} />,
    )
    expect(document.body.textContent).not.toContain('secret-token')
  })
})
