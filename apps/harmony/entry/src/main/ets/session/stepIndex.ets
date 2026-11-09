import type { FeedMessage } from './model'

export function toolNamesByTurn(messages: Array<FeedMessage>): Map<string, string> {
  const result: Map<string, string> = new Map()
  for (const item of messages) {
    if (item.kind === 'block' && item.block.kind === 'tool_use' && item.block.toolUseId.length > 0) {
      result.set(JSON.stringify([item.requestId, item.block.toolUseId]), item.block.toolName)
    }
  }
  return result
}
export function toolNameForResult(index: Map<string, string>, requestId: string, toolUseId: string): string {
  return index.get(JSON.stringify([requestId, toolUseId])) ?? ''
}

export function toolStatusesByTurn(messages: Array<FeedMessage>): Map<string, string> {
  const statuses: Map<string, string> = new Map()
  const terminal: Map<string, string> = new Map()
  for (const item of messages) {
    if (item.kind === 'claude' && item.status !== 'streaming') terminal.set(item.requestId, item.status)
    if (item.kind === 'block' && item.block.kind === 'tool_result') {
      statuses.set(JSON.stringify([item.requestId, item.block.toolUseId]), item.block.isError ? '失败' : '已完成')
    }
  }
  for (const item of messages) {
    if (item.kind !== 'block' || item.block.kind !== 'tool_use') continue
    const key = JSON.stringify([item.requestId, item.block.toolUseId])
    if (!statuses.has(key)) {
      const status = terminal.get(item.requestId)
      statuses.set(key, status === undefined ? '进行中' : status === 'cancelled' ? '已中断' : '结果未收到')
    }
  }
  return statuses
}
