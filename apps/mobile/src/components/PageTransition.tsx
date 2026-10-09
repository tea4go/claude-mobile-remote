import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import './PageTransition.css'

type Direction = 'forward' | 'back'

interface PageTransitionProps {
  pageKey: string
  direction: Direction
  children: ReactNode
  onListScroll?: (scrollTop: number) => void
  onEdgeBack?: () => void
}

interface OutgoingPage {
  content: ReactNode
}

interface TransitionState {
  activeKey: string
  outgoing: OutgoingPage | null
  direction: Direction
  sequence: number
}

const DURATION_MS = 360

export default function PageTransition({ pageKey, direction, children, onListScroll, onEdgeBack }: PageTransitionProps) {
  const latestPage = useRef({ key: pageKey, content: children })
  const edgeTouch = useRef<{ x: number, y: number, cancelled: boolean } | null>(null)
  const [transition, setTransition] = useState<TransitionState>(() => ({
    activeKey: pageKey,
    outgoing: null,
    direction,
    sequence: 0,
  }))

  useLayoutEffect(() => {
    const previous = latestPage.current
    if (previous.key !== pageKey) {
      const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? true
      setTransition((current) => ({
        activeKey: pageKey,
        outgoing: previous.key === 'restoring' || reducedMotion
          ? null
          : { content: previous.content },
        direction,
        sequence: current.sequence + 1,
      }))
    }
    latestPage.current = { key: pageKey, content: children }
  }, [pageKey, direction, children])

  useEffect(() => {
    if (transition.outgoing === null) return
    const sequence = transition.sequence
    const timer = window.setTimeout(() => {
      setTransition((current) => current.sequence === sequence
        ? { ...current, outgoing: null }
        : current)
    }, DURATION_MS)
    return () => window.clearTimeout(timer)
  }, [transition.sequence, transition.outgoing])

  const animating = transition.outgoing !== null

  return <div className="mobile-page-stack"
    onTouchStart={(event) => {
      edgeTouch.current = null
      if (onEdgeBack === undefined || event.touches.length !== 1) return
      const touch = event.touches[0]
      if (touch.clientX > 28 || touch.clientX < 0) return
      const target = event.target
      if (target instanceof Element && target.closest('input, textarea, [contenteditable="true"]')) return
      edgeTouch.current = { x: touch.clientX, y: touch.clientY, cancelled: false }
    }}
    onTouchMove={(event) => {
      const start = edgeTouch.current
      if (start === null || event.touches.length !== 1) return
      const touch = event.touches[0]
      const dx = touch.clientX - start.x
      const dy = Math.abs(touch.clientY - start.y)
      if (dy > 16 && dy > Math.abs(dx)) start.cancelled = true
    }}
    onTouchEnd={(event) => {
      const start = edgeTouch.current
      edgeTouch.current = null
      if (start === null || start.cancelled || event.changedTouches.length !== 1) return
      const touch = event.changedTouches[0]
      const dx = touch.clientX - start.x
      const dy = Math.abs(touch.clientY - start.y)
      if (dx >= 72 && dx > dy * 1.5) onEdgeBack?.()
    }}
    onTouchCancel={() => { edgeTouch.current = null }}
  >
    {transition.outgoing !== null && <div
      key={`outgoing:${transition.sequence}`}
      className={`mobile-page mobile-page--exit-${transition.direction}`}
      aria-hidden="true"
      inert
    >
      {transition.outgoing.content}
    </div>}
    <div
      key={`active:${pageKey}`}
      data-mobile-page-active="true"
      data-mobile-page={pageKey}
      className={`mobile-page${animating ? ` mobile-page--enter-${transition.direction}` : ''}`}
      onScroll={pageKey === 'list' && onListScroll !== undefined
        ? (event) => onListScroll(event.currentTarget.scrollTop)
        : undefined}
    >
      {children}
    </div>
  </div>
}
