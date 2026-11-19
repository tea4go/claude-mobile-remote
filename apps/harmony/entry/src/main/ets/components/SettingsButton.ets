@Component
export struct SettingsButton {
  onOpen: () => void = () => {}

  build() {
    Button({ type: ButtonType.Normal }) {
      Row({ space: 5 }) {
        Image($r('app.media.setting')).width(15).height(15).fillColor($r('app.color.text'))
        Text('设置').fontSize(14).fontColor($r('app.color.text'))
      }
    }.height(44).padding({ left: 4, right: 4 }).borderRadius(12).clip(true).stateEffect(false)
      .stateStyles({ normal: { .backgroundColor('#00000000') }, pressed: { .backgroundColor('#e6effe') } })
      .accessibilityText('设置').onClick(() => this.onOpen())
  }
}
