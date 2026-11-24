import './PendingActionCard.css'
import SessionIcon from './SessionIcon'
import { useEffect, useRef, useState, type RefObject } from 'react'
import { Tag } from 'antd-mobile'
import MessageMarkdown from './MessageMarkdown'
import ActivityGroup, { ActivitySummary, type Activity } from './ActivityGroup'
import type { LoadToolDetail } from './ToolDetailPage'
import { inputDisplayText } from '../session/inputDisplay'
import SelectableMessage from './SelectableMessage'
import { FONT_SIZE_COMPACT } from './ui'
import type { BlockMessage, Message } from '../session/reducer'
import type { SubagentTask } from '../protocol/types'

interface MessageFeedProps {
  messages: Message[]
  subagentTasks?: SubagentTask[]
  scrollContainerRef?: RefObject<HTMLDivElement | null>
  loadToolDetail?: LoadToolDetail
  connected?: boolean
}

// tool_result 关联工具名（B2 §6）：向前找同一 toolUseId 的 tool_use。
// 重连后 snapshot 不重放历史块，找不到时返回空串，绝不编造名称。
function resolveToolName(messages: Message[], index: number, toolUseId: string): string {
  for (let i = index - 1; i >= 0; i -= 1) {
    const candidate = messages[i]
    if (candidate === undefined) continue
    if (candidate.kind !== 'block' || candidate.requestId !== messages[index]?.requestId) continue
    const block = candidate.block
    if (block.kind === 'tool_use' && block.toolUseId === toolUseId) {
      return block.toolName
    }
  }
  return ''
}

type DisplayItem =
  | { kind: 'message'; message: Exclude<Message, { kind: 'block' }> }
  | { kind: 'activities'; requestId: string; activities: Activity[] }
type ActivityItem = Extract<DisplayItem, { kind: 'activities' }>

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

const toolKey = (requestId: string, toolUseId: string) => JSON.stringify([requestId, toolUseId])

function addSubagentSnapshot(messages: Message[], tasks: SubagentTask[]): Message[] {
  if (tasks.length === 0) return messages
  const observed = new Set(messages.flatMap(message => message.kind === 'block' && message.block.kind !== 'thinking'
    ? [toolKey(message.requestId, message.block.kind + ':' + message.block.toolUseId)] : []))
  const roots: BlockMessage[] = []
  const calls = new Map<string, BlockMessage[]>()
  const results: BlockMessage[] = []
  for (const task of tasks) {
    const { requestId, toolUseId } = task
    if (!observed.has(toolKey(requestId, 'tool_use:' + toolUseId))) {
      roots.push({ kind: 'block', requestId, block: {
        id: toolUseId, kind: 'tool_use', toolUseId, toolClass: 'builtin', toolName: 'Agent', mcpServer: '',
      } })
    }
    for (const block of task.blocks ?? []) {
      if (!observed.has(toolKey(requestId, block.kind + ':' + block.toolUseId))) {
        const message = { kind: 'block' as const, requestId, block }
        if (block.kind === 'tool_result') results.push(message)
        else {
          const key = toolKey(requestId, toolUseId)
          const group = calls.get(key)
          if (group) group.push(message)
          else calls.set(key, [message])
        }
      }
    }
  }
  return [...roots, ...messages].flatMap(message => {
    if (message.kind !== 'block' || message.block.kind !== 'tool_use') return [message]
    return [message, ...(calls.get(toolKey(message.requestId, message.block.toolUseId)) ?? [])]
  }).concat(results)
}

function buildDisplayItems(messages: Message[], subagentTasks: SubagentTask[], revised: boolean): DisplayItem[] {
  messages = addSubagentSnapshot(messages, subagentTasks)
  const tasks = new Map(subagentTasks.map(task => [toolKey(task.requestId, task.toolUseId), task]))
  const items: DisplayItem[] = []
  const pendingTools = new Map<string, Extract<Activity, { kind: 'tool' }>>()
  const parentPanels = new Map<string, Extract<Activity, { kind: 'tool' }>>()
  const askTools = new Map<string, Extract<Activity, { kind: 'tool' }>[]>()
  const completedTurns = new Set(messages.filter((message) => message.kind === 'claude' && message.status !== 'streaming').map((message) => message.requestId))

  for (let index = 0; index < messages.length; index += 1) {
    const message = messages[index]
    if (message === undefined) continue
    if (message.kind !== 'block') {
      if (revised && message.kind === 'answer') {
        const pendingQuestions = askTools.get(message.requestId)?.filter(tool => !tool.answers && !tool.result)
        if (pendingQuestions?.length === 1) {
          pendingQuestions[0].answers = message.answers.map(answer => `${answer.question}\n${answer.values.join('、')}`).join('\n\n')
        }
      }
      if (hasVisibleContent(message)) items.push({ kind: 'message', message })
      continue
    }
    const block = message.block
    if (block.kind === 'thinking') {
      items.push({
        kind: 'activities',
        requestId: message.requestId,
        activities: [{ kind: 'thinking', id: block.id, text: block.thinking }],
      })
      continue
    }
    const key = toolKey(message.requestId, block.toolUseId)
    const parentKey = block.parentToolUseId ? toolKey(message.requestId, block.parentToolUseId) : ''
    const parent = (revised ? parentPanels.get(parentKey) : undefined) ?? pendingTools.get(parentKey)
    if (block.kind === 'tool_use') {
      const existing = pendingTools.get(key)
      if (revised && existing) {
        existing.tool = block
        continue
      }
      const activity: Extract<Activity, { kind: 'tool' }> = {
        kind: 'tool',
        id: revised ? key : block.id,
        tool: block,
        subagent: tasks.get(key),
        turnEnded: parent?.subagent ? parent.subagent.status !== 'running' : completedTurns.has(message.requestId),
      }
      pendingTools.set(key, activity)
      if (revised && block.toolClass === 'builtin' && block.toolName === 'AskUserQuestion') {
        const awaiting = askTools.get(message.requestId)
        if (awaiting) awaiting.push(activity)
        else askTools.set(message.requestId, [activity])
      }
      if (parent) {
        parentPanels.set(key, parent)
        const children = parent.children ??= []
        children.push(activity)
      } else items.push({ kind: 'activities', requestId: message.requestId, activities: [activity] })
      continue
    }
    const pending = pendingTools.get(key)
    if (pending !== undefined) {
      pending.result = block
      continue
    }
    const activity: Activity = { kind: 'result', id: revised ? 'result:' + key : block.id, toolName: resolveToolName(messages, index, block.toolUseId), result: block }
    if (parent) (parent.children ??= []).push(activity)
    else items.push({ kind: 'activities', requestId: message.requestId, activities: [activity] })
  }

  // 结果已经吸收到对应工具项；只有文本与输入消息会打断连续活动分组，
  // Thinking 不再单独占位，避免正文被零散披露行切碎。
  const grouped: DisplayItem[] = []
  for (const item of items) {
    const previous = grouped[grouped.length - 1]
    if (item.kind === 'activities' && previous?.kind === 'activities' && item.requestId === previous.requestId) {
      previous.activities.push(...item.activities)
      continue
    }
    grouped.push(item)
  }
  return grouped
}

// 易失消息流（D32）：本连接期内手机输入、来源标记、Claude 流式文本与失败反馈。
// 用户输入和 Claude 文本均使用受限 Markdown 渲染。
function ClaudeTitle({ model }: { model?: string }) {
  return <div style={{ fontSize: FONT_SIZE_COMPACT, color: model ? '#68748a' : '#8c8c8c', overflowWrap: 'anywhere' }}>
    <span className="claude-avatar"><SessionIcon name="logo" size={20} /></span>Claude{model && ` · ${model}`}
  </div>
}

export default function MessageFeed({ messages, subagentTasks = [], scrollContainerRef, loadToolDetail, connected = true }: MessageFeedProps) {
  const feedRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const nearBottomRef = useRef(true)
  const [expandedTurns, setExpandedTurns] = useState<Set<string>>(() => new Set())
  const [drawer, setDrawer] = useState<{ key: number; item: ActivityItem } | null>(null)

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

  const displayItems = buildDisplayItems(messages, subagentTasks, loadToolDetail !== undefined)
  const drawerItem = drawer && (displayItems.find((item): item is ActivityItem =>
    item.kind === 'activities' && item.requestId === drawer.item.requestId &&
    item.activities.some(activity => drawer.item.activities.some(previous => previous.id === activity.id))
  ) ?? drawer.item)
  const steps = (item: ActivityItem, variant: 'inline' | 'card') => loadToolDetail
    ? <ActivitySummary activities={item.activities} variant={variant}
        onOpen={() => setDrawer(current => ({ key: (current?.key ?? 0) + 1, item }))} />
    : <ActivityGroup activities={item.activities} requestId={item.requestId} variant={variant} />
  // Long turns keep the final reply visible while the execution history stays
  // unmounted until explicitly opened. Its original item order is preserved.
  const longTurns = new Map<string, { first: number; last: number; final: number }>()
  for (let i = 0; i < displayItems.length; i += 1) {
    const item = displayItems[i]
    const id = item.kind === 'activities' ? item.requestId : item.message.kind === 'claude' ? item.message.requestId : null
    if (!id) continue
    const group = longTurns.get(id)
    if (!group) longTurns.set(id, { first: i, last: i, final: item.kind === 'message' ? i : -1 })
    else { group.last = i; if (item.kind === 'message') group.final = i }
  }
  const runningSubagentTurns = new Set(subagentTasks.filter(task => task.status === 'running').map(task => task.requestId))
  for (const [id, group] of longTurns) {
    if (runningSubagentTurns.has(id)) { longTurns.delete(id); continue }
    const slice = displayItems.slice(group.first, group.last + 1)
    if (slice.filter(item => item.kind === 'activities').length < 8 || slice.some(item => (item.kind === 'activities' ? item.requestId : item.message.requestId) !== id) || group.final !== group.last) longTurns.delete(id)
  }

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
      {displayItems.map((item, index) => {
        const id = item.kind === 'activities' ? item.requestId : item.message.kind === 'claude' ? item.message.requestId : null
        const group = id ? longTurns.get(id) : undefined
        if (group && index < group.final && !expandedTurns.has(id!)) {
          if (index !== group.first) return null
          return <button key={`collapsed-${id}`} type="button" onClick={() => setExpandedTurns(current => new Set(current).add(id!))}>展开执行过程</button>
        }
        if (group && index === group.first && expandedTurns.has(id!)) {
          // The toggle precedes the first item without changing its position.
          return <div key={`expanded-${id}`}><button type="button" onClick={() => setExpandedTurns(current => { const next = new Set(current); next.delete(id!); return next })}>收起执行过程</button>{item.kind === 'activities' ? steps(item, 'card') : item.message.kind === 'claude' && <><ClaudeTitle model={item.message.model} /><SelectableMessage><MessageMarkdown text={item.message.text} /></SelectableMessage></>}</div>
        }
        if (item.kind === 'activities') {
          // 结构化活动是行式摘要（参考图），占满行宽而不是气泡宽度。
          const previous = displayItems[index - 1]
          return (
            <div key={`activities-${item.requestId}-${item.activities[0]?.id}`} style={{ alignSelf: 'stretch' }}>
              {steps(item, previous?.kind === 'message' && previous.message.kind === 'claude' ? 'inline' : 'card')}
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
          const tip = loadToolDetail
            ? message.status === 'unknown' ? '结果未知（连接已断开）'
              : connected && message.status === 'accepted' && !message.processingStarted ? '等待处理' : ''
            : sourceLabel + statusLabel
          return (
            <div key={index} style={{ alignSelf: 'flex-end', maxWidth: '80%' }}>
              <SelectableMessage selectionTone="inverse" style={{ background: '#3478f6', color: '#fff', borderRadius: 20, padding: '12px 16px' }}>
                <MessageMarkdown text={inputDisplayText(message.text)} />
                {message.attachments?.map((attachment, i) => <div key={i} style={{ marginTop: 6 }}>
                  {attachment.data && /^image\/(jpeg|png|gif|webp)$/.test(attachment.mediaType) && <img alt={attachment.name} src={`data:${attachment.mediaType};base64,${attachment.data}`} style={{ display: 'block', maxWidth: '100%', maxHeight: 240, borderRadius: 8, objectFit: 'contain' }} />}
                  <span style={{ fontSize: FONT_SIZE_COMPACT }}>{attachment.name}</span>
                </div>)}
              </SelectableMessage>
              {tip && <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#888', textAlign: 'right' }}>{tip}</div>}
            </div>
          )
        }
        if (message.kind === 'answer') {
          // 你的回答（D52）：答案由服务端回带，因此别的端答的也看得到。这里只显示
          // 问题与所选取值，不含任何工具结果。
          return (
            <div key={`answer-${index}`} data-testid="answer-echo" className="answer-echo">
              <div className="answer-echo-title"><strong><span aria-hidden="true">✓ &nbsp;</span>你的回答</strong><span>{message.answers.reduce((total, answer) => total + answer.values.length, 0)} 项已选</span></div>
              {message.answers.map((answer) => (
                <div key={answer.questionId} className="answer-echo-question">
                  {answer.header !== '' && <div className="answer-echo-header">{answer.header}</div>}
                  <div className="answer-echo-text">{answer.question}</div>
                  <div className="answer-echo-values">{answer.values.map((value) => <Tag key={value} color="primary" fill="outline">{value}</Tag>)}</div>
                </div>
              ))}
            </div>
          )
        }
        // Claude正文使用浅灰气泡，保留全宽Markdown和横向滚动空间。
        return (
          <div key={`message-${index}`} style={{ alignSelf: 'stretch' }}>
            <ClaudeTitle model={message.model} />
            {message.text !== '' && (
              <SelectableMessage style={{ marginTop: 4, background: '#fff', border: '1px solid #e0e7f1', color: '#34435b', borderRadius: 20, padding: '14px 16px', wordBreak: 'break-word' }}>
                <MessageMarkdown text={message.text} />
              </SelectableMessage>
            )}
            {/* 取消生效与“已请求取消”必须可区分（B2 §7）：此标记只在终态出现。 */}
            {message.status === 'cancelled' && <Tag color="warning">已取消</Tag>}
            {message.status === 'failed' && message.error !== null && (
              <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#e00' }}>Turn 失败：{message.error.message}</div>
            )}
          </div>
        )
      })}
      {drawer && drawerItem && <ActivityGroup key={drawer.key} drawerOnly
        activities={drawerItem.activities} requestId={drawerItem.requestId} loadToolDetail={loadToolDetail} />}
      <div ref={bottomRef} />
    </div>
  )
}
