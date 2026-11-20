import { BuildFooter } from '../components/BuildFooter'
import { SettingsButton } from '../components/SettingsButton'
import { nativeLifecycle } from '../native/lifecycle'
import type { PCGroup, PCProfile } from '../session/projectGroups'
import { QuickPairForm } from './QuickPairForm'
import type { ErrorInfo } from '../protocol/types'
import { ConnectionStage } from '../session/model'
import { PAGE_BACKGROUND, PRIMARY, SECONDARY } from '../ui/styles'

function buildPairingHint(connection: ConnectionStage, error: ErrorInfo | null): string {
  if (connection === ConnectionStage.PAIRING_INVALID || error?.code === 'unauthorized') {
    return '配对凭证无效或已过期，请在电脑刷新二维码后重新配对'
  }
  if (error?.code === 'expired') {
    return '配对凭证已过期，请在电脑刷新二维码后重新配对'
  }
  if (error?.code === 'invalid') {
    return '无法读取配对信息，请重新扫描电脑端的配对二维码'
  }
  if (connection === ConnectionStage.PROTOCOL_ERROR) {
    return '电脑返回的配对或会话数据无法读取，请更新电脑端和手机端后重试'
  }
  if (error !== null) {
    return error.message
  }
  return '请输入配对文本并连接。'
}

@Component
export struct PairingPage {
  @State private editOpen: boolean = false
  @State private editingTest: boolean = false
  private beganAtEdge: boolean = false
  @State manual: boolean = false
  @Prop addressHistory: Array<string> = []
  onHistoryChange: (values: Array<string>) => void = () => {}
  @Prop profiles: Array<PCProfile> = []
  onSaveComputer: (profile: PCProfile) => void = () => {}
  onRemoveComputer: (key: string) => void = () => {}
  @Prop computers: Array<PCGroup> = []
  @Prop selectedPC: string = 'all'
  onSelectPC: (key: string) => void = () => {}
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
  onHome: () => void = () => {}
  onBack: () => void = () => {}
  onSettings: () => void = () => {}
  onPairingInputChange: (value: string) => void = () => {}
  onConnect: () => void = () => {}
  onDebugActionsTop: (top: number) => void = () => {}
  onCancel: () => void = () => {}
  claimInitialRefresh: () => boolean = () => false
  @Prop projectPC: string = ''
  @Prop projectCwd: string = ''

  private connecting(): boolean {
    return this.connection === ConnectionStage.CONNECTING || this.connection === ConnectionStage.AUTHENTICATING ||
      this.connection === ConnectionStage.SUBSCRIBING || this.connection === ConnectionStage.SUBSCRIBED
  }

  build() {
    Stack() {
    Column() {
      if (!this.editOpen) {
      Row() {
        if (this.canBack) { Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(24).height(24) }.width(44).height(44).margin({ right: 24 }).fontSize(30).fontColor(SECONDARY).backgroundColor($r('app.color.surface')).borderRadius(22).accessibilityText('返回').onClick(() => this.onBack()) }
        if (!this.canBack) { Row() { Image($r('app.media.home_logo')).width(30).height(30).borderRadius(7) }.width(68) }
        Text('ccBuddy').fontSize(24).fontWeight(FontWeight.Bolder).layoutWeight(1).textAlign(TextAlign.Center)
        SettingsButton({ onOpen: () => this.onSettings() })
      }.width('100%').height(56).padding({ left: 16, right: 16 })
      }
        Column({ space: 8 }) {
          if (!this.editOpen) Text('连接 Claude 会话').fontSize(26).fontWeight(FontWeight.Bold).width('100%')
            .onClick(() => this.getUIContext().getFocusController().clearFocus())
          QuickPairForm({ editOpen: $editOpen, editTesting: $editingTest, pairingError: this.lastError === null ? '' : buildPairingHint(this.connection, this.lastError), addressHistory: this.addressHistory, onHistoryChange: this.onHistoryChange, profiles: this.profiles, onSaveComputer: this.onSaveComputer, onRemoveComputer: this.onRemoveComputer, computers: this.computers, selectedPC: this.selectedPC, onSelectPC: this.onSelectPC, projectCwd: this.projectCwd, projectPC: this.projectPC, claimInitialRefresh: this.claimInitialRefresh, onHome: this.onHome, disabled: !this.nativeReady || this.scanning || this.connecting() || this.saving,
            onPair: (text: string) => { this.onPairingInputChange(text); this.onConnect() }, onScan: () => this.onScan(), onDebugActionsTop: this.onDebugActionsTop })
        }.padding({ left: this.editOpen ? 0 : 20, right: this.editOpen ? 0 : 20,
          top: this.editOpen ? 0 : 6, bottom: this.editOpen ? 0 : 1 }).width('100%').layoutWeight(1).alignItems(HorizontalAlign.Start)
      if (!this.editOpen) { BuildFooter() }
    }.width('100%').height('100%').backgroundColor(PAGE_BACKGROUND)
    if (this.editingTest) {
      Stack() {
        LoadingProgress().width(36).height(36).color(PRIMARY).accessibilityText('正在测试连接')
      }.width('100%').height('100%').backgroundColor('#40000000').hitTestBehavior(HitTestMode.Block)
    }
    if (this.connecting() || this.saving) {
      Stack() {
      Column({ space: 14 }) {
        LoadingProgress().width(36).height(36).color(Color.White)
        Text(this.scanning ? '正在扫码…' : this.saving ? '正在保存…' : '正在连接…').fontColor(Color.White)
        Button('取消').enabled(!this.saving).onClick(() => this.onCancel())
      }.padding(24).backgroundColor('#cc202020').borderRadius(12)
      }.width('100%').height('100%').backgroundColor('#01000000').hitTestBehavior(HitTestMode.Block)
    }
    }.width('100%').height('100%').hitTestBehavior(HitTestMode.Default)
      .parallelGesture(PanGesture({ direction: PanDirection.Right, distance: 12 })
        .onActionStart((event: GestureEvent) => {
          this.beganAtEdge = event.fingerList.length > 0 && event.fingerList[0].localX - event.offsetX <= 24
        })
        .onActionEnd((event: GestureEvent) => {
          if (this.beganAtEdge && !this.saving && !this.connecting() && event.offsetX >= 65) {
            if (this.editOpen) nativeLifecycle.detailBack?.()
            else if (this.canBack) this.onBack()
          }
          this.beganAtEdge = false
        }))
  }
}
