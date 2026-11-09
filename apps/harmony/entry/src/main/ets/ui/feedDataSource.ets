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

export class FeedDataSource implements IDataSource {
  private messages: Array<FeedRow> = []
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
    this.messages = projectFeed(messages)
    this.listeners.forEach((listener: DataChangeListener) => listener.onDataReloaded())
  }
}
