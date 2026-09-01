import type { ErrorInfo } from '../protocol/types'
import {
  ConnectionStage,
  type HarmonySessionState,
} from '../session/model'

function summarizeSessionId(sessionId: string): string {
  if (sessionId.length <= 18) {
    return sessionId
  }
  return `${sessionId.slice(0, 8)}...${sessionId.slice(sessionId.length - 6)}`
}

function connectionLabel(connection: ConnectionStage): string {
  if (connection === ConnectionStage.AUTHENTICATING) {
    return '鉴权中'
  }
  if (connection === ConnectionStage.SUBSCRIBING) {
    return '订阅中'
  }
  if (connection === ConnectionStage.SUBSCRIBED) {
    return '等待快照'
  }
  if (connection === ConnectionStage.READY) {
    return '已连接'
  }
  if (connection === ConnectionStage.PAIRING_INVALID) {
    return '配对失效'
  }
  if (connection === ConnectionStage.PROTOCOL_ERROR) {
    return '协议异常'
  }
  if (connection === ConnectionStage.CONNECTING) {
    return '连接中'
  }
  return '已断开'
}

@Component
export struct StatusBar {
  @Prop state: HarmonySessionState = {
    pairing: null,
    connection: ConnectionStage.DISCONNECTED,
    sessionId: '',
    sessionStatus: '',
    sessionError: null,
    seqBase: 0,
    queueDepthHint: 0,
    queuedRequests: [],
    pendingAction: null,
    pendingCommand: null,
    messages: [],
    lastError: null,
    lastNotice: null,
    canSend: false,
    canResolveAction: false,
  }

  private sessionStatusText(): string {
    return this.state.sessionStatus.length > 0 ? this.state.sessionStatus : 'idle'
  }

  private turnStatusText(): string {
    if (this.state.pendingAction !== null) {
      return 'waiting'
    }
    for (let index = this.state.messages.length - 1; index >= 0; index -= 1) {
      const message = this.state.messages[index]
      if (message.kind === 'claude') {
        if (message.status === 'streaming') {
          return 'running'
        }
        return message.status
      }
    }
    return 'idle'
  }

  private errorInfo(): ErrorInfo | null {
    if (this.state.lastError !== null) {
      return this.state.lastError
    }
    return this.state.sessionError
  }

  build() {
    Column({ space: 8 }) {
      Text(`连接：${connectionLabel(this.state.connection)}`)
        .id('connection-state')
        .fontWeight(FontWeight.Medium)
      Text(`Session：${this.sessionStatusText()}`)
        .id('session-state')
      Text(`Turn：${this.turnStatusText()}`)
        .id('turn-state')
      Text(`排队：${this.state.queueDepthHint}`)
      Text(`Session ID：${summarizeSessionId(this.state.sessionId.length > 0 ? this.state.sessionId : 'no-session')}`)
        .fontSize(13)
        .opacity(0.7)
      if (this.errorInfo() !== null) {
        Text(`错误：${this.errorInfo()!.message}`)
          .fontSize(13)
          .fontColor('#c0392b')
      }
      if (this.state.lastNotice !== null) {
        Text(`提示：${this.state.lastNotice.message}`)
          .fontSize(13)
          .fontColor('#8a5a00')
      }
    }
    .padding(16)
    .width('100%')
    .alignItems(HorizontalAlign.Start)
    .backgroundColor('#ffffff')
    .borderRadius(12)
  }
}
