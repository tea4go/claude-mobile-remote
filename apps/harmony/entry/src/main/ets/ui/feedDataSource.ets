import type { FeedMessage } from '../session/model'
import type { Attachment } from '../protocol/types'

// Keep displayed changes in keys so streamed text/status replaces its visible row.
// Attachment bytes and pairing credentials must never enter render keys.
export function feedRenderKey(message: FeedMessage, index: number): string {
  if (message.kind === 'input') return JSON.stringify([index, message.kind, message.requestId, message.text,
    message.status, message.attachments?.map((item: Attachment) => item.name + ':' + item.mediaType)])
  return `${index}-${JSON.stringify(message)}`
}

export class FeedDataSource implements IDataSource {
  private messages: Array<FeedMessage> = []
  private listeners: Array<DataChangeListener> = []
  totalCount(): number { return this.messages.length }
  getData(index: number): FeedMessage { return this.messages[index] }
  registerDataChangeListener(listener: DataChangeListener): void {
    if (this.listeners.indexOf(listener) < 0) this.listeners.push(listener)
  }
  unregisterDataChangeListener(listener: DataChangeListener): void {
    this.listeners = this.listeners.filter((value: DataChangeListener) => value !== listener)
  }
  replace(messages: Array<FeedMessage>): void {
    this.messages = messages
    this.listeners.forEach((listener: DataChangeListener) => listener.onDataReloaded())
  }
}
