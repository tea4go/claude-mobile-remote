import { createInitialHarmonySessionState, type HarmonySessionState, type FeedMessage } from './model'
import { decodeServerMessage } from '../protocol/decode'
import { utf8Bytes } from './attachments'

export const CACHE_BYTES = 2 * 1024 * 1024
export const CACHE_TOTAL = 20 * 1024 * 1024
export const CACHE_AGE = 30 * 24 * 60 * 60 * 1000
export interface CachedSession {
  version: number
  sessionId: string
  savedAt: number
  projectCwd?: string
  sessionTitle?: string
  createdAt?: string
  lastActivityAt?: string
  messages: Array<FeedMessage>
}

// Reducer actions clone the feed even when only the connection changes. Compare the
// bounded display projection so disconnect/reconnect cannot recreate a cleared cache.
export class CacheWriteGate {
  private readonly observed: Map<string, string> = new Map()
  changed(id: string, data: string): boolean {
    const value = JSON.parse(data) as CachedSession
    value.savedAt = 0
    const content = JSON.stringify(value)
    if (this.observed.get(id) === content) return false
    this.observed.set(id, content)
    return true
  }
  forget(id: string): void { this.observed.delete(id) }
}
function validText(value: string | undefined, limit: number): boolean {
  return value === undefined || (typeof value === 'string' && value.length <= limit)
}
function displayMessage(message: FeedMessage): FeedMessage | null {
  if (message === null || typeof message !== 'object' || typeof message.requestId !== 'string' || message.requestId.length > 256) return null
  if (message.kind === 'input' || message.kind === 'claude') {
    if (typeof message.text !== 'string' || message.text.length > CACHE_BYTES || (message.source !== 'mobile' && message.source !== 'terminal')) return null
    if (message.kind === 'input') {
      let text = message.text
      if (Array.isArray(message.attachments) && message.attachments.length > 0) text += '\n[附件预览未缓存]'
      return { kind: 'input', requestId: message.requestId, source: message.source, text, status: message.status === 'unknown' ? 'unknown' : 'accepted' }
    }
    return { kind: 'claude', requestId: message.requestId, source: message.source, text: message.text,
      status: message.status === 'failed' || message.status === 'cancelled' ? message.status : 'done', error: null }
  }
  if (message.kind === 'block' || message.kind === 'answer') {
    try {
      const event = message.kind === 'block'
        ? `"kind":"content_block","turnStatus":"","block":${JSON.stringify(message.block)}`
        : `"kind":"action_resolved","turnStatus":"running","answers":${JSON.stringify(message.answers)}`
      const wire = `{"version":4,"type":"event","sessionId":"cache","requestId":${JSON.stringify(message.requestId)},"commandId":"","seq":1,"payload":{"event":{"seq":1,"sessionId":"cache","requestId":${JSON.stringify(message.requestId)},"source":"mobile","text":"","sessionStatus":"","error":null,${event}}}}`
      const decoded = decodeServerMessage(wire)
      if (decoded.type !== 'event') return null
      if (decoded.payload.event.block !== undefined) return { kind: 'block', requestId: message.requestId, block: decoded.payload.event.block }
      if (decoded.payload.event.answers !== undefined) return { kind: 'answer', requestId: message.requestId, answers: decoded.payload.event.answers }
    } catch (_) { return null }
  }
  return null
}
export function encodeCache(id: string, state: HarmonySessionState, now: number): string {
  const messages: Array<FeedMessage> = []
  const cache: CachedSession = { version: 1, sessionId: id, savedAt: now,
    projectCwd: validText(state.projectCwd, 4096) ? state.projectCwd : undefined,
    sessionTitle: validText(state.sessionTitle, 4096) ? state.sessionTitle : undefined,
    createdAt: validText(state.createdAt, 4096) ? state.createdAt : undefined,
    lastActivityAt: validText(state.lastActivityAt, 4096) ? state.lastActivityAt : undefined, messages }
  // Budget the newest suffix once. Re-stringifying a multi-MiB feed after each
  // discarded message caused hundreds of full scans on the UI thread.
  let bytes = utf8Bytes(JSON.stringify(cache))
  for (let index = state.messages.length - 1; index >= Math.max(0, state.messages.length - 200); index -= 1) {
    const value = displayMessage(state.messages[index])
    if (value === null) continue
    const size = utf8Bytes(JSON.stringify(value)) + (messages.length > 0 ? 1 : 0)
    if (bytes + size > CACHE_BYTES) break
    messages.unshift(value)
    bytes += size
  }
  return JSON.stringify(cache)
}
export function decodeCache(data: string, id: string, now: number): HarmonySessionState | null {
  if (utf8Bytes(data) > CACHE_BYTES) return null
  try {
    const value = JSON.parse(data) as CachedSession
    if (value === null || value.version !== 1 || value.sessionId !== id || !Number.isFinite(value.savedAt) ||
      value.savedAt > now || now - value.savedAt > CACHE_AGE || !Array.isArray(value.messages) || value.messages.length > 200 ||
      !validText(value.projectCwd, 4096) || !validText(value.sessionTitle, 4096) ||
      !validText(value.createdAt, 4096) || !validText(value.lastActivityAt, 4096)) return null
    const state = createInitialHarmonySessionState()
    state.sessionId = id
    state.projectCwd = value.projectCwd; state.sessionTitle = value.sessionTitle
    state.createdAt = value.createdAt; state.lastActivityAt = value.lastActivityAt
    value.messages.forEach((item: FeedMessage) => { const message = displayMessage(item); if (message !== null) state.messages.push(message) })
    return state
  } catch (_) { return null }
}
