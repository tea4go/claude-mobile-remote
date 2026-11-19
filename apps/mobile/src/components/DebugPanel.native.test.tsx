import { fireEvent, render, screen, act, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
const ports = vi.hoisted(() => ({ platform: 'ios', copyDebugLog: vi.fn(), exportDebugLog: vi.fn(), get: vi.fn(async () => ({ value: null as string | null })), set: vi.fn(async () => {}) }))
vi.mock('@capacitor/core', () => ({ Capacitor: { getPlatform: () => ports.platform } }))
vi.mock('@capacitor/preferences', () => ({ Preferences: ports }))
vi.mock('../native/experience', () => ({ nativeExperience: ports }))
import DebugPanel from './DebugPanel'
import { debugLog } from '../debug/log'
afterEach(() => { act(() => { debugLog.setEnabled(false); debugLog.clear() }); vi.clearAllMocks(); ports.platform = 'ios' })
test('iOS uses native clipboard and file sharing, with cancel and failure feedback', async () => {
  ports.copyDebugLog.mockResolvedValue(undefined)
  ports.exportDebugLog.mockResolvedValue({ completed: false })
  debugLog.setEnabled(true)
  render(<DebugPanel />)
  fireEvent.click(screen.getByRole('button', { name: '打开调试日志' }))
  fireEvent.click(screen.getByText('复制日志'))
  expect(await screen.findByText('日志已复制')).toBeInTheDocument()
  expect(ports.copyDebugLog).toHaveBeenCalledWith({ text: debugLog.exportText() })
  fireEvent.click(screen.getByText('导出日志'))
  expect(await screen.findByText('已取消导出')).toBeInTheDocument()
  expect(ports.exportDebugLog).toHaveBeenCalledWith({ text: debugLog.exportText() })
  ports.exportDebugLog.mockRejectedValue(new Error('private native error'))
  fireEvent.click(screen.getByText('导出日志'))
  expect(await screen.findByText('导出失败，请使用复制日志')).toBeInTheDocument()
  expect(debugLog.exportText()).not.toContain('private native error')
})

test('Android uses native clipboard and opens the system share panel', async () => {
  ports.platform = 'android'
  ports.copyDebugLog.mockResolvedValue(undefined)
  ports.exportDebugLog.mockResolvedValue({ opened: true })
  debugLog.setEnabled(true)
  render(<DebugPanel />)
  fireEvent.click(screen.getByRole('button', { name: '打开调试日志' }))
  fireEvent.click(screen.getByText('复制日志'))
  expect(await screen.findByText('日志已复制')).toBeInTheDocument()
  expect(ports.copyDebugLog).toHaveBeenCalledWith({ text: debugLog.exportText() })
  fireEvent.click(screen.getByText('导出日志'))
  expect(await screen.findByText('已打开分享面板')).toBeInTheDocument()
  expect(ports.exportDebugLog).toHaveBeenCalledWith({ text: debugLog.exportText() })
})

test.each(['ios', 'android'])('%s saves a completed drag and restores it on remount', async (platform) => {
  ports.platform = platform
  debugLog.setEnabled(true)
  const first = render(<DebugPanel />)
  const button = screen.getByRole('button', { name: '打开调试日志' })
  button.setPointerCapture = vi.fn()
  vi.spyOn(button, 'getBoundingClientRect').mockReturnValue({ left: 150, top: 250 } as DOMRect)
  function pointer(type: string, clientX: number, clientY: number) {
    const event = new Event(type, { bubbles: true })
    Object.assign(event, { isPrimary: true, button: 0, pointerId: 1, clientX, clientY })
    fireEvent(button, event)
  }
  pointer('pointerdown', 70, 70)
  pointer('pointermove', 150, 250)
  expect(ports.set).not.toHaveBeenCalled()
  pointer('pointerup', 150, 250)
  const value = '{"x":150,"y":250}'
  expect(ports.set).toHaveBeenCalledWith({ key: 'cmr.settings.debugPosition', value })
  first.unmount()
  ports.get.mockResolvedValueOnce({ value })
  render(<DebugPanel />)
  await waitFor(() => expect(screen.getByRole('button', { name: '打开调试日志' }))
    .toHaveStyle({ left: '150px', top: '250px' }))
})
