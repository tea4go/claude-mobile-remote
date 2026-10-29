const { test, afterEach } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const ts = require('../../mobile/node_modules/typescript')
const root = path.resolve(__dirname, '../entry/src/main/ets')
const modules = new Map()
const sockets = []
class Socket {
  callbacks = {}; frames = []
  on(name, callback) { this.callbacks[name] = callback }
  connect(_url, callback) { callback(null, true) }
  send(frame) { this.frames.push(JSON.parse(frame)); return Promise.resolve(true) }
  close() { return Promise.resolve(true) }
  emit(name, value) { this.callbacks[name]?.(null, value) }
  wire(frame) { this.emit('message', JSON.stringify(frame)) }
}
function load(name) {
  const file = path.resolve(root, name.endsWith('.ets') ? name : name + '.ets')
  if (modules.has(file)) return modules.get(file).exports
  const output = ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file.replace('.ets', '.ts'), reportDiagnostics: true,
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
  assert.equal(output.diagnostics?.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0, file)
  const module = { exports: {} }; modules.set(file, module)
  const requireModule = (id) => {
    if (id === '@kit.NetworkKit') return { webSocket: { createWebSocket() { const socket = new Socket(); sockets.push(socket); return socket } } }
    if (id.startsWith('@kit.')) return {}
    return load(path.relative(root, path.resolve(path.dirname(file), id)))
  }
  vm.runInThisContext(`(function(require,module,exports){${output.outputText}\n})`, { filename: file })(requireModule, module, module.exports)
  return module.exports
}
const fixtures = load('debug/fixtures')
const { decodeServerMessage } = load('protocol/decode')
const { decodePairingPayload } = load('protocol/pairing')
const { createInitialHarmonySessionState, ConnectionStage } = load('session/model')
const reducer = load('session/reducer')
const { encodeCache, decodeCache, CACHE_BYTES, CACHE_AGE } = load('session/cache')
const { validateAttachments } = load('session/attachments')
const { NotificationGate } = load('session/notifications')
const pairing = { version: 1, wsUrl: 'ws://example.test/ws', sessionId: 'session-1', pairToken: 'synthetic-test-only', expiresAt: '2099-01-01T00:00:00Z' }
const attachment = { name: 'demo.png', mediaType: 'image/png', data: 'AQID' }
const stores = []
afterEach(() => { stores.splice(0).forEach(store => store.disconnect()) })
function envelope(type, payload = {}, version = 4, seq = 0) { return { version, type, sessionId: 'session-1', commandId: '', requestId: '', seq, payload } }
function snapshot() { const value = JSON.parse(fixtures.PRETURN_SNAPSHOT_MESSAGE); value.version = 4; return value }
function event(kind, seq, extra = {}) {
  return { seq, kind, sessionId: 'session-1', requestId: 'r1', source: 'mobile', text: '', sessionStatus: '', turnStatus: '', error: null, ...extra }
}
function eventWire(value, version = 4) { return { ...envelope('event', { event: value }, version, value.seq), requestId: value.requestId } }
function readyStore() {
  const store = load('session/store').createSessionStore(); stores.push(store)
  store.connectWithPairing(pairing)
  const socket = sockets.at(-1); socket.emit('open', {})
  socket.wire(envelope('hello_ack')); socket.wire(envelope('subscribed')); socket.wire(snapshot())
  assert.equal(store.getState().connection, ConnectionStage.READY)
  return { store, socket }
}
test('existing self checks remain valid', () => assert.equal(load('debug/selfCheck').runSelfChecks().length, 8))
test('lazy feed publishes streamed text/status changes and keeps attachment bytes out of render keys', () => {
  const { FeedDataSource, feedRenderKey } = load('ui/feedDataSource')
  const source = new FeedDataSource()
  let notifications = 0
  const listener = { onDataReloaded: () => { notifications++ } }
  source.registerDataChangeListener(listener); source.registerDataChangeListener(listener)
  const first = { kind: 'claude', requestId: 'r', source: 'mobile', text: '一', status: 'streaming', error: null }
  source.replace([first])
  const key = feedRenderKey(source.getData(0), 0)
  source.replace([{ ...first, text: '一二', status: 'done' }])
  assert.equal(notifications, 2); assert.equal(source.totalCount(), 1)
  assert.equal(source.getData(0).text, '一二')
  assert.notEqual(feedRenderKey(source.getData(0), 0), key)
  const input = { kind: 'input', source: 'mobile', requestId: 'r', text: '', status: 'accepted', attachments: [{ ...attachment, data: 'synthetic-base64-must-not-enter-key' }] }
  assert.equal(feedRenderKey(input, 0).includes(input.attachments[0].data), false)
  source.unregisterDataChangeListener(listener); source.replace([])
  assert.equal(notifications, 2); assert.equal(source.totalCount(), 0)
})
test('cache pruning matches the latest UTF-8 suffix without repeatedly serializing discarded messages', () => {
  const state = createInitialHarmonySessionState()
  state.messages = Array.from({ length: 200 }, (_, index) => ({ kind: 'claude', requestId: `r${index}`, source: 'mobile',
    text: '中🐳"\\\n'.repeat(7000), status: 'done', error: null }))
  const encoded = encodeCache('多字节会话', state, 100)
  const value = JSON.parse(encoded)
  assert.ok(Buffer.byteLength(encoded) <= CACHE_BYTES)
  assert.equal(value.messages.at(-1).requestId, 'r199')
  assert.ok(value.messages.length > 0 && value.messages.length < 200)
  const missing = 200 - value.messages.length - 1
  value.messages.unshift(state.messages[missing])
  assert.ok(Buffer.byteLength(JSON.stringify(value)) > CACHE_BYTES, 'must keep the largest newest suffix that fits')
  assert.equal(state.messages.length, 200, 'must not mutate the live feed')
})
test('UTF-8 pairing accepts Chinese and emoji in full QR URL', () => {
  const value = { ...pairing, pcName: '张的电脑 🐳' }
  const raw = 'https://example.test/#pair=' + Buffer.from(JSON.stringify(value)).toString('base64url')
  assert.equal(decodePairingPayload(raw).payload.pcName, value.pcName)
  assert.equal(decodePairingPayload('#pair=_w').ok, false)
})
test('pairing rejects expiry and malformed JSON primitives', () => {
  assert.equal(decodePairingPayload(JSON.stringify({ ...pairing, expiresAt: '2020-01-01' })).error.code, 'expired')
  assert.equal(decodePairingPayload('[]').ok, false)
  assert.equal(decodePairingPayload('{"version":1,"wsUrl":7}').ok, false)
})
test('v4 tool results preserve bounded output and reject v3 output', () => {
  const wire = eventWire(event('content_block', 1, { block: { id: 'b', kind: 'tool_result', toolUseId: 't', isError: false, output: '构建完成', truncated: true } }))
  assert.equal(decodeServerMessage(JSON.stringify(wire)).payload.event.block.output, '构建完成')
  wire.version = 3; assert.throws(() => decodeServerMessage(JSON.stringify(wire)))
  wire.version = 4; wire.payload.event.block.output = '中'.repeat(5462)
  assert.throws(() => decodeServerMessage(JSON.stringify(wire)))
})
test('v2/v3/v4 snapshots and v3 display metadata are accepted', () => {
  for (const version of [2, 3, 4]) {
    const wire = snapshot(); wire.version = version
    if (version >= 3) wire.payload.snapshot.projectCwd = '/project'
    assert.equal(decodeServerMessage(JSON.stringify(wire)).version, version)
  }
})
test('client negotiates v6 to v3 to v2 without command replay', () => {
  const client = new (load('protocol/client').ProtocolClient)(pairing)
  client.connect(); sockets.at(-1).emit('open', {})
  assert.equal(sockets.at(-1).frames[0].version, 6)
  sockets.at(-1).wire(envelope('error', { code: 'unsupported_version', message: 'version', expectedVersion: 3 }, 3))
  sockets.at(-1).emit('open', {}); assert.equal(sockets.at(-1).frames[0].version, 3)
  sockets.at(-1).wire(envelope('error', { code: 'unsupported_version', message: 'version', expectedVersion: 2 }, 2))
  sockets.at(-1).emit('open', {}); assert.equal(sockets.at(-1).frames[0].version, 2)
  client.close()
})
test('client rejects another Session event', () => {
  const { store, socket } = readyStore()
  const value = eventWire(event('turn_started', 1, { text: 'hello', turnStatus: 'running' }))
  value.sessionId = value.payload.event.sessionId = 'session-2'
  socket.wire(value); assert.equal(store.getState().connection, ConnectionStage.PROTOCOL_ERROR)
})
test('attachment limits reject oversized, combined, invalid and traversal names', () => {
  assert.equal(validateAttachments([attachment]), '')
  assert.notEqual(validateAttachments([{ ...attachment, name: '../file' }]), '')
  assert.notEqual(validateAttachments(Array(5).fill(attachment)), '')
  const data = Buffer.alloc(4 * 1024 * 1024).toString('base64')
  assert.equal(validateAttachments([{ ...attachment, data }]), '')
  assert.notEqual(validateAttachments(Array(3).fill({ ...attachment, data })), '')
  assert.notEqual(validateAttachments([{ ...attachment, data: '!' }]), '')
})
test('attachment acceptance carries preview and transformed PC echo stays connected', () => {
  const { store, socket } = readyStore(); let accepted
  store.setSendListener(value => { accepted = value })
  assert.equal(store.sendAttachments('看图', [attachment]), true)
  const frame = socket.frames.at(-1)
  assert.equal(frame.payload.kind, 'send_attachments')
  socket.wire({ ...envelope('command_result', { accepted: true, code: '', message: '' }), commandId: frame.commandId, requestId: 'r1' })
  assert.equal(accepted, true)
  socket.wire(eventWire(event('turn_started', store.getState().seqBase + 1, { text: '看图\n[PC 临时附件路径]', turnStatus: 'running' })))
  assert.equal(store.getState().connection, ConnectionStage.READY)
  assert.equal(store.getState().messages[0].attachments[0].name, 'demo.png')
})
test('event before attachment result does not duplicate input', () => {
  const { store, socket } = readyStore()
  store.sendAttachments('', [attachment]); const frame = socket.frames.at(-1)
  socket.wire(eventWire(event('turn_started', store.getState().seqBase + 1, { text: '[PC path]', turnStatus: 'running' })))
  socket.wire({ ...envelope('command_result', { accepted: true, code: '', message: '' }), commandId: frame.commandId, requestId: 'r1' })
  assert.equal(store.getState().messages.filter(message => message.kind === 'input').length, 1)
  assert.equal(store.getState().messages[0].attachments.length, 1)
})
test('rejection and disconnect report failure without resending attachment', () => {
  const { store, socket } = readyStore(); const results = []
  store.setSendListener(value => results.push(value))
  store.sendAttachments('x', [attachment]); const frame = socket.frames.at(-1)
  socket.wire({ ...envelope('command_result', { accepted: false, code: 'unsupported_command', message: 'old server' }), commandId: frame.commandId })
  assert.deepEqual(results, [false])
  store.sendAttachments('x', [attachment]); socket.emit('close', { code: 1006, reason: '' })
  assert.deepEqual(results, [false, false])
  store.reconnect(); assert.equal(sockets.at(-1).frames.length, 0)
})
test('cache strips credentials, bytes and execution state; restores static content', () => {
  const state = createInitialHarmonySessionState()
  state.pairing = pairing; state.sessionId = pairing.sessionId; state.sessionStatus = 'ready'
  state.pendingCommand = { commandId: 'c', kind: 'send_text', text: 'execute me', targetId: '' }
  state.messages = [ { kind: 'input', source: 'mobile', requestId: 'r', text: 'hello', status: 'accepted', attachments: [attachment] },
    { kind: 'claude', source: 'mobile', requestId: 'r', text: '回复', status: 'streaming', error: null } ]
  const encoded = encodeCache(pairing.sessionId, state, 100)
  assert.equal(encoded.includes(pairing.pairToken), false); assert.equal(encoded.includes(attachment.data), false)
  assert.equal(encoded.includes('pendingCommand'), false)
  const restored = decodeCache(encoded, pairing.sessionId, 101)
  assert.equal(restored.messages[1].status, 'done'); assert.equal(restored.canSend, false)
  assert.equal(restored.pendingCommand, null); assert.equal(restored.pairing, null)
})
test('cache rejects wrong Session, corrupt, future, expired and invalid nested messages', () => {
  const state = createInitialHarmonySessionState(); const encoded = encodeCache('s', state, 100)
  assert.equal(decodeCache(encoded, 'other', 101), null)
  assert.equal(decodeCache('broken', 's', 101), null)
  assert.equal(decodeCache(encoded, 's', 99), null)
  assert.equal(decodeCache(encoded, 's', 101 + CACHE_AGE), null)
  const value = JSON.parse(encoded); value.messages = [{ kind: 'input', requestId: 'r', text: 1 }]
  assert.equal(decodeCache(JSON.stringify(value), 's', 101).messages.length, 0)
})
test('cache keeps latest 200 and respects UTF-8 byte budget', () => {
  const state = createInitialHarmonySessionState()
  state.messages = Array.from({ length: 220 }, (_, index) => ({ kind: 'claude', requestId: `r${index}`, source: 'mobile', text: '中'.repeat(5000), status: 'done', error: null }))
  const encoded = encodeCache('s', state, 100)
  assert.ok(Buffer.byteLength(encoded) <= CACHE_BYTES)
  const value = decodeCache(encoded, 's', 101)
  assert.ok(value.messages.length <= 200); assert.equal(value.messages.at(-1).requestId, 'r219')
})
test('reconnect preserves cached prior turns and snapshot replaces current request', () => {
  let state = createInitialHarmonySessionState(); state.sessionId = pairing.sessionId
  state.messages = [{ kind: 'claude', requestId: 'old', source: 'mobile', text: '旧轮', status: 'done', error: null },
    { kind: 'claude', requestId: 'r1', source: 'mobile', text: '旧快照', status: 'done', error: null }]
  state = reducer.reduceHarmonySessionState(state, reducer.startConnect(pairing))
  assert.equal(state.messages.length, 2)
  const value = decodeServerMessage(JSON.stringify(snapshot())).payload.snapshot
  value.turn = { status: 'running', requestId: 'r1', source: 'mobile', message: '新快照', error: null, pendingAction: null }
  state = reducer.reduceHarmonySessionState(state, reducer.snapshotReceived(value))
  assert.equal(state.messages.length, 2); assert.equal(state.messages[1].text, '新快照')
})
test('notifications are private, deduplicated and foreground Session suppressed', () => {
  const gate = new NotificationGate(); gate.seed('session-1', 5)
  assert.equal(gate.accept(event('turn_completed', 5), true, false, ''), '')
  assert.equal(gate.accept(event('turn_completed', 6), true, true, 'session-1'), '')
  assert.equal(gate.accept(event('turn_completed', 6), true, false, ''), '')
  assert.equal(gate.accept(event('turn_completed', 7, { text: 'private message' }), true, false, ''), '本轮已完成')
  assert.equal(gate.accept(event('turn_failed', 8), false, false, ''), '')
})
test('Markdown preserves code, tables, headings and safe links without executing HTML', () => {
  const { markdownBlocks, inlineMarkdown } = load('session/markdown')
  const blocks = markdownBlocks('# Title\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n```ts\nconst x = 1\n```\n- item')
  assert.deepEqual(blocks.map(item => item.kind), ['heading', 'table', 'code', 'text'])
  assert.equal(blocks[1].rows[1][1], '2'); assert.equal(blocks[2].text, 'const x = 1')
  assert.equal(inlineMarkdown('[unsafe](javascript:alert(1))').some(item => item.style === 'link'), false)
  assert.equal(inlineMarkdown('[docs](https://example.test)').at(-1).url, 'https://example.test')
  assert.equal(inlineMarkdown('<script>bad</script>')[0].text, '<script>bad</script>')
})
test('pairing rejects nonstring ids and repairs legacy UTF-8 machine names', () => {
  assert.equal(decodePairingPayload(JSON.stringify({ ...pairing, sessionId: 12 })).ok, false)
  const pcName = Buffer.from('我的电脑').toString('latin1')
  assert.equal(decodePairingPayload(JSON.stringify({ ...pairing, pcName })).payload.pcName, '我的电脑')
})
test('manual disconnect releases pending send without automatic replay', () => {
  const { store } = readyStore(); let result = null
  store.setSendListener(accepted => { result = accepted })
  store.sendText('pending'); store.disconnect()
  assert.equal(result, false); assert.equal(store.getState().pendingCommand, null)
})
test('late send failure from an old socket cannot close a reconnected Session', async () => {
  const { store, socket } = readyStore(); let reject
  socket.send = () => new Promise((_resolve, fail) => { reject = fail })
  store.sendText('old send'); store.reconnect()
  const current = sockets.at(-1); current.emit('open', {})
  current.wire(envelope('hello_ack')); current.wire(envelope('subscribed')); current.wire(snapshot())
  reject(new Error('old socket')); await new Promise(resolve => setImmediate(resolve))
  assert.equal(store.getState().connection, ConnectionStage.READY)
})
test('cached blocks and answers remain display-only and validate against wire schema', () => {
  const state = createInitialHarmonySessionState()
  state.messages = [{ kind: 'block', requestId: 'r', block: { id: 'b', kind: 'tool_result', toolUseId: 't', isError: false, output: 'safe output', truncated: true } },
    { kind: 'answer', requestId: 'r', answers: [{ questionId: 'q', header: '', question: 'pick', values: ['yes'] }] }]
  const restored = decodeCache(encodeCache('s', state, 100), 's', 101)
  assert.equal(restored.messages.length, 2)
  assert.equal(restored.messages[0].block.output, 'safe output')
  assert.equal(restored.pendingAction, null)
})

test('a second command cannot replace an unconfirmed attachment send', () => {
  const { store, socket } = readyStore(); const results = []
  store.setSendListener(value => results.push(value))
  store.sendAttachments('image', [attachment]); const frame = socket.frames.at(-1)
  assert.equal(store.cancelTurn('r1'), false)
  assert.equal(store.getState().pendingCommand.commandId, frame.commandId)
  socket.wire({ ...envelope('command_result', { accepted: true, code: '', message: '' }), commandId: frame.commandId, requestId: 'r1' })
  assert.deepEqual(results, [true]); assert.equal(store.getState().messages[0].attachments.length, 1)
})
test('sequence failure unlocks the pending draft without replay', () => {
  const { store, socket } = readyStore(); const results = []
  store.setSendListener(value => results.push(value)); store.sendText('keep draft')
  socket.wire(eventWire(event('turn_started', store.getState().seqBase + 2, { text: 'keep draft', turnStatus: 'running' })))
  assert.equal(store.getState().connection, ConnectionStage.PROTOCOL_ERROR)
  assert.equal(store.getState().pendingCommand, null); assert.deepEqual(results, [false])
  store.reconnect(); assert.equal(sockets.at(-1).frames.length, 0)
})
test('explicit disconnect releases management busy state and does not replay it', () => {
  const { store } = readyStore(); const results = []
  store.setManagementListener(value => results.push(value))
  assert.equal(store.createSession(), true); store.disconnect()
  assert.equal(results.length, 1); assert.equal(results[0].code, 'result_unknown')
  store.reconnect(); const socket = sockets.at(-1); socket.emit('open', {})
  socket.wire(envelope('hello_ack')); socket.wire(envelope('subscribed')); socket.wire(snapshot())
  assert.equal(socket.frames.some(frame => frame.type === 'create_session'), false)
  assert.equal(store.createSession(), true)
})
test('permission mode snapshots and changes use existing contracts across all versions', () => {
  for (const version of [2, 3, 4]) {
    const wire = snapshot(); wire.version = version; wire.payload.snapshot.permissionMode = 'plan'
    assert.equal(decodeServerMessage(JSON.stringify(wire)).payload.snapshot.permissionMode, 'plan')
    wire.payload.snapshot.permissionMode = 'invalid'
    assert.throws(() => decodeServerMessage(JSON.stringify(wire)))
  }
  const { store, socket } = readyStore()
  assert.equal(store.getState().permissionMode, 'default')
  assert.equal(store.setPermissionMode('acceptEdits'), true)
  const frame = socket.frames.at(-1)
  assert.equal(frame.payload.kind, 'set_permission_mode'); assert.equal(frame.payload.permissionMode, 'acceptEdits')
  assert.equal(store.getState().permissionMode, 'default')
  socket.wire(eventWire(event('permission_mode_changed', store.getState().seqBase + 1, { requestId: '', permissionMode: 'acceptEdits', turnStatus: 'idle' })))
  socket.wire({ ...envelope('command_result', { accepted: true, code: '', message: '' }), commandId: frame.commandId, requestId: pairing.sessionId })
  assert.equal(store.getState().connection, ConnectionStage.READY); assert.equal(store.getState().permissionMode, 'acceptEdits')
  assert.equal(store.getState().pendingCommand, null)
  assert.equal(decodeCache(encodeCache(pairing.sessionId, store.getState(), 100), pairing.sessionId, 101).permissionMode, 'default')
})
test('permission mode rejection preserves current mode and active approval', () => {
  const { store, socket } = readyStore()
  const action = { actionId: 'a', requestId: 'r1', kind: 'tool_approval', toolName: 'Bash', preview: '', allowSessionAvailable: true }
  socket.wire(eventWire(event('action_required', store.getState().seqBase + 1, { pendingAction: action, turnStatus: 'waiting' })))
  assert.equal(store.setPermissionMode('plan'), true); const frame = socket.frames.at(-1)
  socket.wire({ ...envelope('command_result', { accepted: false, code: 'rejected', message: '' }), commandId: frame.commandId })
  assert.equal(store.getState().permissionMode, 'default'); assert.equal(store.getState().pendingAction.actionId, 'a')
  assert.throws(() => decodeServerMessage(JSON.stringify(eventWire(event('permission_mode_changed', 1)))))
})

test('cleared display cache is not recreated by connection-only changes', () => {
  const gate = new (load('session/cache').CacheWriteGate)()
  const { store } = readyStore()
  const initial = encodeCache(pairing.sessionId, store.getState(), 100)
  assert.equal(gate.changed(pairing.sessionId, initial), true)
  // Clearing records the current projection, as AppShell does before removing files.
  gate.changed(pairing.sessionId, initial)
  store.disconnect()
  assert.equal(gate.changed(pairing.sessionId, encodeCache(pairing.sessionId, store.getState(), 101)), false)
  const changed = { ...store.getState(), sessionTitle: 'New visible content' }
  assert.equal(gate.changed(pairing.sessionId, encodeCache(pairing.sessionId, changed, 102)), true)
})
test('reconnect snapshot preserves cancelled turn status', () => {
  const value = decodeServerMessage(JSON.stringify(snapshot())).payload.snapshot
  value.turn = { status: 'cancelled', requestId: 'r1', source: 'mobile', message: 'partial', error: null, pendingAction: null }
  const state = reducer.reduceHarmonySessionState(createInitialHarmonySessionState(), reducer.snapshotReceived(value))
  assert.equal(state.messages[0].status, 'cancelled')
})

const { debugLog } = load('debug/log')
function decodeObject(value) { return decodeServerMessage(JSON.stringify(value)) }
test('v5/v6 progress, diagnostic and pong decode with strict metadata', () => {
  const progress = { phase: 'thinking', startedAt: 1000, updatedAt: 2000, outputTokens: 5, tokensKnown: true, tokensEstimated: false }
  for (const version of [5, 6]) {
    assert.deepEqual(decodeObject(eventWire(event('turn_progress', 1, { progress }), version)).payload.event.progress, progress)
    assert.equal(decodeObject(eventWire(event('session_diagnostic', 2, { diagnostic: { code: 'api_retry', httpStatus: 429 } }), version)).payload.event.diagnostic.code, 'api_retry')
  }
  assert.throws(() => decodeObject(eventWire(event('turn_progress', 1, { progress }), 4)))
  assert.throws(() => decodeObject(eventWire(event('turn_progress', 1, { progress: { ...progress, updatedAt: 0 } }), 6)))
  assert.throws(() => decodeObject(eventWire(event('session_diagnostic', 1, { diagnostic: { code: 'api_retry', attempt: 1000001 } }), 6)))
  assert.equal(decodeObject(envelope('pong', {}, 6)).type, 'pong')
  assert.throws(() => decodeObject(envelope('pong', {}, 5)))
  assert.throws(() => decodeObject({ toString() { return '' } }))
})
function readyV6() {
  const store = load('session/store').createSessionStore(); stores.push(store)
  store.connectWithPairing(pairing)
  const socket = sockets.at(-1); socket.emit('open', {})
  socket.wire(envelope('hello_ack', {}, 6)); socket.wire(envelope('subscribed', {}, 6))
  const snap = snapshot(); snap.version = 6; socket.wire(snap)
  return { store, socket }
}
test('heartbeat timeout marks disconnected without sending a user command', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 100000 })
  const { store, socket } = readyV6()
  t.mock.timers.tick(10000)
  assert.equal(socket.frames.at(-1).type, 'ping')
  t.mock.timers.tick(5000)
  assert.equal(store.getState().connection, ConnectionStage.DISCONNECTED)
  assert.equal(store.getState().lastError.code, 'heartbeat_timeout')
  assert.equal(socket.frames.some(frame => frame.type === 'command'), false)
  store.disconnect()
})
test('pong keeps ready, wrong session pong disconnects and close cancels probes', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 100000 })
  const { store, socket } = readyV6()
  t.mock.timers.tick(10000); socket.wire(envelope('pong', {}, 6))
  assert.equal(store.getState().connection, ConnectionStage.READY)
  t.mock.timers.tick(10000); socket.wire({ ...envelope('pong', {}, 6), sessionId: 'other' })
  assert.equal(store.getState().connection, ConnectionStage.PROTOCOL_ERROR)
  const count = socket.frames.length; t.mock.timers.tick(30000); assert.equal(socket.frames.length, count)
  store.disconnect()
})
test('connection phase deadline fails before pairing is accepted', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 100000 })
  const store = load('session/store').createSessionStore(); stores.push(store)
  store.connectWithPairing(pairing); t.mock.timers.tick(10000)
  assert.equal(store.getState().connection, ConnectionStage.DISCONNECTED)
  assert.equal(store.getState().lastError.code, 'connect_timeout')
  store.disconnect()
})
test('progress updates and clears with turn lifecycle without extra feed messages', () => {
  const { store, socket } = readyV6()
  socket.wire(eventWire(event('turn_started', 1, { text: 'demo', turnStatus: 'running' }), 6))
  socket.wire(eventWire(event('turn_progress', 2, { progress: { phase: 'working', startedAt: 1000, updatedAt: 1000, outputTokens: 0, tokensKnown: false, tokensEstimated: false } }), 6))
  assert.equal(store.getState().progress.phase, 'working')
  const count = store.getState().messages.length
  socket.wire(eventWire(event('session_diagnostic', 3, { diagnostic: { code: 'receive_failed' } }), 6))
  assert.equal(store.getState().messages.length, count)
  assert.ok(debugLog.text().includes('receive_failed'))
  socket.wire(eventWire(event('turn_completed', 4, { turnStatus: 'completed' }), 6))
  assert.equal(store.getState().progress, undefined)
})
test('logs bounded, disabled clears, and target strips secret paths and userinfo', () => {
  debugLog.setEnabled(true); debugLog.clear()
  for (let i = 0; i < 210; i++) debugLog.record('ping', i)
  assert.equal(debugLog.list().length, 200)
  assert.equal(debugLog.list()[0].detail, 'value=209')
  assert.equal(debugLog.target('wss://example.test:443/private?token=secret'), 'wss://example.test:443')
  assert.equal(debugLog.target('wss://user:secret@example.test/ws'), '目标地址不可显示')
  debugLog.setEnabled(false); debugLog.record('ready'); assert.equal(debugLog.list().length, 0)
  debugLog.setEnabled(true)
})
test('offline demo streams, cancels and resets without pairing or persistence', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 100000 })
  const before = sockets.length
  const demo = new (load('session/demo').DemoController)(() => {})
  assert.equal(demo.state.pairing, null)
  demo.send('hello'); const id = demo.state.messages[0].requestId
  t.mock.timers.tick(500); assert.ok(demo.state.messages[1].text.length > 0)
  assert.equal(demo.cancel(id), true); t.mock.timers.tick(5000)
  assert.equal(demo.state.messages[1].status, 'cancelled')
  demo.send('next'); demo.reset(); t.mock.timers.tick(5000)
  assert.equal(demo.state.messages.length, 0); assert.equal(sockets.length, before)
  demo.dispose()
})
test('offline approvals allow/deny, question and dispose ignore late callbacks', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 100000 })
  const demo = new (load('session/demo').DemoController)(() => {})
  for (const decision of ['allow_once', 'deny']) {
    demo.reset(); demo.send('演示审批'); t.mock.timers.tick(500)
    assert.equal(demo.state.pendingAction.kind, 'tool_approval')
    assert.equal(demo.resolve({ mode: 'decision', actionId: 'wrong', decision }), false)
    demo.resolve({ mode: 'decision', actionId: demo.state.pendingAction.actionId, decision })
    for (let i = 0; i < 40; i++) t.mock.timers.tick(80)
    assert.equal(demo.state.canSend, true)
    assert.ok(demo.state.messages[1].text.includes(decision === 'deny' ? '拒绝' : '允许'))
  }
  demo.reset(); demo.send('演示问答'); t.mock.timers.tick(500)
  demo.resolve({ mode: 'answers', actionId: demo.state.pendingAction.actionId, answers: [{ questionId: 'demo-question', values: ['中文'] }] })
  const text = demo.state.messages[1].text; demo.dispose(); t.mock.timers.tick(5000)
  assert.equal(demo.state.messages[1].text, text)
})
test('v6 negotiates v5/v4 and retains progress only on capable versions', () => {
  for (const version of [5, 4]) {
    const store = load('session/store').createSessionStore(); stores.push(store)
    store.connectWithPairing(pairing); sockets.at(-1).emit('open', {})
    sockets.at(-1).wire(envelope('error', { code: 'unsupported_version', message: 'version', expectedVersion: version }, version))
    const socket = sockets.at(-1); socket.emit('open', {})
    assert.equal(socket.frames[0].version, version)
    socket.wire(envelope('hello_ack', {}, version)); socket.wire(envelope('subscribed', {}, version))
    const snap = snapshot(); snap.version = version; socket.wire(snap)
    assert.equal(store.getState().connection, ConnectionStage.READY)
    store.disconnect()
  }
})
test('network loss marks operation unknown, releases management and ignores old events', () => {
  const { store, socket } = readyV6()
  let result
  store.setManagementListener(value => { result = value })
  assert.equal(store.createSession(), true)
  store.networkLost()
  assert.equal(store.getState().connection, ConnectionStage.DISCONNECTED)
  assert.equal(result.code, 'result_unknown')
  socket.wire(eventWire(event('turn_started', 1, { text: 'late', turnStatus: 'running' }), 6))
  assert.equal(store.getState().messages.length, 0)
})
test('authentication, subscription and snapshot deadlines are distinct and cleaned on close', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 100000 })
  for (const phase of ['auth_timeout', 'subscribe_timeout', 'snapshot_timeout']) {
    const store = load('session/store').createSessionStore(); stores.push(store)
    store.connectWithPairing(pairing); const socket = sockets.at(-1); socket.emit('open', {})
    if (phase !== 'auth_timeout') socket.wire(envelope('hello_ack', {}, 6))
    if (phase === 'snapshot_timeout') socket.wire(envelope('subscribed', {}, 6))
    t.mock.timers.tick(phase === 'snapshot_timeout' ? 10000 : 8000)
    assert.equal(store.getState().lastError.code, phase)
    store.disconnect()
  }
})
