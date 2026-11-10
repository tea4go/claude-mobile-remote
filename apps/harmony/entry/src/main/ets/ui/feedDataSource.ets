import type { FeedMessage, BlockFeedMessage } from '../session/model'
import type { Attachment } from '../protocol/types'

export interface ActivityFeedGroup { kind: 'activity'; requestId: string; blocks: Array<BlockFeedMessage> }
export type FeedRow = FeedMessage | ActivityFeedGroup

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
  if (message.kind === 'input') return JSON.stringify([index, message.kind, message.requestId, message.text,
    message.status, message.attachments?.map((item: Attachment) => item.name + ':' + item.mediaType)])
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
      previous[previous.length - 1].kind === 'claude') {
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
    const next = projectFeed(messages)
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
