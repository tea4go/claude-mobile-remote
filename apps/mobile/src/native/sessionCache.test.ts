import { afterEach, expect, test, vi } from 'vitest'
import { CACHE_AGE, CACHE_BYTES, CACHE_MESSAGES, decodeSessionCache, encodeSessionCache, SessionCacheWriter, type SessionCachePort } from './sessionCache'
import { createInitialState, eventReceived, sessionOpened, snapshotReceived, type MobileState } from '../session/reducer'
import { createAppStore } from '../session/appStore'

const now = 1800000000000
function state(): MobileState {
  return { ...createInitialState(), sessionId: 'a', projectCwd: '/work', sessionTitle: '保留标题',
    sessionStatus: 'ready', canSend: true,
    messages: [
      { kind: 'input', requestId: 'r', source: 'mobile', text: 'hello', status: 'accepted', attachments: [{ name: 'photo.png', mediaType: 'image/png', data: 'PRIVATE_IMAGE_BYTES' }] },
      { kind: 'claude', requestId: 'r', source: 'mobile', text: '正在生成', status: 'streaming', error: null },
      { kind: 'block', requestId: 'r', block: { id: 'tool', kind: 'tool_use', toolUseId: 't', toolClass: 'builtin', toolName: 'Read', mcpServer: '' } },
      { kind: 'block', requestId: 'r', block: { id: 'result', kind: 'tool_result', toolUseId: 't', isError: false, output: '结果' } },
      { kind: 'answer', requestId: 'r', answers: [{ questionId: 'q', question: '选择', header: '问题', values: ['A'] }] },
    ] }
}
afterEach(() => vi.useRealTimers())
test('roundtrip restores display only, preserving tools and answers without attachment bytes or permissions', () => {
  const current = { ...state(), pairToken: 'PRIVATE_TOKEN' }
  const data = encodeSessionCache('a', current, now)
  expect(data).not.toMatch(/PRIVATE_TOKEN|PRIVATE_IMAGE_BYTES|canSend|pendingAction|sessionStatus/)
  const restored = decodeSessionCache(data, 'a', now)!
  expect(restored.projectCwd).toBe('/work')
  expect(restored.messages).toHaveLength(5)
  expect(restored.messages[1]).toMatchObject({ status: 'done' })
  expect(restored).toMatchObject({ cachedAt: now, canSend: false, canResolveAction: false, pendingAction: null, pendingCommand: null, connection: 'disconnected', sessionStatus: null })
})
test('rejects corrupt, expired, future, cross-session and unsupported caches', () => {
  const data = encodeSessionCache('a', state(), now)
  for (const [value, id, time] of [['{', 'a', now], [data, 'b', now], [data, 'a', now + CACHE_AGE + 1], [data, 'a', now - 1], [data.replace('"version":1', '"version":8'), 'a', now]] as const) expect(decodeSessionCache(value, id, time)).toBeNull()
  expect(decodeSessionCache('x'.repeat(CACHE_BYTES + 1), 'a', now)).toBeNull()
})
test('retains newest items under both count and UTF-8 byte limits', () => {
  const current = state()
  current.messages = Array.from({ length: CACHE_MESSAGES + 30 }, (_, i) => ({ kind: 'claude', requestId: String(i), source: 'mobile', text: '中'.repeat(10000), status: 'done', error: null }))
  const data = encodeSessionCache('a', current, now)
  expect(new TextEncoder().encode(data).byteLength).toBeLessThanOrEqual(CACHE_BYTES)
  const messages = decodeSessionCache(data, 'a', now)!.messages
  expect(messages.length).toBeLessThanOrEqual(CACHE_MESSAGES)
  expect(messages.at(-1)?.requestId).toBe('229')
})
test('snapshot corrects restored turn by requestId without duplicating messages or enabling stale actions', () => {
  const store = createAppStore()
  store.getState().restoreCachedSession('a', decodeSessionCache(encodeSessionCache('a', state(), now), 'a', now)!)
  store.getState().dispatchSession('a', sessionOpened(store.getState().sessions.a!))
  store.getState().dispatchSession('a', snapshotReceived({ sessionId: 'a', sessionStatus: 'ready', sessionError: null, seq: 9, queueDepth: 0, queuedRequests: [], turn: { requestId: 'r', source: 'mobile', status: 'completed', message: '完成后的全文', error: null, pendingAction: null } }))
  const restored = store.getState().sessions.a!
  expect(restored.cachedAt).toBeNull()
  expect(restored.messages.filter((item) => item.kind === 'claude')).toEqual([expect.objectContaining({ requestId: 'r', text: '完成后的全文', status: 'done' })])
})
function fixture() {
  const store = createAppStore()
  store.getState().upsertPairing({ version: 1, sessionId: 'a', wsUrl: 'ws://pc/ws', pairToken: 'fixture', expiresAt: '2099-01-01T00:00:00Z' })
  store.getState().restoreCachedSession('a', state())
  const files = new Map<string, string>()
  const port: SessionCachePort = {
    readSessionCache: vi.fn(async ({ sessionId }) => ({ data: files.get(sessionId) ?? null })),
    writeSessionCache: vi.fn(async ({ sessionId, data }) => { files.set(sessionId, data) }),
    removeSessionCache: vi.fn(async ({ sessionId }) => { files.delete(sessionId) }),
    clearSessionCache: vi.fn(async () => { files.clear() }),
  }
  return { store, port, files }
}
test('coalesces stream updates and orders removal after pending writes', async () => {
  vi.useFakeTimers()
  const { store, port, files } = fixture()
  const writer = new SessionCacheWriter(store, port, () => now, vi.fn())
  for (let seq = 1; seq <= 20; seq++) store.getState().dispatchSession('a', eventReceived({ kind: 'text_delta', sessionId: 'a', requestId: 'r', seq, source: 'mobile', text: 'a', sessionStatus: '', turnStatus: '', error: null }))
  expect(port.writeSessionCache).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(1000)
  expect(port.writeSessionCache).toHaveBeenCalledTimes(1)
  const pending = writer.flush()
  store.getState().removePairing('a')
  await pending; await writer.flush()
  expect(files.has('a')).toBe(false)
  writer.dispose()
})
test('clear waits for pending writes and cannot be undone by the old timer', async () => {
  vi.useFakeTimers()
  const { store, port, files } = fixture()
  const writer = new SessionCacheWriter(store, port, () => now, vi.fn())
  void writer.flush()
  await writer.clear()
  await vi.advanceTimersByTimeAsync(2000)
  expect(files.size).toBe(0)
  writer.dispose()
})
test('cache failures are reported without modifying the connected runtime', async () => {
  const { store, port } = fixture()
  port.writeSessionCache = vi.fn(async () => { throw Error('disk full') })
  const onError = vi.fn()
  const writer = new SessionCacheWriter(store, port, () => now, onError)
  await writer.flush()
  expect(onError).toHaveBeenCalledTimes(1)
  expect(store.getState().sessions.a!.messages).toEqual(state().messages)
  writer.dispose()
})
