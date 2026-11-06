import { useState } from 'react'
import { Button, Input, List, Popover } from 'antd-mobile'
import SessionIcon from './SessionIcon'
import BottomDrawer from './BottomDrawer'
import { MoreOutline } from 'antd-mobile-icons'
import { defaultSessionName, type SessionRecord } from '../session/store'
import type { MobileState } from '../session/reducer'
import { projectGroups } from './projectGroups'
import { groupPCs } from './pcGroups'
import { formatActivityTime } from './activityTime'
import './ProjectSessionList.css'

export interface SessionListItem extends SessionRecord {
  runtimeState?: MobileState
  unread?: boolean
}

interface Props {
  sessions: SessionListItem[]
  recentlyViewedId?: string | null
  selectedPC?: string | null
  onSelectPC?(pc: string | null): void
  managementBusy?: Record<string, boolean>
  managementFeedback?: Record<string, string | null>
  onCreate?(sessionId: string): void
  onPairProject?(pcId: string, cwd: string): void
  onError?(message: string): void
  onAdd(): void
  onOpen(sessionId: string): void
  onRemove(sessionId: string): void
  onRename(sessionId: string, name: string): void
}

function displayTitle(session: SessionListItem): string {
  if (session.customName ?? session.name !== defaultSessionName(session.sessionId)) return session.name
  const state = session.runtimeState
  if (state?.sessionTitle) return state.sessionTitle
  if (state?.createdAt) return '新会话'
  return session.name
}

function preview(session: SessionListItem): string {
  const messages = session.runtimeState?.messages ?? []
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i]
    if (message?.kind !== 'input' && message?.kind !== 'claude') continue
    const text = message.text.trim().replace(/\s+/g, ' ')
    if (text) return text.slice(0, 100)
    if (message.kind === 'input' && message.attachments?.length) return message.attachments.map((item) => item.name).join('、').slice(0, 100)
  }
  return session.runtimeState?.pendingAction ? '等待处理' : '暂无会话内容'
}

function statusLabel(session: SessionListItem): string {
  if (session.status === 'needsPairing') return '需要重新配对'
  const state = session.runtimeState
  if (state?.sessionStatus === 'failed') return '会话失败'
  if (state?.sessionStatus === 'stopped') return '已结束'
  if (state?.connection === 'connecting') return '连接中'
  if (state?.connection === 'authenticating') return '认证中'
  if (state?.connection === 'subscribing') return '同步中'
  if (state?.connection !== 'connected') return '已断开'
  if (state.pendingAction) return '等待处理'
  if (state.messages.some((message) => message.kind === 'claude' && message.status === 'streaming')) return '进行中'
  return '已连接'
}

function activityTime(session: SessionListItem): string {
  const value = session.runtimeState?.lastActivityAt ?? session.runtimeState?.createdAt
  if (!value) return ''
  return formatActivityTime(value)
}

export default function ProjectSessionList({ sessions, recentlyViewedId, selectedPC = null, onSelectPC, managementBusy = {}, managementFeedback = {}, onCreate, onPairProject, onError, onAdd, onOpen, onRemove, onRename }: Props) {
  // Keep the initial display order while the list is visible; add newly paired sessions at the end.
  const [initialOrder] = useState(() => projectGroups(sessions).flatMap((group) => group.ids))
  const order = [...initialOrder, ...sessions.map((session) => session.sessionId).filter((id) => !initialOrder.includes(id))]
  const ordered = [...sessions].sort((a, b) => order.indexOf(a.sessionId) - order.indexOf(b.sessionId))
  const pcs = groupPCs(ordered)
  const visible = selectedPC === null ? pcs : pcs.filter((pc) => pc.key === selectedPC)
  const count = visible.reduce((sum, pc) => sum + pc.sessions.length, 0)
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [menuId, setMenuId] = useState<string | null>(null)
  const [filterOpen, setFilterOpen] = useState(false)
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})

  return <div className="project-session-list">
    <div className="project-session-list__heading-row"><h2 className="project-session-list__title">会话列表</h2><span className="project-session-list__count">{count} 个会话</span></div>
    {sessions.length === 0 && <p>还没有已配对的会话，请扫描或粘贴配对信息。</p>}
    {visible.map((pc) => {
      const byId = new Map(pc.sessions.map((session) => [session.sessionId, session]))
      // Preserve the order established above instead of re-sorting on each streamed event.
      const projects = projectGroups(pc.sessions.map((session) => ({ ...session, runtimeState: session.runtimeState ? { ...session.runtimeState, lastActivityAt: null, createdAt: null } : undefined })))
      const connected = pc.sessions.filter((session) => session.status !== 'needsPairing' && session.runtimeState?.connection === 'connected').length
      return <section key={pc.key} className="project-session-list__pc">
        <button className="project-session-list__pc-card" onClick={() => setFilterOpen(true)} aria-expanded={filterOpen} aria-label={`筛选电脑，当前 ${pc.label}`}>
          <span className="project-session-list__pc-icon"><SessionIcon name="pc" size={36} /></span><span className="project-session-list__pc-text"><strong>{pc.label}</strong><small>{connected > 0 ? `${connected} / ${pc.sessions.length} 个会话已连接` : '暂未连接'}</small></span><SessionIcon name="arrow" size={12} />
        </button>
        {projects.map((group) => {
          const key = `${pc.key}:${group.cwd ?? ''}`
          const members = group.ids.map((id) => byId.get(id)!).filter(Boolean)
          const sources = members.filter((session) => session.capabilities?.includes('create_session') && session.status !== 'needsPairing' && session.runtimeState?.connection === 'connected' && session.runtimeState.sessionStatus !== 'stopped' && session.runtimeState.sessionStatus !== 'failed' && session.runtimeState.pendingCommand === null && !managementBusy[session.sessionId])
          const busy = members.some((session) => managementBusy[session.sessionId])
          const canCreate = group.cwd !== null && sources.length > 0 && !busy && onCreate !== undefined
          const hasConnection = members.some((session) => session.status !== 'needsPairing' && session.runtimeState?.connection === 'connected')
          return <section className="project-session-list__project" key={key}>
            <div className="project-session-list__project-heading"><button className="project-session-list__project-toggle" aria-expanded={!collapsed[key]} onClick={() => setCollapsed((current) => ({ ...current, [key]: !current[key] }))}><strong><SessionIcon name={collapsed[key] ? "folderClosed" : "folder"} size={16} style={{ color: "#8e9cb0", marginRight: 6 }} />{group.title}</strong><span title={group.cwd ?? undefined}>{group.cwd ?? '工作目录未知'}</span></button>
              <button className="project-session-list__project-add" aria-label={`在 ${group.title} 新建会话`} onClick={() => {
                if (group.cwd === null) { onError?.('工作目录未知，无法验证'); return }
                if (busy) { onError?.('正在新建会话，请稍候'); return }
                if (!hasConnection) {
                  const pcId = members[0]?.pcId
                  if (!pcId) { onError?.('电脑身份未知，无法验证，请使用底部新会话配对'); return }
                  onPairProject?.(pcId, group.cwd)
                  return
                }
                if (!canCreate) { onError?.('当前连接暂不支持新建会话'); return }
                onCreate(sources[0]!.sessionId)
              }}><SessionIcon name="add" size={14} /></button>
            </div>
            {members.map((session) => managementFeedback[session.sessionId] && <p role="status" key={`feedback-${session.sessionId}`}>{managementFeedback[session.sessionId]}</p>)}
            {!collapsed[key] && <List className="project-session-list__group">
            {members.map((session) => {
              const id = session.sessionId
              const status = statusLabel(session)
              const time = activityTime(session)
              const isRecent = recentlyViewedId === id
              return <List.Item key={id} className={`project-session-list__item${isRecent ? ' project-session-list__item--recent' : ''}`} onClick={() => onOpen(id)} arrowIcon={false} aria-label={`进入 ${displayTitle(session)}`}
                extra={<span onClick={(event) => event.stopPropagation()}><Popover.Menu trigger="click" placement="bottom-end" visible={menuId === id} onVisibleChange={(visible) => setMenuId(visible ? id : null)} actions={[
                  { key: 'rename', text: '重命名', onClick: () => { setName(session.customName ? session.name : displayTitle(session)); setRenamingId(id); setMenuId(null) } },
                  { key: 'remove', text: '删除', onClick: () => { onRemove(id); setMenuId(null) } },
                ]}><Button size="mini" fill="none" className="project-session-list__more" aria-label={`更多操作 ${displayTitle(session)}`}><MoreOutline aria-hidden="true" /></Button></Popover.Menu></span>}>
                <div data-testid="session-list-item" data-session-id={id} className="project-session-list__main">
                  {renamingId === id ? <div className="project-session-list__rename" onClick={(event) => event.stopPropagation()}><Input data-testid="rename-input" value={name} onChange={setName} placeholder="输入本地名称" /><Button size="mini" onClick={() => { onRename(id, name); setRenamingId(null) }}>保存</Button><Button size="mini" onClick={() => setRenamingId(null)}>取消</Button></div> : <>
                    <div className="project-session-list__heading"><strong className="project-session-list__session-title">{displayTitle(session)}</strong>{session.unread && <span className="project-session-list__unread" aria-label="未读" title="未读" />}{isRecent && <span className="project-session-list__recent">刚刚查看</span>}</div>
                    <span className="project-session-list__preview">{preview(session)}</span>
                    <div className="project-session-list__meta"><span className={`project-session-list__status${status === '已连接' || status === '进行中' ? ' project-session-list__status--connected' : status === '等待处理' || status === '需要重新配对' || status === '连接中' || status === '认证中' || status === '同步中' ? ' project-session-list__status--attention' : ''}`}>{status}</span><span>{time}</span></div>
                  </>}
                </div>
              </List.Item>
            })}
            </List>}
          </section>
        })}
      </section>
    })}
    <BottomDrawer compact visible={filterOpen} onMaskClick={() => setFilterOpen(false)}>
      <div className="project-session-list__filter" role="group" aria-label="选择电脑">
        <div className="project-session-list__filter-title"><strong>选择电脑</strong><button aria-label="关闭电脑筛选" onClick={() => setFilterOpen(false)}>✕</button></div>
        <button aria-pressed={selectedPC === null} onClick={() => { onSelectPC?.(null); setFilterOpen(false) }}>全部电脑</button>
        {pcs.map((entry) => <button key={entry.key} aria-pressed={selectedPC === entry.key} onClick={() => { onSelectPC?.(entry.key); setFilterOpen(false) }}>{entry.label}</button>)}
      </div>
    </BottomDrawer>
    <div className="project-session-list__add"><Button color="primary" block aria-label="新增配对" onClick={onAdd}>＋ 新会话</Button></div>
  </div>
}
