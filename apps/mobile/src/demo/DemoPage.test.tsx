import { fireEvent, render, screen, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import DemoPage from './DemoPage'

afterEach(cleanup)
it('keeps disclosure visible and provides exit without native or network ports', () => {
  const exit = vi.fn()
  render(<DemoPage onExit={exit} />)
  expect(screen.getByText('离线演示 · 模拟数据，不连接电脑或 Claude')).toBeTruthy()
  expect((screen.getByLabelText('添加照片或文件') as HTMLButtonElement).disabled).toBe(true)
  expect((screen.getByPlaceholderText('输入消息体验模拟回复') as HTMLTextAreaElement).disabled).toBe(false)
  fireEvent.click(screen.getByText('演示审批'))
  expect(screen.getByText('演示问答').closest('button')?.disabled).toBe(true)
  fireEvent.click(screen.getByText('重新开始'))
  expect(screen.getByText('演示问答').closest('button')?.disabled).toBe(false)
  fireEvent.click(screen.getByText('退出演示'))
  expect(exit).toHaveBeenCalledOnce()
})
