import { useState } from 'react'
import { Button, List } from 'antd-mobile'
import type { SessionListItem } from './ProjectSessionList'
import { groupPCs, type PCGroup } from './pcGroups'

interface PCSettingsProps {
  sessions: SessionListItem[]
  onRemovePC(sessionIds: string[]): void
}

function connectionLabel(group: PCGroup<SessionListItem>): string {
  if (group.sessions.some((session) => session.runtimeState?.connection === 'connected' && session.status !== 'needsPairing')) return '已连接'
  if (group.sessions.every((session) => session.status === 'needsPairing')) return '需重新配对'
  if (group.sessions.some((session) => ['connecting', 'authenticating', 'subscribing'].includes(session.runtimeState?.connection ?? ''))) return '连接中'
  return '未连接'
}

export default function PCSettings({ sessions, onRemovePC }: PCSettingsProps) {
  const [confirmKey, setConfirmKey] = useState<string | null>(null)
  const groups = groupPCs(sessions)
  return <div className="pc-settings">
    <p>连接状态来自本机已配对会话。移除设备只清理本机配对，不结束 PC 上的 Claude 会话。</p>
    <List>
      {groups.map((group) => <List.Item key={group.key}
        extra={<div className="pc-settings-heading-actions">
          {confirmKey === group.key && <Button size="mini" fill="none" onClick={() => setConfirmKey(null)}>取消</Button>}
          <Button size="mini" color="danger" fill="none" onClick={() => {
            if (confirmKey !== group.key) { setConfirmKey(group.key); return }
            setConfirmKey(null)
            onRemovePC(group.sessions.map((session) => session.sessionId))
          }}>{confirmKey === group.key ? '确认移除' : '移除此 PC'}</Button>
        </div>}>
        <div className="pc-settings-device"><strong>{group.label}</strong><span>{connectionLabel(group)}</span></div>
      </List.Item>)}
    </List>
    {groups.length === 0 && <p>暂无已配对的 PC</p>}
  </div>
}
