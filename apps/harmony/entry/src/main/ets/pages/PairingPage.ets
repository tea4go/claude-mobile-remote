import type { ErrorInfo } from '../protocol/types'
import { ConnectionStage } from '../session/model'
import { SelfCheckPanel } from '../components/SelfCheckPanel'

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
  onDemo: () => void = () => {}
  @Prop target: string = ''
  @Prop nativeReady: boolean = false
  @Prop scanning: boolean = false
  onScan: () => void = () => {}
  @Prop pairingInput: string = ''
  @Prop connection: ConnectionStage = ConnectionStage.DISCONNECTED
  @Prop lastError: ErrorInfo | null = null
  @Prop selfChecks: Array<string> = []
  @Prop selfCheckError: string = ''
  @Prop canBack: boolean = false
  onBack: () => void = () => {}
  onPairingInputChange: (value: string) => void = () => {}
  onConnect: () => void = () => {}

  private connecting(): boolean {
    return this.connection === ConnectionStage.CONNECTING || this.connection === ConnectionStage.AUTHENTICATING ||
      this.connection === ConnectionStage.SUBSCRIBING || this.connection === ConnectionStage.SUBSCRIBED
  }

  build() {
    Scroll() {
      Column({ space: 16 }) {
        Button('体验演示（无需连接电脑）').id('open-demo-button').height(44).enabled(!this.scanning).onClick(() => this.onDemo())
        if (this.target.length > 0) { Text(`目标：${this.target} · ${phaseLabel(this.connection)}`).fontSize(12).width('100%') }
        Button(this.scanning ? '正在打开扫码…' : '扫描二维码').id('scan-button').height(48)
          .enabled(this.nativeReady && !this.scanning && !this.connecting()).onClick(() => this.onScan())
        if (this.canBack) {
          Button('返回列表')
            .id('pairing-back-button')
            .onClick(() => {
              this.onBack()
            })
        }

        Text('Whale Remote Control')
          .fontSize(24)
          .fontWeight(FontWeight.Bold)
          .width('100%')
          .textAlign(TextAlign.Start)

        Column({ space: 12 }) {
          Text('新增配对')
            .fontSize(18)
            .fontWeight(FontWeight.Medium)

          TextInput({
            text: this.pairingInput,
            placeholder: '粘贴配对文本',
          })
            .id('pairing-input')
            .onChange((value: string) => {
              this.onPairingInputChange(value)
            })
            .width('100%')

          Button(this.connecting() ? phaseLabel(this.connection) : '连接当前 Session')
            .id('connect-button').enabled(this.nativeReady && !this.scanning && !this.connecting())
            .onClick(() => {
              this.onConnect()
            })

          Text(buildPairingHint(this.connection, this.lastError))
            .fontSize(14)
            .fontColor(this.lastError === null ? '#555555' : '#c0392b')
            .width('100%')
        }
        .padding(16)
        .width('100%')
        .alignItems(HorizontalAlign.Start)
        .backgroundColor('#ffffff')
        .borderRadius(12)
      }
      .padding(16)
      .width('100%')
      .alignItems(HorizontalAlign.Start)
    }
    .width('100%')
    .height('100%')
  }
}
