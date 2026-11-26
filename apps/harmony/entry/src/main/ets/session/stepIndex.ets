import type { BlockFeedMessage, FeedMessage } from './model'
import type { SubagentTask, ToolUseContentBlock } from '../protocol/types'

function ensureParent(requestId: string, toolUseId: string, observed: Set<string>,
  ends: Map<string, number>, roots: Map<number, BlockFeedMessage[]>, length: number): void {
  const key = JSON.stringify([requestId, 'tool_use', toolUseId])
  if (observed.has(key)) return
  observed.add(key)
  const root: ToolUseContentBlock = { id: toolUseId, kind: 'tool_use', toolUseId,
    toolClass: 'builtin', toolName: 'Agent', mcpServer: '' }
  const position = ends.get(requestId) ?? length
  const group = roots.get(position) ?? []
  group.push({ kind: 'block', requestId, block: root })
  roots.set(position, group)
}

export function projectSubagentBlocks(messages: FeedMessage[], tasks: SubagentTask[]): FeedMessage[] {
  const available = new Set<string>()
  const ends = new Map<string, number>()
  messages.forEach((message: FeedMessage, index: number) => {
    if (message.kind === 'input' || (message.kind === 'claude' &&
      (message.text.length > 0 || message.status === 'cancelled' || (message.status === 'failed' && message.error !== null)))) {
      ends.set(message.requestId, index + 1)
    }
    if (message.kind === 'block' && message.block.kind !== 'thinking') {
      available.add(JSON.stringify([message.requestId, message.block.kind, message.block.toolUseId]))
    }
  })
  const roots = new Map<number, BlockFeedMessage[]>()
  const children: BlockFeedMessage[] = []
  for (const task of tasks) {
    for (const block of task.blocks ?? []) {
      if (block.kind === 'thinking') continue
      const key = JSON.stringify([task.requestId, block.kind, block.toolUseId])
      if (!available.has(key)) {
        const child: BlockFeedMessage = { kind: 'block', requestId: task.requestId, block }
        children.push(child)
        available.add(key)
      }
    }
  }
  for (const task of tasks) ensureParent(task.requestId, task.toolUseId, available, ends, roots, messages.length)
  // A child can outlive the cached parent or arrive before its live event.
  for (const message of messages) {
    if (message.kind === 'block' && message.block.kind !== 'thinking' && message.block.parentToolUseId) {
      ensureParent(message.requestId, message.block.parentToolUseId, available, ends, roots, messages.length)
    }
  }
  if (roots.size === 0 && children.length === 0) return messages
  const result: FeedMessage[] = []
  messages.forEach((message: FeedMessage, index: number) => {
    result.push(...(roots.get(index) ?? []), message)
  })
  result.push(...(roots.get(messages.length) ?? []), ...children)
  return result
}

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
