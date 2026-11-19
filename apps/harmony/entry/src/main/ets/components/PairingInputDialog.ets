@CustomDialog
export struct PairingInputDialog {
  controller?: CustomDialogController
  @Link value: string
  @Prop rename: boolean = false
  @Link error: string
  onCancel: () => void = () => {}
  onVerify: () => void = () => {}
  build() {
    Column({ space: 16 }) {
      Text(this.rename ? '重命名会话' : '输入验证码').fontSize(18).fontWeight(FontWeight.Bold)
      Text(this.rename ? '请输入本地名称，最多 40 个字符' : '请输入 PC 上的六位验证码')
        .fontSize(14).fontColor($r('app.color.text_secondary'))
      TextInput({ text: this.value, placeholder: this.rename ? '输入会话名称' : '请输入六位验证码' })
        .type(this.rename ? InputType.Normal : InputType.Number).maxLength(this.rename ? 40 : 6)
        .defaultFocus(this.rename).selectAll(this.rename)
        .onChange((v: string) => { this.value = this.rename ? v : v.replace(/\D/g, '') }).height(52).width('100%')
      if (this.error.length > 0) {
        Text(this.error).width('100%').padding(12).fontSize(14).fontColor('#a42c26')
          .backgroundColor('#fff4f3').borderRadius(12)
      }
      Row({ space: 12 }) {
        Button('取消', { type: ButtonType.Normal }).layoutWeight(1).borderRadius(8)
          .border({ width: 1, color: $r('app.color.border'), radius: 8 })
          .backgroundColor($r('app.color.surface')).fontColor($r('app.color.text')).onClick(() => this.onCancel())
        Button(this.rename ? '保存' : '验证并连接', { type: ButtonType.Normal }).layoutWeight(1)
          .borderRadius(8).backgroundColor($r('app.color.primary_fill')).fontColor(Color.White).enabled(this.rename ? this.value.trim().length > 0 : this.value.length === 6).onClick(() => this.onVerify())
      }.width('100%')
    }.padding(24).width('90%').backgroundColor($r('app.color.surface')).borderRadius(18)
  }
}
