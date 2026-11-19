import { afterEach, expect, test, vi } from 'vitest'
import { ProtocolClient, type SocketLike } from '../protocol/client'
import { debugLog } from './log'
import { debugButtonPosition } from './placement'
afterEach(() => { debugLog.setEnabled(false); debugLog.clear(); vi.useRealTimers() })
function setup() {
  debugLog.setEnabled(true)
  const sockets: SocketLike[] = []
  const client = new ProtocolClient({ version: 1, sessionId: 'private-session', wsUrl: 'ws://host:9000/path?secret=token', pairToken: 'hidden-token', expiresAt: '2099-01-01T00:00:00Z' }, () => {
    const socket: SocketLike = { readyState: 0, send() {}, close() {}, onopen: null, onclose: null, onmessage: null, onerror: null }
    sockets.push(socket); return socket
  })
  client.connect()
  return { client, sockets }
}
test('error and late close retain original connection/attempt, raw reason excluded, callbacks once', () => {
  const { client, sockets } = setup()
  const close = vi.fn(); client.onClose = close
  sockets[0].onerror?.()
  const lateClose = sockets[0].onclose
  client.reconnect()
  const event = { code: 1006, wasClean: false }
  Object.defineProperty(event, 'reason', { get() { throw new Error('never read reason') } })
  lateClose?.(event)
  const entries = debugLog.getSnapshot().entries
  expect(entries.some((entry) => entry.code === 'socket_error')).toBe(true)
  expect(entries.some((entry) => entry.code === 'manual_retry')).toBe(true)
  const closed = entries.find((entry) => entry.code === 'socket_closed')!
  expect(closed).toMatchObject({ attempt: 1, closeCode: 1006, wasClean: false })
  expect(entries.filter((entry) => entry.code === 'started').map((entry) => entry.attempt)).toEqual([1, 2])
  expect(client.connectionState()).toBe('connecting')
  expect(close).toHaveBeenCalledTimes(1)
  expect(debugLog.exportText()).not.toMatch(/private-session|hidden-token|secret=|never read/)
  client.close()
})
test('timeout includes event code, stage, elapsed and deadline; local close recorded', () => {
  vi.useFakeTimers()
  const { client } = setup()
  vi.advanceTimersByTime(8000)
  expect(debugLog.getSnapshot().entries.find((entry) => entry.code === 'connect_timeout')).toMatchObject({ stage: 'connecting', elapsedMs: 8000, phaseElapsedMs: 8000, timeoutMs: 8000, attempt: 1 })
  client.reconnect(); client.close()
  expect(debugLog.getSnapshot().entries.at(-1)?.code).toBe('cancelled')
})
test('floating entry starts at lower right and clamps drags to the visible viewport', () => {
  const viewport = { left: 0, top: 0, width: 400, height: 800 }
  const safe = { right: 4, bottom: 20 }
  expect(debugButtonPosition(viewport, 40, 40, safe, null)).toEqual({ x: 344, y: 728 })
  expect(debugButtonPosition(viewport, 40, 40, safe, null, undefined, 650)).toEqual({ x: 344, y: 602 })
  expect(debugButtonPosition(viewport, 40, 40, safe, { x: 300, y: 690 }, undefined, 650))
    .toEqual({ x: 300, y: 690 })
  expect(debugButtonPosition(viewport, 40, 40, safe, { x: -20, y: 90 })).toEqual({ x: 12, y: 90 })
  expect(debugButtonPosition({ ...viewport, top: 100, height: 480 }, 40, 40, safe, { x: 200, y: 700 }, 500))
    .toEqual({ x: 200, y: 452 })
})
