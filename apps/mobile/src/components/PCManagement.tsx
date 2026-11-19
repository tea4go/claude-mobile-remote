import type { ReactNode } from 'react'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Button, Dialog, Input, NavBar, Popup, SwipeAction } from 'antd-mobile'
import SessionIcon from './SessionIcon'
import { DeleteOutline, MoreOutline, RedoOutline } from 'antd-mobile-icons'
import { copyText } from '../native/clipboard'
import { groupPCs, type PCGroup } from './pcGroups'
import type { SessionListItem } from './ProjectSessionList'
import { replacePCIP, type PCProfile } from '../native/pcProfiles'
import type { PCConnectionResult } from '../session/pcConnectionTest'
import { projectDirectoryName, projectGroups } from './projectGroups'

export type TestPC = (pc: PCGroup<SessionListItem>, ip: string, signal: AbortSignal) => Promise<PCConnectionResult[]>
export function pcIP(pc: PCGroup): string {
  for (const record of pc.sessions) { try { return new URL(record.wsUrl).hostname } catch { /* expired */ } }
  return ''
}
function Results({ results, ip }: { results: PCConnectionResult[]; ip?: string }) {
  const [copyFeedback, setCopyFeedback] = useState('')
  return <div className="pc-test-results" aria-live="polite">{results.map((result, index) => <div key={index} role={result.ok ? 'status' : 'alert'} className={`pairing-notice pairing-notice--${result.ok ? 'success' : 'error'}`}><strong>{result.message}</strong>{ip && <div>IP：{ip} <button type="button" aria-label="复制 IP" className="pc-copy-ip" onClick={() => { void copyText(ip).then(() => setCopyFeedback('已复制')).catch(() => setCopyFeedback('复制失败')) }}><span aria-hidden="true">⧉</span></button>{copyFeedback && <span role="status">{copyFeedback}</span>}</div>}{result.port && <div>端口：{result.port}</div>}</div>)}</div>
}
export interface PCFeedback { pcName: string; ip?: string; testing: boolean; results: PCConnectionResult[] }
export function PCConnectionFeedback({ feedback }: { feedback: PCFeedback | null }) {
  const [detailsOpen, setDetailsOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)
  useEffect(() => { setDetailsOpen(feedback !== null && !feedback.testing && !container.current?.closest('[inert]')) }, [feedback])
  if (!feedback || feedback.testing) return null
  const failed = feedback.results.filter(result => !result.ok).length
  const summary = feedback.testing ? `正在测试 ${feedback.pcName}…` : `${feedback.pcName}：${failed ? '连接失败' : '连接成功'}`
  return <div ref={container}>
    <div className={`pairing-notice pc-test-summary pairing-notice--${feedback.testing ? 'info' : failed ? 'error' : 'success'}`} role="status">
      {feedback.testing && <span className="pc-testing"/>}<span>{summary}</span>
      {!feedback.testing && <button type="button" onClick={() => setDetailsOpen(true)}>查看详情</button>}
    </div>
    <Popup visible={detailsOpen} onMaskClick={() => setDetailsOpen(false)} onClose={() => setDetailsOpen(false)} bodyClassName="pc-test-sheet" destroyOnClose>
      <section role="dialog" aria-modal="true" aria-labelledby="pc-test-sheet-title" aria-hidden={!detailsOpen} inert={!detailsOpen}>
        <header><h3 id="pc-test-sheet-title">{feedback.pcName} · 连接测试</h3><button type="button" onClick={() => setDetailsOpen(false)}>关闭</button></header>
        <Results results={feedback.results} ip={feedback.ip}/>
      </section>
    </Popup>
  </div>
}
function DirectoryTags({ paths }: { paths: string[] }) {
  const row = useRef<HTMLSpanElement>(null)
  const measureTags = useRef<HTMLSpanElement>(null)
  const measureEllipsis = useRef<HTMLSpanElement>(null)
  const [visibleCount, setVisibleCount] = useState<number | null>(null)

  useLayoutEffect(() => {
    const outer = row.current
    const tags = measureTags.current
    const ellipsis = measureEllipsis.current
    if (!outer || !tags || !ellipsis) return
    const measure = () => {
      const width = outer.clientWidth
      if (tags.offsetWidth <= width) {
        setVisibleCount(null)
        return
      }
      const gap = Number.parseFloat(getComputedStyle(tags).columnGap) || 0
      const tagWidth = width - ellipsis.offsetWidth - gap
      let used = 0
      let count = 0
      for (const tag of Array.from(tags.children) as HTMLElement[]) {
        const text = tag.firstElementChild as HTMLElement
        const minWidth = tag.offsetWidth - text.offsetWidth + 8
        const remaining = tagWidth - used
        if (remaining < minWidth) break
        count++
        if (tag.offsetWidth > remaining) break
        used += tag.offsetWidth + gap
      }
      setVisibleCount(count)
    }
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(measure)
    observer.observe(outer)
    observer.observe(tags)
    observer.observe(ellipsis)
    return () => observer.disconnect()
  }, [paths])

  const tags = (items: string[]) => items.map(path => <em key={path}><span>{projectDirectoryName(path)}</span></em>)
  return <span ref={row} className="pc-directory-tags">
    <span className="pc-directory-tags__clip">{tags(paths.slice(0, visibleCount ?? paths.length))}</span>
    {visibleCount !== null && <span className="pc-directory-tags__ellipsis" aria-hidden="true">...</span>}
    <span className="pc-directory-tags__measure" aria-hidden="true">
      <span className="pc-directory-tags__clip" ref={measureTags}>{tags(paths)}</span>
      <span className="pc-directory-tags__ellipsis" ref={measureEllipsis}>...</span>
    </span>
  </span>
}

export function PCList({ claimInitialRefresh, allowInitialRefresh = true, pairingHome, selectedPC, sessions, disabled, refreshOnEntry, onSelect, onEdit, onRemove, onTest, onFeedback }: { claimInitialRefresh?: () => boolean; allowInitialRefresh?: boolean; pairingHome?: boolean; selectedPC?: string | null; sessions: SessionListItem[]; disabled?: boolean; refreshOnEntry?: boolean; onSelect: (key: string | null) => void; onEdit: (key: string) => void; onRemove: (pc: PCGroup<SessionListItem>) => Promise<void>; onTest: TestPC; onFeedback?: (feedback: PCFeedback | null) => void }) {
  const [collapsed, setCollapsed] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [feedback, setFeedback] = useState<PCFeedback | null>(null)
  const [testStates, setTestStates] = useState<Record<string, { ip: string; ok: boolean }>>({})
  function publishFeedback(value: PCFeedback | null) { setFeedback(value); onFeedback?.(value) }
  const controller = useRef<AbortController | null>(null)
  const list = useRef<HTMLDivElement>(null)
  const refreshedOnEntry = useRef(false)
  useEffect(() => () => {
    controller.current?.abort()
    controller.current = null
    refreshedOnEntry.current = false
    onFeedback?.(null)
  }, [onFeedback])
  useEffect(() => { if (disabled) { controller.current?.abort(); setTesting(null); setRefreshing(false); setFeedback(null); onFeedback?.(null) } }, [disabled, onFeedback])
  const pcs = groupPCs(sessions)
  const refresh = useCallback(async (targets: typeof pcs, test: TestPC) => {
    const current = new AbortController()
    controller.current?.abort()
    controller.current = current
    setRefreshing(true)
    try {
      await Promise.all(targets.map(async pc => {
        const ip = pcIP(pc)
        const results = await test(pc, ip, current.signal).catch(() => [])
        if (!current.signal.aborted) {
          setTestStates(previous => ({ ...previous, [pc.key]: { ip, ok: results.length > 0 && results.every(result => result.ok) } }))
        }
      }))
    } finally {
      if (!current.signal.aborted) setRefreshing(false)
    }
  }, [])
  useEffect(() => {
    if (!refreshOnEntry || disabled || refreshedOnEntry.current || list.current?.closest('[inert]')) return
    if (claimInitialRefresh) {
      if (!allowInitialRefresh) return
      let mounted = true
      // Wait for the active mount before consuming the app's one automatic refresh.
      queueMicrotask(() => {
        if (mounted && claimInitialRefresh() && pcs.length > 0) void refresh(pcs, onTest)
      })
      return () => { mounted = false }
    }
    if (pcs.length === 0) return
    refreshedOnEntry.current = true
    void refresh(pcs, onTest)
  }, [refreshOnEntry, disabled, pcs, onTest, refresh, claimInitialRefresh, allowInitialRefresh])
  return <div ref={list} className={`pairing-pc-list${refreshOnEntry ? ' pairing-pc-list--compact' : ''}${pairingHome ? ' pairing-pc-list--home' : ''}`} aria-label="已配对电脑">
    {pcs.length > 0 && <div className="pairing-pc-list__header">
      {pairingHome ? <button type="button" className="pairing-pc-list__all" aria-expanded={!collapsed} onClick={() => setCollapsed(value => !value)}>全部电脑 <span aria-hidden="true">{collapsed ? '▸' : '▾'}</span></button> : <button className="pairing-pc-list__all" disabled={disabled} onClick={() => onSelect(null)}>全部电脑</button>}
      <button className="pairing-pc-list__refresh" type="button" aria-label="刷新电脑状态" disabled={disabled || testing !== null || refreshing} onClick={() => void refresh(pcs, onTest)}>
        {refreshing ? <span className="pc-testing"/> : refreshOnEntry ? '刷新' : <RedoOutline aria-hidden="true"/>}
      </button>
    </div>}
    {!collapsed && pcs.map(pc => {
      const tested = testStates[pc.key]?.ip === pcIP(pc) ? testStates[pc.key] : undefined
      const connected = tested ? tested.ok : pc.sessions.some(s => s.status !== 'needsPairing' && s.runtimeState?.connection === 'connected')
      const pending = !tested && pc.sessions.some(s => ['connecting', 'authenticating', 'subscribing'].includes(s.runtimeState?.connection ?? ''))
      const projects = pairingHome
        ? projectGroups(pc.sessions).flatMap(group => group.cwd ? [group.cwd] : [])
        : [...new Set(pc.sessions.map(s => s.runtimeState?.projectCwd).filter((cwd): cwd is string => Boolean(cwd)))]
      return <SwipeAction key={pc.key} className="pairing-pc-swipe" rightActions={disabled ? [] : [
        { key: 'edit', text: <span role="img" aria-label="编辑" className="pairing-pc-action-icon"><MoreOutline aria-hidden="true"/></span>, color: '#ff8a30', onClick: () => { controller.current?.abort(); setTesting(null); setRefreshing(false); onEdit(pc.key) } },
        { key: 'delete', text: <span role="img" aria-label="删除" className="pairing-pc-action-icon"><DeleteOutline aria-hidden="true"/></span>, color: 'danger', onClick: async () => { if (await Dialog.confirm({ content: `删除 ${pc.label} 的全部本机配对？`, confirmText: '删除', cancelText: '取消' })) { controller.current?.abort(); setTesting(null); setRefreshing(false); publishFeedback(null); await onRemove(pc) } } },
      ]}><div className={`pairing-pc-item${pairingHome && selectedPC === pc.key ? " pairing-pc-item--selected" : ""}`}>
        <button type="button" className="pairing-pc-item__select" aria-pressed={pairingHome ? selectedPC === pc.key : undefined} disabled={disabled} onClick={() => onSelect(pc.key)}>
          <SessionIcon name="pc" size={40} style={{ color: connected ? '#31a577' : pending ? '#e6a23c' : '#d7353e' }}/>
          <span><span className="pairing-pc-item__name"><strong>{pc.label}</strong> <small>{pcIP(pc)}</small></span>{pairingHome ? <DirectoryTags paths={projects} /> : <span className="pairing-pc-item__projects">{projects.map(cwd => <em key={cwd}>{projectDirectoryName(cwd)}</em>)}</span>}<span className="sr-only">{connected ? '可用' : pending ? '连接中' : '不可用'}</span></span>
        </button>
        <button type="button" className={pairingHome ? "pairing-pc-item__test mobile-icon-button" : "pairing-pc-item__test"} aria-label={`测试连接 ${pc.label}`} disabled={disabled || testing !== null || refreshing} onClick={async () => {
          const current = new AbortController(); controller.current?.abort(); controller.current = current; setTesting(pc.key); publishFeedback({ pcName: pc.label, ip: pcIP(pc), testing: true, results: [] })
          const fallback: PCConnectionResult[] = [{ sessionId: '', ok: false, port: '', token: '', message: '测试未完成，请重试' }]
          try {
            const results = await onTest(pc, pcIP(pc), current.signal)
            if (!current.signal.aborted) { const final = results.length ? results : fallback; setTestStates(previous => ({ ...previous, [pc.key]: { ip: pcIP(pc), ok: final.every(result => result.ok) } })); publishFeedback({ pcName: pc.label, ip: pcIP(pc), testing: false, results: final }) }
          } catch {
            if (!current.signal.aborted) { setTestStates(previous => ({ ...previous, [pc.key]: { ip: pcIP(pc), ok: false } })); publishFeedback({ pcName: pc.label, ip: pcIP(pc), testing: false, results: fallback }) }
          } finally { if (!current.signal.aborted) setTesting(null) }
        }}>{testing === pc.key ? <span className="pc-testing"/> : <SessionIcon name={pairingHome ? "testConnectButton" : "testConnect"} size={pairingHome ? 22 : 40}/>}</button>
      </div></SwipeAction>
    })}
    {!onFeedback && <PCConnectionFeedback feedback={feedback}/>}
  </div>
}
export function PCEditor({ pc, onBack, onSave, onTest, compactFeedback, homeButton }: { homeButton?: ReactNode; compactFeedback?: boolean; pc: PCGroup<SessionListItem>; onBack: () => void; onSave: (profile: PCProfile) => Promise<void>; onTest: TestPC }) {
  const [name, setName] = useState(pc.label)
  const [ip, setIP] = useState(pcIP(pc))
  const [busy, setBusy] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [results, setResults] = useState<PCConnectionResult[]>([])
  const controller = useRef<AbortController | null>(null)
  useEffect(() => () => controller.current?.abort(), [])
  const edit = (setter: (value: string) => void) => (value: string) => { controller.current?.abort(); setter(value); setError(''); setResults([]); setBusy(false) }
  function valid() { if (!name.trim()) { setError('请输入 PC 名称'); return false } try { replacePCIP('ws://127.0.0.1:7330/session/v1', ip.trim()); return true } catch { setError('请输入有效 IP，不含端口'); return false } }
  return <div className={`pc-editor mobile-settings-page${compactFeedback ? " native-mobile" : ""}`}>
    <div className="mobile-safe-area-header"><NavBar onBack={onBack} right={homeButton}>编辑连接</NavBar></div>
    <div className="pc-editor__body"><div className="pc-editor__fields">
      <label><strong>名称</strong><Input aria-label="PC 名称" placeholder="请输入 PC 名称" maxLength={255} value={name} onChange={edit(setName)} disabled={busy}/></label>
      <label><strong>IP地址</strong><Input aria-label="PC IP地址" placeholder="请输入 IP 地址" value={ip} onChange={edit(setIP)} disabled={busy}/></label>
    </div></div>
    <div className="pc-editor__footer" data-debug-bottom-actions>{error && <div role="alert" className="pairing-notice pairing-notice--error">{error}</div>}<Results results={results} ip={ip.trim()}/>
      <Button block color="primary" fill="outline" disabled={busy} onClick={async () => { if (!valid()) return; const current = new AbortController(); controller.current = current; setBusy(true); setTesting(true); setResults([]); const started = Date.now(); try { const result = await onTest(pc, ip.trim(), current.signal); if (result.every(item => item.ok)) await new Promise(resolve => setTimeout(resolve, Math.max(0, 600 - (Date.now() - started)))); if (!current.signal.aborted) setResults(result) } catch { if (!current.signal.aborted) setResults([{ sessionId: '', ok: false, port: '', token: '', message: '测试未完成，请重试' }]) } finally { if (!current.signal.aborted) { setBusy(false); setTesting(false) } } }}><SessionIcon name="testConnectButton" size={24}/> {busy ? '正在处理…' : '测试连接'}</Button>
      <Button block color="primary" disabled={busy} onClick={async () => { if (!valid()) return; setBusy(true); try { await onSave({ name: name.trim(), ip: ip.trim() }) } catch { setError('保存失败，请重试'); setBusy(false) } }}>确认</Button>
    </div>
    {testing && <div className="pc-editor__loading" role="status" aria-label="正在测试连接"><span className="pc-testing"/></div>}
  </div>
}
