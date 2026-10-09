import { Tag } from 'antd-mobile'
import type { ConnectionState } from '../session/reducer'
import type { ErrorInfo } from '../protocol/types'
import { FONT_SIZE_COMPACT, FONT_SIZE_SECONDARY } from './ui'

interface SessionStatusProps {
  connection: ConnectionState
  sessionStatus: 'starting' | 'ready' | 'stopped' | 'failed' | null
  sessionError: ErrorInfo | null
  turnStatus: string
  queueDepthHint: number
  variant?: 'full' | 'compact' | 'details'
}

const CONNECTION_LABELS: Record<ConnectionState, string> = {
  connecting: '连接中',
  authenticating: '认证中',
  subscribing: '订阅中',
  connected: '已连接',
  disconnected: '已断开',
}

const SESSION_LABELS: Record<string, string> = {
  starting: '启动中',
  ready: '就绪',
  stopped: '已停止',
  failed: '失败',
}

const TURN_LABELS: Record<string, string> = {
  idle: '空闲',
  running: '运行中',
  waiting: '等待中',
  completed: '已完成',
  cancelled: '已取消',
  failed: '失败',
}

// 终态横幅文案（B2 §6）：只告知会话已结束，不推测原因。
const TERMINAL_BANNERS: Record<string, string> = {
  stopped: '会话已结束，不再接收新输入',
  failed: '会话已失败，不再接收新输入',
}

// 状态栈：连接、Session、当前 Turn 与连接时队列提示（D36：不持续展示推断值）。
// snapshot / session_status 带回的 sessionError 在此展示原因（B1）。
export default function SessionStatus({
  connection,
  sessionStatus,
  sessionError,
  turnStatus,
  queueDepthHint,
  variant = 'full',
}: SessionStatusProps) {
  const terminalBanner =
    sessionStatus === null ? undefined : TERMINAL_BANNERS[sessionStatus]

  if (variant === 'compact') {
    const sessionLabel = sessionStatus === null ? '' : ` · ${SESSION_LABELS[sessionStatus] ?? sessionStatus}`
    return <span data-testid="session-status-compact" title={`${CONNECTION_LABELS[connection]}${sessionLabel} · ${TURN_LABELS[turnStatus] ?? turnStatus}`}>
      <span className="session-detail-connection-line"><span>{CONNECTION_LABELS[connection]}</span>
        {sessionLabel !== '' && <span className="session-detail-secondary-status">· {SESSION_LABELS[sessionStatus!] ?? sessionStatus}</span>}
      </span>
      <span>Turn：{TURN_LABELS[turnStatus] ?? turnStatus}</span>
    </span>
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {variant === 'full' && <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
        <Tag color={connection === 'connected' ? 'success' : 'default'}>
          {CONNECTION_LABELS[connection]}
        </Tag>
        {sessionStatus !== null && (
          <Tag color={sessionStatus === 'failed' ? 'danger' : 'default'}>
            会话：{SESSION_LABELS[sessionStatus] ?? sessionStatus}
          </Tag>
        )}
        <Tag>{`Turn：${TURN_LABELS[turnStatus] ?? turnStatus}`}</Tag>
        {queueDepthHint > 0 && <Tag color="warning">连接时有 {queueDepthHint} 条等待任务</Tag>}
      </div>}
      {terminalBanner !== undefined && (
        <div
          data-testid="session-terminal-banner"
          style={{
            fontSize: FONT_SIZE_SECONDARY,
            color: '#8a5a00',
            background: '#fff7e6',
            border: '1px solid #ffe0a3',
            borderRadius: 6,
            padding: '6px 8px',
          }}
        >
          {terminalBanner}
        </div>
      )}
      {sessionError !== null && (
        <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#e00' }}>会话错误：{sessionError.message}</div>
      )}
    </div>
  )
}
