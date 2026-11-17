import type { FeedMessage, BlockFeedMessage } from '../session/model'
import type { Attachment } from '../protocol/types'

export interface ActivityFeedGroup { kind: 'activity'; requestId: string; blocks: Array<BlockFeedMessage> }
export interface CollapsedTurn { kind: 'collapsed'; requestId: string; expanded: boolean }
export type FeedRow = FeedMessage | ActivityFeedGroup | CollapsedTurn

function turnId(row: FeedRow): string {
  return row.kind === 'activity' || row.kind === 'claude' ? row.requestId : ''
}
interface TurnBounds { first: number; last: number; final: number; activities: number; contiguous: boolean }
export function collapseLongTurns(rows: Array<FeedRow>, expanded: Set<string>): Array<FeedRow> {
  const bounds: Map<string, TurnBounds> = new Map()
  rows.forEach((row: FeedRow, index: number) => {
    const id = turnId(row)
    if (id.length === 0) return
    const previous = bounds.get(id)
    if (previous === undefined) bounds.set(id, { first: index, last: index, final: row.kind === 'claude' ? index : -1, activities: row.kind === 'activity' ? 1 : 0, contiguous: true })
    else {
      for (let i = previous.last + 1; i < index; i += 1) if (turnId(rows[i]) !== id) previous.contiguous = false
      previous.last = index
      if (row.kind === 'claude') previous.final = index
      if (row.kind === 'activity') previous.activities += 1
    }
  })
  const result: Array<FeedRow> = []
  rows.forEach((row: FeedRow, index: number) => {
    const id = turnId(row); const group = bounds.get(id)
    if (group !== undefined && group.activities >= 8 && group.contiguous && group.final === group.last) {
      if (index === group.first) result.push({ kind: 'collapsed', requestId: id, expanded: expanded.has(id) })
      if (index < group.final && !expanded.has(id)) return
    }
    result.push(row)
  })
  return result
}

// Only adjacent blocks in the same turn form a group; no attachment bytes in keys.
export function projectFeed(messages: Array<FeedMessage>): Array<FeedRow> {
  const rows: Array<FeedRow> = []
  for (const message of messages) {
    const previous = rows.length > 0 ? rows[rows.length - 1] : undefined
    if (message.kind === 'block') {
      if (previous !== undefined && previous.kind === 'activity' && previous.requestId === message.requestId) previous.blocks.push(message)
      else rows.push({ kind: 'activity', requestId: message.requestId, blocks: [message] })
    } else rows.push(message)
  }
  return rows
}
export function feedRenderKey(message: FeedRow, index: number): string {
  if (message.kind === 'collapsed') return `collapsed-${message.requestId}-${message.expanded}`
  if (message.kind === 'input') return JSON.stringify([index, message.kind, message.requestId, message.text,
    message.status, message.attachments?.map((item: Attachment) => item.name + ':' + item.mediaType + ':' + (item.data.length > 0 ? 'loaded' : 'missing'))])
  return `${index}-${JSON.stringify(message)}`
}

function sameRow(left: FeedRow, right: FeedRow): boolean {
  if (left === right) return true
  if (left.kind !== 'activity' || right.kind !== 'activity' || left.requestId !== right.requestId ||
    left.blocks.length !== right.blocks.length) return false
  return left.blocks.every((block: BlockFeedMessage, index: number): boolean => block === right.blocks[index])
}

export class FeedDataSource implements IDataSource {
  private messages: Array<FeedRow> = []
  private rawMessages: Array<FeedMessage> = []
  private listeners: Array<DataChangeListener> = []
  private expanded: Set<string> = new Set()
  toggleTurn(requestId: string): void {
    if (this.expanded.has(requestId)) this.expanded.delete(requestId)
    else this.expanded.add(requestId)
    this.messages = collapseLongTurns(projectFeed(this.rawMessages), this.expanded)
    this.listeners.forEach((listener: DataChangeListener) => listener.onDataReloaded())
  }
  totalCount(): number { return this.messages.length }
  getData(index: number): FeedRow { return this.messages[index] }
  registerDataChangeListener(listener: DataChangeListener): void {
    if (this.listeners.indexOf(listener) < 0) this.listeners.push(listener)
  }
  unregisterDataChangeListener(listener: DataChangeListener): void {
    this.listeners = this.listeners.filter((value: DataChangeListener) => value !== listener)
  }
  replace(messages: Array<FeedMessage>): void {
    const previous = this.messages
    const raw = this.rawMessages
    this.rawMessages = messages
    // A streaming delta replaces only the last Claude message. Reuse the
    // projected prefix rather than rebuilding every activity group per chunk.
    if (messages.length > 0 && messages.length === raw.length && previous.length > 0 &&
      messages[messages.length - 1].kind === 'claude' && raw[raw.length - 1].kind === 'claude' &&
      previous[previous.length - 1].kind === 'claude' && !previous.some((row: FeedRow) => row.kind === 'collapsed')) {
      let samePrefix = true
      for (let index = 0; index < messages.length - 1; index += 1) {
        if (messages[index] !== raw[index]) { samePrefix = false; break }
      }
      if (samePrefix) {
        if (messages[messages.length - 1] !== raw[raw.length - 1]) {
          this.messages = previous.slice()
          this.messages[this.messages.length - 1] = messages[messages.length - 1]
          const changedIndex = this.messages.length - 1
          this.listeners.forEach((listener: DataChangeListener) => listener.onDataChange(changedIndex))
        }
        return
      }
    }
    const next = collapseLongTurns(projectFeed(messages), this.expanded)
    this.messages = next
    if (previous.length !== next.length) {
      this.listeners.forEach((listener: DataChangeListener) => listener.onDataReloaded())
      return
    }
    for (let index = 0; index < next.length; index += 1) {
      if (!sameRow(previous[index], next[index])) {
        this.listeners.forEach((listener: DataChangeListener) => listener.onDataChange(index))
      }
    }
  }
}
