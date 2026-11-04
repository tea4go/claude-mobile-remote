import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import App from '../App'
import { debugLog } from './log'
afterEach(() => { act(() => { debugLog.setEnabled(false); debugLog.clear() }); window.location.hash = '' })
test('hidden entry records errors and displays earlier logs after enabling', async () => {
  expect(debugLog.getSnapshot().enabled).toBe(false)
  render(<App cache={null} experience={null} />)
  act(() => window.dispatchEvent(new Event('error')))
  expect(screen.queryByRole('button', { name: '打开调试日志' })).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '设置' }))
  const toggle = await screen.findByRole('switch', { name: '调试日志开关' })
  expect(toggle).toHaveAttribute('aria-checked', 'false')
  fireEvent.click(toggle)
  fireEvent.click(await screen.findByRole('button', { name: '打开调试日志' }))
  expect(screen.getByText('JavaScript 异常（原始内容未采集）')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '关闭' }))
  fireEvent.click(toggle)
  expect(debugLog.getSnapshot().enabled).toBe(false)
  expect(screen.queryByRole('button', { name: '打开调试日志' })).not.toBeInTheDocument()
})
