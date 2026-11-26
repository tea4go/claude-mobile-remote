import { Capacitor, registerPlugin } from '@capacitor/core'
import type { AppStore } from '../session/appStore'
import { createInitialState, repairInputOrder, type BlockMessage, type Message, type MobileState } from '../session/reducer'
import { decodeServerMessage, isMessageTimestamp } from '../protocol/decode'
import { MESSAGE_TIME_PROTOCOL_VERSION } from '../protocol/types'
import { projectSubagentBlocks, toolKey } from '../session/stepIndex'

export const CACHE_BYTES = 2 * 1024 * 1024
export const CACHE_MESSAGES = 200
const CACHE_STEP_ITEMS = 200
const CACHE_STEP_BYTES = 256 * 1024
export const CACHE_AGE = 30 * 24 * 60 * 60 * 1000
export interface SessionCachePort {
  readSessionCache(options: { sessionId: string }): Promise<{ data: string | null }>
  writeSessionCache(options: { sessionId: string; data: string }): Promise<void>
  removeSessionCache(options: { sessionId: string }): Promise<void>
  clearSessionCache(): Promise<void>
}
const plugin = registerPlugin<SessionCachePort>('SessionExperience')
export const nativeSessionCache = ['ios', 'android'].includes(Capacitor.getPlatform()) ? plugin : null

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
  const timing = isMessageTimestamp(value.messageAt) ? { messageAt: value.messageAt } : {}
  if (value.kind === 'input' || value.kind === 'claude') {
    if (!text(value.text) || (value.source !== 'mobile' && value.source !== 'terminal')) return null
    if (value.kind === 'input') {
      const items = Array.isArray(value.attachments) ? value.attachments.filter(object) : []
      const attachments = items.filter(item => text(item.name, 255) && text(item.mediaType, 128) && text(item.ref, 128) && !!item.ref && /^image\/(jpeg|png|gif|webp)$/.test(item.mediaType as string)).map(item => ({ name: item.name as string, mediaType: item.mediaType as string, ref: item.ref as string, data: '' }))
      const names = items.filter(item => !attachments.some(a => a.name === item.name && a.ref === item.ref)).map(item => text(item.name, 255) ? item.name : '').filter(Boolean)
      return { kind: 'input', requestId: requestId || null, source: value.source, text: value.text + (names.length ? `\n[附件：${names.join('、')}；预览未缓存]` : ''), ...(attachments.length ? { attachments } : {}), status: value.status === 'unknown' ? 'unknown' : 'accepted', processingStarted: true, ...timing }
    }
    const model = text(value.model, 128) && new TextEncoder().encode(value.model).length <= 128 && !/\p{Cc}/u.test(value.model) ? value.model : ''
    return { kind: 'claude', requestId, source: value.source, text: value.text, ...(model ? { model } : {}), ...timing,
      status: value.status === 'failed' || value.status === 'cancelled' ? value.status : 'done', error: null }
  }
  // Reuse wire validation for nested tool/answer display data, without invoking any command.
  if (value.kind === 'block' || value.kind === 'answer') {
    try {
      const block = object(value.block) ? { ...value.block } : value.block
      if (object(block)) {
        if (!text(block.id, 256) || (block.kind !== 'thinking' && !text(block.toolUseId, 256)) ||
          (block.kind === 'tool_use' && (!text(block.toolName, 256) || (block.mcpServer !== undefined && !text(block.mcpServer, 256))))) return null
        if (block.kind === 'thinking') block.thinking = ''
        delete block.preview
        delete block.titleSummary
        delete block.output
        delete block.truncated
      }
      const wireVersion = MESSAGE_TIME_PROTOCOL_VERSION
      const event = { seq: 1, sessionId: 'cache', requestId, kind: value.kind === 'block' ? 'content_block' : 'action_resolved',
        source: 'mobile', text: '', sessionStatus: '', turnStatus: value.kind === 'answer' ? 'running' : '', error: null,
        ...(value.kind === 'block' ? { block } : { answers: value.answers }) }
      const decoded = decodeServerMessage(JSON.stringify({ version: wireVersion, type: 'event', sessionId: 'cache', requestId, commandId: '', seq: 1, payload: { event } }))
      if (decoded.type !== 'event') return null
      const decodedBlock = decoded.payload.event.block
      const cachedAgentStatus = value.cachedAgentStatus
      if (value.kind === 'block' && decodedBlock) return { kind: 'block', requestId, block: decodedBlock,
        ...(decodedBlock.kind === 'tool_use' && (cachedAgentStatus === 'running' || cachedAgentStatus === 'completed' ||
          cachedAgentStatus === 'failed' || cachedAgentStatus === 'cancelled') ? { cachedAgentStatus } : {}) }
      if (value.kind === 'answer' && decoded.payload.event.answers) return { kind: 'answer', requestId, answers: decoded.payload.event.answers, ...timing }
    } catch { /* A malformed cached item is not executable or displayable. */ }
  }
  return null
}
const fields = ['projectCwd', 'sessionTitle', 'createdAt', 'lastActivityAt'] as const
interface StepAnchor { kind: 'steps'; key: string; anchor: BlockMessage }
type TimelineEntry = Exclude<Message, BlockMessage> | StepAnchor
interface StepDetails { key: string; items: BlockMessage[] }

function cacheTimeline(state: MobileState) {
  const messages = projectSubagentBlocks(repairInputOrder(state.messages), state.subagentTasks ?? [])
  const statuses = new Map(state.subagentTasks?.map(task => [toolKey(task.requestId, task.toolUseId), task.status]))
  const calls = new Map<string, BlockMessage>()
  for (const message of messages) {
    if (message.kind === 'block' && message.block.kind === 'tool_use') {
      const key = toolKey(message.requestId, message.block.toolUseId)
      if (!calls.has(key)) calls.set(key, message)
    }
  }
  function owner(message: BlockMessage): BlockMessage | undefined {
    const { requestId, block } = message
    if (block.kind === 'thinking') return undefined
    const call = block.kind === 'tool_result' ? calls.get(toolKey(requestId, block.toolUseId)) : undefined
    return call ?? (block.parentToolUseId ? calls.get(toolKey(requestId, block.parentToolUseId)) : undefined)
  }

  const timeline: TimelineEntry[] = []
  const positions = new Map<BlockMessage, string>()
  const extras = new Map<string, BlockMessage[]>()
  const roots = new Set<BlockMessage>()
  function saveMessage(message: Message): Message | null {
    const entry = displayMessage(message)
    if (entry?.kind === 'block' && entry.block.kind === 'tool_use') {
      const status = statuses.get(toolKey(entry.requestId, entry.block.toolUseId))
      if (status) entry.cachedAgentStatus = status
    }
    return entry
  }
  for (const message of messages) {
    if (message.kind === 'block' && owner(message)) continue
    const entry = saveMessage(message)
    if (!entry) continue
    if (entry.kind !== 'block') {
      timeline.push(entry)
      continue
    }
    const previous = timeline.at(-1)
    const key = previous?.kind === 'steps' && previous.anchor.requestId === entry.requestId
      ? previous.key : toolKey(entry.requestId, entry.block.id)
    if (previous?.kind !== 'steps' || previous.key !== key) {
      timeline.push({ kind: 'steps', key, anchor: entry })
      extras.set(key, [])
      roots.add(message as BlockMessage)
    }
    positions.set(message as BlockMessage, key)
  }
  for (const message of messages) {
    if (message.kind !== 'block' || roots.has(message)) continue
    let anchor = owner(message)
    const visited = new Set<BlockMessage>()
    while (anchor && !positions.has(anchor) && !visited.has(anchor)) {
      visited.add(anchor)
      anchor = owner(anchor)
    }
    const key = positions.get(anchor ?? message)
    if (key === undefined) continue
    const group = extras.get(key)!
    const entry = saveMessage(message)
    if (entry?.kind === 'block') group.push(entry)
  }
  return { timeline, extras }
}

export function encodeSessionCache(sessionId: string, state: MobileState, savedAt: number): string {
  const metadata = Object.fromEntries(fields.map(key => [key, text(state[key], 4096) ? state[key] : null]))
  const value = { version: 2, sessionId, savedAt, ...metadata, timeline: [] as TimelineEntry[], steps: [] as StepDetails[] }
  const { timeline, extras } = cacheTimeline(state)
  const inputs = new Map<string, number>()
  timeline.forEach((entry, index) => { if (entry.kind === 'input' && entry.requestId) inputs.set(entry.requestId, index) })

  const encoder = new TextEncoder()
  const sizeOf = (entry: TimelineEntry | BlockMessage | StepDetails) => encoder.encode(JSON.stringify(entry)).byteLength + 1
  let remaining = CACHE_BYTES - encoder.encode(JSON.stringify(value)).byteLength + 1
  const retained = new Map<number, TimelineEntry>()
  for (let index = timeline.length - 1; index >= 0 && retained.size < CACHE_MESSAGES; index -= 1) {
    if (retained.has(index)) continue
    const entry = timeline[index]
    const size = sizeOf(entry)
    if (size > remaining) break
    retained.set(index, entry)
    remaining -= size
    const requestId = entry.kind === 'steps' ? entry.anchor.requestId : entry.requestId
    if (!requestId || retained.size === CACHE_MESSAGES) continue
    const inputIndex = inputs.get(requestId)
    if (inputIndex === undefined || inputIndex >= index) continue
    inputs.delete(requestId)
    const input = timeline[inputIndex]
    const sizeOfInput = sizeOf(input)
    if (sizeOfInput <= remaining) {
      retained.set(inputIndex, input)
      remaining -= sizeOfInput
    }
  }
  value.timeline = [...retained.entries()].sort(([left], [right]) => left - right).map(([, entry]) => entry)

  // A slot stays in the timeline even when its optional tool index is full.
  let stepBytes = Math.min(remaining, CACHE_STEP_BYTES)
  let stepItems = CACHE_STEP_ITEMS
  for (const entry of [...value.timeline].reverse()) {
    if (entry.kind !== 'steps' || stepItems === 0) continue
    const group: StepDetails = { key: entry.key, items: [] }
    let bytes = sizeOf(group)
    for (const item of extras.get(entry.key) ?? []) {
      const itemSize = sizeOf(item)
      if (bytes + itemSize > stepBytes || stepItems === 0) break
      group.items.push(item)
      bytes += itemSize
      stepItems -= 1
    }
    if (group.items.length === 0) continue
    stepBytes -= bytes
    value.steps.push(group)
  }
  return JSON.stringify(value)
}

function decodeTimeline(value: Record<string, unknown>): Message[] | null {
  if (!Array.isArray(value.timeline) || value.timeline.length > CACHE_MESSAGES || !Array.isArray(value.steps) ||
      value.steps.length > CACHE_MESSAGES || new TextEncoder().encode(JSON.stringify(value.steps)).byteLength > CACHE_STEP_BYTES + 2) return null
  const details = new Map<string, BlockMessage[]>()
  let stepCount = 0
  for (const group of value.steps) {
    if (!object(group) || !text(group.key, 4096) || details.has(group.key) || !Array.isArray(group.items)) return null
    stepCount += group.items.length
    if (stepCount > CACHE_STEP_ITEMS) return null
    details.set(group.key, group.items.map(displayMessage).filter((item): item is BlockMessage => item?.kind === 'block'))
  }
  const messages: Message[] = []
  const seen = new Set<string>()
  for (const entry of value.timeline) {
    if (object(entry) && entry.kind === 'steps') {
      const anchor = displayMessage(entry.anchor)
      if (!text(entry.key, 4096) || seen.has(entry.key) || anchor?.kind !== 'block') continue
      seen.add(entry.key)
      messages.push(anchor, ...(details.get(entry.key) ?? []).filter(item => item.requestId === anchor.requestId))
    } else {
      const item = displayMessage(entry)
      if (item && item.kind !== 'block') messages.push(item)
    }
  }
  return messages
}

export function decodeSessionCache(data: string | null, sessionId: string, now: number): MobileState | null {
  if (data === null || new TextEncoder().encode(data).byteLength > CACHE_BYTES) return null
  try {
    const value: unknown = JSON.parse(data)
    if (!object(value) || value.sessionId !== sessionId ||
      typeof value.savedAt !== 'number' || !Number.isFinite(value.savedAt) || value.savedAt > now || now - value.savedAt > CACHE_AGE) return null
    const messages = value.version === 2 ? decodeTimeline(value) :
      value.version === 1 && Array.isArray(value.messages) && value.messages.length <= CACHE_MESSAGES
        ? repairInputOrder(value.messages.map(displayMessage).filter((item): item is Message => item !== null)) : null
    if (messages === null) return null
    const state = createInitialState()
    state.sessionId = sessionId
    state.phase = 'disconnected'
    state.connection = 'disconnected'
    state.cachedAt = value.savedAt
    for (const field of fields) {
      if (value[field] !== null && !text(value[field], 4096)) return null
      state[field] = value[field] as string | null
    }
    state.messages = messages
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
      if (current && (!previous || current.messages !== previous.messages || current.subagentTasks !== previous.subagentTasks ||
        fields.some(field => current[field] !== previous[field]))) this.dirty.add(id)
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
