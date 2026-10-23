import type { QueuedRequest } from '../protocol/types'

@Component
export struct QueuedSummary {
  @Prop queuedRequests: QueuedRequest[] = []
  @Prop cancelAllowed: boolean = false
  onCancelQueued: (requestId: string) => void = () => {}

  build() {
    Column({ space: 8 }) {
      if (this.queuedRequests.length > 0) {
        Text(`等待处理的输入 (${this.queuedRequests.length})`)
          .fontSize(18)
          .fontWeight(FontWeight.Medium)

        ForEach(this.queuedRequests, (item: QueuedRequest) => {
          Row({ space: 8 }) {
            Text(item.source === 'terminal' ? '电脑端' : '手机端')
              .fontSize(12)
              .padding({ left: 8, right: 8, top: 4, bottom: 4 })
              .backgroundColor('#ffffff')
              .borderRadius(999)

            Text(item.preview)
              .fontSize(14)
              .layoutWeight(1)
              .textAlign(TextAlign.Start)

            Button('取消', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor('#1677ff').backgroundColor('#eef5ff').borderRadius(8)
              .enabled(this.cancelAllowed)
              .onClick(() => {
                this.onCancelQueued(item.requestId)
              })
          }
          .width('100%')
          .alignItems(VerticalAlign.Center)
        }, (item: QueuedRequest) => item.requestId)
      }
    }
    .id('queue-summary')
    .padding(16)
    .width('100%')
    .alignItems(HorizontalAlign.Start)
    .backgroundColor('#f4f8ff')
    .borderRadius(12)
  }
}
