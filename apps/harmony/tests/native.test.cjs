// Test native orchestration using Node adapters. Device APIs still require DevEco/device validation.
const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const vm = require('node:vm')
const crypto = require('node:crypto')
const { promisify } = require('node:util')
const ts = require('../../mobile/node_modules/typescript')
const root = path.resolve(__dirname, '../entry/src/main/ets')
const keys = new Map(), handles = new Map(); let handle = 0
const tags = { HUKS_TAG_ALGORITHM: 1, HUKS_TAG_KEY_SIZE: 2, HUKS_TAG_PURPOSE: 3, HUKS_TAG_PADDING: 4, HUKS_TAG_BLOCK_MODE: 5, HUKS_TAG_NONCE: 6, HUKS_TAG_ASSOCIATED_DATA: 7, HUKS_TAG_AE_TAG: 8 }
const huks = {
  HuksTag: tags, HuksKeyAlg: { HUKS_ALG_AES: 1 }, HuksKeySize: { HUKS_AES_KEY_SIZE_256: 256 },
  HuksKeyPurpose: { HUKS_KEY_PURPOSE_ENCRYPT: 1, HUKS_KEY_PURPOSE_DECRYPT: 2 }, HuksKeyPadding: { HUKS_PADDING_NONE: 0 }, HuksCipherMode: { HUKS_MODE_GCM: 1 },
  isKeyItemExist: async alias => keys.has(alias), generateKeyItem: async alias => { keys.set(alias, crypto.randomBytes(32)) },
  initSession: async (alias, options) => { const id = ++handle; handles.set(id, { alias, options }); return { handle: id } },
  finishSession: async (id, options) => {
    const key = keys.get(handles.get(id).alias); handles.delete(id)
    const get = tag => options.properties.find(item => item.tag === tag)?.value
    const encrypt = get(tags.HUKS_TAG_PURPOSE) === 1
    const operation = encrypt ? crypto.createCipheriv('aes-256-gcm', key, get(tags.HUKS_TAG_NONCE)) : crypto.createDecipheriv('aes-256-gcm', key, get(tags.HUKS_TAG_NONCE))
    operation.setAAD(get(tags.HUKS_TAG_ASSOCIATED_DATA))
    if (!encrypt) operation.setAuthTag(get(tags.HUKS_TAG_AE_TAG))
    const bytes = Buffer.concat([operation.update(options.inData), operation.final(), ...(encrypt ? [operation.getAuthTag()] : [])])
    return { outData: new Uint8Array(bytes) }
  }, abortSession: async id => { handles.delete(id) },
}
const fileIo = {
  access: async file => { try { await fs.promises.access(file); return true } catch (error) { if (error.code === 'ENOENT') return false; throw error } },
  mkdir: fs.promises.mkdir, listFile: fs.promises.readdir,
  async stat(value) { const stat = typeof value === 'number' ? await promisify(fs.fstat)(value) : await fs.promises.stat(value); return { size: stat.size, mtime: stat.mtimeMs / 1000 } },
  readText: file => fs.promises.readFile(file, 'utf8'),
  OpenMode: { CREATE: fs.constants.O_CREAT, WRITE_ONLY: fs.constants.O_WRONLY, TRUNC: fs.constants.O_TRUNC },
  open: async (file, mode) => ({ fd: await promisify(fs.open)(file, mode) }),
  write: async (fd, data) => (await promisify(fs.write)(fd, Buffer.from(data))).bytesWritten,
  fsync: promisify(fs.fsync), close: file => promisify(fs.close)(file.fd), rename: fs.promises.rename, unlink: fs.promises.unlink,
}
const util = {
  TextEncoder: class { encodeInto(text) { return new TextEncoder().encode(text) } },
  TextDecoder: { create: () => ({ decodeWithStream: bytes => new TextDecoder('utf-8', { fatal: true }).decode(bytes) }) },
  Base64Helper: class { encodeToStringSync(bytes) { return Buffer.from(bytes).toString('base64') } decodeSync(text) { return new Uint8Array(Buffer.from(text, 'base64')) } },
}
const cryptoFramework = {
  createRandom: () => ({ generateRandomSync: count => ({ data: new Uint8Array(crypto.randomBytes(count)) }) }),
  createMd: () => { const hash = crypto.createHash('sha256'); return { update: async blob => { hash.update(blob.data) }, digest: async () => ({ data: new Uint8Array(hash.digest()) }) } },
}
function loader(overrides = {}) {
  const modules = new Map()
  function load(name) {
    const file = path.resolve(root, name.endsWith('.ets') ? name : name + '.ets')
    if (modules.has(file)) return modules.get(file).exports
    const output = ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file.replace('.ets', '.ts'), compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
    const module = { exports: {} }; modules.set(file, module)
    const kits = { '@kit.CoreFileKit': { fileIo }, '@kit.ArkTS': { util }, '@kit.ArkUI': { FrameCallback: class {} }, '@kit.CryptoArchitectureKit': { cryptoFramework }, '@kit.UniversalKeystoreKit': { huks }, ...overrides }
    vm.runInThisContext(`(function(require,module,exports){${output.outputText}\n})`, { filename: file })(id => {
      if (id === '@ohos.fileio') return overrides[id] ?? { chmod: fs.promises.chmod }
      if (id.startsWith('@kit.')) return kits[id] ?? {}
      return load(path.relative(root, path.resolve(path.dirname(file), id)))
    }, module, module.exports)
    return module.exports
  }
  return load
}
const pairing = { version: 1, wsUrl: 'ws://example.test/ws', sessionId: '测试/session', pairToken: 'synthetic-credential-only', expiresAt: '2099-01-01T00:00:00Z' }
test('native/page work starts only after the feedback frame and cannot run twice', () => {
  const { FeedbackFrame } = loader()('ui/feedbackFrame')
  let calls = 0
  const frame = new FeedbackFrame(() => { calls++ })
  assert.equal(calls, 0)
  frame.onFrame(0); assert.equal(calls, 0)
  frame.onIdle(0); frame.onIdle(0); assert.equal(calls, 1)
})
async function storageTest(run) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cmr-native-'))
  const load = loader(); const { NativeStorage } = load('native/storage')
  try { await run(new NativeStorage({ filesDir: dir }), dir, load) } finally { fs.rmSync(dir, { recursive: true, force: true }) }
}
test('pairing persists encrypted token and UTF-8 metadata; reload works', async () => storageTest(async (store, dir, load) => {
  const record = load('session/records').createSessionRecord({ ...pairing, pcName: '中文机器' }); record.name = '本地名称'
  await store.saveRecords([record])
  assert.equal(fs.readFileSync(path.join(dir, 'cmr/records.json'), 'utf8').includes(pairing.pairToken), false)
  const vault = path.join(dir, 'cmr/pairing'); const files = fs.readdirSync(vault)
  assert.match(files[0], /^[a-f0-9]{64}\.json$/)
  const encrypted = fs.readFileSync(path.join(vault, files[0]), 'utf8')
  assert.equal(encrypted.includes(pairing.pairToken), false)
  const records = await store.loadRecords()
  assert.equal(records[0].pairToken, pairing.pairToken); assert.equal(records[0].pcName, '中文机器'); assert.equal(records[0].name, '本地名称')
  assert.equal(record.pairToken, pairing.pairToken)
}))
test('tampered token fails closed while retaining list entry for rescan', async () => storageTest(async (store, dir, load) => {
  await store.saveRecords([load('session/records').createSessionRecord(pairing)])
  const vault = path.join(dir, 'cmr/pairing'); const file = path.join(vault, fs.readdirSync(vault)[0])
  const envelope = JSON.parse(fs.readFileSync(file, 'utf8')); envelope.data = Buffer.alloc(40).toString('base64'); fs.writeFileSync(file, JSON.stringify(envelope))
  const records = await store.loadRecords()
  assert.equal(records[0].pairToken, ''); assert.equal(records[0].status, 'needsPairing')
}))
test('one malformed saved record cannot block other valid pairings', async () => storageTest(async (store, dir, load) => {
  await store.saveRecords([load('session/records').createSessionRecord(pairing)])
  const file = path.join(dir, 'cmr/records.json'); const records = JSON.parse(fs.readFileSync(file, 'utf8'))
  records.unshift({ ...records[0], sessionId: 'malformed', capabilities: 7 }); fs.writeFileSync(file, JSON.stringify(records))
  const restored = await store.loadRecords()
  assert.equal(restored.length, 1); assert.equal(restored[0].sessionId, pairing.sessionId)
}))
test('delete pairing removes secure token and cannot resurrect after queued writes', async () => storageTest(async (store, dir, load) => {
  const record = load('session/records').createSessionRecord(pairing)
  const saving = store.saveRecords([record]); const removing = store.saveRecords([])
  await Promise.all([saving, removing]); assert.deepEqual(await store.loadRecords(), [])
  assert.deepEqual(fs.readdirSync(path.join(dir, 'cmr/pairing')), [])
}))
test('cache file read/write/delete/clear serial order and expired data', async () => storageTest(async (store, dir, load) => {
  const { encodeCache } = load('session/cache'), { createInitialHarmonySessionState } = load('session/model')
  const state = createInitialHarmonySessionState(); state.messages = [{ kind: 'claude', source: 'mobile', requestId: 'r', text: '中文回复', status: 'done', error: null }]
  await store.writeCache('s', encodeCache('s', state, Date.now()))
  assert.equal((await store.readCache('s')).messages[0].text, '中文回复')
  assert.equal(await store.readCache('other'), null)
  await Promise.all([store.writeCache('s', encodeCache('s', state, Date.now())), store.removeCache('s')])
  assert.equal(await store.readCache('s'), null)
  await Promise.all([store.writeCache('s', encodeCache('s', state, Date.now())), store.clearCache()])
  assert.equal(await store.readCache('s'), null)
  await assert.rejects(store.writeCache('s', encodeCache('s', state, 1)))
}))
test('cache total quota evicts oldest files and corrupted cache cannot load', async () => storageTest(async (store, dir, load) => {
  const { encodeCache, CACHE_TOTAL } = load('session/cache'), { createInitialHarmonySessionState } = load('session/model')
  const state = createInitialHarmonySessionState(); state.messages = [{ kind: 'claude', source: 'mobile', requestId: 'r', text: 'x'.repeat(1800000), status: 'done', error: null }]
  for (let i = 0; i < 13; i++) await store.writeCache(`s${i}`, encodeCache(`s${i}`, state, Date.now()))
  const dirpath = path.join(dir, 'cmr/display')
  assert.ok(fs.readdirSync(dirpath).reduce((sum, name) => sum + fs.statSync(path.join(dirpath, name)).size, 0) <= CACHE_TOTAL)
  assert.ok(await store.readCache('s12'))
  const name = crypto.createHash('sha256').update('s12').digest('hex') + '.json'
  fs.writeFileSync(path.join(dirpath, name), '{invalid')
  assert.equal(await store.readCache('s12'), null)
}))
test('pending disk flush yields to UI timers and flags retain serial read-after-write order', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cmr-async-storage-'))
  let release, entered
  const flushing = new Promise(resolve => { entered = resolve })
  const gate = new Promise(resolve => { release = resolve })
  const load = loader({ '@kit.CoreFileKit': { fileIo: { ...fileIo, fsync: async fd => { entered(); await gate; await fileIo.fsync(fd) } } } })
  const { NativeStorage } = load('native/storage')
  const store = new NativeStorage({ filesDir: dir })
  try {
    const saving = store.setNotificationsEnabled(true)
    await flushing
    await new Promise(resolve => setTimeout(resolve, 0))
    assert.equal(fs.existsSync(path.join(dir, 'cmr/notifications')), false, 'unflushed bytes must not replace the live file')
    const reading = store.readNotificationsEnabled()
    release(); await saving
    assert.equal(await reading, true)
    await Promise.all([store.setPushConsent(true), store.setPushConsent(false)])
    assert.equal(await store.readPushConsent(), false)
    assert.equal(fs.statSync(path.join(dir, 'cmr')).mode & 0o777, 0o700)
    assert.equal(fs.statSync(path.join(dir, 'cmr/notifications')).mode & 0o777, 0o600)
  } finally { release(); fs.rmSync(dir, { recursive: true, force: true }) }
})
test('short writes complete; a failed write preserves the prior value and releases the queue', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cmr-storage-failure-'))
  let fail = false, writes = 0
  const load = loader({ '@kit.CoreFileKit': { fileIo: { ...fileIo, write: async (fd, data) => {
    if (fail) throw new Error('synthetic write failure')
    writes++
    return fileIo.write(fd, Buffer.from(data).subarray(0, 3))
  } } } })
  const store = new (load('native/storage').NativeStorage)({ filesDir: dir })
  try {
    await store.setNotificationsEnabled(true)
    assert.ok(writes > 1)
    fail = true
    await assert.rejects(store.setNotificationsEnabled(false), /synthetic write failure/)
    assert.equal(await store.readNotificationsEnabled(), true)
    fail = false
    await store.setNotificationsEnabled(false)
    assert.equal(await store.readNotificationsEnabled(), false)
  } finally { fs.rmSync(dir, { recursive: true, force: true }) }
})
test('failed private-directory initialization cannot be bypassed by later queued operations', async () => {
  const load = loader({ '@kit.CoreFileKit': { fileIo: { ...fileIo, access: async () => { throw new Error('synthetic permission failure') } } } })
  const store = new (load('native/storage').NativeStorage)({ filesDir: '/unused-synthetic' })
  await assert.rejects(store.readNotificationsEnabled(), /storage initialization failed/)
  await assert.rejects(store.setNotificationsEnabled(true), /storage initialization failed/)
})
test('cold notification route is retained until consumed and lifecycle delivered', () => {
  const { nativeLifecycle } = loader()('native/lifecycle'); const calls = []
  nativeLifecycle.attach({}, { parameters: { cmrSessionId: 's1' } }); assert.equal(nativeLifecycle.pendingSessionId, 's1')
  const remove = nativeLifecycle.subscribe(() => calls.push(nativeLifecycle.foreground))
  nativeLifecycle.active(false); nativeLifecycle.active(true); remove(); nativeLifecycle.active(false)
  assert.deepEqual(calls, [false, true]); assert.equal(nativeLifecycle.pendingSessionId, 's1')
})
function speechEnvironment(permission = 0) {
  let callbacks, mic, engine; const calls = []
  const load = loader({
    '@kit.AbilityKit': { abilityAccessCtrl: { createAtManager: () => ({ requestPermissionsFromUser: async () => ({ authResults: [permission] }) }) } },
    '@kit.CoreSpeechKit': { speechRecognizer: { createEngine: async () => engine = {
      setListener(value) { callbacks = value }, startListening() { calls.push('start') }, writeAudio() { calls.push('audio') }, finish() { calls.push('finish') }, cancel() { calls.push('cancel') }, shutdown() { calls.push('shutdown') },
    } } },
    '@kit.AudioKit': { audio: { AudioSamplingRate: {}, AudioChannel: {}, AudioSampleFormat: {}, AudioEncodingType: {}, SourceType: {}, createAudioCapturer: async () => mic = {
      on() {}, start: async () => calls.push('mic-start'), stop: async () => calls.push('mic-stop'), release: async () => calls.push('mic-release'),
    } } },
  })
  return { SpeechDraft: load('native/speech').SpeechDraft, calls, listener: () => callbacks }
}
test('speech produces draft only; cancellation releases microphone and ignores late result', async () => {
  const env = speechEnvironment(); const speech = new env.SpeechDraft(); const values = []
  await speech.start({}, (...value) => values.push(value))
  env.listener().onResult('x', { result: '测试', isLast: false })
  assert.equal(values.at(-1)[0], '测试')
  await speech.cancel(); const count = values.length
  env.listener().onResult('x', { result: 'late', isLast: false })
  assert.equal(values.length, count); assert.ok(env.calls.includes('mic-release')); assert.ok(env.calls.includes('shutdown'))
})
test('speech denied permission reports failure and never starts microphone', async () => {
  const env = speechEnvironment(-1); const speech = new env.SpeechDraft(); const values = []
  await speech.start({}, (...value) => values.push(value))
  assert.equal(values.at(-1)[1], false); assert.notEqual(values.at(-1)[2], ''); assert.equal(env.calls.length, 0)
})
test('notification removal during pending WantAgent creation prevents late publication', async () => {
  let resolveAgent; const published = []
  const load = loader({
    '@kit.AbilityKit': { wantAgent: { OperationType: { START_ABILITY: 1 }, WantAgentFlags: { UPDATE_PRESENT_FLAG: 1 }, getWantAgent: () => new Promise(resolve => { resolveAgent = resolve }) } },
    '@kit.NotificationKit': { notificationManager: { SlotType: {}, ContentType: {}, publish: async value => published.push(value), cancel: async () => {} } },
  })
  const notifier = new (load('native/notifications').LocalNotifications)({ abilityInfo: { bundleName: 'test', name: 'EntryAbility' } })
  notifier.enabled = true
  const sending = notifier.event({ kind: 'turn_completed', sessionId: 's', seq: 1 })
  await Promise.resolve()
  await notifier.remove('s'); resolveAgent({}); await sending
  assert.equal(published.length, 0)
})

test('failed notification cancellation still prevents late publication', async () => {
  let resolveAgent; const published = []
  const load = loader({
    '@kit.AbilityKit': { wantAgent: { OperationType: {}, WantAgentFlags: {}, getWantAgent: () => new Promise(resolve => { resolveAgent = resolve }) } },
    '@kit.NotificationKit': { notificationManager: { SlotType: {}, ContentType: {}, publish: async value => published.push(value), cancel: async () => { throw new Error('platform unavailable') } } },
  })
  const notifier = new (load('native/notifications').LocalNotifications)({ abilityInfo: {} }); notifier.enabled = true
  const sending = notifier.event({ kind: 'turn_completed', sessionId: 's', seq: 1 }); await Promise.resolve()
  await assert.rejects(notifier.remove('s')); resolveAgent({}); await sending
  assert.equal(published.length, 0)
})
test('notification publication is ordered and opening a Session preserves the sequence floor', async () => {
  let resolveAgent; let calls = 0; const published = []
  const load = loader({
    '@kit.AbilityKit': { wantAgent: { OperationType: {}, WantAgentFlags: {}, getWantAgent: () => ++calls === 1 ? new Promise(resolve => { resolveAgent = resolve }) : Promise.resolve({}) } },
    '@kit.NotificationKit': { notificationManager: { SlotType: {}, ContentType: {}, publish: async value => published.push(value.content.normal.text), cancel: async () => {} } },
  })
  const notifier = new (load('native/notifications').LocalNotifications)({ abilityInfo: {} }); notifier.enabled = true
  const first = notifier.event({ kind: 'turn_completed', sessionId: 's', seq: 1 }); await Promise.resolve()
  const second = notifier.event({ kind: 'turn_failed', sessionId: 's', seq: 2 })
  assert.equal(calls, 1); resolveAgent({}); await Promise.all([first, second])
  assert.deepEqual(published, ['本轮已完成', '会话发生错误'])
  await notifier.dismiss('s'); await notifier.event({ kind: 'turn_failed', sessionId: 's', seq: 2 })
  assert.equal(published.length, 2)
})
test('speech finishing an older microphone cannot finish a newly started draft', async () => {
  const env = speechEnvironment(); const speech = new env.SpeechDraft()
  await speech.start({}, () => {})
  const stopping = speech.stop()
  await speech.cancel()
  await speech.start({}, () => {})
  await stopping
  // Old finish may run before cancel; the current recognition still accepts new results.
  let observed = ''
  await speech.start({}, text => { observed = text })
  env.listener().onResult('current', { result: 'new draft', isLast: false })
  assert.equal(observed, 'new draft')
  await speech.cancel()
})
test('concurrent speech starts keep only the latest listener', async () => {
  const env = speechEnvironment(); const speech = new env.SpeechDraft(); const first = [], second = []
  await Promise.all([speech.start({}, text => first.push(text)), speech.start({}, text => second.push(text))])
  env.listener().onResult('current', { result: 'latest', isLast: false })
  assert.equal(first.length, 0); assert.equal(second.at(-1), 'latest')
  assert.equal(env.calls.filter(value => value === 'start').length, 1)
  await speech.cancel()
})
