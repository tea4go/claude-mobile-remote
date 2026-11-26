import type { BlockMessage, Message } from './reducer'
import type { SubagentTask } from '../protocol/types'

export const toolKey = (requestId: string, toolUseId: string) => JSON.stringify([requestId, toolUseId])

export function projectSubagentBlocks(messages: Message[], tasks: SubagentTask[]): Message[] {
  const available = new Set<string>()
  const entries = [...messages]
  const roots = new Map<number, BlockMessage[]>()
  const turnEnds = new Map<string, number>()
  messages.forEach((message, index) => {
    if (message.requestId && (message.kind === 'input' || (message.kind === 'claude' &&
      (message.text !== '' || message.status === 'cancelled' || (message.status === 'failed' && message.error !== null))))) {
      turnEnds.set(message.requestId, index + 1)
    }
    if (message.kind === 'block' && message.block.kind !== 'thinking') {
      available.add(toolKey(message.requestId, message.block.kind + ':' + message.block.toolUseId))
    }
  })
  function ensureParent(requestId: string, toolUseId: string) {
    const key = toolKey(requestId, 'tool_use:' + toolUseId)
    if (available.has(key)) return
    available.add(key)
    const position = turnEnds.get(requestId) ?? messages.length
    const group = roots.get(position) ?? []
    group.push({ kind: 'block', requestId, block: {
      id: toolUseId, kind: 'tool_use', toolUseId, toolClass: 'builtin', toolName: 'Agent', mcpServer: '',
    } })
    roots.set(position, group)
  }
  for (const { requestId, blocks } of tasks) {
    for (const block of blocks ?? []) {
      const key = toolKey(requestId, block.kind + ':' + block.toolUseId)
      if (available.has(key)) continue
      available.add(key)
      entries.push({ kind: 'block', requestId, block })
    }
  }
  for (const task of tasks) ensureParent(task.requestId, task.toolUseId)

  // Children can survive their parent's cache eviction or precede its event.
  // Render them from the parent so neither case creates a top-level step.
  const children = new Map<string, BlockMessage[]>()
  const ordered: Message[] = []
  for (const message of entries) {
    if (message.kind === 'block' && message.block.kind !== 'thinking') {
      const { requestId } = message
      const parentId = message.block.parentToolUseId
      if (!parentId) continue
      ensureParent(requestId, parentId)
      const key = toolKey(requestId, parentId)
      const group = children.get(key) ?? []
      group.push(message)
      children.set(key, group)
    }
  }
  if (roots.size === 0 && children.size === 0 && entries.length === messages.length) return messages
  function emit(message: Message) {
    ordered.push(message)
    if (message.kind !== 'block' || message.block.kind !== 'tool_use') return
    const key = toolKey(message.requestId, message.block.toolUseId)
    const nested = children.get(key)
    if (!nested) return
    children.delete(key)
    nested.forEach(emit)
  }
  entries.forEach((message, index) => {
    roots.get(index)?.forEach(emit)
    if (message.kind !== 'block' || message.block.kind === 'thinking' || !message.block.parentToolUseId) emit(message)
  })
  if (entries.length === messages.length) roots.get(messages.length)?.forEach(emit)
  return ordered
}
