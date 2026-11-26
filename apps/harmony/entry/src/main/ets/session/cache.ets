import { createInitialHarmonySessionState, type HarmonySessionState, type FeedMessage, type InputFeedMessage, type ClaudeFeedMessage, type AnswerFeedMessage, type BlockFeedMessage } from './model'
import { repairInputOrder } from './reducer'
import { decodeServerMessage, isMessageTimestamp } from '../protocol/decode'
import type { Attachment, ContentBlock, SubagentStatus } from '../protocol/types'
import { projectSubagentBlocks } from './stepIndex'
import { utf8Bytes } from './attachments'

export const CACHE_BYTES = 2 * 1024 * 1024
export const CACHE_TOTAL = 20 * 1024 * 1024
export const CACHE_AGE = 30 * 24 * 60 * 60 * 1000
const CACHE_MESSAGES = 200
const CACHE_STEP_ITEMS = 200
const CACHE_STEP_BYTES = 256 * 1024

interface StepAnchor { kind: 'steps'; key: string; anchor: BlockFeedMessage }
type TimelineEntry = InputFeedMessage | ClaudeFeedMessage | AnswerFeedMessage | StepAnchor
interface StepDetails { key: string; items: BlockFeedMessage[] }
interface CacheTimeline { timeline: TimelineEntry[]; extras: Map<string, BlockFeedMessage[]> }
export interface CachedSession {
  version: number
  sessionId: string
  savedAt: number
  projectCwd?: string
  sessionTitle?: string
  createdAt?: string
  lastActivityAt?: string
  messages?: Array<FeedMessage>
  timeline?: TimelineEntry[]
  steps?: StepDetails[]
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
    const messageAt = isMessageTimestamp(message.messageAt) ? message.messageAt : undefined
    if (typeof message.text !== 'string' || message.text.length > CACHE_BYTES || (message.source !== 'mobile' && message.source !== 'terminal')) return null
    if (message.kind === 'input') {
      const attachments: Array<Attachment> = []
      if (Array.isArray(message.attachments)) message.attachments.slice(0, 4).forEach((item: Attachment) => {
        if (item !== null && typeof item.name === 'string' && item.name.length <= 255 && typeof item.mediaType === 'string' && item.mediaType.length <= 128) {
          const saved: Attachment = { name: item.name, mediaType: item.mediaType, data: '' }
          if (typeof item.ref === 'string' && item.ref.length > 0 && item.ref.length <= 128 && ['image/png','image/jpeg','image/gif','image/webp'].indexOf(item.mediaType) >= 0) saved.ref = item.ref
          attachments.push(saved)
        }
      })
      return { kind: 'input', requestId: message.requestId, source: message.source, text: message.text, messageAt, attachments, processingStarted: true, status: message.status === 'unknown' ? 'unknown' : 'accepted' }
    }
    return { kind: 'claude', requestId: message.requestId, source: message.source, text: message.text, messageAt, model: validText(message.model, 128) ? message.model : undefined,
      status: message.status === 'failed' || message.status === 'cancelled' ? message.status : 'done', error: null }
  }
  if (message.kind === 'block' || message.kind === 'answer') {
    try {
      let event: string
      if (message.kind === 'block') {
        const source = message.block
        if (!validText(source.id, 256) || (source.kind !== 'thinking' && !validText(source.toolUseId, 256)) ||
          (source.kind === 'tool_use' && (!validText(source.toolName, 256) || !validText(source.mcpServer, 256)))) return null
        let block: ContentBlock
        if (source.kind === 'thinking') block = { id: source.id, kind: 'thinking', thinking: '' }
        else if (source.kind === 'tool_use') block = { id: source.id, kind: 'tool_use', toolUseId: source.toolUseId,
          toolName: source.toolName, toolClass: source.toolClass, mcpServer: source.mcpServer,
          parentToolUseId: source.parentToolUseId, detailAvailable: source.detailAvailable }
        else block = { id: source.id, kind: 'tool_result', toolUseId: source.toolUseId, isError: source.isError,
          parentToolUseId: source.parentToolUseId, detailAvailable: source.detailAvailable }
        event = `"kind":"content_block","turnStatus":"","block":${JSON.stringify(block)}`
      } else {
        event = `"kind":"action_resolved","turnStatus":"running","answers":${JSON.stringify(message.answers)}`
      }
      const wire = `{"version":14,"type":"event","sessionId":"cache","requestId":${JSON.stringify(message.requestId)},"commandId":"","seq":1,"payload":{"event":{"seq":1,"sessionId":"cache","requestId":${JSON.stringify(message.requestId)},"source":"mobile","text":"","sessionStatus":"","error":null,${event}}}}`
      const decoded = decodeServerMessage(wire)
      if (decoded.type !== 'event') return null
      const block = decoded.payload.event.block
      if (block !== undefined && message.kind === 'block') {
        const status = message.cachedAgentStatus
        return { kind: 'block', requestId: message.requestId, block, cachedAgentStatus: block.kind === 'tool_use' &&
          (status === 'running' || status === 'completed' || status === 'failed' || status === 'cancelled') ? status : undefined }
      }
      if (message.kind === 'answer' && decoded.payload.event.answers !== undefined) {
        return { kind: 'answer', requestId: message.requestId, answers: decoded.payload.event.answers,
          messageAt: isMessageTimestamp(message.messageAt) ? message.messageAt : undefined }
      }
    } catch (_) { return null }
  }
  return null
}
function toolKey(requestId: string, toolUseId: string): string { return JSON.stringify([requestId, toolUseId]) }

function owner(message: BlockFeedMessage, calls: Map<string, BlockFeedMessage>): BlockFeedMessage | undefined {
  const block = message.block
  if (block.kind === 'thinking') return undefined
  const call = block.kind === 'tool_result' ? calls.get(toolKey(message.requestId, block.toolUseId)) : undefined
  return call ?? (block.parentToolUseId ? calls.get(toolKey(message.requestId, block.parentToolUseId)) : undefined)
}

function cacheTimeline(state: HarmonySessionState): CacheTimeline {
  const messages = projectSubagentBlocks(repairInputOrder(state.messages), state.subagentTasks ?? [])
  const statuses = new Map<string, SubagentStatus>()
  for (const task of state.subagentTasks ?? []) statuses.set(toolKey(task.requestId, task.toolUseId), task.status)
  const saveMessage = (message: FeedMessage): FeedMessage | null => {
    const entry = displayMessage(message)
    if (entry?.kind === 'block' && entry.block.kind === 'tool_use') {
      entry.cachedAgentStatus = statuses.get(toolKey(entry.requestId, entry.block.toolUseId)) ?? entry.cachedAgentStatus
    }
    return entry
  }
  const calls = new Map<string, BlockFeedMessage>()
  for (const message of messages) {
    if (message.kind === 'block' && message.block.kind === 'tool_use') {
      const key = toolKey(message.requestId, message.block.toolUseId)
      if (!calls.has(key)) calls.set(key, message)
    }
  }
  const timeline: TimelineEntry[] = []
  const positions = new Map<BlockFeedMessage, string>()
  const roots = new Set<BlockFeedMessage>()
  const extras = new Map<string, BlockFeedMessage[]>()
  for (const message of messages) {
    if (message.kind === 'block' && owner(message, calls) !== undefined) continue
    const entry = saveMessage(message)
    if (entry === null) continue
    if (entry.kind !== 'block') {
      timeline.push(entry)
      continue
    }
    const previous = timeline[timeline.length - 1]
    const key = previous?.kind === 'steps' && previous.anchor.requestId === entry.requestId
      ? previous.key : toolKey(entry.requestId, entry.block.id)
    const original = message as BlockFeedMessage
    if (previous?.kind !== 'steps' || previous.key !== key) {
      timeline.push({ kind: 'steps', key, anchor: entry })
      extras.set(key, [])
      roots.add(original)
    }
    positions.set(original, key)
  }
  for (const message of messages) {
    if (message.kind !== 'block' || roots.has(message)) continue
    let anchor = owner(message, calls)
    const visited = new Set<BlockFeedMessage>()
    while (anchor !== undefined && !positions.has(anchor) && !visited.has(anchor)) {
      visited.add(anchor)
      anchor = owner(anchor, calls)
    }
    const key = positions.get(anchor ?? message)
    if (key === undefined) continue
    const entry = saveMessage(message)
    if (entry?.kind === 'block') extras.get(key)?.push(entry)
  }
  return { timeline, extras }
}

function sizeOf(value: Object): number { return utf8Bytes(JSON.stringify(value)) + 1 }

export function encodeCache(id: string, state: HarmonySessionState, now: number): string {
  const cache: CachedSession = { version: 2, sessionId: id, savedAt: now,
    projectCwd: validText(state.projectCwd, 4096) ? state.projectCwd : undefined,
    sessionTitle: validText(state.sessionTitle, 4096) ? state.sessionTitle : undefined,
    createdAt: validText(state.createdAt, 4096) ? state.createdAt : undefined,
    lastActivityAt: validText(state.lastActivityAt, 4096) ? state.lastActivityAt : undefined, timeline: [], steps: [] }
  const projected = cacheTimeline(state)
  const timeline = projected.timeline
  const inputs = new Map<string, number>()
  timeline.forEach((entry: TimelineEntry, index: number) => {
    if (entry.kind === 'input' && entry.requestId.length > 0) inputs.set(entry.requestId, index)
  })
  let remaining = CACHE_BYTES - sizeOf(cache) + 2
  const retained = new Map<number, TimelineEntry>()
  for (let index = timeline.length - 1; index >= 0 && retained.size < CACHE_MESSAGES; index -= 1) {
    if (retained.has(index)) continue
    const entry = timeline[index]
    const size = sizeOf(entry)
    if (size > remaining) break
    retained.set(index, entry)
    remaining -= size
    const requestId = entry.kind === 'steps' ? entry.anchor.requestId : entry.requestId
    const inputIndex = inputs.get(requestId)
    if (inputIndex === undefined || inputIndex >= index || retained.size === CACHE_MESSAGES) continue
    inputs.delete(requestId)
    const input = timeline[inputIndex]
    const inputSize = sizeOf(input)
    if (inputSize <= remaining) {
      retained.set(inputIndex, input)
      remaining -= inputSize
    }
  }
  const indexes = Array.from(retained.keys()).sort((left: number, right: number) => left - right)
  const entries = indexes.map((index: number) => retained.get(index) as TimelineEntry)
  const steps: StepDetails[] = []
  let stepBytes = Math.min(remaining, CACHE_STEP_BYTES)
  let stepItems = CACHE_STEP_ITEMS
  for (let index = entries.length - 1; index >= 0 && stepItems > 0; index -= 1) {
    const entry = entries[index]
    if (entry.kind !== 'steps') continue
    const group: StepDetails = { key: entry.key, items: [] }
    let bytes = sizeOf(group)
    for (const item of projected.extras.get(entry.key) ?? []) {
      const itemSize = sizeOf(item)
      if (bytes + itemSize > stepBytes || stepItems === 0) break
      group.items.push(item)
      bytes += itemSize
      stepItems -= 1
    }
    if (group.items.length === 0) continue
    stepBytes -= bytes
    steps.push(group)
  }
  cache.timeline = entries
  cache.steps = steps
  return JSON.stringify(cache)
}

function decodeTimeline(value: CachedSession): FeedMessage[] | null {
  if (!Array.isArray(value.timeline) || value.timeline.length > CACHE_MESSAGES || !Array.isArray(value.steps) ||
    value.steps.length > CACHE_MESSAGES || utf8Bytes(JSON.stringify(value.steps)) > CACHE_STEP_BYTES + 2) return null
  const details = new Map<string, BlockFeedMessage[]>()
  let stepCount = 0
  for (const group of value.steps) {
    if (group === null || typeof group.key !== 'string' || group.key.length > 4096 || details.has(group.key) || !Array.isArray(group.items)) return null
    stepCount += group.items.length
    if (stepCount > CACHE_STEP_ITEMS) return null
    const items: BlockFeedMessage[] = []
    group.items.forEach((raw: BlockFeedMessage) => {
      const item = displayMessage(raw)
      if (item?.kind === 'block') items.push(item)
    })
    details.set(group.key, items)
  }
  const messages: FeedMessage[] = []
  const seen = new Set<string>()
  for (const entry of value.timeline) {
    if (entry === null || typeof entry !== 'object') continue
    if (entry.kind === 'steps') {
      if (typeof entry.key !== 'string' || entry.key.length > 4096 || seen.has(entry.key)) continue
      const anchor = displayMessage(entry.anchor)
      if (anchor?.kind !== 'block') continue
      seen.add(entry.key)
      messages.push(anchor)
      const group = details.get(entry.key) ?? []
      group.forEach((item: BlockFeedMessage) => { if (item.requestId === anchor.requestId) messages.push(item) })
    } else {
      const message = displayMessage(entry)
      if (message !== null && message.kind !== 'block') messages.push(message)
    }
  }
  return messages
}

export function decodeCache(data: string, id: string, now: number): HarmonySessionState | null {
  if (utf8Bytes(data) > CACHE_BYTES) return null
  try {
    const value = JSON.parse(data) as CachedSession
    if (value === null || value.sessionId !== id || !Number.isFinite(value.savedAt) || value.savedAt > now || now - value.savedAt > CACHE_AGE ||
      !validText(value.projectCwd, 4096) || !validText(value.sessionTitle, 4096) ||
      !validText(value.createdAt, 4096) || !validText(value.lastActivityAt, 4096)) return null
    const state = createInitialHarmonySessionState()
    state.sessionId = id
    state.projectCwd = value.projectCwd; state.sessionTitle = value.sessionTitle
    state.createdAt = value.createdAt; state.lastActivityAt = value.lastActivityAt
    if (value.version === 2) {
      const messages = decodeTimeline(value)
      if (messages === null) return null
      state.messages = messages
    } else if (value.version === 1 && Array.isArray(value.messages) && value.messages.length <= CACHE_MESSAGES) {
      value.messages.forEach((item: FeedMessage) => { const message = displayMessage(item); if (message !== null) state.messages.push(message) })
      state.messages = repairInputOrder(state.messages)
    } else return null
    return state
  } catch (_) { return null }
}
