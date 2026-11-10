import { inlineMarkdown, markdownBlocks, markdownSelectionProjection, safeMarkdownUrl, type InlineText, type MarkdownBlock } from '../session/markdown'
import { nativeLifecycle } from '../native/lifecycle'
import { copyVisibleText } from '../native/clipboard'
import { selectionHaptic } from '../native/haptics'
import { messageSelection, type ActiveMessageSelection } from '../ui/messageSelection'
import { TextSelectionLayout, emptySelectionView, type MessageSelectionView, type SelectionHandle, type SelectionRect } from '../ui/textSelectionLayout'

@Component
export struct MarkdownText {
  @Prop @Watch('parse') text: string = ''
  @Prop plain: boolean = false
  scrollController: Scroller = new Scroller()
  @State private blocks: Array<MarkdownBlock> = []
  @State private selectionOpen: boolean = false
  @State private selectionView: MessageSelectionView = emptySelectionView()
  @State private dragging: boolean = false
  private readonly selection: TextSelectionLayout = new TextSelectionLayout()
  private start: number = 0
  private end: number = 0
  private anchorX: number = 0
  private anchorY: number = 0
  private dragEdge: string = ''
  private dragOriginX: number = 0
  private dragOriginY: number = 0
  private dragX: number = 0
  private dragY: number = 0
  private dragTimer: number = -1
  private scrolls: Map<number, Scroller> = new Map()
  private tableWidths: Map<MarkdownBlock, Array<number>> = new Map()
  private tableWidthCache: Map<string, Array<number>> = new Map()
  aboutToAppear(): void { this.parse() }
  aboutToDisappear(): void { this.closeSelection() }
  private readonly selectionOwner: ActiveMessageSelection = {
    close: (): void => this.closeSelection(),
    refresh: (): void => this.refreshSelection(),
    contains: (x: number, y: number): boolean => {
      try {
        const root = this.selection.bounds(this.getUIContext(), this.selection.id)
        return x >= root.left - 24 && x <= root.right + 24 && y >= root.top - 12 && y <= root.bottom + 24
      } catch (_) { return false }
    }
  }
  private closeSelection(): void {
    messageSelection.release(this.selectionOwner)
    this.selectionOpen = false
    this.dragging = false
    this.dragEdge = ''
    if (this.dragTimer >= 0) clearTimeout(this.dragTimer)
    this.dragTimer = -1
    this.selectionView = emptySelectionView()
  }
  private selectAll(event: GestureEvent): void {
    if (this.selection.projection.text.length === 0) return
    const finger = event.fingerList.find((item: FingerInfo): boolean => item !== undefined)
    if (finger === undefined) return
    messageSelection.open(this.selectionOwner)
    this.start = 0
    this.end = this.selection.projection.text.length
    this.anchorX = finger.globalX
    this.anchorY = finger.globalY
    this.selectionOpen = true
    this.refreshSelection()
    if (this.selectionOpen && this.selectionView.rects.length > 0) selectionHaptic()
    else this.closeSelection()
  }
  private refreshSelection(): void {
    if (!this.selectionOpen) return
    try { this.selectionView = this.selection.view(this.getUIContext(), this.start, this.end, this.anchorX, this.anchorY) }
    catch (_) { this.closeSelection() }
  }
  private scrollFor(block: number): Scroller {
    let scroller = this.scrolls.get(block)
    if (scroller === undefined) { scroller = new Scroller(); this.scrolls.set(block, scroller) }
    return scroller
  }
  private beginDrag(edge: string, event: GestureEvent): void {
    if (!this.selectionOpen) return
    try {
    const handle = edge === 'start' ? this.selectionView.start : this.selectionView.end
    if (handle === null) return
    const root = this.selection.bounds(this.getUIContext(), this.selection.id)
    this.dragging = true
    this.dragEdge = edge
    this.dragOriginX = root.left + handle.x
    this.dragOriginY = root.top + (handle.top + handle.bottom) / 2
    this.updateDrag(event)
    this.autoScroll()
    } catch (_) { this.closeSelection() }
  }
  private updateDrag(event: GestureEvent): void {
    if (!this.dragging) return
    this.dragX = this.dragOriginX + event.offsetX
    this.dragY = this.dragOriginY + event.offsetY
    this.adjustBoundary()
  }
  private adjustBoundary(): void {
    try {
      const offset = this.selection.offsetAt(this.getUIContext(), this.dragX, this.dragY)
      if (offset !== null) {
        const range = this.selection.move(this.start, this.end, this.dragEdge, offset)
        this.start = range[0]; this.end = range[1]
      }
      this.anchorX = this.dragX; this.anchorY = this.dragY
      this.refreshSelection()
    } catch (_) { this.closeSelection() }
  }
  private autoScroll(): void {
    if (!this.dragging) return
    try {
      const viewport = this.selection.bounds(this.getUIContext(), 'message-list')
      const speed = (point: number, min: number, max: number): number =>
        point < min + 28 ? -Math.min(12, (min + 28 - point) / 2) : point > max - 28 ? Math.min(12, (point - max + 28) / 2) : 0
      const dy = speed(this.dragY, viewport.top, viewport.bottom)
      if (dy !== 0) this.scrollController.scrollBy(0, dy)
      for (const entry of this.scrolls.entries()) {
        const clip = this.selection.bounds(this.getUIContext(), this.selection.clipId(entry[0]))
        if (this.dragY < clip.top || this.dragY > clip.bottom) continue
        const dx = speed(this.dragX, Math.max(clip.left, viewport.left), Math.min(clip.right, viewport.right))
        if (dx !== 0) entry[1].scrollBy(dx, 0)
      }
      this.adjustBoundary()
      if (this.dragging) this.dragTimer = setTimeout(() => this.autoScroll(), 32)
    } catch (_) { this.closeSelection() }
  }
  private endDrag(): void {
    this.dragging = false
    if (this.dragTimer >= 0) clearTimeout(this.dragTimer)
    this.dragTimer = -1
    this.refreshSelection()
  }
  private copySelection(): void {
    const value = this.selection.projection.text.slice(this.start, this.end)
    this.closeSelection()
    void copyVisibleText(value)
  }
  private parse(): void {
    this.closeSelection()
    this.scrolls.clear()
    const blocks: Array<MarkdownBlock> = this.plain ? [] : markdownBlocks(this.text)
    this.selection.reset(this.plain ? { text: this.text, segments: [{ block: 0, row: -1, column: -1, start: 0, end: this.text.length, text: this.text }] } : markdownSelectionProjection(blocks))
    this.tableWidths.clear()
    for (const block of blocks) {
      if (block.kind !== 'table') continue
      // Streaming frequently reparses finished tables; retain only a bounded set
      // and include system configuration so font-scale changes invalidate widths.
      const cacheKey = JSON.stringify([nativeLifecycle.context?.config, block.rows, block.alignments])
      const cached = this.tableWidthCache.get(cacheKey)
      if (cached !== undefined) {
        this.tableWidths.set(block, cached)
        continue
      }
      const widths = block.alignments.map((): number => 70)
      block.rows.forEach((row: Array<string>, rowIndex: number) => {
        row.forEach((cell: string, column: number) => {
          let width = 0
          for (const part of inlineMarkdown(cell)) {
            const measured = this.getUIContext().getMeasureUtils().measureTextSize({ textContent: part.text,
              fontSize: part.code ? '14fp' : '16fp', fontWeight: rowIndex === 0 || part.bold ? '700' : '400',
              fontFamily: part.code ? 'monospace' : 'sans-serif', fontStyle: part.italic ? FontStyle.Italic : FontStyle.Normal })
            const pixels = measured.width
            width += this.getUIContext().px2vp(typeof pixels === 'number' ? pixels : 0)
          }
          widths[column] = Math.max(widths[column], Math.ceil(width) + 22)
        })
      })
      this.tableWidths.set(block, widths)
      if (this.tableWidthCache.size >= 32) this.tableWidthCache.clear()
      this.tableWidthCache.set(cacheKey, widths)
    }
    this.blocks = blocks
  }
  private openLink(url: string): void {
    this.closeSelection()
    const safe = safeMarkdownUrl(url)
    if (safe.length === 0) return
    void nativeLifecycle.context?.startAbility({ action: 'ohos.want.action.viewData', uri: safe }).catch(() => {})
  }
  private alignment(value: string): TextAlign {
    return value === 'center' ? TextAlign.Center : value === 'right' ? TextAlign.End : TextAlign.Start
  }
  @Builder
  private inline(text: string, block: number, size: number = 16, strong: boolean = false, alignment: string = 'left', row: number = -1, column: number = -1) {
    Text(undefined, { controller: this.selection.controller(block, row, column, row >= 0) }) {
      ForEach(inlineMarkdown(text), (part: InlineText) => {
        if (part.url.length > 0) {
          Span(part.text).fontColor('#1677ff').fontWeight(strong || part.bold ? FontWeight.Bold : FontWeight.Normal)
            .fontStyle(part.italic ? FontStyle.Italic : FontStyle.Normal)
            .fontFamily(part.code ? 'monospace' : 'sans-serif')
            .decoration({ type: part.strike ? TextDecorationType.LineThrough : TextDecorationType.Underline })
            .onClick(() => this.openLink(part.url))
        } else {
          Span(part.text).fontWeight(strong || part.bold ? FontWeight.Bold : FontWeight.Normal)
            .fontStyle(part.italic ? FontStyle.Italic : FontStyle.Normal)
            .fontFamily(part.code ? 'monospace' : 'sans-serif').fontSize(part.code ? 14 : size)
            .fontColor('#252525')
            .textBackgroundStyle({ color: part.code ? '#f0f0f0' : Color.Transparent, radius: 4 })
            .decoration({ type: part.strike ? TextDecorationType.LineThrough : TextDecorationType.None })
        }
      }, (part: InlineText, index: number) => `${index}-${JSON.stringify(part)}`)
    }.id(this.selection.nodeId(block, row, column)).fontSize(size).lineHeight(size * 1.5).copyOption(CopyOptions.None).width('100%').textAlign(this.alignment(alignment))
  }
  @Builder
  private content(block: MarkdownBlock, index: number) {
    if (block.kind === 'code') {
      Column() {
        if (block.language.length > 0) {
          Text(block.language).fontSize(11).fontColor('#6e6e73').padding({ left: 10, top: 6 }).width('100%')
        }
        Scroll(this.scrollFor(index)) {
          Column() {
            Text(block.text, { controller: this.selection.controller(index, -1, -1, true) }).id(this.selection.nodeId(index))
              .fontSize(14).lineHeight(21).fontFamily('monospace').copyOption(CopyOptions.None).textAlign(TextAlign.Start)
          }.padding(10)
        }.id(this.selection.clipId(index)).scrollable(ScrollDirection.Horizontal).width('100%').align(Alignment.TopStart)
          .onScroll(() => messageSelection.refresh())
      }.width('100%').backgroundColor('#f6f8fa').border({ width: 1, color: '#e3e6ec' }).borderRadius(6).clip(true)
    } else if (block.kind === 'table') {
      Scroll(this.scrollFor(index)) {
        Column() {
          ForEach(block.rows, (row: Array<string>, rowIndex: number) => {
            Row() {
              ForEach(row, (cell: string, column: number) => {
                Column() { this.inline(cell, index, 16, rowIndex === 0, block.alignments[column], rowIndex, column) }
                  .width(this.tableWidths.get(block)?.[column] ?? 180).padding({ left: 8, right: 8, top: 5, bottom: 5 }).border({ width: 0.5, color: '#e3e6ec' })
                  .backgroundColor(rowIndex === 0 ? '#f5f7fa' : '#ffffff')
              }, (cell: string, index: number) => `${index}-${cell}`)
            }.alignItems(VerticalAlign.Top)
          }, (row: Array<string>, index: number) => `${index}-${JSON.stringify(row)}`)
        }.alignItems(HorizontalAlign.Start)
      }.id(this.selection.clipId(index)).scrollable(ScrollDirection.Horizontal).width('100%').borderRadius(8).clip(true)
        .onScroll(() => messageSelection.refresh())
    } else if (block.kind === 'rule') {
      Divider().color('#e3e6ec').margin({ top: 8, bottom: 8 })
    } else if (block.kind === 'heading') {
      this.inline(block.text, index, Math.max(16, 28 - block.level * 2), true)
    } else {
      this.inline(block.text, index)
    }
  }
  @Builder
  private selectionHandle(edge: string, handle: SelectionHandle) {
    Stack({ alignContent: Alignment.TopStart }) {
      Rect().width(2).height(handle.bottom - handle.top).fill('#1677ff').position({ x: 21, y: 12 }).hitTestBehavior(HitTestMode.None)
      Circle().width(10).height(10).fill('#1677ff').position({ x: 17, y: edge === 'start' ? 3 : handle.bottom - handle.top + 11 }).hitTestBehavior(HitTestMode.None)
    }.width(44).height(Math.max(44, handle.bottom - handle.top + 24)).backgroundColor(Color.Transparent)
      .position({ x: handle.x - 22, y: handle.top - 12 })
      .accessibilityText(edge === 'start' ? '调整选区起点' : '调整选区终点')
      .priorityGesture(PanGesture({ direction: PanDirection.All, distance: 1 })
        .onActionStart((event: GestureEvent) => this.beginDrag(edge, event))
        .onActionUpdate((event: GestureEvent) => this.updateDrag(event))
        .onActionEnd(() => this.endDrag()).onActionCancel(() => this.endDrag()))
  }
  build() {
    Stack({ alignContent: Alignment.TopStart }) {
    if (this.plain) {
      Text(this.text, { controller: this.selection.controller(0) }).id(this.selection.nodeId(0))
        .fontSize(14).fontColor('#ffffff').copyOption(CopyOptions.None).width('100%').textAlign(TextAlign.Start)
    } else {
    Column({ space: 8 }) {
      ForEach(this.blocks, (block: MarkdownBlock, index: number) => {
        Row({ space: 6 }) {
          if (block.marker.length > 0) {
            Text(block.marker, { controller: this.selection.controller(index, -2, -2) }).id(this.selection.nodeId(index, -2, -2))
              .fontSize(16).lineHeight(24).fontColor('#6e6e73').constraintSize({ minWidth: 18 }).copyOption(CopyOptions.None)
              .accessibilityText(block.marker === '☑' ? '已完成' : block.marker === '☐' ? '未完成' : block.marker)
          }
          Column() { this.content(block, index) }.layoutWeight(1).alignItems(HorizontalAlign.Start)
        }.width('100%').alignItems(VerticalAlign.Top)
          .padding({ left: Math.max(0, Math.min(block.indent, 8) - (block.marker.length > 0 ? 1 : 0)) * 20 + Math.min(block.quote, 4) * 12, top: 2, bottom: 2 })
          .border({ width: { left: block.quote > 0 ? 3 : 0 }, color: '#d0d7de' })
      }, (block: MarkdownBlock, index: number) => `${index}-${JSON.stringify(block)}`)
    }.width('100%').alignItems(HorizontalAlign.Start)
    }
    if (this.selectionOpen) {
      ForEach(this.selectionView.rects, (rect: SelectionRect) => {
        Rect().width(rect.right - rect.left).height(rect.bottom - rect.top).fill(this.plain ? '#60b8e4ff' : '#553478f6')
          .position({ x: rect.left, y: rect.top }).hitTestBehavior(HitTestMode.None)
      }, (rect: SelectionRect, index: number) => `${index}`)
      if (this.selectionView.start !== null) { this.selectionHandle('start', this.selectionView.start as SelectionHandle) }
      if (this.selectionView.end !== null) { this.selectionHandle('end', this.selectionView.end as SelectionHandle) }
      if (!this.dragging && this.selectionView.rects.length > 0) {
        Button('复制', { type: ButtonType.Normal }).width(80).height(40).fontSize(14).fontColor('#ffffff').backgroundColor('#252525').borderRadius(10)
          .position({ x: this.selectionView.menuX, y: this.selectionView.menuY }).onClick(() => this.copySelection())
      }
    }
    }.id(this.selection.id).width('100%')
      .gesture(LongPressGesture({ duration: 500, repeat: false }).onAction((event: GestureEvent) => this.selectAll(event)))
      .onVisibleAreaChange([0], (visible: boolean) => { if (!visible) this.closeSelection() })
      .onAreaChange((oldArea: Area, area: Area) => {
        if (oldArea.width !== area.width || oldArea.height !== area.height) this.closeSelection()
      })
  }
}
