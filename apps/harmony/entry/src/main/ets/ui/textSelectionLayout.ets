import { text as drawingText } from '@kit.ArkGraphics2D'
import type { UIContext } from '@kit.ArkUI'
import type { MarkdownSelectionProjection, MarkdownSelectionSegment } from '../session/markdown'

export interface SelectionRect { left: number; top: number; right: number; bottom: number }
export interface SelectionHandle { x: number; top: number; bottom: number }
export interface MessageSelectionView {
  rects: Array<SelectionRect>
  start: SelectionHandle | null
  end: SelectionHandle | null
  menuX: number
  menuY: number
}
interface SelectionNode {
  segment: MarkdownSelectionSegment
  controller: TextController
  id: string
  clipId: string
}

let nextSelectionId: number = 0
export function emptySelectionView(): MessageSelectionView {
  return { rects: [], start: null, end: null, menuX: 0, menuY: 0 }
}
function intersect(a: SelectionRect, b: SelectionRect): SelectionRect {
  return { left: Math.max(a.left, b.left), top: Math.max(a.top, b.top),
    right: Math.min(a.right, b.right), bottom: Math.min(a.bottom, b.bottom) }
}
function contains(rect: SelectionRect, x: number, y: number): boolean {
  return x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom
}

// Keep a UTF-16 range on character boundaries (TextController uses UTF-16),
// including combining marks, variation selectors, emoji modifiers and ZWJ.
export function selectionBoundaries(value: string): Array<number> {
  const result: Array<number> = [0]
  let index = 0
  let join = false
  let regional = 0
  while (index < value.length) {
    const code = value.codePointAt(index) ?? 0
    const combining = (code >= 0x300 && code <= 0x36f) || (code >= 0x1ab0 && code <= 0x1aff) ||
      (code >= 0x1dc0 && code <= 0x1dff) || (code >= 0x20d0 && code <= 0x20ff) ||
      (code >= 0xfe00 && code <= 0xfe0f) || (code >= 0xfe20 && code <= 0xfe2f) ||
      (code >= 0x1f3fb && code <= 0x1f3ff) || (code >= 0xe0100 && code <= 0xe01ef)
    const flag = code >= 0x1f1e6 && code <= 0x1f1ff
    if (index > 0 && !join && !combining && code !== 0x200d && !(flag && regional % 2 === 1)) result.push(index)
    join = code === 0x200d
    regional = flag ? regional + 1 : 0
    index += code > 0xffff ? 2 : 1
  }
  if (result[result.length - 1] !== value.length) result.push(value.length)
  return result
}
export function moveSelectionBoundary(value: string, start: number, end: number, edge: string, offset: number): Array<number> {
  return moveBoundary(selectionBoundaries(value), start, end, edge, offset)
}
function boundaryIndex(boundaries: Array<number>, offset: number): number {
  let low = 0; let high = boundaries.length - 1
  while (low < high) {
    const middle = Math.floor((low + high) / 2)
    if (boundaries[middle] < offset) low = middle + 1
    else high = middle
  }
  return low
}
function moveBoundary(boundaries: Array<number>, start: number, end: number, edge: string, offset: number): Array<number> {
  const index = boundaryIndex(boundaries, offset)
  const before = boundaries[Math.max(0, index - 1)]
  const after = boundaries[index]
  const nearest = Math.abs(before - offset) <= Math.abs(after - offset) ? before : after
  if (edge === 'start') return [Math.min(nearest, boundaries[Math.max(0, boundaryIndex(boundaries, end) - 1)]), end]
  return [start, Math.max(nearest, boundaries[Math.min(boundaries.length - 1, boundaryIndex(boundaries, start) + 1)])]
}

export class TextSelectionLayout {
  readonly id: string = `message-selection-${nextSelectionId++}`
  private nodes: Map<string, SelectionNode> = new Map()
  private segments: Map<string, MarkdownSelectionSegment> = new Map()
  private boundaries: Array<number> = []
  projection: MarkdownSelectionProjection = { text: '', segments: [] }
  private key(block: number, row: number, column: number): string { return `${block}:${row}:${column}` }
  reset(projection: MarkdownSelectionProjection): void {
    this.segments.clear(); this.boundaries = []; this.projection = projection
    for (const segment of projection.segments) this.segments.set(this.key(segment.block, segment.row, segment.column), segment)
    // ForEach retains finished blocks during streaming. Their Text nodes keep
    // their controllers, so retain those bindings and update only text offsets.
    for (const entry of this.nodes.entries()) {
      const segment = this.segments.get(entry[0])
      if (segment === undefined) this.nodes.delete(entry[0])
      else entry[1].segment = segment
    }
  }
  private characterBoundaries(): Array<number> {
    if (this.boundaries.length === 0) this.boundaries = selectionBoundaries(this.projection.text)
    return this.boundaries
  }
  move(start: number, end: number, edge: string, offset: number): Array<number> {
    return moveBoundary(this.characterBoundaries(), start, end, edge, offset)
  }
  nodeId(block: number, row: number = -1, column: number = -1): string { return `${this.id}-${this.key(block, row, column)}` }
  clipId(block: number): string { return `${this.id}-scroll-${block}` }
  controller(block: number, row: number = -1, column: number = -1, clipped: boolean = false): TextController {
    const key = this.key(block, row, column)
    const existing = this.nodes.get(key)
    if (existing !== undefined) {
      existing.clipId = clipped ? this.clipId(block) : ''
      return existing.controller
    }
    const segment = this.segments.get(key) ??
      { block, row, column, start: 0, end: 0, text: '' }
    const node: SelectionNode = { segment, controller: new TextController(), id: this.nodeId(block, row, column),
      clipId: clipped ? this.clipId(block) : '' }
    this.nodes.set(key, node)
    return node.controller
  }
  // ComponentUtils and layout rectangles use px; gesture/window UI uses vp.
  bounds(ui: UIContext, id: string): SelectionRect {
    const info = ui.getComponentUtils().getRectangleById(id)
    return { left: ui.px2vp(info.windowOffset.x), top: ui.px2vp(info.windowOffset.y),
      right: ui.px2vp(info.windowOffset.x + info.size.width), bottom: ui.px2vp(info.windowOffset.y + info.size.height) }
  }
  private clipping(ui: UIContext, node: SelectionNode, viewport: SelectionRect): SelectionRect {
    return node.clipId.length === 0 ? viewport : intersect(viewport, this.bounds(ui, node.clipId))
  }
  private rects(ui: UIContext, node: SelectionNode, start: number, end: number): Array<SelectionRect> {
    const origin = this.bounds(ui, node.id)
    return node.controller.getLayoutManager().getRectsForRange({ start, end }, drawingText.RectWidthStyle.TIGHT,
      drawingText.RectHeightStyle.TIGHT).map((box: drawingText.TextBox): SelectionRect => ({
      left: origin.left + ui.px2vp(box.rect.left), top: origin.top + ui.px2vp(box.rect.top),
      right: origin.left + ui.px2vp(box.rect.right), bottom: origin.top + ui.px2vp(box.rect.bottom) }))
  }
  view(ui: UIContext, start: number, end: number, anchorX: number, anchorY: number): MessageSelectionView {
    const root = this.bounds(ui, this.id)
    const viewport = intersect(root, this.bounds(ui, 'message-list'))
    const result = emptySelectionView()
    const boundaries = this.characterBoundaries()
    const selected = this.projection.segments.filter((segment: MarkdownSelectionSegment): boolean =>
      segment.end > start && segment.start < end && segment.text.length > 0)
    const firstSelected = selected[0]
    const lastSelected = selected[selected.length - 1]
    for (const node of this.nodes.values()) {
      const segment = node.segment
      if (segment.end <= start || segment.start >= end) continue
      const first = Math.max(start, segment.start) - segment.start
      const last = Math.min(end, segment.end) - segment.start
      const clip = this.clipping(ui, node, viewport)
      const nodeVisible = intersect(this.bounds(ui, node.id), clip)
      if (nodeVisible.right <= nodeVisible.left || nodeVisible.bottom <= nodeVisible.top) continue
      for (const rect of this.rects(ui, node, first, last)) {
        const visible = intersect(rect, clip)
        if (visible.right > visible.left && visible.bottom > visible.top) result.rects.push({
          left: visible.left - root.left, top: visible.top - root.top,
          right: visible.right - root.left, bottom: visible.bottom - root.top })
      }
      if (segment === firstSelected) {
        const actualStart = Math.max(start, segment.start)
        const next = boundaries[Math.min(boundaries.length - 1, boundaryIndex(boundaries, actualStart) + 1)]
        const rect = this.rects(ui, node, first, Math.min(segment.end, next) - segment.start)[0]
        if (rect !== undefined && contains(clip, rect.left, (rect.top + rect.bottom) / 2)) result.start = {
          x: rect.left - root.left, top: Math.max(rect.top, clip.top) - root.top, bottom: Math.min(rect.bottom, clip.bottom) - root.top }
      }
      if (segment === lastSelected) {
        const actualEnd = Math.min(end, segment.end)
        const previous = boundaries[Math.max(0, boundaryIndex(boundaries, actualEnd) - 1)]
        const rects = this.rects(ui, node, Math.max(segment.start, previous) - segment.start, last)
        const rect = rects[rects.length - 1]
        if (rect !== undefined && contains(clip, rect.right, (rect.top + rect.bottom) / 2)) result.end = {
          x: rect.right - root.left, top: Math.max(rect.top, clip.top) - root.top, bottom: Math.min(rect.bottom, clip.bottom) - root.top }
      }
    }
    result.menuX = Math.max(viewport.left, Math.min(anchorX - 40, viewport.right - 80)) - root.left
    result.menuY = Math.max(viewport.top + 4, Math.min(anchorY - 52, viewport.bottom - 44)) - root.top
    return result
  }
  offsetAt(ui: UIContext, x: number, y: number): number | null {
    const viewport = intersect(this.bounds(ui, this.id), this.bounds(ui, 'message-list'))
    let best: SelectionNode | null = null
    let distance = Number.MAX_VALUE
    for (const node of this.nodes.values()) {
      if (node.segment.text.length === 0) continue
      const visible = intersect(this.bounds(ui, node.id), this.clipping(ui, node, viewport))
      if (visible.right <= visible.left || visible.bottom <= visible.top) continue
      const dx = Math.max(visible.left - x, 0, x - visible.right)
      const dy = Math.max(visible.top - y, 0, y - visible.bottom)
      const next = dx * dx + dy * dy
      if (next < distance) { distance = next; best = node }
    }
    if (best === null) return null
    const bounds = this.bounds(ui, best.id)
    const position = best.controller.getLayoutManager().getCharacterPositionAtCoordinate(
      ui.vp2px(x - bounds.left), ui.vp2px(y - bounds.top))?.position
    return position === undefined ? null : best.segment.start + Math.max(0, Math.min(best.segment.text.length, position))
  }
}
