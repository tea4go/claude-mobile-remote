import { Tag } from 'antd-mobile'
import type { ConnectionState } from '../session/reducer'

interface SessionStatusProps {
  connection: ConnectionState
  sessionStatus: 'starting' | 'ready' | 'stopped' | 'failed' | null
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

// 状态栏：连接、Session、当前 Turn 与连接时队列提示（D36：不持续展示推断值）。
export default function SessionStatus({
  connection,
  sessionStatus,
  turnStatus,
  queueDepthHint,
}: SessionStatusProps) {
  return (
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
  )
}
