import { useState, type CSSProperties, type ReactNode } from 'react'
import { Popup, Tag } from 'antd-mobile'
import type { ContentBlock } from '../protocol/types'
import {
  DRAWER_HEADER_STYLE,
  DRAWER_MASK_STYLE,
  DRAWER_SHEET_STYLE,
  GLASS_PANEL_STYLE,
  ICON_BUTTON_STYLE,
  FONT_SIZE_BODY,
  FONT_SIZE_COMPACT,
  FONT_SIZE_SECONDARY,
} from './ui'

type ToolUse = Extract<ContentBlock, { kind: 'tool_use' }>
type ToolResult = Extract<ContentBlock, { kind: 'tool_result' }>

// 抽屉列表里的一条活动。Thinking 与工具统一进同一个分组：思考块不再打断
// 连续工具，正文里也不会留下零散的披露行。
// `result` 变体只用于重连后孤立的 tool_result（找不到对应 tool_use）。
export type Activity =
  | { kind: 'thinking'; id: string; text: string }
  | { kind: 'tool'; id: string; tool: ToolUse; result?: ToolResult }
  | { kind: 'result'; id: string; toolName: string; result: ToolResult }

type ActivityStatus = 'running' | 'success' | 'failed'

// 抽屉是一个带两级视图的状态机：`view` 总是有值，因此不存在
// 「详情已开但没有目标活动」这类无效组合；关闭后保留 `view`，
// 退场动画期间不会闪回上一级。
type DrawerView = { kind: 'list' } | { kind: 'detail'; activityId: string }

interface DrawerState {
  open: boolean
  view: DrawerView
}

const STATUS_LABEL: Record<ActivityStatus, string> = {
  running: '执行中',
  success: '成功',
  failed: '失败',
}

const STATUS_COLOR: Record<ActivityStatus, 'default' | 'success' | 'danger'> = {
  running: 'default',
  success: 'success',
  failed: 'danger',
}

const STATUS_DOT_COLOR: Record<ActivityStatus, string> = {
  running: '#bfbfbf',
  success: '#00b578',
  failed: '#ff3141',
}

const ROW_DETAIL_LIMIT = 48

const MONO_FONT = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'

// 摘要行：灰色小字占满行宽、无背景与边框，右侧箭头提示可进入抽屉。
const SUMMARY_STYLE: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  width: '100%',
  padding: '4px 0',
  border: 'none',
  background: 'none',
  color: '#8c8c8c',
  fontSize: FONT_SIZE_SECONDARY,
  fontFamily: 'inherit',
  textAlign: 'left',
  cursor: 'pointer',
}

const ROW_STYLE: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  width: '100%',
  padding: '10px 4px',
  border: 'none',
  background: 'none',
  fontFamily: 'inherit',
  textAlign: 'left',
  cursor: 'pointer',
}

const SECTION_LABEL_STYLE: CSSProperties = { fontSize: FONT_SIZE_COMPACT, color: '#8c8c8c' }

const BODY_TEXT_STYLE: CSSProperties = {
  fontSize: FONT_SIZE_SECONDARY,
  color: '#333',
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-word',
}

function statusOf(activity: Activity): ActivityStatus | null {
  if (activity.kind === 'thinking') return null
  if (activity.kind === 'result') return activity.result.isError ? 'failed' : 'success'
  if (activity.result === undefined) return 'running'
  return activity.result.isError ? 'failed' : 'success'
}

function titleOf(activity: Activity): string {
  if (activity.kind === 'thinking') return '思考过程'
  if (activity.kind === 'result') {
    return activity.toolName === '' ? '工具结果' : `工具结果：${activity.toolName}`
  }
  const tool = activity.tool
  return tool.toolClass === 'mcp'
    ? `MCP：${tool.mcpServer} / ${tool.toolName}`
    : `工具：${tool.toolName}`
}

// 协议只下发服务端按白名单抽取的 preview，没有原始 input 也没有工具输出，
// 因此详情里的「输入」只能来自 preview，缺失时如实说明而不是留空。
function inputOf(activity: Activity): string {
  if (activity.kind === 'thinking') return activity.text
  if (activity.kind === 'result') return ''
  return activity.tool.preview ?? ''
}

function firstLine(text: string): string {
  const line = text.split('\n', 1)[0] ?? ''
  return line.length > ROW_DETAIL_LIMIT ? `${line.slice(0, ROW_DETAIL_LIMIT)}…` : line
}

function summaryOf(activities: Activity[]): string {
  const tools = activities.filter((activity) => activity.kind !== 'thinking').length
  const steps = `查看 ${activities.length} 个步骤`
  return tools === 0 ? steps : `${steps}，运行 ${tools} 个工具`
}

function Chevron({
  direction,
  size = 12,
  color = '#bfbfbf',
}: {
  direction: 'right' | 'left'
  size?: number
  color?: string
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 12 12"
      aria-hidden="true"
      style={{
        flexShrink: 0,
        color,
        transform: direction === 'left' ? 'rotate(180deg)' : undefined,
      }}
    >
      <path
        d="M4.5 2.5 8 6l-3.5 3.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function StatusDot({ status }: { status: ActivityStatus | null }) {
  return (
    <span
      aria-hidden="true"
      style={{
        width: 8,
        height: 8,
        flexShrink: 0,
        borderRadius: 4,
        background: status === null ? 'transparent' : STATUS_DOT_COLOR[status],
        border: status === null ? '1px solid #bfbfbf' : 'none',
      }}
    />
  )
}

// 状态跟在标题后面：详情里不再单独占一块区域，省下的高度留给输入正文。
function DrawerHeader({
  title,
  status,
  onBack,
  onClose,
}: {
  title: string
  status?: ActivityStatus | null
  onBack?: () => void
  onClose: () => void
}) {
  return (
    <div style={DRAWER_HEADER_STYLE}>
      {onBack === undefined ? (
        <span style={{ width: 32, flexShrink: 0 }} />
      ) : (
        <button type="button" aria-label="返回" style={ICON_BUTTON_STYLE} onClick={onBack}>
          <Chevron direction="left" size={18} color="#333" />
        </button>
      )}
      <span
        style={{
          flex: 1,
          minWidth: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 6,
        }}
      >
        <span style={{ fontSize: FONT_SIZE_BODY, color: '#333', wordBreak: 'break-word' }}>{title}</span>
        {status !== undefined && status !== null && (
          <Tag
            style={{ fontSize: FONT_SIZE_COMPACT, padding: '0 5px', flexShrink: 0 }}
            color={STATUS_COLOR[status]}
          >
            {STATUS_LABEL[status]}
          </Tag>
        )}
      </span>
      <button type="button" aria-label="关闭" style={ICON_BUTTON_STYLE} onClick={onClose}>
        ✕
      </button>
    </div>
  )
}

function Section({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <span style={SECTION_LABEL_STYLE}>{label}</span>
      <div style={GLASS_PANEL_STYLE}>{children}</div>
    </div>
  )
}

function ActivityRow({ activity, onOpen }: { activity: Activity; onOpen: () => void }) {
  const status = statusOf(activity)
  const detail = firstLine(inputOf(activity))
  return (
    <button type="button" data-testid="activity-row" style={ROW_STYLE} onClick={onOpen}>
      <StatusDot status={status} />
      <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <span style={{ fontSize: FONT_SIZE_SECONDARY, color: '#333', wordBreak: 'break-word' }}>
          {titleOf(activity)}
        </span>
        {detail !== '' && (
          <span style={{ fontSize: FONT_SIZE_COMPACT, color: '#8c8c8c', wordBreak: 'break-word' }}>{detail}</span>
        )}
      </span>
      {status !== null && (
        <Tag style={{ fontSize: FONT_SIZE_COMPACT, padding: '0 5px' }} color={STATUS_COLOR[status]}>
          {STATUS_LABEL[status]}
        </Tag>
      )}
      <Chevron direction="right" />
    </button>
  )
}

// 详情只展示协议已有字段：标题与状态在抽屉标题行，这里只放输入
// （thinking 正文或工具 preview）。工具输出不在协议内，不设输出区，
// 也不用 pre/code 承载正文（B2 §6）。
function ActivityDetail({ activity }: { activity: Activity }) {
  const input = inputOf(activity)
  const emptyMessage =
    activity.kind === 'result'
      ? '工具结果正文未下发，请在电脑端查看'
      : '本工具未提供输入预览'
  return (
    <div
      data-testid="activity-detail"
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 14,
        padding: '12px 12px 20px',
        overflowY: 'auto',
      }}
    >
      {activity.kind === 'thinking' ? (
        <Section label="内容">
          <div style={{ ...BODY_TEXT_STYLE, color: input === '' ? '#8c8c8c' : BODY_TEXT_STYLE.color }}>
            {input === '' ? '本思考步骤未提供可展示内容' : input}
          </div>
        </Section>
      ) : activity.kind === 'result' ? (
        <Section label="结果">
          <div style={{ ...BODY_TEXT_STYLE, color: '#8c8c8c' }}>{emptyMessage}</div>
        </Section>
      ) : (
        <Section label="输入">
          {input === '' ? (
            <div style={{ ...BODY_TEXT_STYLE, color: '#8c8c8c' }}>本工具未提供输入预览</div>
          ) : (
            <div style={{ ...BODY_TEXT_STYLE, fontFamily: MONO_FONT }}>{input}</div>
          )}
        </Section>
      )}
    </div>
  )
}

// B2 会话详情：连续的结构化活动收进一行摘要，点击从底部弹出抽屉，
// 展开不再挤占正文空间；抽屉内点击某条活动推进二级详情视图。
// 两级视图共用同一个 Popup：避开两层遮罩叠加，也不会在切换动画
// 期间同时保留两份内容。
export default function ActivityGroup({ activities }: { activities: Activity[] }) {
  const [drawer, setDrawer] = useState<DrawerState>({ open: false, view: { kind: 'list' } })
  const close = () => setDrawer((current) => ({ ...current, open: false }))
  const openList = () => setDrawer({ open: true, view: { kind: 'list' } })
  const summary = summaryOf(activities)
  const view = drawer.view
  const detail =
    view.kind === 'detail'
      ? activities.find((activity) => activity.id === view.activityId)
      : undefined

  return (
    <div
      data-testid="activity-group"
      data-drawer={drawer.open ? view.kind : 'closed'}
      style={{ width: '100%' }}
    >
      <button
        type="button"
        data-testid="activity-group-summary"
        style={SUMMARY_STYLE}
        onClick={openList}
      >
        <span style={{ flex: 1, wordBreak: 'break-word' }}>{summary}</span>
        <Chevron direction="right" />
      </button>

      <Popup
        visible={drawer.open}
        position="bottom"
        destroyOnClose
        bodyStyle={DRAWER_SHEET_STYLE}
        maskStyle={DRAWER_MASK_STYLE}
        onMaskClick={close}
        onClose={close}
      >
        {detail === undefined ? (
          <>
            <DrawerHeader title={summary} onClose={close} />
            <div
              data-testid="activity-list"
              style={{
                position: 'relative',
                display: 'flex',
                flexDirection: 'column',
                padding: '4px 12px 20px',
                overflowY: 'auto',
              }}
            >
              {/* 时间轴虚线：连接各活动的状态点，纯装饰。 */}
              <span
                aria-hidden="true"
                style={{
                  position: 'absolute',
                  left: 15,
                  top: 18,
                  bottom: 32,
                  borderLeft: '1px dashed rgba(15, 23, 42, 0.14)',
                }}
              />
              {activities.map((activity) => (
                <ActivityRow
                  key={activity.id}
                  activity={activity}
                  onOpen={() =>
                    setDrawer({ open: true, view: { kind: 'detail', activityId: activity.id } })
                  }
                />
              ))}
            </div>
          </>
        ) : (
          <>
            <DrawerHeader
              title={titleOf(detail)}
              status={statusOf(detail)}
              onBack={openList}
              onClose={close}
            />
            <ActivityDetail activity={detail} />
          </>
        )}
      </Popup>
    </div>
  )
}
