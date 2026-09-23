import { useState } from 'react'
import { Button, Input, List, Popover } from 'antd-mobile'
import { MoreOutline } from 'antd-mobile-icons'
import { defaultSessionName, type SessionRecord } from '../session/store'
import type { MobileState } from '../session/reducer'
import { projectGroups } from './projectGroups'
import './ProjectSessionList.css'

export interface SessionListItem extends SessionRecord {
  runtimeState?: MobileState
  unread?: boolean
}

interface Props {
  sessions: SessionListItem[]
  recentlyViewedId?: string | null
  onAdd(): void
  onOpen(sessionId: string): void
  onRemove(sessionId: string): void
  onRename(sessionId: string, name: string): void
}

function displayTitle(session: SessionListItem): string {
  if (session.customName ?? session.name !== defaultSessionName(session.sessionId)) return session.name
  const state = session.runtimeState
  if (state?.sessionTitle) return state.sessionTitle
  if (state?.createdAt) return `新会话 · ${new Date(state.createdAt).toLocaleString()}`
  return session.name
}

function preview(session: SessionListItem): string {
  const messages = session.runtimeState?.messages ?? []
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i]
    if (message?.kind !== 'input' && message?.kind !== 'claude') continue
    const text = message.text.trim().replace(/\s+/g, ' ')
    if (text) return text.slice(0, 100)
  }
  return session.runtimeState?.pendingAction ? '等待处理' : '暂无会话内容'
}

function statusLabel(session: SessionListItem): string {
  if (session.status === 'needsPairing') return '需要重新配对'
  const state = session.runtimeState
  if (state?.sessionStatus === 'failed') return '会话失败'
  if (state?.sessionStatus === 'stopped') return '已结束'
  if (session.status === 'disconnected') return '已断开'
  if (state?.pendingAction) return '等待处理'
  if (state?.messages.some((message) => message.kind === 'claude' && message.status === 'streaming')) return '进行中'
  return ''
}

function activityTime(session: SessionListItem): string {
  const value = session.runtimeState?.lastActivityAt ?? session.runtimeState?.createdAt
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  return date.toDateString() === now.toDateString()
    ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : date.toLocaleDateString()
}

export default function ProjectSessionList({ sessions, recentlyViewedId, onAdd, onOpen, onRemove, onRename }: Props) {
  // The component remounts on list entry, so order is stable while visible.
  const [initialGroups] = useState(() => projectGroups(sessions))
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [menuId, setMenuId] = useState<string | null>(null)
  const byId = new Map(sessions.map((session) => [session.sessionId, session]))
  const known = new Set(initialGroups.flatMap((group) => group.ids))
  const groups = initialGroups.map((group) => ({ ...group, ids: group.ids.filter((id) => byId.has(id)) }))
  for (const session of sessions) {
    if (known.has(session.sessionId)) continue
    const cwd = session.runtimeState?.projectCwd ?? null
    const group = groups.find((item) => item.cwd === cwd)
    if (group) group.ids.push(session.sessionId)
    else groups.push(projectGroups([session])[0]!)
  }
  return <div className="project-session-list">
    <h2 className="project-session-list__title">会话列表</h2>
    {sessions.length === 0 && <p>还没有已配对的会话，粘贴配对 JSON 新增一个。刷新页面后列表会清空。</p>}
    {groups.filter((group) => group.ids.length > 0).map((group) => <List
      key={group.cwd ?? '_unknown'}
      className="project-session-list__group"
      header={<div className="project-session-list__group-heading"><strong>{group.title}</strong><span title={group.cwd ?? undefined}>{group.cwd ?? '旧版服务未提供工作目录'}</span></div>}
    >
      {group.ids.map((id) => {
        const session = byId.get(id)!
        const status = statusLabel(session)
        const time = activityTime(session)
        const isRecent = recentlyViewedId === id
        return <List.Item
          key={id}
          className={`project-session-list__item${isRecent ? ' project-session-list__item--recent' : ''}`}
          onClick={() => onOpen(id)}
          arrowIcon={false}
          aria-label={`进入 ${displayTitle(session)}`}
          extra={<div className="project-session-list__extra">
            <div className="project-session-list__meta"><span className={`project-session-list__status${status === '等待处理' || status === '需要重新配对' ? ' project-session-list__status--attention' : ''}`}>{status}</span><span className="project-session-list__time">{time}</span></div>
            <span onClick={(event) => event.stopPropagation()}>
              <Popover.Menu
              trigger="click"
              placement="bottom-end"
              visible={menuId === id}
              onVisibleChange={(visible) => setMenuId(visible ? id : null)}
              actions={[
                { key: 'rename', text: '重命名', onClick: () => { setName(session.customName ? session.name : displayTitle(session)); setRenamingId(id); setMenuId(null) } },
                { key: 'remove', text: '删除', onClick: () => { onRemove(id); setMenuId(null) } },
              ]}
              >
                <Button size="mini" fill="none" className="project-session-list__more" aria-label={`更多操作 ${displayTitle(session)}`}><MoreOutline aria-hidden="true" /></Button>
              </Popover.Menu>
            </span>
          </div>}
        >
          <div data-testid="session-list-item" data-session-id={id} className="project-session-list__main">
            {renamingId === id ? <div className="project-session-list__rename" onClick={(event) => event.stopPropagation()}>
              <Input data-testid="rename-input" value={name} onChange={setName} placeholder="输入本地名称" />
              <Button size="mini" onClick={() => { onRename(id, name); setRenamingId(null) }}>保存</Button>
              <Button size="mini" onClick={() => setRenamingId(null)}>取消</Button>
            </div> : <>
              <div className="project-session-list__heading"><strong className="project-session-list__session-title">{displayTitle(session)}</strong>{session.unread && <span className="project-session-list__unread" aria-label="未读" title="未读" />}{isRecent && <span className="project-session-list__recent">刚刚查看</span>}</div>
              <span className="project-session-list__preview">{preview(session)}</span>
            </>}
          </div>
        </List.Item>
      })}
    </List>)}
    <Button color="primary" fill="outline" onClick={onAdd}>新增配对</Button>
  </div>
}
