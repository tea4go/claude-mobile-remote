import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import useEdgeBack from './useEdgeBack'
import './PageTransition.css'

type Direction = 'forward' | 'back'

interface PageTransitionProps {
  pageKey: string
  direction: Direction
  children: ReactNode
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

export default function PageTransition({ pageKey, direction, children, onEdgeBack }: PageTransitionProps) {
  const latestPage = useRef({ key: pageKey, content: children })
  const edgeBack = useEdgeBack(onEdgeBack)
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
    {...edgeBack}
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
    >
      {children}
    </div>
  </div>
}
