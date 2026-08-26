import { Tag } from 'antd-mobile'
import type { ConnectionState } from '../session/reducer'
import type { ErrorInfo } from '../protocol/types'

interface SessionStatusProps {
  connection: ConnectionState
  sessionStatus: 'starting' | 'ready' | 'stopped' | 'failed' | null
  sessionError: ErrorInfo | null
  turnStatus: string
  queueDepthHint: number
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

// 状态栈：连接、Session、当前 Turn 与连接时队列提示（D36：不持续展示推断值）。
// snapshot / session_status 带回的 sessionError 在此展示原因（B1）。
export default function SessionStatus({
  connection,
  sessionStatus,
  sessionError,
  turnStatus,
  queueDepthHint,
}: SessionStatusProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
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
      </div>
      {sessionError !== null && (
        <div style={{ fontSize: 12, color: '#e00' }}>会话错误：{sessionError.message}</div>
      )}
    </div>
  )
}
