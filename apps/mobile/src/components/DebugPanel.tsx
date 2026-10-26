import { debugButtonBottom } from '../debug/placement'
import { Capacitor } from '@capacitor/core'
import { nativeExperience } from '../native/experience'
import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { createPortal } from 'react-dom'
import { debugLog, observeDebugErrors } from '../debug/log'
import './DebugPanel.css'

export default function DebugPanel() {
  const { enabled } = useSyncExternalStore(debugLog.subscribe, debugLog.getSnapshot)
  return enabled ? <EnabledDebugPanel /> : null
}

function EnabledDebugPanel() {
  const { entries } = useSyncExternalStore(debugLog.subscribe, debugLog.getSnapshot)
  const floatRef = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [feedback, setFeedback] = useState('')
  useEffect(observeDebugErrors, [])
  useEffect(() => {
    let frame = 0
    const resize = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule)
    const observed = new Set<Element>()
    function update() {
      const button = floatRef.current
      if (!button) return
      const viewport = window.visualViewport
      const bottom = viewport ? viewport.offsetTop + viewport.height : window.innerHeight
      const inputs = [...document.querySelectorAll<HTMLElement>('[data-debug-input-region], textarea, input:not([type="file"]):not([type="hidden"])')]
      const tops = inputs.filter((element) => !element.closest('[data-mobile-page-active="false"], .debug-dialog')).map((element) => {
        if (!observed.has(element)) { resize?.observe(element); observed.add(element) }
        const rect = element.getBoundingClientRect()
        return rect.width > 0 && rect.height > 0 ? rect.top : -1
      })
      for (const element of observed) if (!element.isConnected) { resize?.unobserve(element); observed.delete(element) }
      const inset = debugButtonBottom(window.innerHeight, bottom, tops, viewport?.offsetTop ?? 0)
      button.style.bottom = `max(env(safe-area-inset-bottom, 0px), ${inset}px)`
    }
    function schedule() { cancelAnimationFrame(frame); frame = requestAnimationFrame(update) }
    const mutations = new MutationObserver(schedule)
    mutations.observe(document.body, { childList: true, subtree: true })
    window.addEventListener('resize', schedule)
    window.addEventListener('scroll', schedule, true)
    window.addEventListener('focusin', schedule)
    window.addEventListener('focusout', schedule)
    window.visualViewport?.addEventListener('resize', schedule)
    window.visualViewport?.addEventListener('scroll', schedule)
    schedule()
    return () => {
      cancelAnimationFrame(frame); resize?.disconnect(); mutations.disconnect()
      window.removeEventListener('resize', schedule); window.removeEventListener('scroll', schedule, true)
      window.removeEventListener('focusin', schedule); window.removeEventListener('focusout', schedule)
      window.visualViewport?.removeEventListener('resize', schedule); window.visualViewport?.removeEventListener('scroll', schedule)
    }
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
    try { if (Capacitor.getPlatform() === 'ios' && nativeExperience?.copyDebugLog) await nativeExperience.copyDebugLog({ text: debugLog.exportText() }); else await navigator.clipboard.writeText(debugLog.exportText()); setFeedback('日志已复制') }
    catch { setFeedback('复制失败，请使用导出或在日志区域选择文字') }
  }
  async function download() {
    if (Capacitor.getPlatform() === 'ios' && nativeExperience?.exportDebugLog) {
      try {
        const result = await nativeExperience.exportDebugLog({ text: debugLog.exportText() })
        setFeedback(result.completed ? '日志已分享或保存' : '已取消导出')
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
    <button ref={floatRef} className="debug-float" aria-label="打开调试日志" onClick={() => { setFeedback(''); setOpen(true) }}>调试</button>
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
