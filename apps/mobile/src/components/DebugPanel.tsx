import SessionIcon from './SessionIcon'
import { debugButtonPosition } from '../debug/placement'
import { Capacitor } from '@capacitor/core'
import { Preferences } from '@capacitor/preferences'
import { nativeExperience } from '../native/experience'
import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { createPortal } from 'react-dom'
import { debugLog, observeDebugErrors } from '../debug/log'
import './DebugPanel.css'

const positionKey = 'cmr.settings.debugPosition'
const isNativeMobile = () => ['ios', 'android'].includes(Capacitor.getPlatform())

export default function DebugPanel({ pageKey }: { pageKey?: string }) {
  const { enabled } = useSyncExternalStore(debugLog.subscribe, debugLog.getSnapshot)
  useEffect(observeDebugErrors, [])
  return enabled ? <EnabledDebugPanel pageKey={pageKey} /> : null
}

function EnabledDebugPanel({ pageKey }: { pageKey?: string }) {
  const { entries } = useSyncExternalStore(debugLog.subscribe, debugLog.getSnapshot)
  const floatRef = useRef<HTMLButtonElement>(null)
  const preferred = useRef<{ x: number; y: number } | null>(null)
  const drag = useRef<{ id: number; pointerX: number; pointerY: number; x: number; y: number } | null>(null)
  const didDrag = useRef(false)
  const moved = useRef(false)
  const updatePosition = useRef<() => void>(() => {})
  const [open, setOpen] = useState(false)
  const [feedback, setFeedback] = useState('')
  useEffect(() => {
    const openDebug = () => { setFeedback(''); setOpen(true) }
    window.addEventListener('mobile-open-debug', openDebug)
    return () => window.removeEventListener('mobile-open-debug', openDebug)
  }, [])
  useEffect(() => {
    let frame = 0
    const activePage = document.querySelector('[data-mobile-page-active]')
    const actions = activePage?.querySelector('[data-debug-bottom-actions]')
    const buildFooter = activePage?.querySelector('.mobile-entry-page__build')
    function update() {
      const button = floatRef.current
      if (!button) return
      const viewport = window.visualViewport
      const safeArea = getComputedStyle(button)
      const focused = document.activeElement?.closest('textarea, input:not([type="file"]):not([type="hidden"])')
      const position = debugButtonPosition({
        left: viewport?.offsetLeft ?? 0, top: viewport?.offsetTop ?? 0,
        width: viewport?.width ?? window.innerWidth, height: viewport?.height ?? window.innerHeight,
      }, button.offsetWidth, button.offsetHeight, {
        right: parseFloat(safeArea.getPropertyValue('--debug-safe-right')) || 0,
        bottom: parseFloat(safeArea.getPropertyValue('--debug-safe-bottom')) || 0,
      }, preferred.current, focused?.getBoundingClientRect().top, actions?.getBoundingClientRect().top)
      button.style.left = `${position.x}px`
      button.style.top = `${position.y}px`
    }
    updatePosition.current = update
    function schedule() { cancelAnimationFrame(frame); frame = requestAnimationFrame(update) }
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule)
    if (actions) observer?.observe(actions)
    if (buildFooter) observer?.observe(buildFooter)
    window.addEventListener('resize', schedule)
    window.addEventListener('scroll', schedule, true)
    window.addEventListener('focusin', schedule)
    window.addEventListener('focusout', schedule)
    window.visualViewport?.addEventListener('resize', schedule)
    window.visualViewport?.addEventListener('scroll', schedule)
    schedule()
    return () => {
      cancelAnimationFrame(frame)
      observer?.disconnect()
      window.removeEventListener('resize', schedule); window.removeEventListener('scroll', schedule, true)
      window.removeEventListener('focusin', schedule); window.removeEventListener('focusout', schedule)
      window.visualViewport?.removeEventListener('resize', schedule); window.visualViewport?.removeEventListener('scroll', schedule)
    }
  }, [pageKey])
  useEffect(() => {
    if (!isNativeMobile()) return
    let active = true
    void Preferences.get({ key: positionKey }).then(({ value }) => {
      if (!active || moved.current || value === null) return
      const point = JSON.parse(value)
      if (Number.isFinite(point?.x) && Number.isFinite(point?.y)) {
        preferred.current = point
        updatePosition.current()
      }
    }).catch(() => {})
    return () => { active = false }
  }, [])
  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    const close = document.getElementById('debug-panel-close')
    close?.focus()
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
      if (event.key !== 'Tab') return
      const buttons = [...document.querySelectorAll<HTMLElement>('.debug-dialog button, .debug-dialog [tabindex="0"]')]
      const first = buttons[0], last = buttons.at(-1)
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
    }
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('keydown', onKey); previous?.focus() }
  }, [open])
  async function copy() {
    try { if (['ios', 'android'].includes(Capacitor.getPlatform()) && nativeExperience?.copyDebugLog) await nativeExperience.copyDebugLog({ text: debugLog.exportText() }); else await navigator.clipboard.writeText(debugLog.exportText()); setFeedback('日志已复制') }
    catch { setFeedback('复制失败，请使用导出或在日志区域选择文字') }
  }
  async function download() {
    if (['ios', 'android'].includes(Capacitor.getPlatform()) && nativeExperience?.exportDebugLog) {
      try {
        const result = await nativeExperience.exportDebugLog({ text: debugLog.exportText() })
        setFeedback(result.opened ? '已打开分享面板' : result.completed ? '日志已分享或保存' : '已取消导出')
      } catch { setFeedback('导出失败，请使用复制日志') }
      return
    }
    let url: string | undefined
    try {
      url = URL.createObjectURL(new Blob([debugLog.exportText()], { type: 'text/plain;charset=utf-8' }))
      const link = document.createElement('a')
      link.href = url; link.download = `mobile-debug-${Date.now()}.txt`
      document.body.append(link); link.click(); link.remove()
      setFeedback('已请求导出；若系统未保存文件，请使用复制日志')
    } catch { setFeedback('导出失败，请使用复制日志') }
    finally { if (url) setTimeout(() => URL.revokeObjectURL(url!), 1000) }
  }
  return createPortal(<>
    <button ref={floatRef} className="debug-float" aria-label="打开调试日志"
      onPointerDown={(event) => {
        if (!event.isPrimary || event.button !== 0) return
        const rect = event.currentTarget.getBoundingClientRect()
        didDrag.current = false
        drag.current = { id: event.pointerId, pointerX: event.clientX, pointerY: event.clientY, x: rect.left, y: rect.top }
        event.currentTarget.setPointerCapture(event.pointerId)
      }}
      onPointerMove={(event) => {
        const start = drag.current
        if (!start || start.id !== event.pointerId) return
        const dx = event.clientX - start.pointerX
        const dy = event.clientY - start.pointerY
        if (!didDrag.current && Math.hypot(dx, dy) < 6) return
        didDrag.current = true
        moved.current = true
        preferred.current = { x: start.x + dx, y: start.y + dy }
        updatePosition.current()
      }}
      onPointerUp={(event) => {
        if (drag.current?.id !== event.pointerId || !didDrag.current || !isNativeMobile()) return
        const { left: x, top: y } = event.currentTarget.getBoundingClientRect()
        preferred.current = { x, y }
        void Preferences.set({ key: positionKey, value: JSON.stringify(preferred.current) }).catch(() => {})
      }}
      onLostPointerCapture={() => { drag.current = null }}
      onClick={(event) => {
        if (event.detail !== 0 && didDrag.current) { didDrag.current = false; return }
        setFeedback(''); setOpen(true)
      }}><SessionIcon name="debug" size={22} /></button>
    {open && <div className="debug-overlay" onClick={() => setOpen(false)}>
      <section className="debug-dialog" role="dialog" aria-modal="true" aria-labelledby="debug-title" onClick={(event) => event.stopPropagation()}>
        <header><strong id="debug-title">调试日志（{entries.length}/200）</strong><button id="debug-panel-close" onClick={() => setOpen(false)}>关闭</button></header>
        {/* <p className="debug-hint">实时记录 · 最新在上 · 仅保留脱敏信息</p> */}
        <div className="debug-toolbar">
          <button onClick={() => { debugLog.clear(); setFeedback('日志已清空') }}>清空日志</button>
          <button onClick={() => { void copy() }}>复制日志</button>
          <button onClick={() => { void download() }}>导出日志</button>
        </div>
        <div role="status">{feedback}</div>
        <div className="debug-entries" tabIndex={0}>
          {entries.length === 0 ? <p>暂无日志，请关闭面板后复现操作。</p> : [...entries].reverse().map((entry) => <article key={entry.id} data-level={entry.level ?? (entry.category === 'error' ? 'error' : 'info')}>
            <div className="debug-row-meta"><time>{new Date(entry.time).toLocaleTimeString('en-GB', { hour12: false })}.{String(entry.time % 1000).padStart(3, '0')}</time><span>{entry.level?.toUpperCase() ?? (entry.category === 'error' ? 'ERROR' : 'INFO')} · {entry.category === 'host' ? 'PC' : entry.category === 'network' ? 'WS' : entry.category === 'error' ? 'JS' : '动作'}</span>{entry.connectionId !== undefined && <span>C{entry.connectionId}{entry.attempt !== undefined ? ` / #${entry.attempt}` : ''}</span>}</div>
            {entry.code && <code className="debug-code">{entry.code}</code>}
            <div className="debug-message">{entry.text}</div>
            {entry.target && <div className="debug-target">{entry.target}</div>}
            {entry.stage && <div className="debug-facts">阶段 {entry.stage} · 总耗时 {entry.elapsedMs} ms{entry.phaseElapsedMs !== undefined && ` · 阶段耗时 ${entry.phaseElapsedMs} ms`}{entry.protocolVersion !== undefined && ` · v${entry.protocolVersion}`}{entry.timeoutMs ? ` · 期限 ${entry.timeoutMs} ms` : ''}{entry.readyState !== undefined && ` · readyState=${entry.readyState}`}{entry.closeCode !== undefined && ` · close=${entry.closeCode}`}{entry.wasClean !== undefined && ` · wasClean=${entry.wasClean}`}</div>}
          </article>)}
        </div>
      </section>
    </div>}
  </>, document.body)
}
