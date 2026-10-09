import { useEffect, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { DRAWER_MASK_STYLE, DRAWER_SHEET_STYLE } from './ui'
import './BottomDrawer.css'

const EXIT_MS = 260

interface BottomDrawerProps {
  visible: boolean
  onMaskClick?: () => void
  children: ReactNode
}

// Keep content mounted through exit so switching between list/detail never flashes.
export default function BottomDrawer({ visible, onMaskClick, children }: BottomDrawerProps) {
  const [mounted, setMounted] = useState(visible)
  const [entered, setEntered] = useState(false)

  useEffect(() => {
    if (visible) {
      const frame = window.requestAnimationFrame(() => {
        setMounted(true)
        setEntered(true)
      })
      return () => window.cancelAnimationFrame(frame)
    }
    const frame = window.requestAnimationFrame(() => setEntered(false))
    const timer = window.setTimeout(() => setMounted(false), EXIT_MS)
    return () => { window.cancelAnimationFrame(frame); window.clearTimeout(timer) }
  }, [visible])

  if (!visible && !mounted) return null
  return createPortal(<div className={`mobile-drawer${entered ? ' mobile-drawer--entered' : ''}`}>
    <div className="mobile-drawer-mask" style={DRAWER_MASK_STYLE} onClick={onMaskClick} />
    <div className="mobile-drawer-sheet" style={DRAWER_SHEET_STYLE}>{children}</div>
  </div>, document.body)
}
