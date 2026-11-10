import type { QueuedRequest } from '../protocol/types'
import { PRIMARY } from '../ui/styles'

@Component
export struct QueuedSummary {
  @Prop queuedRequests: QueuedRequest[] = []
  @Prop cancelAllowed: boolean = false
  @Prop steerAllowed: boolean = false
  onCancelQueued: (requestId: string) => void = () => {}
  onSteerQueued: (requestId: string) => void = () => {}

  build() {
    Column({ space: 8 }) {
      ForEach(this.queuedRequests, (item: QueuedRequest, index: number) => {
        Column({ space: 6 }) {
          Text(`${index + 1} · ${item.source === 'terminal' ? '电脑端' : '手机端'}`).fontSize(12).fontColor('#777777').width('100%')
          Text(item.preview).fontSize(14).maxLines(3).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
          Row() {
            Blank()
            if (this.steerAllowed) {
              Button('引导', { type: ButtonType.Normal }).height(44).fontColor(PRIMARY).backgroundColor(Color.Transparent)
                .enabled(this.cancelAllowed).onClick(() => this.onSteerQueued(item.requestId))
            }
            Button('取消排队', { type: ButtonType.Normal }).height(44).fontColor('#b3261e').backgroundColor(Color.Transparent)
              .enabled(this.cancelAllowed).onClick(() => this.onCancelQueued(item.requestId))
          }.width('100%')
        }.padding(12).width('100%').backgroundColor('#f4f8ff').borderRadius(12)
      }, (item: QueuedRequest) => item.requestId)
    }.id('queue-summary').width('100%').alignItems(HorizontalAlign.Start)
  }
}
