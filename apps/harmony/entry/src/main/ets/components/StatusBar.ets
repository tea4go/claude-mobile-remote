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
    permissionMode: 'default',
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

  private statusText(): string {
    if (this.state.connection !== ConnectionStage.READY) return '待同步'
    if (this.state.sessionStatus === 'failed') return '会话失败'
    if (this.state.sessionStatus === 'stopped') return '会话已停止'
    const status = this.turnStatusText()
    if (status === 'waiting') return '等待处理'
    if (status === 'running') return '进行中'
    if (status === 'done') return '已完成'
    if (status === 'failed') return '本轮失败'
    if (status === 'cancelled') return '已取消'
    return '空闲'
  }
  build() {
    Column({ space: 1 }) {
      Text(connectionLabel(this.state.connection)).id('connection-state').fontSize(11).fontColor('#6e6e73').maxLines(1)
      Text(this.statusText()).id('turn-state').fontSize(11).fontColor('#6e6e73').maxLines(1).fontWeight(FontWeight.Medium)
      if (this.state.queueDepthHint > 0) { Text(`排队 ${this.state.queueDepthHint}`).fontSize(12) }
    }.width(78).alignItems(HorizontalAlign.End)
  }
}
