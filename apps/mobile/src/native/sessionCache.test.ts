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
function legacyCache(messages: MobileState['messages']) {
  return JSON.stringify({ version: 1, sessionId: 'a', savedAt: now, projectCwd: null, sessionTitle: null,
    createdAt: null, lastActivityAt: null, messages })
}
afterEach(() => vi.useRealTimers())
test('v1 cache migration strips inline tool data and retains model names', () => {
  const current = state()
  const migrated = decodeSessionCache(legacyCache(current.messages), 'a', now)!
  expect(migrated.messages[3]).toEqual({ kind: 'block', requestId: 'r', block: { id: 'result', kind: 'tool_result', toolUseId: 't', isError: false } })
  current.messages.push({ kind: 'claude', requestId: 'r', source: 'mobile', model: 'example-model', text: '答复', status: 'done', error: null })
  const saved = encodeSessionCache('a', current, now)
  expect(saved).not.toContain('"output"')
  const restored = decodeSessionCache(saved, 'a', now)!
  expect(restored.messages[0]).toMatchObject({ processingStarted: true })
  expect(restored.messages.at(-1)).toEqual(current.messages.at(-1))
})

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
  for (const [value, id, time] of [['{', 'a', now], [data, 'b', now], [data, 'a', now + CACHE_AGE + 1], [data, 'a', now - 1], [data.replace('"version":2', '"version":8'), 'a', now]] as const) expect(decodeSessionCache(value, id, time)).toBeNull()
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
test('an older cache corrects same-turn replies and steps before any PC connection', () => {
  const [question, reply, step, result, answer] = state().messages
  if (question.kind !== 'input') throw new Error('test fixture must start with a question')
  const unrelated = { ...question, attachments: undefined, requestId: 'previous', text: 'earlier question' }
  const recovered = decodeSessionCache(legacyCache([reply, step, unrelated, question, result, answer]), 'a', now)!
  expect(recovered.connection).toBe('disconnected')
  expect(recovered.messages.map(message => message.requestId)).toEqual(['previous', 'r', 'r', 'r', 'r', 'r'])
  expect(recovered.messages.map(message => message.kind)).toEqual(['input', 'input', 'claude', 'block', 'block', 'answer'])
})
test('step data uses a separate budget, with MCP identity, original anchors and message times', () => {
  const current = state()
  const inputAt = '2026-11-26T06:01:00.123Z'
  const replyAt = '2026-11-26T06:02:00Z'
  const [question, reply] = current.messages
  if (question.kind !== 'input' || reply.kind !== 'claude') throw new Error('invalid fixture')
  current.messages = [{ ...question, messageAt: inputAt, attachments: undefined },
    { ...reply, messageAt: replyAt, text: 'before' },
    { kind: 'block', requestId: 'r', block: { id: 'first', kind: 'thinking', thinking: 'THINKING'.repeat(100000) } },
    { ...reply, text: 'between' },
    { kind: 'block', requestId: 'r', block: { id: 'agent', kind: 'tool_use', toolUseId: 'agent', toolClass: 'builtin', toolName: 'Agent', mcpServer: '' } },
    ...Array.from({ length: CACHE_MESSAGES + 20 }, (_, index) => ({ kind: 'block' as const, requestId: 'r', block: {
      id: 'mcp-' + index, kind: 'tool_use' as const, toolUseId: 'call-' + index, toolClass: 'mcp' as const, toolName: 'search',
      mcpServer: 'local', parentToolUseId: 'agent', preview: 'PRIVATE_ARGUMENTS', detailAvailable: true,
    } })),
    { ...reply, text: 'after' },
    { kind: 'block', requestId: 'r', block: { id: 'result', kind: 'tool_result', toolUseId: 'agent', isError: false, output: 'PRIVATE_RESULT' } },
  ]
  current.subagentTasks = [{ requestId: 'r', toolUseId: 'agent', status: 'completed', summary: 'PRIVATE_SUBAGENT' }]
  const encoded = encodeSessionCache('a', current, now)
  expect(encoded).not.toMatch(/THINKING|PRIVATE_ARGUMENTS|PRIVATE_RESULT|PRIVATE_SUBAGENT/)
  expect(JSON.parse(encoded).timeline.map((item: { kind: string }) => item.kind)).toEqual(['input', 'claude', 'steps', 'claude', 'steps', 'claude'])
  expect(JSON.parse(encoded).steps[0].items).toHaveLength(200)
  const restored = decodeSessionCache(encoded, 'a', now)!
  expect(restored.messages[0]).toMatchObject({ kind: 'input', messageAt: inputAt })
  expect(restored.messages[1]).toMatchObject({ kind: 'claude', messageAt: replyAt })
  expect(restored.messages[2]).toMatchObject({ kind: 'block', block: { thinking: '' } })
  expect(restored.messages[4]).toMatchObject({ kind: 'block', cachedAgentStatus: 'completed', block: { toolUseId: 'agent' } })
  expect(restored.messages[5]).toMatchObject({ kind: 'block', block: { toolClass: 'mcp', mcpServer: 'local', parentToolUseId: 'agent' } })
  expect(restored.messages.at(-1)).toMatchObject({ kind: 'claude', text: 'after' })
})
test('reconnecting with a truncated snapshot keeps known positions and persists the PC message times', () => {
  const inputAt = '2026-11-26T06:01:00Z'
  const firstAt = '2026-11-26T06:02:00Z'
  const finalAt = '2026-11-26T06:03:00Z'
  const store = createAppStore()
  const source = 'terminal' as const
  const snapshot = { sessionId: 'a', sessionStatus: 'ready' as const, sessionError: null, seq: 0, queueDepth: 0, queuedRequests: [],
    turn: { requestId: 'r', source, status: 'running' as const, inputText: 'question', inputAt, message: '', error: null, pendingAction: null } }
  store.getState().dispatchSession('a', snapshotReceived(snapshot))
  const event = { sessionId: 'a', requestId: 'r', source, sessionStatus: '' as const, turnStatus: '' as const, error: null }
  store.getState().dispatchSession('a', eventReceived({ ...event, seq: 1, kind: 'text_delta', text: 'first', messageAt: firstAt }))
  store.getState().dispatchSession('a', eventReceived({ ...event, seq: 2, kind: 'content_block', text: '',
    block: { id: 'step', kind: 'thinking', thinking: 'body' } }))
  store.getState().dispatchSession('a', eventReceived({ ...event, seq: 3, kind: 'text_delta', text: 'final', messageAt: finalAt }))
  const readCache = () => decodeSessionCache(encodeSessionCache('a', store.getState().sessions.a!, now), 'a', now)!
  const persisted = readCache()
  expect(persisted.messages.filter(message => message.kind === 'claude').map(message => message.messageAt)).toEqual([firstAt, finalAt])
  const restarted = createAppStore()
  restarted.getState().restoreCachedSession('a', persisted)
  restarted.getState().dispatchSession('a', snapshotReceived({ ...snapshot, seq: 4, turn: { ...snapshot.turn,
    message: 'firstfinal', display: { truncated: true, items: [{ text: 'first' }, { text: 'final' }] } } }))
  const messages = restarted.getState().sessions.a!.messages
  expect(messages.map(item => item.kind === 'block' ? item.block.id : item.kind === 'answer' ? '' : item.text)).toEqual([
    'question', '[较早的执行过程未同步]', 'first', 'step', 'final',
  ])
  expect(messages[0]).toMatchObject({ messageAt: inputAt })
  expect(messages.at(-1)).toMatchObject({ messageAt: finalAt })
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

test('a legacy snapshot fills an unanswered cached turn before the next observed turn', () => {
  const current = state()
  const question = current.messages[0]
  current.messages = [question,
    { kind: 'input', requestId: 'next', source: 'terminal', status: 'accepted', text: 'next question' },
    { kind: 'claude', requestId: 'next', source: 'terminal', status: 'done', text: 'next reply', error: null },
  ]
  const store = createAppStore()
  store.getState().restoreCachedSession('a', decodeSessionCache(encodeSessionCache('a', current, now), 'a', now)!)
  store.getState().dispatchSession('a', snapshotReceived({ sessionId: 'a', sessionStatus: 'ready', sessionError: null,
    seq: 9, queueDepth: 0, queuedRequests: [], turn: { requestId: 'r', source: 'mobile', status: 'completed',
      message: 'original reply', error: null, pendingAction: null } }))
  expect(store.getState().sessions.a!.messages.map(message => [message.kind, message.requestId])).toEqual([
    ['input', 'r'], ['claude', 'r'], ['input', 'next'], ['claude', 'next'],
  ])
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
test('image references survive restart without persisting image bytes', () => {
  const current = state()
  current.messages = [{ kind: 'input', requestId: 'r', source: 'mobile', text: 'picture', status: 'accepted', attachments: [
    { name: 'photo.png', mediaType: 'image/png', data: 'PRIVATE_IMAGE_BYTES', ref: 'upload-123:0' },
    { name: 'notes.txt', mediaType: 'text/plain', data: 'PRIVATE_FILE_BYTES' },
  ] }]
  const encoded = encodeSessionCache('a', current, now)
  expect(encoded).not.toMatch(/PRIVATE_IMAGE_BYTES|PRIVATE_FILE_BYTES/)
  expect(decodeSessionCache(encoded, 'a', now)!.messages[0]).toMatchObject({
    text: 'picture\n[附件：notes.txt；预览未缓存]', attachments: [{ name: 'photo.png', ref: 'upload-123:0', data: '' }],
  })
})
