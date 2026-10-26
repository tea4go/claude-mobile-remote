import { fireEvent, render, screen, act } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
const ports = vi.hoisted(() => ({ copyDebugLog: vi.fn(), exportDebugLog: vi.fn() }))
vi.mock('@capacitor/core', () => ({ Capacitor: { getPlatform: () => 'ios' } }))
vi.mock('../native/experience', () => ({ nativeExperience: ports }))
import DebugPanel from './DebugPanel'
import { debugLog } from '../debug/log'
afterEach(() => { act(() => { debugLog.setEnabled(false); debugLog.clear() }); vi.clearAllMocks() })
test('iOS uses native clipboard and file sharing, with cancel and failure feedback', async () => {
  ports.copyDebugLog.mockResolvedValue(undefined)
  ports.exportDebugLog.mockResolvedValue({ completed: false })
  debugLog.setEnabled(true)
  render(<DebugPanel />)
  fireEvent.click(screen.getByText('调试'))
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
