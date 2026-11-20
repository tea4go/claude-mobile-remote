import { useLayoutEffect, useRef, useState } from 'react'
import { Button, Dialog, Input, List, Popover, SwipeAction, type InputRef } from 'antd-mobile'
import MiddleEllipsisPath from './MiddleEllipsisPath'
import SessionIcon from './SessionIcon'
import { PairingInputDialog } from './PairingFeedback'
import { DeleteOutline, EditSOutline, MoreOutline } from 'antd-mobile-icons'
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
  compactPCHeader?: boolean
  revisedLayout?: boolean
  hideHeadingArrow?: boolean
  recentlyViewedId?: string | null
  selectedPC?: string | null
  managementBusy?: Record<string, boolean>
  managementFeedback?: Record<string, string | null>
  onCreate?(sessionId: string): void
  onPairProject?(pcId: string, cwd: string): void
  onError?(message: string): void
  onAdd?(): void
  onOpen(sessionId: string): void
  onRemove(sessionId: string): void
  onRename(sessionId: string, name: string): void
}

const MAX_SESSION_NAME_LENGTH = 40

function limitSessionName(name: string): string {
  return Array.from(name).slice(0, MAX_SESSION_NAME_LENGTH).join('')
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

function maskPairingToken(token: string): string {
  return token.length > 9 ? `${token.slice(0, 5)}***${token.slice(-4)}` : '***'
}

const turnLabels = { complete: '已完成', needCheck: '等待处理', processing: '进行中', interrupted: '已中断' }

function sessionTurnIcon(session: SessionListItem): keyof typeof turnLabels | null {
  const state = session.runtimeState
  if (!state || state.cachedAt != null || session.status === 'needsPairing' || state.turnStatus == null) return null
  if (state.sessionStatus === 'failed') return 'interrupted'
  if (state.pendingAction || state.turnStatus === 'waiting') return 'needCheck'
  if (state.turnStatus === 'failed' || state.turnStatus === 'cancelled') return 'interrupted'
  if (state.turnStatus === 'running') return 'processing'
  return 'complete'
}

function connectionLabel(session: SessionListItem): { tone: string; label: string } {
  if (session.status === 'needsPairing') return { tone: 'attention', label: '需要重新配对' }
  switch (session.runtimeState?.connection) {
    case 'connected': return { tone: 'connected', label: '已连接' }
    case 'connecting': return { tone: 'attention', label: '连接中' }
    case 'authenticating': return { tone: 'attention', label: '认证中' }
    case 'subscribing': return { tone: 'attention', label: '同步中' }
    default: return { tone: 'disconnected', label: '已断开' }
  }
}

function activityTime(session: SessionListItem): string {
  const value = session.runtimeState?.lastActivityAt ?? session.runtimeState?.createdAt
  if (!value) return ''
  return formatActivityTime(value)
}

export default function ProjectSessionList({ sessions, compactPCHeader, revisedLayout, hideHeadingArrow, recentlyViewedId, selectedPC = null, managementBusy = {}, managementFeedback = {}, onCreate, onPairProject, onError, onAdd, onOpen, onRemove, onRename }: Props) {
  // Keep the initial display order while the list is visible; add newly paired sessions at the end.
  const [initialOrder] = useState(() => revisedLayout
    ? groupPCs(sessions).flatMap(pc => projectGroups(pc.sessions).flatMap(group => group.ids))
    : projectGroups(sessions).flatMap(group => group.ids))
  const order = [...initialOrder, ...sessions.map((session) => session.sessionId).filter((id) => !initialOrder.includes(id))]
  const ordered = [...sessions].sort((a, b) => order.indexOf(a.sessionId) - order.indexOf(b.sessionId))
  const pcs = groupPCs(ordered)
  const visible = selectedPC === null ? pcs : pcs.filter((pc) => pc.key === selectedPC)
  const count = visible.reduce((sum, pc) => sum + pc.sessions.length, 0)
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [removingId, setRemovingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [menuId, setMenuId] = useState<string | null>(null)
  const [collapseByDefault, setCollapseByDefault] = useState(false)
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})

  const trimmedName = name.trim()
  const renameInput = useRef<InputRef>(null)
  useLayoutEffect(() => {
    if (!compactPCHeader || renamingId === null) return
    const input = renameInput.current?.nativeElement
    input?.focus()
    input?.select()
  }, [compactPCHeader, renamingId])

  const projectKeys = visible.flatMap(pc => projectGroups(pc.sessions).map(group => `${pc.key}:${group.cwd ?? ''}`))
  const hasExpandedDirectory = projectKeys.some(key => !(collapsed[key] ?? collapseByDefault))

  return <div className={`project-session-list${revisedLayout ? " project-session-list--revised" : ""}`}>
    {compactPCHeader && renamingId !== null && <PairingInputDialog
      title="重命名会话"
      description="请输入本地名称，最多 40 个字符"
      submitText="保存"
      submitDisabled={trimmedName === ''}
      preserveInputFocus
      onCancel={() => setRenamingId(null)}
      onSubmit={() => {
        if (trimmedName !== '') onRename(renamingId, trimmedName)
        setRenamingId(null)
      }}
    >
      <Input ref={renameInput} className="session-rename-input" aria-label="会话名称" value={name} onChange={value => setName(limitSessionName(value))} placeholder="输入本地名称" />
    </PairingInputDialog>}
    {compactPCHeader && <Dialog
      visible={removingId !== null}
      content="删除此会话的本机配对？"
      destroyOnClose
      actions={[[{ key: 'cancel', text: '取消' }, { key: 'remove', text: '删除', danger: true }]]}
      onAction={action => {
        if (action.key === 'remove' && removingId !== null) onRemove(removingId)
        setRemovingId(null)
      }}
    />}
    <div className="project-session-list__heading-row"><h2 className="project-session-list__title">{revisedLayout && projectKeys.length > 0 ? <button type="button" aria-expanded={hasExpandedDirectory} onClick={() => {
      setCollapseByDefault(hasExpandedDirectory)
      setCollapsed({})
    }}>会话列表{!hideHeadingArrow && <span aria-hidden="true"> {hasExpandedDirectory ? '▾' : '▸'}</span>}</button> : '会话列表'}</h2><span className="project-session-list__count">{count} 个会话</span></div>
    {sessions.length === 0 && <p>还没有已配对的会话，请扫描或粘贴配对信息。</p>}
    {visible.map((pc) => {
      const byId = new Map(pc.sessions.map((session) => [session.sessionId, session]))
      // Preserve the order established above instead of re-sorting on each streamed event.
      const projects = projectGroups(pc.sessions.map((session) => ({ ...session, runtimeState: session.runtimeState ? { ...session.runtimeState, lastActivityAt: null, createdAt: null } : undefined })))
      const connected = pc.sessions.filter((session) => session.status !== 'needsPairing' && session.runtimeState?.connection === 'connected').length
      const connecting = pc.sessions.some((session) => session.status !== 'needsPairing' &&
        ['connecting', 'authenticating', 'subscribing'].includes(session.runtimeState?.connection ?? ''))
      const token = pc.sessions[0]?.pairToken
      const tokenLabel = token ? revisedLayout ? maskPairingToken(token) : '••••' + (token.length > 8 ? token.slice(-4) : '') : '需重新配对'
      const pcStatus = connected > 0 ? 'connected' : connecting ? 'connecting' : 'disconnected'
      return <section key={pc.key} className="project-session-list__pc">
        <div className="project-session-list__pc-card" aria-label={`当前电脑 ${pc.label}`}>
          <span className="project-session-list__pc-icon"><SessionIcon name="pc" size={36} /></span>
          <span className="project-session-list__pc-text">
            <strong>{pc.label}</strong>
            <small>Token：{tokenLabel}</small>
          </span>
          <span className="project-session-list__pc-status">
            <small>{(() => { try { return new URL(pc.sessions[0]!.wsUrl).hostname } catch { return '' } })()}</small>
            <span className={`project-session-list__pc-tag project-session-list__pc-tag--${pcStatus}`}>{connected > 0 ? '已连接' : connecting ? '连接中' : '暂未连接'}</span>
          </span>
        </div>
        {projects.map((group) => {
          const key = `${pc.key}:${group.cwd ?? ''}`
          const isCollapsed = collapsed[key] ?? collapseByDefault
          const members = group.ids.map((id) => byId.get(id)!).filter(Boolean)
          const sources = members.filter((session) => session.capabilities?.includes('create_session') && session.status !== 'needsPairing' && session.runtimeState?.connection === 'connected' && session.runtimeState.sessionStatus !== 'stopped' && session.runtimeState.sessionStatus !== 'failed' && session.runtimeState.pendingCommand === null && !managementBusy[session.sessionId])
          const busy = members.some((session) => managementBusy[session.sessionId])
          const canCreate = group.cwd !== null && sources.length > 0 && !busy && onCreate !== undefined
          const hasConnection = members.some((session) => session.status !== 'needsPairing' && session.runtimeState?.connection === 'connected')
          return <section className="project-session-list__project" key={key}>
            <div className="project-session-list__project-heading"><button className="project-session-list__project-toggle" aria-expanded={!isCollapsed} onClick={() => setCollapsed((current) => ({ ...current, [key]: !isCollapsed }))}><strong><SessionIcon name={revisedLayout ? (isCollapsed ? "collapse" : "expand") : (isCollapsed ? "folderClosed" : "folder")} size={16} style={{ color: "#8e9cb0", marginRight: 6 }} />{group.title}</strong>{revisedLayout && group.cwd ? <MiddleEllipsisPath path={group.cwd} /> : <span title={group.cwd ?? undefined}>{group.cwd ?? '工作目录未知'}</span>}</button>
              {(!compactPCHeader || revisedLayout) && <button type="button" className={revisedLayout ? "project-session-list__project-add mobile-icon-button" : "project-session-list__project-add"} aria-label={`在 ${group.title} 新建会话`} onClick={() => {
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
              }}><SessionIcon name="add" size={14} /></button>}
            </div>
            {members.map((session) => managementFeedback[session.sessionId] && <p role="status" key={`feedback-${session.sessionId}`}>{managementFeedback[session.sessionId]}</p>)}
            {!isCollapsed && <List className="project-session-list__group">
            {members.map((session) => {
              const id = session.sessionId
              const status = statusLabel(session)
              const time = activityTime(session)
              const turnIcon = revisedLayout ? sessionTurnIcon(session) : null
              const connection = revisedLayout ? connectionLabel(session) : null
              const turnLabel = turnIcon && (connection?.tone === 'connected' ? turnLabels[turnIcon] : `上次同步：${turnLabels[turnIcon]}`)
              const isRecent = recentlyViewedId === id
              const item = <List.Item key={id} className={`project-session-list__item${isRecent ? ' project-session-list__item--recent' : ''}`} onClick={() => onOpen(id)} arrowIcon={false} aria-label={`进入 ${displayTitle(session)}`}
                extra={!compactPCHeader && <span onClick={(event) => event.stopPropagation()}><Popover.Menu trigger="click" placement="bottom-end" visible={menuId === id} onVisibleChange={(visible) => setMenuId(visible ? id : null)} actions={[
                  { key: 'rename', text: '重命名', onClick: () => { setName(session.customName ? session.name : displayTitle(session)); setRenamingId(id); setMenuId(null) } },
                  { key: 'remove', text: '删除', onClick: () => { onRemove(id); setMenuId(null) } },
                ]}><Button size="mini" fill="none" className="project-session-list__more" aria-label={`更多操作 ${displayTitle(session)}`}><MoreOutline aria-hidden="true" /></Button></Popover.Menu></span>}>
                <div data-testid="session-list-item" data-session-id={id} className="project-session-list__main">
                  {!compactPCHeader && renamingId === id ? <div className="project-session-list__rename" onClick={(event) => event.stopPropagation()}><Input data-testid="rename-input" value={name} onChange={setName} placeholder="输入本地名称" /><Button size="mini" onClick={() => { onRename(id, name); setRenamingId(null) }}>保存</Button><Button size="mini" onClick={() => setRenamingId(null)}>取消</Button></div> : connection ? <>
                    <div className="project-session-list__row">
                      <div className="project-session-list__heading">
                        <span className={`project-session-list__dot project-session-list__dot--${connection.tone}`} role="img" aria-label={connection.label} />
                        <strong className="project-session-list__session-title">{displayTitle(session)}</strong>
                        {session.unread && <span className="project-session-list__unread" role="img" aria-label="未读" />}
                        {isRecent && <span className="sr-only">刚刚查看</span>}
                      </div>
                      <span className="project-session-list__time">{time}</span>
                    </div>
                    <div className="project-session-list__row">
                      <span className="project-session-list__preview">{preview(session)}</span>
                      {turnIcon && <span role="img" aria-label={turnLabel ?? undefined} title={turnLabel ?? undefined} className={turnIcon === 'processing' && connection.tone === 'connected' ? 'project-session-list__processing' : undefined}><SessionIcon name={turnIcon} size={16} /></span>}
                    </div>
                  </> : <>
                    <div className="project-session-list__heading"><strong className="project-session-list__session-title">{displayTitle(session)}</strong>{session.unread && <span className="project-session-list__unread" aria-label="未读" title="未读" />}{isRecent && <span className="project-session-list__recent">刚刚查看</span>}</div>
                    <span className="project-session-list__preview">{preview(session)}</span>
                    <div className="project-session-list__meta"><span className={`project-session-list__status${status === '已连接' || status === '进行中' ? ' project-session-list__status--connected' : status === '等待处理' || status === '需要重新配对' || status === '连接中' || status === '认证中' || status === '同步中' ? ' project-session-list__status--attention' : ''}`}>{status}</span><span>{time}</span></div>
                  </>}
                </div>
              </List.Item>
              if (!compactPCHeader) return item
              return <SwipeAction key={id} className="pairing-pc-swipe project-session-list__swipe" rightActions={[
                { key: 'rename', text: <span role="img" aria-label="重命名" className="pairing-pc-action-icon"><EditSOutline aria-hidden="true" /></span>, color: '#ff8a30', onClick: () => {
                  setName(limitSessionName(displayTitle(session)))
                  setRenamingId(id)
                } },
                { key: 'remove', text: <span role="img" aria-label="删除" className="pairing-pc-action-icon"><DeleteOutline aria-hidden="true" /></span>, color: 'danger', onClick: () => setRemovingId(id) },
              ]}>{item}</SwipeAction>
            })}
            </List>}
          </section>
        })}
      </section>
    })}
    {onAdd && <div className="project-session-list__add"><Button color="primary" block aria-label="新增配对" onClick={onAdd}>＋ 新会话</Button></div>}
  </div>
}
