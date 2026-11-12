import { QuickPairForm } from './QuickPairForm'
import type { ErrorInfo } from '../protocol/types'
import { ConnectionStage } from '../session/model'
import { PAGE_BACKGROUND, PRIMARY, SECONDARY } from '../ui/styles'

function phaseLabel(connection: ConnectionStage): string {
  switch (connection) {
    case ConnectionStage.CONNECTING: return '正在连接（10秒）'
    case ConnectionStage.AUTHENTICATING: return '正在认证（8秒）'
    case ConnectionStage.SUBSCRIBING: return '正在订阅（8秒）'
    case ConnectionStage.SUBSCRIBED: return '正在同步快照（10秒）'
    case ConnectionStage.READY: return '连接已验证'
    case ConnectionStage.PAIRING_INVALID: return '配对失效'
    case ConnectionStage.PROTOCOL_ERROR: return '协议错误'
    case ConnectionStage.DISCONNECTED: return '未连接'
  }
}

function buildPairingHint(connection: ConnectionStage, error: ErrorInfo | null): string {
  if (connection === ConnectionStage.PAIRING_INVALID || error?.code === 'unauthorized') {
    return '当前配对已失效，需要重新配对。'
  }
  if (error?.code === 'expired') {
    return '配对串已过期，请重新配对。'
  }
  if (error?.code === 'invalid') {
    return '配对串解析失败，或 wsUrl 非法，请检查后重试。'
  }
  if (error !== null) {
    return error.message
  }
  return '请输入配对文本并连接。'
}

@Component
export struct PairingPage {
  @State manual: boolean = false
  onDemo: () => void = () => {}
  @Prop target: string = ''
  @Prop nativeReady: boolean = false
  @Prop scanning: boolean = false
  @Prop saving: boolean = false
  onScan: () => void = () => {}
  @Prop pairingInput: string = ''
  @Prop connection: ConnectionStage = ConnectionStage.DISCONNECTED
  @Prop lastError: ErrorInfo | null = null
  @Prop selfChecks: Array<string> = []
  @Prop selfCheckError: string = ''
  @Prop canBack: boolean = false
  onBack: () => void = () => {}
  onSettings: () => void = () => {}
  onPairingInputChange: (value: string) => void = () => {}
  onConnect: () => void = () => {}
  onCancel: () => void = () => {}
  @Prop projectCwd: string = ''

  private connecting(): boolean {
    return this.connection === ConnectionStage.CONNECTING || this.connection === ConnectionStage.AUTHENTICATING ||
      this.connection === ConnectionStage.SUBSCRIBING || this.connection === ConnectionStage.SUBSCRIBED
  }

  build() {
    Column() {
      Row() {
        if (this.canBack) { Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(24).height(24) }.width(44).height(44).fontSize(30).fontColor(SECONDARY).backgroundColor('#ffffff').borderRadius(22).accessibilityText('返回').onClick(() => this.onBack()) }
        if (!this.canBack) { Blank().width(44) }
        Text('ccBuddy').fontSize(17).fontWeight(FontWeight.Medium).layoutWeight(1).textAlign(TextAlign.Center)
        Button('设置', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(14).fontColor(PRIMARY).backgroundColor(Color.Transparent).onClick(() => this.onSettings())
      }.width('100%').height(56).padding({ left: 8, right: 8 })
      Scroll() {
        Column({ space: 16 }) {
          Button('• 体验演示（无需连接电脑）', { type: ButtonType.Normal }).id('open-demo-button').height(36).fontSize(13).fontColor(PRIMARY).backgroundColor('#e9f1ff').borderRadius(18).alignSelf(ItemAlign.Center).enabled(!this.scanning && !this.connecting()).onClick(() => this.onDemo())
          Text('连接 Claude 会话').fontSize(30).fontWeight(FontWeight.Bold).width('100%')
          Button(this.manual ? '通过 PC IP 快速连接' : '导入配对信息').onClick(() => { this.manual = !this.manual }).enabled(!this.connecting() && !this.saving)
          if (!this.manual) {
            QuickPairForm({ disabled: !this.nativeReady || this.scanning || this.connecting() || this.saving, onPair: (text: string) => { this.onPairingInputChange(text); this.onConnect() } })
          }
          if (this.manual) {
          Text('粘贴配对 JSON 或完整 #pair Fragment').fontSize(14).fontColor(SECONDARY).width('100%')
          if (this.projectCwd.length > 0) { Text(`请扫描目标电脑和目录：${this.projectCwd}`).fontSize(13).fontColor(PRIMARY).width('100%') }
          TextArea({ text: this.pairingInput, placeholder: '粘贴配对文本' }).id('pairing-input').height(140).fontSize(15).padding(14).backgroundColor('#ffffff').border({ width: 1, color: '#dfe6f1' }).borderRadius(16).enabled(!this.connecting() && !this.scanning).onChange((value: string) => this.onPairingInputChange(value)).width('100%')
          }
          Text('目标 PC 地址（IP / 主机名及端口）').fontSize(14).fontWeight(FontWeight.Medium).width('100%')
          Text(this.target.length > 0 ? this.target : '粘贴或扫码后显示目标地址').fontSize(14).fontColor(SECONDARY).padding(16).width('100%').backgroundColor('#ffffff').border({ width: 1, color: '#dfe6f1' }).borderRadius(16)
          Text('请与电脑配对窗口核对，手机所在网络需要能访问此地址。').fontSize(12).fontColor(SECONDARY).width('100%')
          Text(this.saving ? '连接验证成功，正在保存配对…' : this.connecting() ? phaseLabel(this.connection) : buildPairingHint(this.connection, this.lastError)).fontSize(13).fontColor(this.lastError === null ? SECONDARY : '#c0392b').width('100%')
          if (this.manual) { Button(this.saving ? '正在保存…' : this.connecting() ? phaseLabel(this.connection) : '连接', { type: ButtonType.Normal }).id('connect-button').height(48).width('100%').fontColor('#ffffff').backgroundColor(PRIMARY).borderRadius(16).enabled(this.nativeReady && !this.scanning && !this.connecting()).onClick(() => this.onConnect()) }
          if (this.connecting() && !this.saving) { Button('取消连接', { type: ButtonType.Normal }).height(44).width('100%').onClick(() => this.onCancel()) }
          Button(this.scanning ? '正在打开扫码…' : '扫描二维码', { type: ButtonType.Normal }).id('scan-button').height(48).width('100%').fontColor(PRIMARY).backgroundColor('#ffffff').border({ width: 1, color: PRIMARY }).borderRadius(16).enabled(this.nativeReady && !this.scanning && !this.connecting()).onClick(() => this.onScan())
        }.padding(24).width('100%').alignItems(HorizontalAlign.Start)
      }.layoutWeight(1).width('100%').align(Alignment.TopStart)
    }.width('100%').height('100%').backgroundColor(PAGE_BACKGROUND)
  }
}
