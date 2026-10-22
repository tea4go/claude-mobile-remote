import { Capacitor, registerPlugin } from '@capacitor/core'
import type { AppStore } from '../session/appStore'
import { createInitialState, type Message, type MobileState } from '../session/reducer'
import { decodeServerMessage } from '../protocol/decode'

export const CACHE_BYTES = 2 * 1024 * 1024
export const CACHE_MESSAGES = 200
export const CACHE_AGE = 30 * 24 * 60 * 60 * 1000
export interface SessionCachePort {
  readSessionCache(options: { sessionId: string }): Promise<{ data: string | null }>
  writeSessionCache(options: { sessionId: string; data: string }): Promise<void>
  removeSessionCache(options: { sessionId: string }): Promise<void>
  clearSessionCache(): Promise<void>
}
const plugin = registerPlugin<SessionCachePort>('SessionExperience')
export const iosSessionCache = Capacitor.getPlatform() === 'ios' ? plugin : null

function object(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
function text(value: unknown, limit = CACHE_BYTES): value is string {
  return typeof value === 'string' && value.length <= limit
}
// Whitelist the display model. Never serialize the runtime, Pairing, actions or attachment bytes.
function displayMessage(value: unknown): Message | null {
  if (!object(value) || !(text(value.requestId, 256) || (value.kind === 'input' && value.requestId === null))) return null
  const requestId = typeof value.requestId === 'string' ? value.requestId : ''
  if (value.kind === 'input' || value.kind === 'claude') {
    if (!text(value.text) || (value.source !== 'mobile' && value.source !== 'terminal')) return null
    if (value.kind === 'input') {
      const names = Array.isArray(value.attachments) ? value.attachments.filter(object).map((item) => text(item.name, 255) ? item.name : '').filter(Boolean) : []
      return { kind: 'input', requestId: requestId || null, source: value.source, text: value.text + (names.length ? `\n[附件：${names.join('、')}；预览未缓存]` : ''), status: value.status === 'unknown' ? 'unknown' : 'accepted' }
    }
    return { kind: 'claude', requestId, source: value.source, text: value.text,
      status: value.status === 'failed' || value.status === 'cancelled' ? value.status : 'done', error: null }
  }
  // Reuse wire validation for nested tool/answer display data, without invoking any command.
  if (value.kind === 'block' || value.kind === 'answer') {
    try {
      const event = { seq: 1, sessionId: 'cache', requestId, kind: value.kind === 'block' ? 'content_block' : 'action_resolved',
        source: 'mobile', text: '', sessionStatus: '', turnStatus: value.kind === 'answer' ? 'running' : '', error: null,
        ...(value.kind === 'block' ? { block: value.block } : { answers: value.answers }) }
      const decoded = decodeServerMessage(JSON.stringify({ version: 4, type: 'event', sessionId: 'cache', requestId, commandId: '', seq: 1, payload: { event } }))
      if (decoded.type !== 'event') return null
      if (value.kind === 'block' && decoded.payload.event.block) return { kind: 'block', requestId, block: decoded.payload.event.block }
      if (value.kind === 'answer' && decoded.payload.event.answers) return { kind: 'answer', requestId, answers: decoded.payload.event.answers }
    } catch { /* A malformed cached item is not executable or displayable. */ }
  }
  return null
}
const fields = ['projectCwd', 'sessionTitle', 'createdAt', 'lastActivityAt'] as const
export function encodeSessionCache(sessionId: string, state: MobileState, savedAt: number): string {
  const messages = state.messages.slice(-CACHE_MESSAGES).map(displayMessage).filter((item): item is Message => item !== null)
  const metadata = Object.fromEntries(fields.map((key) => [key, text(state[key], 4096) ? state[key] : null]))
  const value = { version: 1, sessionId, savedAt, ...metadata, messages }
  let result = JSON.stringify(value)
  while (new TextEncoder().encode(result).byteLength > CACHE_BYTES && messages.length) {
    messages.shift()
    result = JSON.stringify(value)
  }
  return result
}
export function decodeSessionCache(data: string | null, sessionId: string, now: number): MobileState | null {
  if (data === null || new TextEncoder().encode(data).byteLength > CACHE_BYTES) return null
  try {
    const value: unknown = JSON.parse(data)
    if (!object(value) || value.version !== 1 || value.sessionId !== sessionId ||
      typeof value.savedAt !== 'number' || !Number.isFinite(value.savedAt) || value.savedAt > now || now - value.savedAt > CACHE_AGE ||
      !Array.isArray(value.messages) || value.messages.length > CACHE_MESSAGES) return null
    const state = createInitialState()
    state.sessionId = sessionId
    state.phase = 'disconnected'
    state.connection = 'disconnected'
    state.cachedAt = value.savedAt
    for (const field of fields) {
      if (value[field] !== null && !text(value[field], 4096)) return null
      state[field] = value[field] as string | null
    }
    state.messages = value.messages.map(displayMessage).filter((item): item is Message => item !== null)
    return state
  } catch { return null }
}

// One I/O queue orders writes, removals and explicit clearing. A slow write cannot resurrect removed files.
export class SessionCacheWriter {
  private queue: Promise<void> = Promise.resolve()
  private timer: ReturnType<typeof setTimeout> | null = null
  private readonly dirty = new Set<string>()
  private readonly observed = new Map<string, MobileState>()
  private ids = new Set<string>()
  private readonly unsubscribe: () => void
  private readonly store: AppStore
  private readonly port: SessionCachePort
  private readonly now: () => number
  private readonly onError: () => void
  constructor(store: AppStore, port: SessionCachePort, now: () => number, onError: () => void) {
    this.store = store; this.port = port; this.now = now; this.onError = onError
    this.ids = new Set(store.getState().pairings.records.map((item) => item.sessionId))
    this.unsubscribe = store.subscribe(() => this.observe())
    this.observe()
  }
  private enqueue(operation: () => Promise<unknown>): void {
    this.queue = this.queue.then(operation).then(() => undefined).catch(() => this.onError())
  }
  private observe(): void {
    const state = this.store.getState()
    const ids = new Set(state.pairings.records.map((item) => item.sessionId))
    for (const id of this.ids) {
      if (!ids.has(id)) {
        this.dirty.delete(id); this.observed.delete(id)
        this.enqueue(() => this.port.removeSessionCache({ sessionId: id }))
      }
    }
    this.ids = ids
    let turnFinished = false
    for (const id of ids) {
      const current = state.sessions[id]
      const previous = this.observed.get(id)
      if (current) this.observed.set(id, current)
      if (previous && current && current.messages !== previous.messages && previous.messages.some((message) => message.kind === 'claude' && message.status === 'streaming') && !current.messages.some((message) => message.kind === 'claude' && message.status === 'streaming')) turnFinished = true
      if (current && (!previous || current.messages !== previous.messages || fields.some((field) => current[field] !== previous[field]))) this.dirty.add(id)
    }
    if (turnFinished) { void this.flush(); return }
    if (this.dirty.size && this.timer === null) this.timer = setTimeout(() => { this.timer = null; void this.flush() }, 1000)
  }
  flush(): Promise<void> {
    if (this.timer !== null) { clearTimeout(this.timer); this.timer = null }
    for (const sessionId of this.dirty) {
      const state = this.store.getState().sessions[sessionId]
      if (!state || !this.ids.has(sessionId)) continue
      const data = encodeSessionCache(sessionId, state, this.now())
      this.enqueue(() => this.port.writeSessionCache({ sessionId, data }))
    }
    this.dirty.clear()
    return this.queue
  }
  async clear(): Promise<void> {
    if (this.timer !== null) { clearTimeout(this.timer); this.timer = null }
    this.dirty.clear()
    // Propagate an explicit clear failure to the settings UI.
    const cleared = this.queue.then(() => this.port.clearSessionCache())
    this.queue = cleared.catch(() => undefined)
    await cleared
  }
  dispose(): void { this.unsubscribe(); void this.flush() }
}
