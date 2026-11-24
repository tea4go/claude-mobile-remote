import { HomeButton } from '../components/HomeButton'
import { DirectoryTags } from '../components/DirectoryTags'
import { PairingInputDialog } from '../components/PairingInputDialog'
import { PAGE_BACKGROUND } from '../ui/styles'
import { pageSlide } from '../ui/transitions'
import { validComputerIP, type PCGroup, type PCProfile } from '../session/projectGroups'
import { nativeLifecycle } from '../native/lifecycle'
import { webSocket } from '@kit.NetworkKit'
import { RectShape } from '@kit.ArkUI'
import { BusinessError } from '@kit.BasicServicesKit'
import { PRIMARY, SECONDARY } from '../ui/styles'
import { pasteboard } from '@kit.BasicServicesKit'
import { decodePairingPayload } from '../protocol/pairing'

const ACTION_BUTTON_RADIUS = 14
const PC_TEST_BUTTON_SIZE = 34
const PC_TEST_BUTTON_RADIUS = 8

export function validPairAddress(value: string): boolean {
  const match = /^(\[[0-9a-f:]+\]|(?:\d{1,3}\.){3}\d{1,3})(?::([0-9]{1,5}))?$/i.exec(value)
  return match !== null && validComputerIP(match[1]) &&
    (!match[2] || (Number(match[2]) > 0 && Number(match[2]) <= 65535))
}

class QuickPayload {
  code: string = ''
  expiresAt: string = ''
  wsUrl: string = ''
}
class QuickFrame {
  version: number = 1
  type: string = ''
  requestId: string = ''
  payload: QuickPayload = new QuickPayload()
}
@CustomDialog
struct LoadingDialog {
  controller?: CustomDialogController
  onCancel: () => void = () => {}
  build() {
    Column({ space: 14 }) {
      LoadingProgress().width(36).height(36).color(Color.White)
      Text('正在连接…').fontColor(Color.White)
      Button('取消').onClick(() => this.onCancel())
    }.padding(24).backgroundColor('#cc202020').borderRadius(12)
  }
}

@Component
export struct QuickPairForm {
  @Prop addressHistory: Array<string> = []
  onHistoryChange: (values: Array<string>) => void = () => {}
  @State private historyOpen: boolean = false
  private addressTop: number = 0
  private addressBottom: number = 0
  private historyDirty: boolean = false
  private historyTimer: number = -1
  private blurTimer: number = -1

  private matchingAddresses(): Array<string> {
    const input = this.ip.trim().toLowerCase()
    return this.addressHistory.filter((address: string) => address.toLowerCase().includes(input))
  }

  private rememberAddress(): void {
    clearTimeout(this.historyTimer)
    const value = this.ip.trim()
    if (this.historyDirty && !this.legacy && validPairAddress(value)) {
      this.historyDirty = false
      this.onHistoryChange([value].concat(this.addressHistory.filter((item: string) =>
        item.toLowerCase() !== value.toLowerCase())).slice(0, 10))
    }
  }

  private updateAddress(value: string): void {
    this.ip = value
    this.historyDirty = true
    this.legacy = ''
    clearTimeout(this.historyTimer)
    this.historyTimer = setTimeout(() => this.rememberAddress(), 1000)
  }

  @Prop profiles: Array<PCProfile> = []
  onSaveComputer: (profile: PCProfile) => void = () => {}
  onRemoveComputer: (key: string) => void = () => {}
  @Link editTesting: boolean
  @State private editResult: string = ''
  @State private editResultOK: boolean = false
  @State private editedHost: string = ''
  @Prop @Watch('pairingErrorChanged') pairingError: string = ''

  private pairingErrorChanged(): void {
    this.error = this.pairingError
    if (this.error.length > 0) this.clearTestResult()
  }
  @Link editOpen: boolean
  @State private editKey: string = ''
  @State private editName: string = ''
  @State private editIP: string = ''

  private computerHost(computer: PCGroup): string {
    return this.profiles.find((profile: PCProfile) => profile.key === computer.key)?.ip ??
      (/^wss?:\/\/(\[[^\]]+\]|[^/:]+)/.exec(computer.sessions[0].wsUrl)?.[1] ?? '')
  }

  private computerName(computer: PCGroup): string {
    return this.profiles.find((profile: PCProfile) => profile.key === computer.key)?.name ?? computer.label
  }

  @Prop @Watch('refreshOnEntry') computers: Array<PCGroup> = []
  @Prop selectedPC: string = 'all'
  @State private computersCollapsed: boolean = false
  claimInitialRefresh: () => boolean = () => false
  onHome: () => void = () => {}
  @Prop projectPC: string = ''
  @Prop @Watch('showProjectNotice') projectCwd: string = ''
  @State private projectNoticeDismissed: boolean = false

  private showProjectNotice(): void { this.projectNoticeDismissed = false }
  onSelectPC: (key: string) => void = () => {}
  onPair: (text: string) => void = () => {}
  onScan: () => void = () => {}
  onDebugActionsTop: (top: number) => void = () => {}
  @Prop @Watch('disabledChanged') disabled: boolean = false
  @State ip: string = ''
  @State code: string = ''
  @State phase: string = 'idle'
  @State error: string = ''
  @State private pressedPC: string = ''
  @State private refreshing: boolean = false
  @State private pendingPCs: Array<string> = []
  @State private reachablePCs: Array<string> = []
  @State private unreachablePCs: Array<string> = []
  private refreshQueue: Array<PCGroup> = []


  private refreshOnEntry(): void {
    if (this.disabled || this.editOpen || !nativeLifecycle.foreground ||
      this.computers.length === 0 || this.probe !== null) return
    if (this.claimInitialRefresh()) this.refreshComputers()
  }

  private refreshComputers(): void {
    this.refreshQueue = this.computers.slice()
    this.pendingPCs = this.refreshQueue.map((computer: PCGroup) => computer.key)
    this.refreshing = true
    this.refreshNext()
  }

  private refreshNext(): void {
    const next = this.refreshQueue.shift()
    if (next === undefined) {
      this.refreshing = false
      return
    }
    this.testComputer(next, true)
  }

  private computerStatusColor(computer: PCGroup): string {
    if (this.pendingPCs.includes(computer.key)) return '#e6a23c'
    if (this.reachablePCs.includes(computer.key)) return '#31a577'
    if (this.unreachablePCs.includes(computer.key)) return '#d7353e'
    return computer.connected > 0 ? '#31a577' : '#d7353e'
  }

  private finishPCRefresh(key: string): void {
    this.pendingPCs = this.pendingPCs.filter((pending: string) => pending !== key)
    this.refreshNext()
  }

  @State private testingPC: string = ''
  @State private testResult: string = ''
  @State private resultOpen: boolean = false
  @State private testedHost: string = ''
  @State private testedName: string = ''
  @State private testResultOK: boolean = false
  @State private copyFeedback: string = ''

  private clearTestResult(): void {
    this.testResult = ''
    this.resultOpen = false
    this.copyFeedback = ''
  }

  private dismissAddressInput(): void {
    this.getUIContext().getFocusController().clearFocus()
    this.historyOpen = false
  }
  private probe: webSocket.WebSocket | null = null
  private probeTimeout: number = -1

  private cancelProbe(): void {
    clearTimeout(this.probeTimeout)
    const socket = this.probe
    this.probe = null
    this.testingPC = ''
    this.editTesting = false
    socket?.close().catch(() => {})
  }

  private testComputer(computer: PCGroup, silent: boolean = false): void {
    this.cancelProbe()
    if (!silent) {
      this.dismissAddressInput()
      this.clearTestResult()
      this.error = ''
    }
    const host = this.computerHost(computer)
    if (!host) { if (silent) this.finishPCRefresh(computer.key); return }
    const socket = webSocket.createWebSocket()
    this.probe = socket
    if (!silent) this.testingPC = computer.key
    const finish = (result: string, reachable: boolean = false): void => {
      if (this.probe !== socket) return
      this.cancelProbe()
      this.reachablePCs = this.reachablePCs.filter((key: string) => key !== computer.key)
      this.unreachablePCs = this.unreachablePCs.filter((key: string) => key !== computer.key)
      if (reachable) this.reachablePCs = this.reachablePCs.concat([computer.key])
      else this.unreachablePCs = this.unreachablePCs.concat([computer.key])
      if (silent) { this.finishPCRefresh(computer.key); return }
      this.testedHost = host
      this.testedName = this.computerName(computer)
      this.testResult = result
      this.testResultOK = reachable
    }
    this.probeTimeout = setTimeout(() => finish('连接超时'), 3000)
    socket.on('open', () => finish('PC 服务连接成功', true))
    socket.on('error', () => finish('PC 服务连接失败'))
    socket.on('close', () => finish('PC 服务连接失败'))
    socket.connect(`ws://${host}:7330/quick-pair/v1`).catch(() => finish('PC 服务连接失败'))
  }

  private legacy: string = ''
  private verifyDialog: CustomDialogController = new CustomDialogController({
    builder: PairingInputDialog({ value: $code, error: $error, onCancel: () => { this.error = ''; this.stop() }, onVerify: () => { this.error = ''; this.verifyDialog.close(); this.phase = 'verifying'; this.loadingDialog.open(); this.send('verify') } }),
    autoCancel: false, customStyle: true
  })
  private loadingDialog: CustomDialogController = new CustomDialogController({ builder: LoadingDialog({ onCancel: () => this.stop() }), autoCancel: false, customStyle: true })
  private socket: webSocket.WebSocket | null = null
  private timer: number = -1
  private requestId: string = ''
  private host: string = ''
  private unsubscribe: (() => void) | null = null
  aboutToAppear(): void { nativeLifecycle.detailBack = this.editorBack; this.unsubscribe = nativeLifecycle.subscribe(() => { if (!nativeLifecycle.foreground) { this.stopRefresh(); this.stop() } else { this.refreshOnEntry() } }) }
  aboutToDisappear(): void { if (nativeLifecycle.detailBack === this.editorBack) nativeLifecycle.detailBack = null; this.unsubscribe?.(); this.unsubscribe = null; clearTimeout(this.historyTimer); clearTimeout(this.blurTimer); this.stopRefresh(); this.stop() }
  private readonly editorBack: () => boolean = () => {
    if (!this.editOpen) return false
    this.closeEditor()
    return true
  }
  private disabledChanged(): void { if (this.disabled) this.stop(); else this.refreshOnEntry() }
  private stopRefresh(): void {
    this.cancelProbe()
    this.refreshQueue = []
    this.pendingPCs = []
    this.refreshing = false
  }
  private stop(): void {
    if (this.timer >= 0) clearTimeout(this.timer)
    this.timer = -1
    const socket = this.socket
    this.socket = null
    if (socket !== null) socket.close().catch(() => {})
    this.verifyDialog.close()
    this.loadingDialog.close()
    this.phase = 'idle'
    this.requestId = ''
  }
  private fail(message: string): void {
    this.stop()
    this.clearTestResult()
    this.error = message
  }
  private send(kind: string): void {
    const frame = new QuickFrame()
    frame.type = kind
    frame.requestId = this.requestId
    frame.payload.code = kind === 'verify' ? this.code : ''
    const socket = this.socket
    socket?.send(JSON.stringify(frame)).catch(() => {
      if (this.socket === socket) this.fail(kind === 'verify' ? '验证码配对连接已断开，请重新获取验证码' : '已连接电脑，但未能获取验证码。请检查电脑端配对窗口后重试')
    })
  }
  private start(): void {
    this.rememberAddress(); this.dismissAddressInput(); this.clearTestResult()
    this.stop(); this.error = ''; this.code = ''
    if (this.legacy.length > 0) { this.onPair(this.legacy); return }
    const match = /^(\[[0-9a-fA-F:]+\]|(?:\d{1,3}\.){3}\d{1,3})(?::([0-9]{1,5}))?$/.exec(this.ip.trim())
    if (match === null || (!match[1].startsWith('[') && !match[1].split('.').every((v: string) => Number(v) <= 255)) || (match[2] && (Number(match[2]) < 1 || Number(match[2]) > 65535))) { this.fail('请输入有效的 IP 地址或配对信息'); return }
    this.host = match[1]
    const socket = webSocket.createWebSocket()
    this.socket = socket; this.phase = 'connecting'; this.loadingDialog.open()
    const address = `${this.host}:${match[2] || '7330'}`
    const unavailable = `连接失败（${address}）。请核对电脑 IP、确认远程服务已开启，并检查手机网络`
    let opened = false
    const interrupted = (): void => {
      if (this.socket !== socket) return
      this.fail(this.requestId.length > 0 ? '验证码配对连接已断开，请重新获取验证码' :
        opened ? '已连接电脑，但未能获取验证码。请检查电脑端配对窗口后重试' : unavailable)
    }
    this.timer = setTimeout(() => {
      if (this.socket !== socket) return
      this.fail(opened ? '电脑未及时返回验证码，请检查电脑端配对窗口后重试' : `连接电脑超时（${address}）。请核对 IP、远程服务和手机网络`)
    }, 8000)
    socket.on('open', (err: BusinessError) => {
      if (this.socket !== socket) return
      if (err !== null && err !== undefined) { interrupted(); return }
      opened = true
      this.send('begin')
    })
    socket.on('error', interrupted)
    socket.on('close', interrupted)
    socket.on('message', (_err: BusinessError, value: string | ArrayBuffer) => {
      if (this.socket !== socket) return
      if (typeof value !== 'string') { this.fail('配对响应无效'); return }
      try {
        const frame = JSON.parse(value) as QuickFrame
        if (frame.version !== 1) { this.fail('配对响应无效'); return }
        if (frame.type === 'ready') {
          const ttl = Date.parse(frame.payload.expiresAt) - Date.now()
          if (!frame.requestId || !Number.isFinite(ttl) || ttl <= 0 || ttl > 125000) { this.fail('配对已失效'); return }
          this.requestId = frame.requestId; this.phase = 'code'; this.loadingDialog.close(); this.verifyDialog.open(); clearTimeout(this.timer)
          this.timer = setTimeout(() => this.fail('验证码已过期，请重新连接'), ttl)
        } else if (frame.type === 'paired' && this.requestId.length > 0 && frame.requestId === this.requestId) {
          // Replace only the address host; the existing pairing parser validates all remaining fields.
          frame.payload.wsUrl = frame.payload.wsUrl.replace(/^(wss?:\/\/)(\[[^\]]+\]|[^/:]+)/, `$1${this.host}`)
          const text = JSON.stringify(frame.payload)
          this.stop(); this.onPair(text)
        } else if (frame.type === 'error') {
          if (frame.payload.code === 'invalid_code' || frame.payload.code === 'target_required') {
            this.error = frame.payload.code === 'invalid_code' ? '验证码不正确' : '请先在 PC 选择会话'; this.phase = 'code'; this.loadingDialog.close(); this.verifyDialog.open()
          } else this.fail(frame.payload.code === 'busy' ? 'PC 正在处理其他配对，请稍后重试' : '配对已失效，请重试')
        } else this.fail('配对响应无效')
      } catch (_) { this.fail('配对响应无效') }
    })
    socket.connect(`ws://${address}/quick-pair/v1`).then((started: boolean) => { if (!started) interrupted() }).catch(interrupted)
  }
  private async paste(): Promise<void> {
    try {
      const data = await pasteboard.getSystemPasteboard().getData()
      this.acceptInput(data.getPrimaryText() || '')
    } catch (_) { this.fail('无法读取剪贴板，请使用系统粘贴') }
  }
  private acceptInput(text: string): void {
      const value = text.trim()
      const parsed = decodePairingPayload(value)
      if (parsed.ok && parsed.payload !== null) {
        const match = /^wss?:\/\/(\[[^\]]+\]|[^/:]+)/.exec(parsed.payload.wsUrl)
        if (match !== null) { this.legacy = value; this.ip = match[1]; return }
      }
      const match = /^(\[[0-9a-fA-F:]+\]|(?:\d{1,3}\.){3}\d{1,3})(?::([0-9]{1,5}))?$/.exec(value)
      if (match !== null && (!match[2] || (Number(match[2]) > 0 && Number(match[2]) <= 65535)) && (match[1].startsWith('[') || match[1].split('.').every((v: string) => Number(v) <= 255))) {
        this.legacy = ''; this.updateAddress(value); return
      }
      this.legacy = ''; this.fail(parsed.error?.code === 'expired' ? '配对凭证已过期，请重新配对' : '请输入有效的 IP 地址或配对信息')
  }
  private testEditedAddress(): void {
    this.dismissAddressInput()
    this.cancelProbe()
    const input = this.editIP.trim()
    if (!validComputerIP(input)) return
    const host = input.includes(':') && !input.startsWith('[') ? '[' + input + ']' : input
    const socket = webSocket.createWebSocket()
    this.probe = socket
    this.editTesting = true
    this.editResult = ''
    this.copyFeedback = ''
    const started = Date.now()
    let finished = false
    const finish = (message: string, ok: boolean = false): void => {
      if (finished || this.probe !== socket) return
      finished = true
      clearTimeout(this.probeTimeout)
      this.probeTimeout = setTimeout(() => {
        if (this.probe !== socket) return
        this.cancelProbe()
        if (this.editOpen) {
          this.editedHost = host
          this.editResult = message
          this.editResultOK = ok
        }
      }, Math.max(0, 600 - (Date.now() - started)))
    }
    this.probeTimeout = setTimeout(() => finish('连接超时'), 3000)
    socket.on('open', () => finish('PC 服务连接成功', true))
    socket.on('error', () => finish('PC 服务连接失败'))
    socket.on('close', () => finish('PC 服务连接失败'))
    socket.connect(`ws://${host}:7330/quick-pair/v1`).catch(() => finish('PC 服务连接失败'))
  }

  private closeEditor(): void {
    this.cancelProbe()
    this.setEditorOpen(false)
  }

  private setEditorOpen(open: boolean): void {
    this.getUIContext().animateTo({ duration: 220, curve: Curve.EaseOut }, () => {
      this.editOpen = open
    })
  }

  @Builder
  private editComputer(): void {
    Column({ space: 10 }) {
      Row() {
        Button({ type: ButtonType.Normal }) {
          Image($r('app.media.nav_back')).width(24).height(24)
        }.width(44).height(44).backgroundColor($r('app.color.surface')).borderRadius(22)
          .accessibilityText('返回').onClick(() => this.closeEditor())
        Text('编辑连接').fontSize(17).fontWeight(FontWeight.Medium).layoutWeight(1).textAlign(TextAlign.Center)
        HomeButton({ onOpen: () => { this.closeEditor(); this.onHome() } })
      }.width('100%').height(48).padding({ left: 8, right: 8 })
      Column({ space: 8 }) {
        Column() {
          Row({ space: 12 }) {
            Text('名称').fontSize(14).fontWeight(FontWeight.Medium)
            TextInput({ text: this.editName, placeholder: '请输入 PC 名称' })
              .layoutWeight(1).height(46).padding(0).backgroundColor(Color.Transparent)
              .textAlign(TextAlign.End).fontSize(14).fontColor(SECONDARY).maxLength(255)
              .onChange((value: string) => { this.editName = value; this.editResult = '' }).enabled(!this.editTesting)
          }.width('100%')
          Divider().color($r('app.color.border'))
          Row({ space: 12 }) {
            Text('IP地址').fontSize(14).fontWeight(FontWeight.Medium)
            TextInput({ text: this.editIP, placeholder: '请输入 IP 地址' })
              .layoutWeight(1).height(46).padding(0).backgroundColor(Color.Transparent)
              .textAlign(TextAlign.End).fontSize(14).fontColor(SECONDARY)
              .onChange((value: string) => { this.editIP = value; this.editResult = '' }).enabled(!this.editTesting)
          }.width('100%')
        }.width('100%').padding({ left: 14, right: 14 }).borderRadius(16)
          .backgroundColor($r('app.color.surface')).border({ width: 1, color: $r('app.color.border') })
        Blank().layoutWeight(1)
        if (this.editResult.length > 0) {
          this.resultCard(this.editResult, this.editedHost, this.editResultOK)
        }
        Button({ type: ButtonType.Normal }) {
          Row({ space: 6 }) {
            Image($r('app.media.test_connect_in_button')).width(20).height(20)
            Text('测试连接').fontSize(14).fontWeight(FontWeight.Medium).fontColor(PRIMARY)
          }
        }.width('100%').height(48).border({ width: 1, color: PRIMARY, radius: ACTION_BUTTON_RADIUS })
          .backgroundColor($r('app.color.surface')).enabled(!this.refreshing && !this.editTesting && validComputerIP(this.editIP.trim()))
          .onClick(() => this.testEditedAddress())
          .onAreaChange((_previous: Area, current: Area) => this.onDebugActionsTop(Number(current.globalPosition.y)))
        Button('确认', { type: ButtonType.Normal }).width('100%').height(48).borderRadius(ACTION_BUTTON_RADIUS).fontSize(14)
          .backgroundColor($r('app.color.primary_fill')).fontColor(Color.White)
          .enabled(!this.editTesting && this.editName.trim().length > 0 && validComputerIP(this.editIP.trim())).onClick(() => {
            const ip = this.editIP.trim()
            this.onSaveComputer({ key: this.editKey, name: this.editName.trim(),
              ip: ip.includes(':') && !ip.startsWith('[') ? '[' + ip + ']' : ip })
            this.closeEditor()
          })
      }.width('100%').layoutWeight(1).padding({ left: 20, right: 20, bottom: 16 })
    }.width('100%').height('100%').backgroundColor(PAGE_BACKGROUND).transition(pageSlide())
  }

  @Builder
  private computerActions(computer: PCGroup): void {
    Row() {
      Button({ type: ButtonType.Normal }) {
        Image($r('app.media.pc_action_more')).width(18).height(18)
      }.width(54).height('100%').padding(0).borderRadius(0).backgroundColor('#ff8a30').accessibilityText('编辑电脑').onClick(() => {
        this.stopRefresh()
        this.resultOpen = false
        this.editResult = ''
        this.editName = this.computerName(computer)
        this.editIP = this.computerHost(computer)
        this.editKey = computer.key
        this.setEditorOpen(true)
      })
      Button({ type: ButtonType.Normal }) {
        Image($r('app.media.pc_action_delete')).width(18).height(18)
      }.width(54).height('100%').padding(0).borderRadius(0).backgroundColor('#ff3141').accessibilityText('删除电脑')
        .onClick(() => AlertDialog.show({
          title: '删除本地配对',
          message: '删除后需重新配对此电脑下的会话。',
          primaryButton: { value: '取消', action: () => {} },
          secondaryButton: { value: '删除', action: () => this.onRemoveComputer(computer.key) }
        }))
    }
  }

  private copyIP(host: string): void {
    const data = pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, host)
    pasteboard.getSystemPasteboard().setData(data).then(() => {
      this.copyFeedback = '已复制'
    }).catch(() => { this.copyFeedback = '复制失败' })
  }

  @Builder
  private resultCard(message: string, host: string, ok: boolean): void {
    Column({ space: 4 }) {
      Text(message).width('100%').fontSize(12).fontWeight(FontWeight.Bold)
        .fontColor(ok ? '#247443' : '#a42c26')
      Row({ space: 6 }) {
        Text('IP：' + host).fontSize(14).fontColor(ok ? '#247443' : '#a42c26')
          .constraintSize({ maxWidth: '75%' })
        Button('⧉', { type: ButtonType.Normal }).padding(0).width(32).height(32)
          .fontColor(PRIMARY).backgroundColor(Color.Transparent).accessibilityText('复制 IP')
          .onClick(() => this.copyIP(host))
        if (this.copyFeedback.length > 0) Text(this.copyFeedback).fontSize(12).fontColor(SECONDARY)
      }.width('100%')
      Text('端口：7330').width('100%').fontSize(14).fontColor(ok ? '#247443' : '#a42c26')
    }.width('100%').padding({ left: 16, right: 16, top: 12, bottom: 12 }).borderRadius(15)
      .backgroundColor(ok ? '#effaf2' : '#fff4f3').border({ width: 1, color: ok ? '#c8e9d1' : '#ffd2cf' })
  }

  @Builder
  private connectionDetails(): void {
    Column({ space: 16 }) {
      Row({ space: 12 }) {
        Text(this.testedName + ' · 连接测试').fontSize(18).fontWeight(FontWeight.Bold)
          .layoutWeight(1).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis })
        Button('关闭', { type: ButtonType.Normal }).padding(0).height(44).fontSize(14)
          .fontColor(PRIMARY).backgroundColor(Color.Transparent).onClick(() => { this.resultOpen = false })
      }.width('100%')
      Scroll() {
        this.resultCard(this.testResult, this.testedHost, this.testResultOK)
      }.width('100%').constraintSize({ maxHeight: 200 })
    }.width('100%').padding({ left: 24, right: 24, top: 16, bottom: 24 })
  }

  @Builder
  private pairingInput(): void {
    Column({ space: 4 }) {
      Column({ space: 6 }) {
        Row() {
          TextInput({ text: this.ip, placeholder: '请输入IP' }).layoutWeight(1).height(44).backgroundColor($r('app.color.surface'))
            .enabled(this.phase === 'idle' && !this.disabled).onChange((value: string) => this.updateAddress(value))
            .onFocus(() => { clearTimeout(this.blurTimer); this.historyOpen = true })
            .onBlur(() => {
              this.rememberAddress()
              this.blurTimer = setTimeout(() => { this.historyOpen = false }, 250)
            })
            .onPaste((value: string, event: PasteEvent) => { event.preventDefault?.(); this.acceptInput(value) })
          PasteButton({ icon: PasteIconStyle.LINES, buttonType: ButtonType.Normal })
            .width(34).height(34).backgroundColor($r('app.color.surface_blue')).fontColor(PRIMARY).enabled(this.phase === 'idle' && !this.disabled)
            .onClick((_event: ClickEvent, result: PasteButtonOnClickResult) => { if (result === PasteButtonOnClickResult.SUCCESS) void this.paste() })
        }.width('100%').padding({ left: 8, right: 8 }).backgroundColor($r('app.color.surface')).border({ width: 1, color: $r('app.color.border') }).borderRadius(15)
        if (this.historyOpen && this.matchingAddresses().length > 0) {
          Column() {
            Scroll() {
              Column() {
                ForEach(this.matchingAddresses(), (address: string) => {
                  Button({ type: ButtonType.Normal }) {
                    Text(address).width('100%').fontSize(14).fontColor(PRIMARY)
                  }.width('100%').height(44).padding({ left: 14, right: 14 })
                    .backgroundColor(Color.Transparent).onClick(() => {
                      this.updateAddress(address)
                      this.dismissAddressInput()
                    })
                }, (address: string) => address)
              }.width('100%')
            }.width('100%').constraintSize({ maxHeight: 132 })
            Divider().color($r('app.color.border'))
            Button('清空历史', { type: ButtonType.Normal }).height(44).fontSize(13)
              .fontColor(SECONDARY).backgroundColor(Color.Transparent).onClick(() => {
                clearTimeout(this.historyTimer)
                this.historyDirty = false
                this.onHistoryChange([])
                this.historyOpen = false
              })
          }.width('100%').alignItems(HorizontalAlign.End).borderRadius(12)
            .border({ width: 1, color: $r('app.color.border') }).backgroundColor($r('app.color.surface')).clip(true)
          }
      }.width('100%').margin({ top: 5, bottom: 6 }).onAreaChange((_previous: Area, current: Area) => {
        this.addressTop = Number(current.globalPosition.y)
        this.addressBottom = this.addressTop + Number(current.height)
      })
      Text('请与 PC 配对窗口核对；手机所在网络需要能访问此地址。不同路由器共用宽带不代表设备能够互通。').fontSize(11).fontColor(SECONDARY).width('100%')
    }.width('100%')
  }

  @Builder
  private pairingActions(): void {
    Column({ space: 8 }) {
      Button('连接', { type: ButtonType.Normal }).fontColor(Color.White).width('100%').height(46).borderRadius(ACTION_BUTTON_RADIUS).backgroundColor($r('app.color.primary_fill')).enabled(!this.disabled && this.phase === 'idle').onClick(() => this.start())
        .onAreaChange((_previous: Area, current: Area) => this.onDebugActionsTop(Number(current.globalPosition.y)))
      Button({ type: ButtonType.Normal }) {
        Row({ space: 7 }) {
          Image($r('app.media.pair_scan')).width(16).height(16)
          Text('扫描二维码').fontSize(16).fontColor(PRIMARY)
        }
      }.width('100%').height(46).backgroundColor($r('app.color.surface'))
        .border({ width: 1, color: PRIMARY, radius: ACTION_BUTTON_RADIUS })
        .enabled(!this.disabled && this.phase === 'idle')
        .onClick(() => { this.dismissAddressInput(); this.clearTestResult(); this.error = ''; this.onScan() })
    }.width('100%')
  }

  @Builder
  private computerHeading(): void {
    Row() {
      Button({ type: ButtonType.Normal }) {
        Text('全部电脑').width('100%').textAlign(TextAlign.Start)
          .fontSize(17).fontWeight(FontWeight.Bold).fontColor('#202938')
      }.layoutWeight(1).height(44).padding({ left: 10, right: 10 }).borderRadius(12).stateEffect(false)
        .stateStyles({
          normal: { .backgroundColor(this.computersCollapsed ? '#eaf0fa' : Color.Transparent) },
          pressed: { .backgroundColor('#e3efff') }
        })
        .accessibilityDescription(this.computersCollapsed ? '已收起' : '已展开')
        .onClick(() => { this.computersCollapsed = !this.computersCollapsed })
      if (!this.computersCollapsed) {
        Stack() {
          if (this.refreshing) {
            LoadingProgress().width(17).height(17).color(PRIMARY)
          } else {
            Button('刷新', { type: ButtonType.Normal }).width(48).height(44).padding(0)
              .fontSize(14).fontWeight(FontWeight.Bold).fontColor(PRIMARY).borderRadius(12).stateEffect(false)
              .stateStyles({ normal: { .backgroundColor(Color.Transparent) }, pressed: { .backgroundColor('#e3efff') } })
              .accessibilityText('刷新电脑状态').enabled(this.testingPC.length === 0)
              .onClick(() => this.refreshComputers())
          }
        }.width(48).height(44)
      }
    }.width('100%')
  }

  @Builder
  private pairingFeedback(message: string, result: boolean, warning: boolean): void {
    Row({ space: 8 }) {
      Button('×', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(20)
        .fontColor(warning ? '#8a5300' : result && this.testResultOK ? '#247443' : '#a42c26')
        .backgroundColor(Color.Transparent).accessibilityText('关闭提示').onClick(() => {
          if (result) this.clearTestResult()
          else if (warning) this.projectNoticeDismissed = true
          else this.error = ''
        })
      Text(message).layoutWeight(1).fontSize(12).maxLines(3).textOverflow({ overflow: TextOverflow.Ellipsis })
        .fontColor(warning ? '#8a5300' : result && this.testResultOK ? '#247443' : '#a42c26')
      if (result) {
        Button('查看详情', { type: ButtonType.Normal }).padding({ left: 6, right: 6 }).height(44).fontSize(14)
          .fontColor(PRIMARY).backgroundColor(Color.Transparent).onClick(() => { this.resultOpen = true })
      }
    }.width('100%').padding({ right: 10 }).borderRadius(14)
      .backgroundColor(warning ? '#fff3d6' : result && this.testResultOK ? '#effaf2' : '#fff4f3')
      .border({ width: 1, color: warning ? '#f4d991' : result && this.testResultOK ? '#c8e9d1' : '#ffd2cf' })
      .shadow({ radius: 8, color: '#20345326', offsetY: 3 })
  }

  build() {
    Stack({ alignContent: Alignment.Bottom }) {
      if (this.editOpen) {
        this.editComputer()
      } else {
        Column() {
          if (this.computers.length > 0 && !this.computersCollapsed) {
            List() {
              ListItem() { this.pairingInput() }.padding({ bottom: 10 })
              ListItem() { this.pairingActions() }.padding({ bottom: 24 })
              ListItem() { this.computerHeading() }
              ForEach(this.computers, (computer: PCGroup) => {
                ListItem() {
                  Row({ space: 8 }) {
                    Row({ space: 10 }) {
                      Image($r('app.media.pc_status')).width(26).height(24)
                        .objectFit(ImageFit.Contain)
                        .fillColor(this.computerStatusColor(computer))
                      Column({ space: 3 }) {
                        Row({ space: 6 }) {
                          Text(this.computerName(computer)).fontSize(14).fontWeight(FontWeight.Bold)
                            .maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).flexShrink(1)
                          Text(this.computerHost(computer)).fontSize(12).fontColor(SECONDARY)
                            .maxLines(1).flexShrink(0)
                        }.width('100%').height(20)
                        if (computer.projects.length > 0) {
                          DirectoryTags({ projects: computer.projects })
                        }
                      }.layoutWeight(1).alignItems(HorizontalAlign.Start)
                    }.layoutWeight(1).constraintSize({ minHeight: 44 }).onTouch((event: TouchEvent) => {
                      if (event.type === TouchType.Down) this.pressedPC = computer.key
                      if (event.type === TouchType.Up || event.type === TouchType.Cancel) this.pressedPC = ''
                    }).onClick(() => this.onSelectPC(computer.key))
                    Button({ type: ButtonType.Normal }) {
                      if (this.testingPC === computer.key) {
                        LoadingProgress().width(20).height(20).color(PRIMARY)
                      } else {
                        Image($r('app.media.test_connect_in_button')).width(16).height(16)
                      }
                    }.width(PC_TEST_BUTTON_SIZE).height(PC_TEST_BUTTON_SIZE).padding(0)
                      .border({ width: 1, color: '#dfe6f1', radius: PC_TEST_BUTTON_RADIUS })
                      .clipShape(new RectShape({
                        width: PC_TEST_BUTTON_SIZE, height: PC_TEST_BUTTON_SIZE, radius: PC_TEST_BUTTON_RADIUS
                      }))
                      .stateEffect(false)
                      .stateStyles({ normal: { .backgroundColor('#eaf3ff') }, pressed: { .backgroundColor('#d8e9ff') } })
                      .accessibilityText('测试连接')
                      .onClick(() => {
                        if (!this.refreshing && this.testingPC.length === 0) this.testComputer(computer)
                      })
                  }.width('100%').padding({ left: 10, right: 10, top: 6, bottom: 8 })
                    .backgroundColor(this.pressedPC === computer.key ? '#e3efff' : Color.Transparent)
                }.swipeAction({ end: this.computerActions(computer) }).border({ width: { top: 1 }, color: '#dfe6f1' }).clip(true)
              }, (computer: PCGroup) => computer.key)
              ListItem() { Divider().color('#dfe6f1') }
            }.width('100%').layoutWeight(1).scrollBar(BarState.Auto)
          } else {
            this.pairingInput()
            Blank()
            this.pairingActions()
            if (this.computers.length > 0) {
              Column() { this.computerHeading() }.width('100%').padding({ top: 24 })
            }
          }
        }.width('100%').height('100%')
          .onClick((event: ClickEvent) => {
            if (event.windowY < this.addressTop || event.windowY > this.addressBottom) this.dismissAddressInput()
          })
          .onAppear(() => this.refreshOnEntry())
          .bindSheet($$this.resultOpen, this.connectionDetails(), { height: SheetSize.FIT_CONTENT, showClose: false, dragBar: false })
        if (this.error.length > 0 && this.phase === 'idle') {
          this.pairingFeedback(this.error, false, false)
        } else if (this.testResult.length > 0) {
          this.pairingFeedback(this.testedName + '：' + (this.testResultOK ? '连接成功' : '连接失败'), true, false)
        } else if (this.projectCwd.length > 0 && !this.projectNoticeDismissed) {
          this.pairingFeedback('请扫描目标电脑和目录下新会话的配对码：\n' +
            [this.projectPC, this.projectCwd].filter((value: string) => value.length > 0).join('\n'), false, true)
        }
      }
    }.width('100%').layoutWeight(1)
  }
}
