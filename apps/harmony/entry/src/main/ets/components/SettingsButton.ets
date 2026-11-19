@Component
export struct SettingsButton {
  onOpen: () => void = () => {}

  build() {
    Button({ type: ButtonType.Normal }) {
      Row({ space: 5 }) {
        Image($r('app.media.setting')).width(15).height(15).fillColor($r('app.color.text'))
        Text('设置').fontSize(14).fontColor($r('app.color.text'))
      }
    }.height(40).padding({ left: 12, right: 12 }).backgroundColor($r('app.color.surface'))
      .border({ width: 1, color: $r('app.color.border'), radius: 20 })
      .accessibilityText('设置').onClick(() => this.onOpen())
  }
}
