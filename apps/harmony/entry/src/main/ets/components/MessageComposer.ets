import { MAX_TEXT_BYTES } from '../protocol/types'

function utf8ByteLength(text: string): number {
  let total = 0
  for (let index = 0; index < text.length; index += 1) {
    const codeUnit = text.charCodeAt(index)
    if (codeUnit <= 0x7f) {
      total += 1
    } else if (codeUnit <= 0x7ff) {
      total += 2
    } else if (codeUnit >= 0xd800 && codeUnit <= 0xdbff && index + 1 < text.length) {
      const nextCodeUnit = text.charCodeAt(index + 1)
      if (nextCodeUnit >= 0xdc00 && nextCodeUnit <= 0xdfff) {
        total += 4
        index += 1
      } else {
        total += 3
      }
    } else {
      total += 3
    }
  }
  return total
}

@Component
export struct MessageComposer {
  @Prop composerEnabled: boolean = false
  onSend: (text: string) => void = () => {}

  @State private text: string = ''

  private submit(): void {
    const trimmed = this.trimmedText()
    if (!this.composerEnabled || trimmed.length === 0 || this.isOversize()) {
      return
    }
    this.onSend(trimmed)
    this.text = ''
  }

  private trimmedText(): string {
    return this.text.trim()
  }

  private isOversize(): boolean {
    const trimmed = this.trimmedText()
    return trimmed.length > 0 && utf8ByteLength(trimmed) > MAX_TEXT_BYTES
  }

  build() {
    Column({ space: 8 }) {
      if (this.isOversize()) {
        Text('文本超出长度上限')
          .fontSize(12)
          .fontColor('#c0392b')
          .width('100%')
      }

      TextInput({
        text: this.text,
        placeholder: '发送文本给 Claude',
      })
        .id('send-input')
        .enabled(this.composerEnabled)
        .onChange((value: string) => {
          this.text = value
        })
        .width('100%')

      Button('发送')
        .id('send-button')
        .enabled(this.composerEnabled && this.trimmedText().length > 0 && !this.isOversize())
        .onClick(() => {
          this.submit()
        })
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
  }
}
