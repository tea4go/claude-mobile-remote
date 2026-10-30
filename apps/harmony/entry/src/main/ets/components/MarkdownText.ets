import { inlineMarkdown, markdownBlocks, safeMarkdownUrl, type InlineText, type MarkdownBlock } from '../session/markdown'
import { nativeLifecycle } from '../native/lifecycle'

@Component
export struct MarkdownText {
  @Prop @Watch('parse') text: string = ''
  @State private blocks: Array<MarkdownBlock> = []
  private tableWidths: Map<MarkdownBlock, Array<number>> = new Map()
  aboutToAppear(): void { this.parse() }
  private parse(): void {
    const blocks = markdownBlocks(this.text)
    this.tableWidths.clear()
    for (const block of blocks) {
      if (block.kind !== 'table') continue
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
    }
    this.blocks = blocks
  }
  private openLink(url: string): void {
    const safe = safeMarkdownUrl(url)
    if (safe.length === 0) return
    void nativeLifecycle.context?.startAbility({ action: 'ohos.want.action.viewData', uri: safe }).catch(() => {})
  }
  private alignment(value: string): TextAlign {
    return value === 'center' ? TextAlign.Center : value === 'right' ? TextAlign.End : TextAlign.Start
  }
  @Builder
  private inline(text: string, size: number = 16, strong: boolean = false, alignment: string = 'left') {
    Text() {
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
    }.fontSize(size).lineHeight(size * 1.5).copyOption(CopyOptions.LocalDevice).width('100%').textAlign(this.alignment(alignment))
  }
  @Builder
  private content(block: MarkdownBlock) {
    if (block.kind === 'code') {
      Column() {
        if (block.language.length > 0) {
          Text(block.language).fontSize(11).fontColor('#6e6e73').padding({ left: 10, top: 6 }).width('100%')
        }
        Scroll() {
          Text(block.text).fontSize(14).lineHeight(21).fontFamily('monospace').copyOption(CopyOptions.LocalDevice)
            .padding(10).textAlign(TextAlign.Start)
        }.scrollable(ScrollDirection.Horizontal).width('100%').align(Alignment.TopStart)
      }.width('100%').backgroundColor('#f6f8fa').border({ width: 1, color: '#e3e6ec' }).borderRadius(6).clip(true)
    } else if (block.kind === 'table') {
      Scroll() {
        Column() {
          ForEach(block.rows, (row: Array<string>, rowIndex: number) => {
            Row() {
              ForEach(row, (cell: string, column: number) => {
                Column() { this.inline(cell, 16, rowIndex === 0, block.alignments[column]) }
                  .width(this.tableWidths.get(block)?.[column] ?? 180).padding({ left: 8, right: 8, top: 5, bottom: 5 }).border({ width: 0.5, color: '#e3e6ec' })
                  .backgroundColor(rowIndex === 0 ? '#f5f7fa' : '#ffffff')
              }, (cell: string, index: number) => `${index}-${cell}`)
            }.alignItems(VerticalAlign.Top)
          }, (row: Array<string>, index: number) => `${index}-${JSON.stringify(row)}`)
        }.alignItems(HorizontalAlign.Start)
      }.scrollable(ScrollDirection.Horizontal).width('100%').borderRadius(8).clip(true)
    } else if (block.kind === 'rule') {
      Divider().color('#e3e6ec').margin({ top: 8, bottom: 8 })
    } else if (block.kind === 'heading') {
      this.inline(block.text, Math.max(16, 28 - block.level * 2), true)
    } else {
      this.inline(block.text)
    }
  }
  build() {
    Column({ space: 8 }) {
      ForEach(this.blocks, (block: MarkdownBlock) => {
        Row({ space: 6 }) {
          if (block.marker.length > 0) {
            Text(block.marker).fontSize(16).lineHeight(24).fontColor('#6e6e73').constraintSize({ minWidth: 18 })
              .accessibilityText(block.marker === '☑' ? '已完成' : block.marker === '☐' ? '未完成' : block.marker)
          }
          Column() { this.content(block) }.layoutWeight(1).alignItems(HorizontalAlign.Start)
        }.width('100%').alignItems(VerticalAlign.Top)
          .padding({ left: Math.max(0, Math.min(block.indent, 8) - (block.marker.length > 0 ? 1 : 0)) * 20 + Math.min(block.quote, 4) * 12, top: 2, bottom: 2 })
          .border({ width: { left: block.quote > 0 ? 3 : 0 }, color: '#d0d7de' })
      }, (block: MarkdownBlock, index: number) => `${index}-${JSON.stringify(block)}`)
    }.width('100%').alignItems(HorizontalAlign.Start)
  }
}
