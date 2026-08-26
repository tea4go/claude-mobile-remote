import { useEffect, useRef } from 'react'
import { Tag } from 'antd-mobile'
import MessageMarkdown from './MessageMarkdown'
import type { Message } from '../session/reducer'

interface MessageFeedProps {
  messages: Message[]
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
      {messages.map((message, index) => {
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
          <div key={index} style={{ alignSelf: 'flex-start', maxWidth: '90%' }}>
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
