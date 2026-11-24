import { useRef, type TouchEvent } from 'react'

export default function useEdgeBack(onEdgeBack?: () => void) {
  const edgeTouch = useRef<{ x: number, y: number, cancelled: boolean } | null>(null)
  return {
    onTouchStart: (event: TouchEvent) => {
      edgeTouch.current = null
      if (onEdgeBack === undefined || event.touches.length !== 1) return
      const touch = event.touches[0]
      if (touch.clientX > 28 || touch.clientX < 0) return
      const target = event.target
      if (target instanceof Element && target.closest('input, textarea, [contenteditable="true"], [data-horizontal-scroll], .tool-detail-page__markdown pre, .tool-detail-page__markdown table')) return
      edgeTouch.current = { x: touch.clientX, y: touch.clientY, cancelled: false }
    },
    onTouchMove: (event: TouchEvent) => {
      const start = edgeTouch.current
      if (event.touches.length !== 1) { edgeTouch.current = null; return }
      if (start === null) return
      const touch = event.touches[0]
      const dx = touch.clientX - start.x
      const dy = Math.abs(touch.clientY - start.y)
      if (dy > 16 && dy > Math.abs(dx)) start.cancelled = true
    },
    onTouchEnd: (event: TouchEvent) => {
      const start = edgeTouch.current
      edgeTouch.current = null
      if (start === null || start.cancelled || event.changedTouches.length !== 1) return
      const touch = event.changedTouches[0]
      const dx = touch.clientX - start.x
      const dy = Math.abs(touch.clientY - start.y)
      if (dx >= 72 && dx > dy * 1.5) onEdgeBack?.()
    },
    onTouchCancel: () => { edgeTouch.current = null },
  }
}
