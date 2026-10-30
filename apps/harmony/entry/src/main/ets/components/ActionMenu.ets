import { PRIMARY } from '../ui/styles'
import { pageFade } from '../ui/transitions'

export interface ActionMenuItem {
  title: string
  action: () => void
  danger?: boolean
  enabled?: boolean
}

// Explicit dimensions also keep the preview and device menus visually consistent.
@CustomDialog
export struct ActionMenu {
  controller?: CustomDialogController
  title: string = '会话操作'
  items: Array<ActionMenuItem> = []
  onClose: () => void = () => {}

  build() {
    Scroll() {
      Column({ space: 8 }) {
        Text(this.title).fontSize(18).fontWeight(FontWeight.Medium)
          .fontColor('#252525').width('100%').padding({ top: 4, bottom: 12 })
        ForEach(this.items, (item: ActionMenuItem) => {
          Button({ type: ButtonType.Normal }) {
            Text(item.title).fontSize(16).fontColor(item.danger === true ? '#d4380d' : '#252525')
              .width('100%').textAlign(TextAlign.Start)
          }.width('100%').constraintSize({ minHeight: 52 }).padding({ left: 16, right: 16, top: 14, bottom: 14 })
            .backgroundColor('#f2f2f7').borderRadius(12).hoverEffect(HoverEffect.Highlight)
            .enabled(item.enabled !== false).onClick(() => { this.onClose(); item.action() })
        }, (item: ActionMenuItem) => item.title)
        Button('关闭', { type: ButtonType.Normal }).width('100%').height(44).fontSize(16)
          .fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(12)
          .hoverEffect(HoverEffect.Highlight).onClick(() => this.onClose())
      }.padding(16).width('100%').alignItems(HorizontalAlign.Start)
    }.constraintSize({ maxHeight: '80%' }).width('92%').align(Alignment.TopStart)
      .backgroundColor('#ffffff').borderRadius(20).clip(true).margin({ bottom: 12 }).transition(pageFade())
  }
}
