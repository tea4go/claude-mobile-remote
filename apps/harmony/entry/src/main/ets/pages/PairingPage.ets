import type { ErrorInfo } from '../protocol/types'
import { ConnectionStage } from '../session/model'
import { SelfCheckPanel } from '../components/SelfCheckPanel'

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
  @Prop pairingInput: string = ''
  @Prop connection: ConnectionStage = ConnectionStage.DISCONNECTED
  @Prop lastError: ErrorInfo | null = null
  @Prop selfChecks: Array<string> = []
  @Prop selfCheckError: string = ''
  @Prop canBack: boolean = false
  onBack: () => void = () => {}
  onPairingInputChange: (value: string) => void = () => {}
  onConnect: () => void = () => {}

  build() {
    Scroll() {
      Column({ space: 16 }) {
        if (this.canBack) {
          Button('返回列表')
            .id('pairing-back-button')
            .onClick(() => {
              this.onBack()
            })
        }

        Text('Claude Mobile Remote')
          .fontSize(24)
          .fontWeight(FontWeight.Bold)
          .width('100%')
          .textAlign(TextAlign.Start)

        SelfCheckPanel({
          results: this.selfChecks,
          error: this.selfCheckError,
        })

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

          Button('连接当前 Session')
            .id('connect-button')
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
