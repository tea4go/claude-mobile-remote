import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import App from '../App'
import { debugLog } from './log'
afterEach(() => { act(() => { debugLog.setEnabled(false); debugLog.clear() }); window.location.hash = '' })
test('first pairing screen shows logs by default and settings can disable them', async () => {
  expect(debugLog.getSnapshot().enabled).toBe(true)
  render(<App cache={null} experience={null} />)
  fireEvent.click(screen.getByRole('button', { name: '设置' }))
  const toggle = await screen.findByRole('switch', { name: '调试日志开关' })
  expect(toggle).toHaveAttribute('aria-checked', 'true')
  expect(await screen.findByRole('button', { name: '打开调试日志' })).toBeInTheDocument()
  fireEvent.click(toggle)
  expect(debugLog.getSnapshot().enabled).toBe(false)
  expect(screen.queryByRole('button', { name: '打开调试日志' })).not.toBeInTheDocument()
})
