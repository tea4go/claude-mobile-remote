import { useEffect, useRef } from 'react'
import { Tag } from 'antd-mobile'
import MessageMarkdown from './MessageMarkdown'
import StructuredBlock, { ToolActivityGroup, type ToolActivity } from './StructuredBlock'
import type { Message } from '../session/reducer'

interface MessageFeedProps {
  messages: Message[]
}

// tool_result 关联工具名（B2 §6）：向前找同一 toolUseId 的 tool_use。
// 重连后 snapshot 不重放历史块，找不到时返回空串，绝不编造名称。
function resolveToolName(messages: Message[], index: number, toolUseId: string): string {
  for (let i = index - 1; i >= 0; i -= 1) {
    const candidate = messages[i]
    if (candidate === undefined) continue
    if (candidate.kind !== 'block') continue
    const block = candidate.block
    if (block.kind === 'tool_use' && block.toolUseId === toolUseId) {
      return block.toolName
    }
  }
  return ''
}

type DisplayItem =
  | { kind: 'message'; message: Exclude<Message, { kind: 'block' }> }
  | { kind: 'block'; block: Extract<Message, { kind: 'block' }>['block']; toolName?: string }
  | { kind: 'tools'; activities: ToolActivity[] }

function buildDisplayItems(messages: Message[]): DisplayItem[] {
  const items: DisplayItem[] = []
  const activities = new Map<string, ToolActivity>()

  for (let index = 0; index < messages.length; index += 1) {
    const message = messages[index]
    if (message === undefined) continue
    if (message.kind !== 'block') {
      items.push({ kind: 'message', message })
      continue
    }
    if (message.block.kind === 'tool_use') {
      const activity: ToolActivity = { tool: message.block }
      activities.set(message.block.toolUseId, activity)
      items.push({ kind: 'tools', activities: [activity] })
      continue
    }
    if (message.block.kind === 'tool_result') {
      const activity = activities.get(message.block.toolUseId)
      if (activity !== undefined) {
        activity.result = message.block
        continue
      }
      items.push({ kind: 'block', block: message.block, toolName: resolveToolName(messages, index, message.block.toolUseId) })
      continue
    }
    items.push({ kind: 'block', block: message.block })
  }

  // 结果已经吸收到对应工具项；文本、Thinking 或输入消息会打断连续工具分组。
  const grouped: DisplayItem[] = []
  for (const item of items) {
    const previous = grouped[grouped.length - 1]
    if (item.kind === 'tools' && previous?.kind === 'tools') {
      previous.activities.push(...item.activities)
    } else {
      grouped.push(item)
    }
  }
  return grouped
}

// 易失消息流（D32）：本连接期内手机输入、来源标记、Claude 流式文本与失败反馈。
// Claude 文本经受限 Markdown 渲染（B1）；输入消息保持纯文本，不解析语法。
export default function MessageFeed({ messages }: MessageFeedProps) {
  const feedRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const nearBottomRef = useRef(true)

  // 用户接近底部时自动滚动；向上阅读时不强制拉回（§8）。
  useEffect(() => {
    if (nearBottomRef.current) {
      bottomRef.current?.scrollIntoView?.({ block: 'end' })
    }
  }, [messages])

  return (
    <div
      ref={feedRef}
      data-testid="message-feed"
      onScroll={() => {
        const feed = feedRef.current
        if (feed === null) return
        nearBottomRef.current =
          feed.scrollHeight - feed.scrollTop - feed.clientHeight <= 48
      }}
      style={{ display: 'flex', flexDirection: 'column', gap: 12, height: '100%', overflowY: 'auto', padding: '8px 0' }}
    >
      {buildDisplayItems(messages).map((item, index) => {
        if (item.kind === 'tools') {
          return <ToolActivityGroup key={`tools-${index}`} activities={item.activities} />
        }
        if (item.kind === 'block') {
          return (
            <div key={`block-${index}`} style={{ alignSelf: 'flex-start', maxWidth: '90%' }}>
              <StructuredBlock block={item.block} toolName={item.toolName} />
            </div>
          )
        }
        const message = item.message
        if (message.kind === 'input') {
          // 输入消息（A5.1）：终端与其他手机输入无确认状态；本连接手机
          // 输入按 command_result 显示已接收/结果未知。
          const sourceLabel = message.source === 'terminal' ? '电脑端' : '手机端'
          const statusLabel =
            message.status === 'accepted'
              ? ' · 已接收'
              : message.status === 'unknown'
                ? ' · 结果未知（连接已断开）'
                : ''
          return (
            <div key={index} style={{ alignSelf: 'flex-end', maxWidth: '80%' }}>
              <div style={{ background: '#e6f4ff', borderRadius: 8, padding: '8px 12px' }}>
                {message.text}
              </div>
              <div style={{ fontSize: 12, color: '#888', textAlign: 'right' }}>
                {sourceLabel}
                {statusLabel}
              </div>
            </div>
          )
        }
        return (
          <div key={`message-${index}`} style={{ alignSelf: 'flex-start', maxWidth: '90%' }}>
            <Tag>Claude</Tag>
            {message.text !== '' && (
              <div
                style={{
                  background: '#fff',
                  borderRadius: 8,
                  padding: '8px 12px',
                  marginTop: 4,
                  wordBreak: 'break-word',
                }}
              >
                <MessageMarkdown text={message.text} />
              </div>
            )}
            {/* 取消生效与“已请求取消”必须可区分（B2 §7）：此标记只在终态出现。 */}
            {message.status === 'cancelled' && <Tag color="warning">已取消</Tag>}
            {message.status === 'failed' && message.error !== null && (
              <div style={{ fontSize: 12, color: '#e00' }}>Turn 失败：{message.error.message}</div>
            )}
          </div>
        )
      })}
      <div ref={bottomRef} />
    </div>
  )
}
