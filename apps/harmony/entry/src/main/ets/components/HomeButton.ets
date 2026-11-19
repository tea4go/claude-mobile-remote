@Component
export struct HomeButton {
  onOpen: () => void = () => {}

  build() {
    Button({ type: ButtonType.Normal }) {
      Image($r('app.media.nav_home')).width(15).height(15)
    }.width(44).height(44).padding(0).borderRadius(12).clip(true).stateEffect(false)
      .stateStyles({ normal: { .backgroundColor('#00000000') }, pressed: { .backgroundColor('#e6effe') } })
      .accessibilityText('返回首页').onClick(() => this.onOpen())
  }
}
