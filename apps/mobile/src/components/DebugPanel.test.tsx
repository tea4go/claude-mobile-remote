import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import DebugPanel from './DebugPanel'
import { debugLog, recordAction } from '../debug/log'
afterEach(() => { act(() => { debugLog.setEnabled(false); debugLog.clear() }); vi.restoreAllMocks() })
test('floating panel displays actions, copies sanitized export, clears and closes', async () => {
  debugLog.setEnabled(true)
  recordAction('scan')
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  render(<DebugPanel />)
  fireEvent.click(screen.getByRole('button', { name: '打开调试日志' }))
  expect(screen.getByRole('dialog')).toBeInTheDocument()
  expect(screen.getByText('打开扫码')).toBeInTheDocument()
  fireEvent.click(screen.getByText('复制日志'))
  expect(await screen.findByText('日志已复制')).toBeInTheDocument()
  expect(writeText).toHaveBeenCalledWith(debugLog.exportText())
  fireEvent.click(screen.getByText('清空日志'))
  expect(screen.getByText('暂无日志，请关闭面板后复现操作。')).toBeInTheDocument()
  fireEvent.click(screen.getByText('关闭'))
  expect(screen.queryByRole('dialog')).toBeNull()
  act(() => debugLog.setEnabled(false))
  expect(screen.queryByRole('button', { name: '打开调试日志' })).toBeNull()
})

test('new records appear first while panel is open without mutating store order', () => {
  debugLog.setEnabled(true)
  render(<DebugPanel />)
  fireEvent.click(screen.getByRole('button', { name: '打开调试日志' }))
  act(() => recordAction('scan'))
  act(() => recordAction('saved'))
  const articles = screen.getByRole('dialog').querySelectorAll('article')
  expect(articles[0]).toHaveTextContent('配对保存成功')
  expect(articles[1]).toHaveTextContent('打开扫码')
  expect(debugLog.getSnapshot().entries.at(-1)?.text).toBe('配对保存成功')
})
