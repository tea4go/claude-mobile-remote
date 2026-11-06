import { useCallback, useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { copyText } from '../native/clipboard'
import { selectionFeedback } from '../native/haptics'
import { boundaryIndex, clipFor, indexMessage, measureSelection, moveBoundary, offsetAtPoint,
  type Edge, type Handle, type MessageText, type Point, type SelectionGeometry } from './messageSelection'
import './SelectableMessage.css'

type Selection = { index: MessageText; start: number; end: number; anchor: { x: number; offsetY: number } }
type Drag = { edge: Edge; id: number; point: Point; offset: Point }
type View = SelectionGeometry & { menu: Point; menuVisible: boolean; dragging: boolean }
const selectionOpened = 'cmr:message-selection-opened'
const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(value, max))
const interactive = 'a, button, input, textarea, select, video, audio, [contenteditable="true"]'

export default function SelectableMessage({ children, style, selectionTone = 'normal' }: { children: ReactNode; style?: CSSProperties; selectionTone?: 'normal' | 'inverse' }) {
  const content = useRef<HTMLDivElement>(null)
  const overlay = useRef<HTMLDivElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const frame = useRef<number | null>(null)
  const dragFrame = useRef<number | null>(null)
  const touch = useRef<{ id: number; point: Point; held: boolean } | null>(null)
  const touchUntil = useRef(0)
  const drag = useRef<Drag | null>(null)
  const selection = useRef<Selection | null>(null)
  const copying = useRef(false)
  const mounted = useRef(true)
  const [open, setOpen] = useState(false)
  const [view, setView] = useState<View | null>(null)
  const [feedback, setFeedback] = useState('')
  const [busy, setBusy] = useState(false)

  const stopTouch = useCallback(() => {
    if (timer.current !== null) clearTimeout(timer.current)
    timer.current = null
    if (touch.current) touchUntil.current = Date.now() + 1200
    touch.current = null
  }, [])
  const stopDrag = useCallback(() => {
    drag.current = null
    if (dragFrame.current !== null) cancelAnimationFrame(dragFrame.current)
    dragFrame.current = null
  }, [])
  const close = useCallback(() => {
    stopTouch()
    stopDrag()
    selection.current = null
    if (frame.current !== null) cancelAnimationFrame(frame.current)
    frame.current = null
    setOpen(false)
    setView(null)
    setFeedback('')
  }, [stopTouch, stopDrag])
  const refresh = useCallback(() => {
    const current = selection.current
    const node = content.current
    if (!current || !node) return
    if (node.closest('[data-mobile-page-active="false"]')) { close(); return }
    const geometry = measureSelection(current.index, current.start, current.end)
    const bounds = clipFor(node, false)
    const rect = node.getBoundingClientRect()
    const width = menu.current?.offsetWidth || 64
    const height = menu.current?.offsetHeight || 44
    const menuPoint = {
      x: clamp(current.anchor.x - width / 2, bounds.left + 4, bounds.right - width - 4),
      y: clamp(rect.top + current.anchor.offsetY - height - 8, bounds.top + 4, bounds.bottom - height - 4),
    }
    setView({ ...geometry, menu: menuPoint, dragging: drag.current !== null,
      menuVisible: rect.bottom > bounds.top && rect.top < bounds.bottom && bounds.bottom - bounds.top >= height + 8 && bounds.right - bounds.left >= width + 8 })
  }, [close])
  const schedule = useCallback(() => {
    if (frame.current !== null) return
    frame.current = requestAnimationFrame(() => { frame.current = null; refresh() })
  }, [refresh])
  const selectAll = useCallback((point: Point) => {
    const node = content.current
    if (!node) return false
    const index = indexMessage(node)
    if (!index.text.trim()) return false
    const focused = document.activeElement
    if (focused instanceof HTMLElement && focused.matches('input, textarea, [contenteditable="true"]')) focused.blur()
    document.dispatchEvent(new CustomEvent(selectionOpened, { detail: node }))
    const rect = node.getBoundingClientRect()
    selection.current = { index, start: 0, end: index.text.length, anchor: { x: point.x, offsetY: point.y - rect.top } }
    setFeedback('')
    setOpen(true)
    refresh()
    return true
  }, [refresh])
  const copy = useCallback(() => {
    const current = selection.current
    if (!current || copying.current) return
    const start = current.start
    const end = current.end
    const text = current.index.text.slice(start, end)
    if (!text) return
    copying.current = true
    setBusy(true)
    void copyText(text).then(() => {
      if (mounted.current && selection.current === current && current.start === start && current.end === end) { close(); setFeedback('已复制') }
    }).catch(() => {
      if (mounted.current && selection.current === current && current.start === start && current.end === end) setFeedback('复制失败，请重试')
    }).finally(() => {
      copying.current = false
      if (mounted.current) setBusy(false)
    })
  }, [close])

  useEffect(() => {
    mounted.current = true
    const node = content.current
    if (!node) return
    const begin = (event: TouchEvent) => {
      stopTouch()
      touchUntil.current = Date.now() + 1200
      if (event.touches.length !== 1) return
      if (event.target instanceof Element && event.target.closest(interactive)) { if (selection.current) close(); return }
      const first = event.touches[0]!
      touch.current = { id: first.identifier, point: { x: first.clientX, y: first.clientY }, held: false }
      timer.current = setTimeout(() => {
        timer.current = null
        const current = touch.current
        if (!current) return
        current.held = selectAll(current.point)
        if (current.held) void selectionFeedback()
      }, 500)
    }
    const move = (event: TouchEvent) => {
      const current = touch.current
      if (!current) return
      if (event.touches.length !== 1) { stopTouch(); return }
      const next = Array.from(event.touches).find((item) => item.identifier === current.id)
      if (!next) { stopTouch(); return }
      if (current.held) {
        if (event.cancelable) event.preventDefault()
      } else if (Math.hypot(next.clientX - current.point.x, next.clientY - current.point.y) > 10) stopTouch()
    }
    const end = (event: TouchEvent) => {
      if (touch.current?.held && event.cancelable) event.preventDefault()
      // A tap dismisses selection; a scrolling touch was already canceled by move
      // and leaves the current selection available while the message scrolls.
      if (touch.current && !touch.current.held && selection.current) close()
      stopTouch()
    }
    const contextMenu = (event: MouseEvent) => {
      event.preventDefault()
      const pointerType = (event as PointerEvent).pointerType
      if (pointerType && pointerType !== 'mouse') return
      // A delayed native contextmenu after touchcancel must never select again.
      // Only an explicit desktop secondary click enters through this path.
      if (event.button === 2 && !selection.current && !touch.current && Date.now() > touchUntil.current &&
        !(event.target instanceof Element && event.target.closest(interactive))) {
        selectAll({ x: event.clientX, y: event.clientY })
      }
    }
    const guardSelection = (event: Event) => event.preventDefault()
    const suppressClick = (event: MouseEvent) => {
      if (selection.current && Date.now() <= touchUntil.current) { event.preventDefault(); event.stopPropagation() }
    }
    // Keep native scrolling on ordinary touches. CSS permanently disables native
    // selection on this content; it is never re-enabled during a held gesture.
    node.addEventListener('touchstart', begin, { passive: true })
    node.addEventListener('touchmove', move, { passive: false })
    node.addEventListener('touchend', end, { passive: false })
    node.addEventListener('touchcancel', stopTouch)
    node.addEventListener('selectstart', guardSelection)
    node.addEventListener('contextmenu', contextMenu)
    node.addEventListener('click', suppressClick, true)
    const cancel = () => { stopTouch(); close() }
    const visibility = () => { if (document.hidden) cancel() }
    const scrolling = (event: Event) => {
      if (touch.current && !touch.current.held && event.target instanceof Node &&
        (event.target.contains(node) || node.contains(event.target))) stopTouch()
    }
    window.addEventListener('blur', cancel)
    window.addEventListener('pagehide', cancel)
    document.addEventListener('visibilitychange', visibility)
    document.addEventListener('scroll', scrolling, true)
    return () => {
      mounted.current = false
      stopTouch()
      stopDrag()
      selection.current = null
      if (frame.current !== null) cancelAnimationFrame(frame.current)
      frame.current = null
      node.removeEventListener('touchstart', begin)
      node.removeEventListener('touchmove', move)
      node.removeEventListener('touchend', end)
      node.removeEventListener('touchcancel', stopTouch)
      node.removeEventListener('selectstart', guardSelection)
      node.removeEventListener('contextmenu', contextMenu)
      node.removeEventListener('click', suppressClick, true)
      window.removeEventListener('blur', cancel)
      window.removeEventListener('pagehide', cancel)
      document.removeEventListener('visibilitychange', visibility)
      document.removeEventListener('scroll', scrolling, true)
    }
  }, [close, selectAll, stopTouch, stopDrag])

  useEffect(() => {
    if (!open) return
    const node = content.current!
    const outside = (event: Event) => {
      if (event.target instanceof Node && !node.contains(event.target) && !overlay.current?.contains(event.target) && !menu.current?.contains(event.target)) close()
    }
    const anotherSelection = (event: Event) => { if ((event as CustomEvent).detail !== node) close() }
    const keys = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close()
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'c') { event.preventDefault(); copy() }
    }
    // Streaming text invalidates the index. Do not silently copy text using stale offsets.
    const changes = new MutationObserver(close)
    changes.observe(node, { subtree: true, childList: true, characterData: true, attributes: true })
    const page = node.closest('[data-mobile-page-active]')
    const pageChanges = new MutationObserver(() => { if (page?.getAttribute('data-mobile-page-active') === 'false') close() })
    if (page) pageChanges.observe(page, { attributes: true, attributeFilter: ['data-mobile-page-active'] })
    const resize = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule)
    for (let element: HTMLElement | null = node; element; element = element.parentElement) resize?.observe(element)
    document.addEventListener('pointerdown', outside, true)
    document.addEventListener('focusin', outside, true)
    document.addEventListener(selectionOpened, anotherSelection)
    document.addEventListener('keydown', keys)
    document.addEventListener('scroll', schedule, true)
    window.addEventListener('resize', schedule)
    window.visualViewport?.addEventListener('resize', schedule)
    window.visualViewport?.addEventListener('scroll', schedule)
    schedule()
    return () => {
      changes.disconnect()
      pageChanges.disconnect()
      resize?.disconnect()
      document.removeEventListener('pointerdown', outside, true)
      document.removeEventListener('focusin', outside, true)
      document.removeEventListener(selectionOpened, anotherSelection)
      document.removeEventListener('keydown', keys)
      document.removeEventListener('scroll', schedule, true)
      window.removeEventListener('resize', schedule)
      window.visualViewport?.removeEventListener('resize', schedule)
      window.visualViewport?.removeEventListener('scroll', schedule)
    }
  }, [open, close, copy, schedule])

  const updateDrag = useCallback(() => {
    const current = selection.current
    const active = drag.current
    if (!current || !active) return
    const bounds = clipFor(current.index.root)
    if (bounds.right <= bounds.left || bounds.bottom <= bounds.top) return
    const point = { x: clamp(active.point.x, bounds.left + 1, bounds.right - 1), y: clamp(active.point.y, bounds.top + 1, bounds.bottom - 1) }
    const offset = offsetAtPoint(current.index, point)
    if (offset !== null) [current.start, current.end] = moveBoundary(current.index, current.start, current.end, active.edge, offset)
    const rect = current.index.root.getBoundingClientRect()
    current.anchor = { x: point.x, offsetY: point.y - rect.top }
    refresh()
  }, [refresh])

  useEffect(() => {
    if (!open) return
    const move = (event: PointerEvent) => {
      const active = drag.current
      if (!active || event.pointerId !== active.id) return
      event.preventDefault()
      active.point = { x: event.clientX - active.offset.x, y: event.clientY - active.offset.y }
      updateDrag()
    }
    const end = (event: PointerEvent) => {
      if (event.pointerId !== drag.current?.id) return
      if (event.type === 'pointerup') move(event)
      stopDrag()
      refresh()
    }
    document.addEventListener('pointermove', move, { passive: false })
    document.addEventListener('pointerup', end)
    document.addEventListener('pointercancel', end)
    return () => {
      stopDrag()
      document.removeEventListener('pointermove', move)
      document.removeEventListener('pointerup', end)
      document.removeEventListener('pointercancel', end)
    }
  }, [open, refresh, stopDrag, updateDrag])

  const beginDrag = (edge: Edge, handle: Handle, event: React.PointerEvent<HTMLButtonElement>) => {
    if (event.button !== 0 || drag.current) return
    event.preventDefault()
    const midpoint = (handle.top + handle.bottom) / 2
    drag.current = { edge, id: event.pointerId, point: { x: handle.x, y: midpoint }, offset: { x: event.clientX - handle.x, y: event.clientY - midpoint } }
    // Capture on the persistent overlay: an endpoint may leave the visible area
    // during scrolling, removing its button while the drag must continue.
    overlay.current?.setPointerCapture?.(event.pointerId)
    refresh()
    // A held handle near a scroll edge keeps extending the selection even when
    // the finger is stationary. Only the active message's scroll ancestors move.
    let previousTime: number | null = null
    const tick = (time: number) => {
      dragFrame.current = null
      const active = drag.current
      const node = content.current
      if (!active || !node) return
      const scale = previousTime === null ? 1 : Math.min(2, (time - previousTime) / 16.67)
      previousTime = time
      const bounds = clipFor(node, false)
      const speed = (value: number, min: number, max: number) => value < min + 28 ? -clamp((min + 28 - value) / 2, 0, 12) : value > max - 28 ? clamp((value - max + 28) / 2, 0, 12) : 0
      const y = speed(active.point.y, bounds.top, bounds.bottom) * scale
      const hit = document.elementFromPoint?.(clamp(active.point.x, bounds.left + 1, bounds.right - 1), clamp(active.point.y, bounds.top + 1, bounds.bottom - 1))
      const current = selection.current
      const endpointRun = current ? (active.edge === 'start' ? current.index.runs.find((run) => run.end > current.start) : current.index.runs.findLast((run) => run.start < current.end)) : undefined
      let horizontal = false
      let vertical = false
      let scrolled = false
      for (let element: HTMLElement | null = hit instanceof HTMLElement && node.contains(hit) ? hit : endpointRun?.node.parentElement ?? node; element; element = element.parentElement) {
        const style = getComputedStyle(element)
        if (!vertical && /auto|scroll/.test(style.overflowY || style.overflow) && element.scrollHeight > element.clientHeight) {
          const before = element.scrollTop
          element.scrollTop += y
          scrolled ||= element.scrollTop !== before
          vertical = true
        }
        if (!horizontal && node.contains(element) && /auto|scroll/.test(style.overflowX || style.overflow) && element.scrollWidth > element.clientWidth) {
          const box = element.getBoundingClientRect()
          const before = element.scrollLeft
          element.scrollLeft += speed(active.point.x, Math.max(bounds.left, box.left), Math.min(bounds.right, box.right)) * scale
          scrolled ||= element.scrollLeft !== before
          horizontal = true
        }
        if (vertical && horizontal) break
      }
      if (scrolled) updateDrag()
      dragFrame.current = requestAnimationFrame(tick)
    }
    dragFrame.current = requestAnimationFrame(tick)
  }
  const keyHandle = (edge: Edge, event: React.KeyboardEvent<HTMLButtonElement>) => {
    const current = selection.current
    if (!current) return
    const offset = current[edge]
    const at = boundaryIndex(current.index.boundaries, offset)
    let next: number | undefined
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = current.index.boundaries[Math.max(0, at - 1)]
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = current.index.boundaries[Math.min(current.index.boundaries.length - 1, at + 1)]
    if (event.key === 'Home') next = 0
    if (event.key === 'End') next = current.index.text.length
    if (next === undefined) return
    event.preventDefault()
    ;[current.start, current.end] = moveBoundary(current.index, current.start, current.end, edge, next)
    refresh()
  }

  return <div className="selectable-message">
    <div ref={content} className="selectable-message-content" style={style} tabIndex={0} onKeyDown={(event) => {
      if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')) {
        event.preventDefault()
        const rect = event.currentTarget.getBoundingClientRect()
        selectAll({ x: rect.left + rect.width / 2, y: Math.max(rect.top, clipFor(event.currentTarget, false).top) + 48 })
      }
    }}>{children}</div>
    {open && view && createPortal(<>
      <div ref={overlay} className={`message-selection-overlay message-selection-overlay-${selectionTone}`} onLostPointerCapture={() => { stopDrag(); refresh() }} style={{ left: view.clip.left, top: view.clip.top, width: Math.max(0, view.clip.right - view.clip.left), height: Math.max(0, view.clip.bottom - view.clip.top) }}>
        {view.rects.map((rect, i) => <span key={i} aria-hidden="true" className="message-selection-highlight" style={{ left: rect.left - view.clip.left, top: rect.top - view.clip.top, width: rect.right - rect.left, height: rect.bottom - rect.top }} />)}
        {(['start', 'end'] as const).map((edge) => {
          const handle = view[edge]
          return handle && <button key={edge} type="button" aria-label={edge === 'start' ? '调整选区起点' : '调整选区终点'} className={`message-selection-handle message-selection-handle-${edge}`}
            style={{ left: handle.x - view.clip.left - 22, top: handle.top - view.clip.top - 12, height: Math.max(44, handle.bottom - handle.top + 24), '--selection-line-height': `${handle.bottom - handle.top}px` } as CSSProperties}
            onPointerDown={(event) => beginDrag(edge, handle, event)} onKeyDown={(event) => keyHandle(edge, event)} onContextMenu={(event) => event.preventDefault()} />
        })}
      </div>
      <div ref={menu} className="selectable-message-menu" style={{ left: view.menu.x, top: view.menu.y, visibility: view.menuVisible && !view.dragging ? 'visible' : 'hidden' }} onContextMenu={(event) => event.preventDefault()}>
        <button type="button" disabled={busy} onClick={copy}>复制</button>
      </div>
    </>, document.body)}
    {feedback && <span className="selectable-message-feedback" role="status">{feedback}</span>}
  </div>
}
