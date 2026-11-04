import { afterEach, expect, test } from 'vitest'
import { debugLog, recordAction, observeDebugErrors } from './log'
import { recordNetworkDiagnostic } from '../protocol/diagnostics'
test('debug entry is hidden on a fresh app start', () => { expect(debugLog.getSnapshot().enabled).toBe(false) })
afterEach(() => { debugLog.setEnabled(false); debugLog.clear() })
test('logs continue while hidden, remain bounded and can be cleared', () => {
  debugLog.setEnabled(false)
  recordAction('scan'); expect(debugLog.getSnapshot().entries).toHaveLength(1)
  debugLog.setEnabled(true)
  for (let i = 0; i < 250; i++) recordAction('scan')
  expect(debugLog.getSnapshot().entries).toHaveLength(200)
  debugLog.setEnabled(false); recordAction('saved')
  expect(debugLog.getSnapshot().entries).toHaveLength(200)
  expect(debugLog.getSnapshot().entries.at(-1)?.text).toBe('配对保存成功')
  debugLog.clear(); expect(JSON.parse(debugLog.exportText()).entries).toEqual([])
})
test('connection logs strip credentials, path, query, fragment and session ID', () => {
  debugLog.setEnabled(true)
  recordNetworkDiagnostic('private-session', 'ws://user:password@192.168.1.2:9000/private?token=secret#qr', 'connecting', 'started', Date.now())
  const text = debugLog.exportText()
  expect(text).toContain('ws://192.168.1.2:9000')
  for (const value of ['user', 'password', 'private', 'token', 'secret', '#qr']) expect(text).not.toContain(value)
})
test('error observation never reads sensitive error fields and detaches', () => {
  debugLog.setEnabled(true)
  const stop = observeDebugErrors()
  const event = new Event('unhandledrejection')
  Object.defineProperty(event, 'reason', { get: () => { throw new Error('must not read') } })
  window.dispatchEvent(event)
  expect(debugLog.exportText()).toContain('未处理的异步异常')
  const count = debugLog.getSnapshot().entries.length
  stop(); window.dispatchEvent(event)
  expect(debugLog.getSnapshot().entries).toHaveLength(count)
})
