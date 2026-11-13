import { nativeLifecycle } from '../native/lifecycle'
import { webSocket } from '@kit.NetworkKit'
import { BusinessError } from '@kit.BasicServicesKit'
import { PRIMARY, SECONDARY } from '../ui/styles'
import { pasteboard } from '@kit.BasicServicesKit'
import { promptAction } from '@kit.ArkUI'
import { decodePairingPayload } from '../protocol/pairing'

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
struct VerifyDialog {
  controller?: CustomDialogController
  @Link code: string
  onCancel: () => void = () => {}
  onVerify: () => void = () => {}
  build() {
    Column({ space: 16 }) {
      Text('输入验证码').fontSize(18).fontWeight(FontWeight.Bold)
      Text('请输入 PC 上的六位验证码').fontSize(14)
      TextInput({ text: this.code, placeholder: '请输入六位验证码' }).type(InputType.Number).maxLength(6)
        .onChange((v: string) => { this.code = v.replace(/\D/g, '') }).height(52).width('100%')
      Row({ space: 12 }) {
        Button('取消').layoutWeight(1).onClick(() => this.onCancel())
        Button('验证并连接').layoutWeight(1).enabled(this.code.length === 6).onClick(() => this.onVerify())
      }.width('100%')
    }.padding(24).width('90%').backgroundColor($r('app.color.surface')).borderRadius(18)
  }
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
  onPair: (text: string) => void = () => {}
  onScan: () => void = () => {}
  @Prop @Watch('disabledChanged') disabled: boolean = false
  @State ip: string = ''
  @State code: string = ''
  @State phase: string = 'idle'
  @State error: string = ''
  private legacy: string = ''
  private verifyDialog: CustomDialogController = new CustomDialogController({
    builder: VerifyDialog({ code: $code, onCancel: () => this.stop(), onVerify: () => { this.verifyDialog.close(); this.phase = 'verifying'; this.loadingDialog.open(); this.send('verify') } }),
    autoCancel: false, customStyle: true
  })
  private loadingDialog: CustomDialogController = new CustomDialogController({ builder: LoadingDialog({ onCancel: () => this.stop() }), autoCancel: false, customStyle: true })
  private socket: webSocket.WebSocket | null = null
  private timer: number = -1
  private requestId: string = ''
  private host: string = ''
  private unsubscribe: (() => void) | null = null
  aboutToAppear(): void { this.unsubscribe = nativeLifecycle.subscribe(() => { if (!nativeLifecycle.foreground) this.stop() }) }
  aboutToDisappear(): void { this.unsubscribe?.(); this.unsubscribe = null; this.stop() }
  private disabledChanged(): void { if (this.disabled) this.stop() }
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
  private fail(message: string): void { this.stop(); promptAction.showToast({ message, duration: 3000 }) }
  private send(kind: string): void {
    const frame = new QuickFrame()
    frame.type = kind
    frame.requestId = this.requestId
    frame.payload.code = kind === 'verify' ? this.code : ''
    this.socket?.send(JSON.stringify(frame)).catch(() => this.fail('连接断开，请重试'))
  }
  private start(): void {
    this.stop(); this.error = ''; this.code = ''
    if (this.legacy.length > 0) { this.onPair(this.legacy); return }
    const match = /^(\[[0-9a-fA-F:]+\]|(?:\d{1,3}\.){3}\d{1,3})(?::([0-9]{1,5}))?$/.exec(this.ip.trim())
    if (match === null || (!match[1].startsWith('[') && !match[1].split('.').every((v: string) => Number(v) <= 255)) || (match[2] && (Number(match[2]) < 1 || Number(match[2]) > 65535))) { promptAction.showToast({ message: '请输入有效的 IP 地址或配对信息' }); return }
    this.host = match[1]
    const socket = webSocket.createWebSocket()
    this.socket = socket; this.phase = 'connecting'; this.loadingDialog.open()
    this.timer = setTimeout(() => this.fail('无法连接，请先在 PC 启动远程服务，并检查网络'), 8000)
    socket.on('open', () => { if (this.socket === socket) this.send('begin') })
    socket.on('error', () => { if (this.socket === socket) this.fail('无法连接，请先在 PC 启动远程服务，并检查网络') })
    socket.on('close', () => { if (this.socket === socket) this.fail('配对已结束，请重新连接') })
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
            promptAction.showToast({ message: frame.payload.code === 'invalid_code' ? '验证码不正确' : '请先在 PC 选择会话' }); this.phase = 'code'; this.loadingDialog.close(); this.verifyDialog.open()
          } else this.fail(frame.payload.code === 'busy' ? 'PC 正在处理其他配对，请稍后重试' : '配对已失效，请重试')
        } else this.fail('配对响应无效')
      } catch (_) { this.fail('配对响应无效') }
    })
    socket.connect(`ws://${this.host}:${match[2] || '7330'}/quick-pair/v1`).catch(() => { if (this.socket === socket) this.fail('无法连接，请先在 PC 启动远程服务，并检查网络') })
  }
  private async paste(): Promise<void> {
    try {
      const data = await pasteboard.getSystemPasteboard().getData()
      this.acceptInput(data.getPrimaryText() || '')
    } catch (_) { promptAction.showToast({ message: '无法读取剪贴板，请使用系统粘贴' }) }
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
        this.legacy = ''; this.ip = value; return
      }
      this.legacy = ''; promptAction.showToast({ message: parsed.error?.code === 'expired' ? '配对凭证已过期，请重新配对' : '请输入有效的 IP 地址或配对信息' })
  }
  build() {
    Column({ space: 16 }) {
      Row() {
        TextInput({ text: this.ip, placeholder: '请输入IP' }).layoutWeight(1).height(54).backgroundColor($r('app.color.surface'))
          .enabled(this.phase === 'idle' && !this.disabled).onChange((value: string) => { this.ip = value; this.legacy = '' })
          .onPaste((value: string, event: PasteEvent) => { event.preventDefault?.(); this.acceptInput(value) })
        PasteButton({ icon: PasteIconStyle.LINES, buttonType: ButtonType.Normal })
          .width(44).height(44).backgroundColor($r('app.color.surface_blue')).fontColor(PRIMARY).enabled(this.phase === 'idle' && !this.disabled)
          .onClick((_event: ClickEvent, result: PasteButtonOnClickResult) => { if (result === PasteButtonOnClickResult.SUCCESS) void this.paste() })
      }.width('100%').padding({ left: 8, right: 8 }).backgroundColor($r('app.color.surface')).border({ width: 1, color: $r('app.color.border') }).borderRadius(15)
      Text('请与 PC 配对窗口核对；手机所在网络需要能访问此地址。不同路由器共用宽带不代表设备能够互通。').fontSize(13).fontColor(SECONDARY).width('100%')
      Button('连接').fontColor(Color.White).width('100%').height(48).margin({ top: 52 }).backgroundColor($r('app.color.primary_fill')).enabled(!this.disabled && this.phase === 'idle').onClick(() => this.start())
      Button('扫描二维码').width('100%').height(48).fontColor(PRIMARY).backgroundColor($r('app.color.surface')).border({ width: 1, color: PRIMARY }).enabled(!this.disabled && this.phase === 'idle').onClick(() => this.onScan())
    }.width('100%')
  }
}
