import { common } from '@kit.AbilityKit'
import { fileIo as fs } from '@kit.CoreFileKit'
import { util } from '@kit.ArkTS'
import { cryptoFramework } from '@kit.CryptoArchitectureKit'
import { huks } from '@kit.UniversalKeystoreKit'
import { type SessionRecord, createSessionRecord, pairingPayloadFromRecord } from '../session/records'
import { decodePairingPayload, type PairingDecodeResult } from '../protocol/pairing'
import { CACHE_AGE, CACHE_BYTES, CACHE_TOTAL, decodeCache } from '../session/cache'
import type { HarmonySessionState } from '../session/model'

const KEY_ALIAS = 'cmr.pairing.aes256.v1'
const encoder = new util.TextEncoder()
const decoder = util.TextDecoder.create('utf-8', { fatal: true })
const base64 = new util.Base64Helper()
interface CipherEnvelope { version: number; nonce: string; data: string }
interface CacheFile { name: string; size: number; time: number }

export class NativeStorage {
  private queue: Promise<void> = Promise.resolve()
  private readonly root: string
  private readonly vault: string
  private readonly cache: string
  constructor(context: common.UIAbilityContext) {
    this.root = context.filesDir + '/cmr'
    this.vault = this.root + '/pairing'
    this.cache = this.root + '/display'
    for (const path of [this.root, this.vault, this.cache]) {
      if (!fs.accessSync(path)) fs.mkdirSync(path)
      fs.chmodSync(path, 0o700)
    }
  }
  private serial<T>(operation: () => Promise<T>): Promise<T> {
    const result = this.queue.then(operation)
    this.queue = result.then(() => {}).catch(() => {})
    return result
  }
  private async key(id: string): Promise<string> {
    const md = cryptoFramework.createMd('SHA256')
    await md.update({ data: encoder.encodeInto(id) })
    const digest = await md.digest()
    return Array.from(digest.data).map((byte: number) => byte.toString(16).padStart(2, '0')).join('')
  }
  private read(path: string, limit: number): string | null {
    if (!fs.accessSync(path)) return null
    if (fs.statSync(path).size > limit) throw new Error('stored file exceeds limit')
    return fs.readTextSync(path)
  }
  private write(path: string, data: string): void {
    const temporary = path + '.tmp'
    const file = fs.openSync(temporary, fs.OpenMode.CREATE | fs.OpenMode.WRITE_ONLY | fs.OpenMode.TRUNC)
    try {
      fs.chmodSync(temporary, 0o600)
      const bytes = encoder.encodeInto(data)
      let offset = 0
      while (offset < bytes.length) {
        const written = fs.writeSync(file.fd, bytes.slice(offset).buffer)
        if (written <= 0) throw new Error('short file write')
        offset += written
      }
      fs.fsyncSync(file.fd)
    } finally { fs.closeSync(file) }
    fs.renameSync(temporary, path)
  }
  private remove(path: string): void { if (fs.accessSync(path)) fs.unlinkSync(path) }
  private async ensureKey(): Promise<void> {
    if (await huks.isKeyItemExist(KEY_ALIAS, { properties: [] })) return
    await huks.generateKeyItem(KEY_ALIAS, { properties: [
      { tag: huks.HuksTag.HUKS_TAG_ALGORITHM, value: huks.HuksKeyAlg.HUKS_ALG_AES },
      { tag: huks.HuksTag.HUKS_TAG_KEY_SIZE, value: huks.HuksKeySize.HUKS_AES_KEY_SIZE_256 },
      { tag: huks.HuksTag.HUKS_TAG_PURPOSE, value: huks.HuksKeyPurpose.HUKS_KEY_PURPOSE_ENCRYPT | huks.HuksKeyPurpose.HUKS_KEY_PURPOSE_DECRYPT },
    ] })
  }
  private async crypt(id: string, input: Uint8Array, nonce: Uint8Array, encrypt: boolean): Promise<Uint8Array> {
    const properties: Array<huks.HuksParam> = [
      { tag: huks.HuksTag.HUKS_TAG_ALGORITHM, value: huks.HuksKeyAlg.HUKS_ALG_AES },
      { tag: huks.HuksTag.HUKS_TAG_KEY_SIZE, value: huks.HuksKeySize.HUKS_AES_KEY_SIZE_256 },
      { tag: huks.HuksTag.HUKS_TAG_PURPOSE, value: encrypt ? huks.HuksKeyPurpose.HUKS_KEY_PURPOSE_ENCRYPT : huks.HuksKeyPurpose.HUKS_KEY_PURPOSE_DECRYPT },
      { tag: huks.HuksTag.HUKS_TAG_PADDING, value: huks.HuksKeyPadding.HUKS_PADDING_NONE },
      { tag: huks.HuksTag.HUKS_TAG_BLOCK_MODE, value: huks.HuksCipherMode.HUKS_MODE_GCM },
      { tag: huks.HuksTag.HUKS_TAG_NONCE, value: nonce },
      { tag: huks.HuksTag.HUKS_TAG_ASSOCIATED_DATA, value: encoder.encodeInto('cmr-v1:' + id) },
    ]
    if (!encrypt) properties.push({ tag: huks.HuksTag.HUKS_TAG_AE_TAG, value: input.slice(-16) })
    const options: huks.HuksOptions = { properties, inData: encrypt ? input : input.slice(0, -16) }
    const handle = await huks.initSession(KEY_ALIAS, options)
    try {
      const result = await huks.finishSession(handle.handle, options)
      if (result.outData === undefined) throw new Error('secure storage failed')
      return result.outData
    } catch (_) {
      await huks.abortSession(handle.handle, { properties: [] }).catch(() => {})
      throw new Error('secure storage failed')
    }
  }
  saveRecords(records: Array<SessionRecord>): Promise<void> {
    // Capture a whitelist before queuing, never persist the runtime or a plaintext Pair Token.
    const captured = records.map((record: SessionRecord): SessionRecord => {
      const copy = createSessionRecord(pairingPayloadFromRecord(record))
      copy.name = record.name; copy.customName = record.customName
      return copy
    })
    return this.serial(async () => {
      await this.ensureKey()
      const retained: Array<string> = []
      for (const record of captured) {
        const key = await this.key(record.sessionId)
        retained.push(key + '.json')
        if (record.pairToken.length > 0 && Date.parse(record.expiresAt) > Date.now()) {
          if (record.pairToken.length > 4096) throw new Error('pairing exceeds storage limit')
          const nonce = cryptoFramework.createRandom().generateRandomSync(12).data
          const encrypted = await this.crypt(record.sessionId, encoder.encodeInto(record.pairToken), nonce, true)
          const envelope: CipherEnvelope = { version: 1, nonce: base64.encodeToStringSync(nonce), data: base64.encodeToStringSync(encrypted) }
          this.write(this.vault + '/' + key + '.json', JSON.stringify(envelope))
        } else this.remove(this.vault + '/' + key + '.json')
        record.pairToken = ''
      }
      this.write(this.root + '/records.json', JSON.stringify(captured))
      for (const file of fs.listFileSync(this.vault)) if (retained.indexOf(file) < 0) this.remove(this.vault + '/' + file)
    })
  }
  loadRecords(): Promise<Array<SessionRecord>> {
    return this.serial(async () => {
      const raw = this.read(this.root + '/records.json', 2 * 1024 * 1024)
      if (raw === null) return []
      const values = JSON.parse(raw) as Array<SessionRecord>
      if (!Array.isArray(values) || values.length > 1000) throw new Error('invalid saved pairings')
      const records: Array<SessionRecord> = []
      for (const value of values) {
        if (value === null || typeof value.sessionId !== 'string' || records.some((item) => item.sessionId === value.sessionId)) continue
        // Validate the public metadata separately, retaining expired entries for re-pairing.
        value.pairToken = 'metadata-validation-only'
        let parsed: PairingDecodeResult
        try { parsed = decodePairingPayload(JSON.stringify(pairingPayloadFromRecord(value)), 0) }
        catch (_) { continue }
        if (!parsed.ok || parsed.payload === null) continue
        const record = createSessionRecord(parsed.payload)
        record.pairToken = ''; record.status = 'needsPairing'
        if (typeof value.name === 'string' && value.name.length <= 255) record.name = value.name
        record.customName = value.customName === true
        if (Date.parse(record.expiresAt) > Date.now()) {
          try {
            const key = await this.key(record.sessionId)
            const encrypted = this.read(this.vault + '/' + key + '.json', 16384)
            if (encrypted !== null) {
              const envelope = JSON.parse(encrypted) as CipherEnvelope
              if (envelope.version !== 1) throw new Error('invalid cipher')
              const nonce = base64.decodeSync(envelope.nonce)
              const data = base64.decodeSync(envelope.data)
              if (nonce.length !== 12 || data.length < 17) throw new Error('invalid cipher')
              record.pairToken = decoder.decodeWithStream(await this.crypt(record.sessionId, data, nonce, false))
              if (record.pairToken.length > 0) record.status = 'disconnected'
            }
          } catch (_) { record.pairToken = ''; record.status = 'needsPairing' }
        }
        records.push(record)
      }
      return records
    })
  }
  readCache(id: string): Promise<HarmonySessionState | null> {
    return this.serial(async () => {
      const path = this.cache + '/' + await this.key(id) + '.json'
      try {
        const data = this.read(path, CACHE_BYTES)
        const state = data === null ? null : decodeCache(data, id, Date.now())
        if (state === null) this.remove(path)
        return state
      } catch (_) { this.remove(path); return null }
    })
  }
  writeCache(id: string, data: string): Promise<void> {
    return this.serial(async () => {
      if (decodeCache(data, id, Date.now()) === null) throw new Error('invalid cache')
      this.write(this.cache + '/' + await this.key(id) + '.json', data)
      this.prune()
    })
  }
  private prune(): void {
    const files: Array<CacheFile> = []
    let total = 0
    for (const name of fs.listFileSync(this.cache)) {
      const path = this.cache + '/' + name
      const stat = fs.statSync(path)
      if (!/^[0-9a-f]{64}\.json$/.test(name) || stat.size > CACHE_BYTES || Date.now() - stat.mtime * 1000 > CACHE_AGE) this.remove(path)
      else { files.push({ name, size: stat.size, time: stat.mtime }); total += stat.size }
    }
    files.sort((a: CacheFile, b: CacheFile) => a.time - b.time)
    for (const file of files) {
      if (total <= CACHE_TOTAL) break
      this.remove(this.cache + '/' + file.name); total -= file.size
    }
  }
  removeCache(id: string): Promise<void> { return this.serial(async () => this.remove(this.cache + '/' + await this.key(id) + '.json')) }
  clearCache(): Promise<void> { return this.serial(async () => { for (const name of fs.listFileSync(this.cache)) this.remove(this.cache + '/' + name) }) }
  readNotificationsEnabled(): boolean { return this.read(this.root + '/notifications', 16) === 'enabled' }
  setNotificationsEnabled(enabled: boolean): void { this.write(this.root + '/notifications', enabled ? 'enabled' : 'disabled') }
}
