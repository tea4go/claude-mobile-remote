import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import QueuedRequestList from './QueuedRequestList'
import type { QueuedRequest } from '../protocol/types'

const requests: QueuedRequest[] = [
  { requestId: 'req-2', source: 'terminal', preview: '继续改配置' },
  { requestId: 'req-3', source: 'mobile', preview: '再跑一次测试' },
]

describe('QueuedRequestList（B2 §6）', () => {
  test('空队列不渲染任何内容', () => {
    const { container } = render(
      <QueuedRequestList requests={[]} enabled onCancel={vi.fn()} />,
    )
    expect(container.textContent).toBe('')
  })

  test('按 queuedRequests 顺序渲染预览与来源', () => {
    render(<QueuedRequestList requests={requests} enabled onCancel={vi.fn()} />)
    const items = screen.getAllByTestId('queued-request-item')
    expect(items).toHaveLength(2)
    expect(items[0]?.textContent).toContain('继续改配置')
    expect(items[0]?.textContent).toContain('电脑端')
    expect(items[1]?.textContent).toContain('再跑一次测试')
    expect(items[1]?.textContent).toContain('手机端')
  })

  test('逐项取消只回传对应 request ID', () => {
    const onCancel = vi.fn()
    render(<QueuedRequestList requests={requests} enabled onCancel={onCancel} />)
    const buttons = screen.getAllByRole('button', { name: '取消' })
    expect(buttons).toHaveLength(2)
    fireEvent.click(buttons[1]!)
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onCancel).toHaveBeenCalledWith('req-3')
  })

  test('不可发送命令时取消按钮禁用', () => {
    render(<QueuedRequestList requests={requests} enabled={false} onCancel={vi.fn()} />)
    for (const button of screen.getAllByRole('button', { name: '取消' })) {
      expect(button).toBeDisabled()
    }
  })

  test('不展示 request ID', () => {
    const { container } = render(
      <QueuedRequestList requests={requests} enabled onCancel={vi.fn()} />,
    )
    expect(container.textContent).not.toContain('req-2')
  })
})
