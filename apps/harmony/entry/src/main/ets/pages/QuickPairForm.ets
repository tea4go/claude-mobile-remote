import { nativeLifecycle } from '../native/lifecycle'
import { webSocket } from '@kit.NetworkKit'
import { BusinessError } from '@kit.BasicServicesKit'
import { PRIMARY } from '../ui/styles'

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
@Component
export struct QuickPairForm {
  onPair: (text: string) => void = () => {}
  @Prop @Watch('disabledChanged') disabled: boolean = false
  @State ip: string = ''
  @State code: string = ''
  @State phase: string = 'idle'
  @State error: string = ''
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
    this.phase = 'idle'
    this.requestId = ''
  }
  private fail(message: string): void { this.stop(); this.error = message }
  private send(kind: string): void {
    const frame = new QuickFrame()
    frame.type = kind
    frame.requestId = this.requestId
    frame.payload.code = kind === 'verify' ? this.code : ''
    this.socket?.send(JSON.stringify(frame)).catch(() => this.fail('连接断开，请重试'))
  }
  private start(): void {
    this.stop(); this.error = ''; this.code = ''
    const match = /^(\[[0-9a-fA-F:]+\]|[a-zA-Z0-9.-]+)(?::([0-9]{1,5}))?$/.exec(this.ip.trim())
    if (match === null || (match[2] && (Number(match[2]) < 1 || Number(match[2]) > 65535))) { this.error = '请输入有效 PC IP'; return }
    this.host = match[1]
    const socket = webSocket.createWebSocket()
    this.socket = socket; this.phase = 'connecting'
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
          this.requestId = frame.requestId; this.phase = 'code'; clearTimeout(this.timer)
          this.timer = setTimeout(() => this.fail('验证码已过期，请重新连接'), ttl)
        } else if (frame.type === 'paired' && this.requestId.length > 0 && frame.requestId === this.requestId) {
          // Replace only the address host; the existing pairing parser validates all remaining fields.
          frame.payload.wsUrl = frame.payload.wsUrl.replace(/^(wss?:\/\/)(\[[^\]]+\]|[^/:]+)/, `$1${this.host}`)
          const text = JSON.stringify(frame.payload)
          this.stop(); this.onPair(text)
        } else if (frame.type === 'error') {
          if (frame.payload.code === 'invalid_code' || frame.payload.code === 'target_required') {
            this.error = frame.payload.code === 'invalid_code' ? '验证码不正确' : '请先在 PC 选择会话'; this.phase = 'code'
          } else this.fail(frame.payload.code === 'busy' ? 'PC 正在处理其他配对，请稍后重试' : '配对已失效，请重试')
        } else this.fail('配对响应无效')
      } catch (_) { this.fail('配对响应无效') }
    })
    socket.connect(`ws://${this.host}:${match[2] || '7330'}/quick-pair/v1`).catch(() => { if (this.socket === socket) this.fail('无法连接，请先在 PC 启动远程服务，并检查网络') })
  }
  build() {
    Column({ space: 16 }) {
      TextInput({ text: this.ip, placeholder: '请输入 PC IP' }).enabled(this.phase === 'idle' && !this.disabled).onChange((value: string) => { this.ip = value }).height(52)
      if (this.phase === 'code' || this.phase === 'verifying') {
        Text('请查看 WhaleTerm 上的六位验证码').fontSize(14)
        TextInput({ text: this.code, placeholder: '六位验证码' }).type(InputType.Number).maxLength(6).onChange((value: string) => { this.code = value.replace(/\D/g, '') })
      }
      if (this.error.length > 0) { Text(this.error).fontColor('#c0392b').fontSize(13) }
      Button(this.phase === 'idle' ? '连接' : this.phase === 'code' ? '验证并连接' : '正在连接…').width('100%').height(48).backgroundColor(PRIMARY).enabled(!this.disabled && (this.phase === 'idle' || (this.phase === 'code' && this.code.length === 6))).onClick(() => {
        if (this.phase === 'idle') this.start()
        else { this.phase = 'verifying'; this.error = ''; this.send('verify') }
      })
      if (this.phase !== 'idle') { Button('取消连接').onClick(() => this.stop()) }
    }.width('100%')
  }
}
