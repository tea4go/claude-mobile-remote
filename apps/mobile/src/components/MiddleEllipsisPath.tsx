import { useLayoutEffect, useRef, useState } from 'react'

export function fitPath(path: string, width: number, measure: (text: string) => number): string {
  if (measure(path) <= width) return path

  const parts = Array.from(path)
  const withoutTrailingSeparators = path.replace(/[\\/]+$/, '')
  const lastSeparator = Math.max(withoutTrailingSeparators.lastIndexOf('/'), withoutTrailingSeparators.lastIndexOf('\\'))
  const suffix = lastSeparator >= 0 ? path.slice(lastSeparator) : ''
  const firstSegment = path.match(/^[\\/]*[^\\/]+/)?.[0]
  if (firstSegment && suffix && firstSegment.length + suffix.length < path.length) {
    const firstSeparator = suffix[0]!
    const compact = `${firstSegment}${firstSeparator}...${suffix}`
    if (measure(compact) <= width) return compact
  }

  // When even the outer directories are too long, retain characters from both ends.
  let low = 0
  let high = parts.length - 1
  let result = '...'
  while (low <= high) {
    const count = Math.floor((low + high) / 2)
    const prefix = parts.slice(0, Math.ceil(count / 2)).join('')
    const end = parts.slice(parts.length - Math.floor(count / 2)).join('')
    const candidate = `${prefix}...${end}`
    if (measure(candidate) <= width) {
      result = candidate
      low = count + 1
    } else {
      high = count - 1
    }
  }
  return result
}

export default function MiddleEllipsisPath({ path }: { path: string }) {
  const ref = useRef<HTMLSpanElement>(null)
  const [visiblePath, setVisiblePath] = useState(path)

  useLayoutEffect(() => {
    const element = ref.current
    if (!element) return
    const resize = () => {
      const width = element.clientWidth
      if (width === 0) return
      const context = document.createElement('canvas').getContext('2d')
      if (!context) return
      const style = getComputedStyle(element)
      context.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`
      setVisiblePath(fitPath(path, width, text => context.measureText(text).width))
    }
    resize()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(resize)
    observer.observe(element)
    return () => observer.disconnect()
  }, [path])

  return <span ref={ref} className="middle-ellipsis-path" title={path} aria-label={path}>
    <span aria-hidden="true">{visiblePath}</span>
  </span>
}
