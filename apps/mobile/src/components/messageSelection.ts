// Ranges here are used for geometry only. Never register them with window.Selection.
export type Point = { x: number; y: number }
export type Box = { left: number; top: number; right: number; bottom: number }
type Run = { node: Text; start: number; end: number }
export type MessageText = { root: HTMLElement; text: string; runs: Run[]; boundaries: number[] }
export type Handle = { x: number; top: number; bottom: number }
export type SelectionGeometry = { clip: Box; rects: Box[]; start: Handle | null; end: Handle | null }
export type Edge = 'start' | 'end'

const blocks = new Set(['DIV', 'P', 'PRE', 'BLOCKQUOTE', 'UL', 'OL', 'LI', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'TR'])
const ignored = 'script, style, button, input, textarea, select, [hidden], [aria-hidden="true"], [data-selection-ignore]'
const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(value, max))

export function graphemeBoundaries(text: string): number[] {
  if (typeof Intl.Segmenter === 'function') {
    return [...Array.from(new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(text), (part) => part.index), text.length]
  }
  // Older engines: at least keep surrogate pairs, combining marks and emoji chains together.
  const result = [0]
  let offset = 0
  let previous = ''
  let regionalCount = 0
  for (const char of text) {
    const regional = /\p{Regional_Indicator}/u.test(char)
    const joins = /[\p{Mark}\p{Emoji_Modifier}\uFE0E\uFE0F\u200D\u{E0020}-\u{E007F}]/u.test(char) || previous === '\u200D' ||
      (previous === '\r' && char === '\n') || (regional && regionalCount % 2 === 1)
    if (offset && !joins) result.push(offset)
    offset += char.length
    regionalCount = regional ? regionalCount + 1 : 0
    previous = char
  }
  if (offset) result.push(offset)
  return result
}

export function indexMessage(root: HTMLElement): MessageText {
  let text = ''
  let separator = ''
  const runs: Run[] = []
  const separate = (value: string) => { if (text && (value === '\n' || separator !== '\n')) separator = value }
  const visit = (node: Node) => {
    if (node instanceof Text) {
      if (!node.data) return
      if (separator && !text.endsWith('\n') && !node.data.startsWith('\n')) text += separator
      separator = ''
      const start = text.length
      text += node.data
      runs.push({ node, start, end: text.length })
      return
    }
    if (!(node instanceof HTMLElement) || node.matches(ignored)) return
    const style = getComputedStyle(node)
    if (style.display === 'none' || style.visibility === 'hidden') return
    if (node.tagName === 'BR') {
      text += '\n'
      separator = ''
      return
    }
    const block = node !== root && blocks.has(node.tagName)
    if (block) separate('\n')
    node.childNodes.forEach(visit)
    if (block) separate('\n')
    else if (node.tagName === 'TD' || node.tagName === 'TH') separate('\t')
  }
  visit(root)
  return { root, text, runs, boundaries: graphemeBoundaries(text) }
}

export function intersect(a: Box, b: Box): Box {
  return { left: Math.max(a.left, b.left), top: Math.max(a.top, b.top), right: Math.min(a.right, b.right), bottom: Math.min(a.bottom, b.bottom) }
}
const nonempty = (rect: Box) => rect.right > rect.left && rect.bottom > rect.top

// Include all clipping ancestors, including horizontally scrolling code and tables.
export function clipFor(element: HTMLElement, includeSelf = true): Box {
  const viewport = window.visualViewport
  let box: Box = { left: viewport?.offsetLeft ?? 0, top: viewport?.offsetTop ?? 0,
    right: (viewport?.offsetLeft ?? 0) + (viewport?.width ?? window.innerWidth),
    bottom: (viewport?.offsetTop ?? 0) + (viewport?.height ?? window.innerHeight) }
  if (includeSelf) box = intersect(box, element.getBoundingClientRect())
  for (let parent: HTMLElement | null = element; parent; parent = parent.parentElement) {
    const style = getComputedStyle(parent)
    const clipX = /auto|scroll|hidden|clip/.test(style.overflowX || style.overflow)
    const clipY = /auto|scroll|hidden|clip/.test(style.overflowY || style.overflow)
    if (!clipX && !clipY) continue
    const rect = parent.getBoundingClientRect()
    // The padding box, rather than the border, is the scrollport.
    const left = rect.left + parent.clientLeft
    const top = rect.top + parent.clientTop
    if (clipX) box = { ...box, left: Math.max(box.left, left), right: Math.min(box.right, left + parent.clientWidth) }
    if (clipY) box = { ...box, top: Math.max(box.top, top), bottom: Math.min(box.bottom, top + parent.clientHeight) }
  }
  return box
}

function rangeFor(run: Run, start: number, end: number): Range {
  const range = document.createRange()
  range.setStart(run.node, clamp(start - run.start, 0, run.node.length))
  range.setEnd(run.node, clamp(end - run.start, 0, run.node.length))
  return range
}
function rectsOf(range: Range): DOMRect[] {
  // JSDOM and old nonvisual engines do not implement range geometry.
  return typeof range.getClientRects === 'function' ? Array.from(range.getClientRects()) : []
}

export function boundaryIndex(boundaries: number[], offset: number): number {
  let low = 0
  let high = boundaries.length - 1
  while (low < high) {
    const mid = Math.ceil((low + high) / 2)
    if (boundaries[mid]! <= offset) low = mid
    else high = mid - 1
  }
  return low
}
export function snapBoundary(index: MessageText, offset: number): number {
  const i = boundaryIndex(index.boundaries, offset)
  const left = index.boundaries[i]!
  const right = index.boundaries[i + 1] ?? left
  return offset - left <= right - offset ? left : right
}
export function moveBoundary(index: MessageText, start: number, end: number, edge: Edge, offset: number): [number, number] {
  const value = snapBoundary(index, clamp(offset, 0, index.text.length))
  if (edge === 'start') return [Math.min(value, index.boundaries[boundaryIndex(index.boundaries, end) - 1] ?? 0), end]
  return [start, Math.max(value, index.boundaries[boundaryIndex(index.boundaries, start) + 1] ?? index.text.length)]
}

function endpoint(index: MessageText, offset: number, edge: Edge, clip: Box): Handle | null {
  const run = edge === 'start' ? index.runs.find((item) => item.end > offset) : index.runs.findLast((item) => item.start < offset)
  if (!run) return null
  // Synthetic paragraph/table separators have no glyph. Anchor their handle to
  // the next/previous real glyph rather than hiding it on a collapsed range.
  const at = boundaryIndex(index.boundaries, edge === 'start' ? Math.max(offset, run.start) : Math.min(offset, run.end))
  const start = edge === 'start' ? Math.max(offset, run.start) : index.boundaries[Math.max(0, at - 1)]!
  const end = edge === 'end' ? Math.min(offset, run.end) : index.boundaries[Math.min(index.boundaries.length - 1, at + 1)]!
  // A trailing newline may have a zero-width line box, which is still a valid
  // caret location for the handle even though it paints no highlight.
  const boxes = rectsOf(rangeFor(run, Math.max(start, run.start), Math.min(end, run.end))).filter((rect) => rect.bottom > rect.top)
  const rect = edge === 'start' ? boxes[0] : boxes.at(-1)
  if (!rect) return null
  const rtl = getComputedStyle(run.node.parentElement!).direction === 'rtl'
  const x = (edge === 'start') !== rtl ? rect.left : rect.right
  // Do not pin an offscreen endpoint to the viewport; it would misrepresent its position.
  const localClip = intersect(clip, clipFor(run.node.parentElement!, false))
  if (x < localClip.left || x > localClip.right || rect.top < localClip.top || rect.bottom > localClip.bottom) return null
  return { x, top: rect.top, bottom: rect.bottom }
}

export function measureSelection(index: MessageText, start: number, end: number): SelectionGeometry {
  const clip = clipFor(index.root)
  const rects: Box[] = []
  const clips = new Map<HTMLElement, Box>()
  if (nonempty(clip)) for (const run of index.runs) {
    if (run.end <= start || run.start >= end) continue
    const element = run.node.parentElement!
    let bounds = clips.get(element)
    if (!bounds) { bounds = intersect(clip, clipFor(element, false)); clips.set(element, bounds) }
    if (!nonempty(bounds)) continue
    for (const rect of rectsOf(rangeFor(run, Math.max(start, run.start), Math.min(end, run.end)))) {
      const visible = intersect(rect, bounds)
      if (nonempty(visible)) rects.push(visible)
    }
  }
  // Adjacent inline nodes must not paint translucent highlights on top of each other.
  rects.sort((a, b) => a.top - b.top || a.left - b.left)
  const merged: Box[] = []
  for (const rect of rects) {
    const previous = merged.at(-1)
    if (previous && Math.abs(previous.top - rect.top) < 1 && Math.abs(previous.bottom - rect.bottom) < 1 && rect.left <= previous.right + 1) {
      previous.right = Math.max(previous.right, rect.right)
    } else merged.push({ ...rect })
  }
  return { clip, rects: merged, start: endpoint(index, start, 'start', clip), end: endpoint(index, end, 'end', clip) }
}

function distance(rects: Box[], point: Point): number {
  let best = Infinity
  for (const rect of rects) {
    if (!nonempty(rect)) continue
    const dy = Math.max(rect.top - point.y, 0, point.y - rect.bottom)
    const dx = Math.max(rect.left - point.x, 0, point.x - rect.right)
    // Prefer the nearest line over a horizontally nearer line above or below it.
    best = Math.min(best, dy * 10000 + dx)
  }
  return best
}

export function offsetAtPoint(index: MessageText, point: Point): number | null {
  const document = index.root.ownerDocument
  const caret = document.caretPositionFromPoint?.(point.x, point.y)
  const legacy = !caret ? document.caretRangeFromPoint?.(point.x, point.y) : null
  const node = caret?.offsetNode ?? legacy?.startContainer
  const offset = caret?.offset ?? legacy?.startOffset
  const run = index.runs.find((item) => item.node === node)
  if (run && offset !== undefined) return snapBoundary(index, run.start + offset)

  // Some engines exclude user-select:none text from their caret API. Measure text
  // directly in that case; recursive bisection avoids scanning every character.
  let nearest: Run | null = null
  let best = Infinity
  for (const item of index.runs) {
    const clip = clipFor(item.node.parentElement!, false)
    const rects = rectsOf(rangeFor(item, item.start, item.end)).map((rect) => intersect(rect, clip))
    const score = distance(rects, point)
    if (score < best) { nearest = item; best = score }
  }
  if (!nearest) return null
  const item = nearest
  const boundaries = index.boundaries.filter((at) => at >= item.start && at <= item.end)
  if (boundaries[0] !== item.start) boundaries.unshift(item.start)
  if (boundaries.at(-1) !== item.end) boundaries.push(item.end)
  let low = 0
  let high = boundaries.length - 1
  while (high - low > 1) {
    const mid = Math.floor((low + high) / 2)
    const left = distance(rectsOf(rangeFor(item, boundaries[low]!, boundaries[mid]!)), point)
    const right = distance(rectsOf(rangeFor(item, boundaries[mid]!, boundaries[high]!)), point)
    if (left <= right) high = mid
    else low = mid
  }
  const rect = rectsOf(rangeFor(item, boundaries[low]!, boundaries[high]!))[0]
  if (!rect) return null
  const rtl = getComputedStyle(item.node.parentElement!).direction === 'rtl'
  const after = (point.x > (rect.left + rect.right) / 2) !== rtl
  return snapBoundary(index, boundaries[after ? high : low]!)
}
