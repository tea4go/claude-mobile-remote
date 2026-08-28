import { useState } from 'react'
import { Button, Card, Input, Tag } from 'antd-mobile'
import type { SessionLocalStatus, SessionRecord } from '../session/store'
import { BLOCK_BUTTON_STYLE, COMPACT_BUTTON_STYLE } from './ui'

interface SessionListProps {
  sessions: SessionRecord[]
  onAdd: () => void
  onOpen: (sessionId: string) => void
  onRemove: (sessionId: string) => void
  onRename: (sessionId: string, name: string) => void
}

const STATUS_LABELS: Record<SessionLocalStatus, string> = {
  new: '未连接',
  connected: '已连接',
  disconnected: '已断开',
  needsPairing: '需要重新配对',
}

const ID_DIGEST_LENGTH = 8

// 只展示 host:port，不显示完整路径，也不显示 Pair Token（硬约束 7）。
function addressDigest(wsUrl: string): string {
  try {
    return new URL(wsUrl).host
  } catch {
    return '地址无效'
  }
}

function lastConnectedLabel(lastConnectedAt: number | null): string {
  if (lastConnectedAt === null) return '从未连接'
  return `最近连接 ${new Date(lastConnectedAt).toLocaleTimeString()}`
}

// 会话列表（B1）：展示页面内存中的已配对记录，一次只进入一个会话。
// 记录随页面刷新清空；这里不做任何持久化。
export default function SessionList({
  sessions,
  onAdd,
  onOpen,
  onRemove,
  onRename,
}: SessionListProps) {
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [draftName, setDraftName] = useState('')

  return (
    <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <h2 style={{ margin: 0 }}>会话列表</h2>
      {sessions.length === 0 ? (
        <p style={{ margin: 0, color: '#666' }}>
          还没有已配对的会话，粘贴配对 JSON 新增一个。刷新页面后列表会清空。
        </p>
      ) : (
        sessions.map((session) => (
          <Card key={session.sessionId} data-testid="session-list-item">
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {renamingId === session.sessionId ? (
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <Input
                    data-testid="rename-input"
                    value={draftName}
                    onChange={setDraftName}
                    placeholder="输入本地名称"
                  />
                  <Button
                    size="mini"
                    color="primary"
                    style={COMPACT_BUTTON_STYLE}
                    onClick={() => {
                      onRename(session.sessionId, draftName)
                      setRenamingId(null)
                    }}
                  >
                    保存
                  </Button>
                  <Button size="mini" style={COMPACT_BUTTON_STYLE} onClick={() => setRenamingId(null)}>
                    取消
                  </Button>
                </div>
              ) : (
                <div style={{ fontWeight: 600 }}>{session.name}</div>
              )}
              <div style={{ fontSize: 12, color: '#888' }}>
                ID {session.sessionId.slice(0, ID_DIGEST_LENGTH)} · {addressDigest(session.wsUrl)}
              </div>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                <Tag color={session.status === 'needsPairing' ? 'warning' : 'default'}>
                  {STATUS_LABELS[session.status]}
                </Tag>
                <span style={{ fontSize: 12, color: '#888' }}>
                  {lastConnectedLabel(session.lastConnectedAt)}
                </span>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Button
                  size="small"
                  color="primary"
                  style={COMPACT_BUTTON_STYLE}
                  onClick={() => onOpen(session.sessionId)}
                >
                  进入
                </Button>
                <Button
                  size="small"
                  style={COMPACT_BUTTON_STYLE}
                  onClick={() => {
                    setRenamingId(session.sessionId)
                    setDraftName(session.name)
                  }}
                >
                  重命名
                </Button>
                <Button
                  size="small"
                  color="danger"
                  style={COMPACT_BUTTON_STYLE}
                  onClick={() => onRemove(session.sessionId)}
                >
                  删除
                </Button>
              </div>
            </div>
          </Card>
        ))
      )}
      <Button color="primary" fill="outline" style={BLOCK_BUTTON_STYLE} onClick={onAdd}>
        新增配对
      </Button>
    </div>
  )
}
