import { inlineMarkdown, markdownBlocks, type InlineText, type MarkdownBlock } from '../session/markdown'
import { nativeLifecycle } from '../native/lifecycle'

@Component
export struct MarkdownText {
  @Prop text: string = ''
  private openLink(url: string): void {
    if (!/^https?:\/\//.test(url)) return
    void nativeLifecycle.context?.startAbility({ action: 'ohos.want.action.viewData', uri: url }).catch(() => {})
  }
  @Builder
  private inline(text: string, size: number = 14) {
    Text() {
      ForEach(inlineMarkdown(text), (part: InlineText) => {
        Span(part.text)
          .fontWeight(part.style === 'bold' ? FontWeight.Bold : FontWeight.Normal)
          .fontFamily(part.style === 'code' ? 'monospace' : 'sans-serif')
          .fontColor(part.style === 'link' ? '#1677ff' : '#252525')
          .decoration({ type: part.style === 'strike' ? TextDecorationType.LineThrough : TextDecorationType.None })
          .onClick(() => { if (part.style === 'link') this.openLink(part.url) })
      }, (part: InlineText, index: number) => `${index}-${JSON.stringify(part)}`)
    }.fontSize(size).copyOption(CopyOptions.LocalDevice).width('100%').textAlign(TextAlign.Start)
  }
  build() {
    Column({ space: 6 }) {
      ForEach(markdownBlocks(this.text), (block: MarkdownBlock) => {
        if (block.kind === 'code') {
          Scroll() {
            Text(block.text).fontSize(13).fontFamily('monospace').copyOption(CopyOptions.LocalDevice).padding(10)
          }.scrollable(ScrollDirection.Horizontal).width('100%').backgroundColor('#f4f6f8').borderRadius(8)
        } else if (block.kind === 'table') {
          Scroll() {
            Column() {
              ForEach(block.rows, (row: Array<string>, rowIndex: number) => {
                Row() {
                  ForEach(row, (cell: string) => {
                    Column() { this.inline(cell) }.width(180).padding(8).border({ width: 0.5, color: '#d9d9d9' })
                      .backgroundColor(rowIndex === 0 ? '#f4f6f8' : '#ffffff')
                  }, (cell: string, index: number) => `${index}-${cell}`)
                }.alignItems(VerticalAlign.Top)
              }, (row: Array<string>, index: number) => `${index}-${JSON.stringify(row)}`)
            }.alignItems(HorizontalAlign.Start)
          }.scrollable(ScrollDirection.Horizontal).width('100%')
        } else if (block.kind === 'heading') {
          this.inline(block.text, Math.max(16, 26 - block.level * 2))
        } else {
          this.inline(block.text)
        }
      }, (block: MarkdownBlock, index: number) => `${index}-${JSON.stringify(block)}`)
    }.width('100%').alignItems(HorizontalAlign.Start)
  }
}
