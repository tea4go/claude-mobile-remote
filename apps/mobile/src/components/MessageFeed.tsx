import { useEffect, useRef, type RefObject } from 'react'
import { Tag } from 'antd-mobile'
import MessageMarkdown from './MessageMarkdown'
import ActivityGroup, { type Activity } from './ActivityGroup'
import { FONT_SIZE_COMPACT, FONT_SIZE_SECONDARY } from './ui'
import type { Message } from '../session/reducer'

interface MessageFeedProps {
  messages: Message[]
  scrollContainerRef?: RefObject<HTMLDivElement | null>
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
  | { kind: 'activities'; activities: Activity[] }

// turn_started 会先追加一条空文本的 Claude 占位消息，随后的文本事件因为中间夹了
// 内容块而另起一条（reducer）。占位消息没有任何可展示内容，渲染出来只是一个孤立的
// 「Claude」标记，反而把活动摘要与正文切开，因此在展示层跳过。
function hasVisibleContent(message: Exclude<Message, { kind: 'block' }>): boolean {
  if (message.kind === 'input') return true
  // 答案回显只在有答案时入流（reducer），到这里必然可展示。
  if (message.kind === 'answer') return true
  if (message.text !== '') return true
  // 取消与失败即使没有正文也必须留痕（B2 §7）。
  if (message.status === 'cancelled') return true
  return message.status === 'failed' && message.error !== null
}

function buildDisplayItems(messages: Message[]): DisplayItem[] {
  const items: DisplayItem[] = []
  const pendingTools = new Map<string, Extract<Activity, { kind: 'tool' }>>()

  for (let index = 0; index < messages.length; index += 1) {
    const message = messages[index]
    if (message === undefined) continue
    if (message.kind !== 'block') {
      if (hasVisibleContent(message)) items.push({ kind: 'message', message })
      continue
    }
    const block = message.block
    if (block.kind === 'thinking') {
      items.push({
        kind: 'activities',
        activities: [{ kind: 'thinking', id: block.id, text: block.thinking }],
      })
      continue
    }
    if (block.kind === 'tool_use') {
      const activity: Extract<Activity, { kind: 'tool' }> = {
        kind: 'tool',
        id: block.id,
        tool: block,
      }
      pendingTools.set(block.toolUseId, activity)
      items.push({ kind: 'activities', activities: [activity] })
      continue
    }
    const pending = pendingTools.get(block.toolUseId)
    if (pending !== undefined) {
      pending.result = block
      continue
    }
    items.push({
      kind: 'activities',
      activities: [
        {
          kind: 'result',
          id: block.id,
          toolName: resolveToolName(messages, index, block.toolUseId),
          result: block,
        },
      ],
    })
  }

  // 结果已经吸收到对应工具项；只有文本与输入消息会打断连续活动分组，
  // Thinking 不再单独占位，避免正文被零散披露行切碎。
  const grouped: DisplayItem[] = []
  for (const item of items) {
    const previous = grouped[grouped.length - 1]
    if (item.kind === 'activities' && previous?.kind === 'activities') {
      previous.activities.push(...item.activities)
      continue
    }
    grouped.push(item)
  }
  return grouped
}

// 易失消息流（D32）：本连接期内手机输入、来源标记、Claude 流式文本与失败反馈。
// Claude 文本经受限 Markdown 渲染（B1）；输入消息保持纯文本，不解析语法。
export default function MessageFeed({ messages, scrollContainerRef }: MessageFeedProps) {
  const feedRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const nearBottomRef = useRef(true)

  useEffect(() => {
    const scrollContainer = scrollContainerRef?.current
    if (scrollContainer === undefined || scrollContainer === null) return
    const updateNearBottom = () => {
      nearBottomRef.current =
        scrollContainer.scrollHeight - scrollContainer.scrollTop - scrollContainer.clientHeight <= 48
    }
    scrollContainer.addEventListener('scroll', updateNearBottom)
    return () => scrollContainer.removeEventListener('scroll', updateNearBottom)
  }, [scrollContainerRef])

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
        if (scrollContainerRef !== undefined) return
        const feed = feedRef.current
        if (feed === null) return
        nearBottomRef.current =
          feed.scrollHeight - feed.scrollTop - feed.clientHeight <= 48
      }}
      style={{ display: 'flex', flexDirection: 'column', gap: 12, height: scrollContainerRef === undefined ? '100%' : undefined, overflowY: scrollContainerRef === undefined ? 'auto' : undefined, padding: '8px 0' }}
    >
      {buildDisplayItems(messages).map((item, index) => {
        if (item.kind === 'activities') {
          // 结构化活动是行式摘要（参考图），占满行宽而不是气泡宽度。
          return (
            <div key={`activities-${index}`} style={{ alignSelf: 'stretch' }}>
              <ActivityGroup activities={item.activities} />
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
              <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#888', textAlign: 'right' }}>
                {sourceLabel}
                {statusLabel}
              </div>
            </div>
          )
        }
        if (message.kind === 'answer') {
          // 你的回答（D52）：答案由服务端回带，因此别的端答的也看得到。这里只显示
          // 问题与所选取值，不含任何工具结果。
          return (
            <div key={`answer-${index}`} data-testid="answer-echo" style={{ alignSelf: 'stretch' }}>
              <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#8c8c8c' }}>你的回答</div>
              {message.answers.map((answer) => (
                <div key={answer.questionId} style={{ marginTop: 4, wordBreak: 'break-word' }}>
                  <div style={{ fontSize: FONT_SIZE_SECONDARY, color: '#333' }}>
                    {answer.header === '' ? answer.question : `${answer.header} · ${answer.question}`}
                  </div>
                  <div style={{ marginTop: 2, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                    {answer.values.map((value) => (
                      <Tag key={value} color="primary" fill="outline">
                        {value}
                      </Tag>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )
        }
        // Claude 回复占满行宽且不用气泡背景（参考 qoder/sessionDetail.png），
        // 让正文获得最大展示空间；来源标记压成小灰字。
        return (
          <div key={`message-${index}`} style={{ alignSelf: 'stretch' }}>
            <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#8c8c8c' }}>Claude</div>
            {message.text !== '' && (
              <div style={{ marginTop: 2, wordBreak: 'break-word' }}>
                <MessageMarkdown text={message.text} />
              </div>
            )}
            {/* 取消生效与“已请求取消”必须可区分（B2 §7）：此标记只在终态出现。 */}
            {message.status === 'cancelled' && <Tag color="warning">已取消</Tag>}
            {message.status === 'failed' && message.error !== null && (
              <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#e00' }}>Turn 失败：{message.error.message}</div>
            )}
          </div>
        )
      })}
      <div ref={bottomRef} />
    </div>
  )
}
