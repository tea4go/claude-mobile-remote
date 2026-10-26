import { recordAction, actionText, type ConnectionDetails } from '../debug/log'
import { recordNetworkDiagnostic, type NetworkCode } from './diagnostics'
// 单 WebSocket 生命周期与协议编排（设计 §5）。
// 不依赖 React 或 DOM；WebSocket 工厂可注入以便测试。

import { decodeServerMessage } from './decode'
import type { PairingPayload } from './pairing'
import type {
  Attachment,
  ActionDecision,
  CommandPayload,
  QuestionAnswer,
  PermissionMode,
  UnsupportedCode,
} from './types'
import { safeNetworkTarget } from './safeTarget'
import { PREFERRED_PROTOCOL_VERSION } from './types'

// 连接内部阶段；onEvent 通过 connectionStateChanged 导出给 reducer。
type Stage = 'connecting' | 'authenticating' | 'subscribing' | 'ready'

// 两种决议形态（B2 §5）：工具审批只带 decision，问答只带 answers。
export type ActionResolution =
  | { actionId: string; decision: ActionDecision }
  | { actionId: string; answers: QuestionAnswer[] }

export type ClientEvent =
  | { kind: 'connectionStateChanged'; connection: 'connecting' | 'authenticating' | 'subscribing' | 'connected' }
  | { kind: 'snapshot'; snapshot: import('./types').SessionSnapshot }
  | { kind: 'event'; event: import('./types').DomainEvent }
  // 服务端不支持通知（B2 §4.4）：属于提示，不终止连接。通知经服务端
  // publish 递增并占用一个 seq，必须透传给 reducer 推进 seqBase。
  | { kind: 'unsupported'; code: UnsupportedCode; seq: number }
  | {
      kind: 'commandResult'
      commandId: string
      requestId: string
      accepted: boolean
      code: string
      message: string
    }
  | {
      kind: 'managementResult'
      operation: 'create_session' | 'stop_session'
      commandId: string
      accepted: boolean
      code: string
      message: string
      pairing?: PairingPayload
    }

export type CloseKind =
  // 非正常 WebSocket close（含服务端溢出关闭），统一按已断开 + 结果未知处理。
  | 'disconnected'
  // 服务端 error/unauthorized：配对已失效，禁用重连。
  | 'pairingInvalid'
  // 解码或顺序错误：状态不可信。
  | 'protocolError'

export interface CloseInfo {
  kind: CloseKind
  pendingCommandId: string | null
}

export interface SendResult {
  ok: boolean
  reason?: 'notReady' | 'busy'
}

/* eslint-disable @typescript-eslint/no-explicit-any */
export type SocketLike = {
  readonly readyState?: number
  send(data: string): void
  close(): void
  // 事件参数类型宽松：兼容 DOM WebSocket 与测试假实现。
  onopen: ((event?: any) => void) | null
  onmessage: ((event: { data: any }) => void) | null
  onclose: ((event?: any) => void) | null
  onerror: ((event?: any) => void) | null
}

export type SocketFactory = (url: string) => SocketLike

function envelope(version: number, type: string, sessionId: string, commandId: string, requestId: string, seq: number, payload: unknown): string {
  return JSON.stringify({
    version,
    type,
    sessionId,
    commandId,
    requestId,
    seq,
    payload,
  })
}

let nextDiagnosticConnection = 0

export class ProtocolClient {
  debugContext(): { target: string; connectionId: number } { return { target: safeNetworkTarget(this.payload.wsUrl), connectionId: this.diagnosticConnection } }
  private readonly payload: PairingPayload
  private readonly socketFactory: SocketFactory
  private socket: SocketLike | null = null
  private stage: Stage = 'connecting'
  private pendingCommandId: string | null = null
  private pendingKind: 'command' | 'create_session' | 'stop_session' | null = null
  private closedByUser = false
  private generation = 0
  private wireVersion = PREFERRED_PROTOCOL_VERSION
  private stageTimer: ReturnType<typeof setTimeout> | null = null
  private attemptTimer: ReturnType<typeof setTimeout> | null = null
  private commandTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setTimeout> | null = null
  private pongDeadline = 0
  private nextProbeAt = 0

  // Called by timers and lifecycle hooks; readyState alone cannot prove reachability.
  checkConnection(probe = false): void {
    if (!this.socket) return
    if (this.socket.readyState !== undefined && this.socket.readyState !== 1 && this.stage === 'ready') {
      this.diagnose('socket_closed')
      this.terminate('disconnected')
      return
    }
    if (this.stage !== 'ready' || this.wireVersion < 6) return
    if (this.pongDeadline !== 0) {
      if (Date.now() >= this.pongDeadline) {
        this.diagnose('heartbeat_timeout', { timeoutMs: 5000 })
        this.terminate('disconnected')
      }
      return
    }
    if (!probe && Date.now() < this.nextProbeAt) return
    this.pongDeadline = Date.now() + 5000
    this.diagnose('heartbeat_sent', { timeoutMs: 5000 })
    this.scheduleHeartbeat(5000)
    try { this.socket.send(envelope(this.wireVersion, 'ping', this.payload.sessionId, '', '', 0, {})) }
    catch { this.diagnose('socket_error'); this.terminate('disconnected') }
  }

  networkOffline(): void {
    if (!this.socket) return
    this.diagnose('network_offline')
    this.terminate('disconnected')
  }

  private scheduleHeartbeat(ms: number): void {
    if (this.heartbeatTimer !== null) clearTimeout(this.heartbeatTimer)
    const generation = this.generation
    this.heartbeatTimer = setTimeout(() => {
      this.heartbeatTimer = null
      if (generation === this.generation) this.checkConnection()
    }, ms)
  }

  private startedAt = 0
  private deadline = 0
  private readonly diagnosticConnection = ++nextDiagnosticConnection
  private diagnosticAttempt = 0
  private phaseStartedAt = 0
  private phaseTimeoutMs = 0

  private diagnosticDetails(): Omit<ConnectionDetails, 'code' | 'stage'> {
    return { connectionId: this.diagnosticConnection, attempt: this.diagnosticAttempt,
      protocolVersion: this.wireVersion, phaseElapsedMs: Math.max(0, Date.now() - this.phaseStartedAt),
      timeoutMs: this.phaseTimeoutMs, readyState: this.socket?.readyState }
  }

  private diagnose(code: NetworkCode, extra: Partial<ConnectionDetails> = {}): void {
    recordNetworkDiagnostic(this.payload.sessionId, this.payload.wsUrl, this.stage === 'ready' ? 'connected' : this.stage, code, this.startedAt, { ...this.diagnosticDetails(), ...extra })
  }

  // Whitelist numeric/boolean CloseEvent metadata; never read reason or raw error.
  private closeDetails(event?: { code?: unknown; wasClean?: unknown }): Partial<ConnectionDetails> {
    return {
      ...(typeof event?.code === 'number' && Number.isInteger(event.code) && event.code >= 1000 && event.code <= 4999 ? { closeCode: event.code } : {}),
      ...(typeof event?.wasClean === 'boolean' ? { wasClean: event.wasClean } : {}),
    }
  }

  private clearTimers(): void {
    if (this.heartbeatTimer !== null) clearTimeout(this.heartbeatTimer)
    this.heartbeatTimer = null
    this.pongDeadline = 0
    this.nextProbeAt = 0
    if (this.stageTimer !== null) clearTimeout(this.stageTimer)
    if (this.attemptTimer !== null) clearTimeout(this.attemptTimer)
    if (this.commandTimer !== null) clearTimeout(this.commandTimer)
    this.stageTimer = this.attemptTimer = this.commandTimer = null
  }

  private waitForStage(ms: number, code: NetworkCode): void {
    if (this.stageTimer !== null) clearTimeout(this.stageTimer)
    const generation = this.generation
    this.phaseStartedAt = Date.now()
    this.phaseTimeoutMs = ms
    this.stageTimer = setTimeout(() => {
      if (generation !== this.generation || this.socket === null) return
      this.diagnose(code)
      this.terminate('disconnected')
    }, ms)
  }

  private waitForCommand(ms: number): void {
    this.commandTimer = setTimeout(() => {
      this.diagnose('command_timeout', { timeoutMs: ms })
      this.terminate('disconnected')
    }, ms)
  }

  onEvent: ((event: ClientEvent) => void) | null = null
  onClose: ((info: CloseInfo) => void) | null = null

  constructor(payload: PairingPayload, socketFactory: SocketFactory) {
    this.payload = payload
    this.socketFactory = socketFactory
  }

  isReady(): boolean {
    return this.stage === 'ready' && this.socket !== null
  }

  connectionState(): 'connecting' | 'authenticating' | 'subscribing' | 'connected' | 'disconnected' {
    if (this.socket === null) {
      return this.generation === 0 ? 'connecting' : 'disconnected'
    }
    if (this.stage === 'ready') return 'connected'
    return this.stage
  }

  connect(preserveBudget = false): void {
    if (this.socket !== null) return
    this.closedByUser = false
    const generation = ++this.generation
    if (!preserveBudget) {
      this.startedAt = Date.now()
      this.deadline = this.startedAt + 15000
    }
    this.stage = 'connecting'
    this.phaseStartedAt = Date.now()
    this.phaseTimeoutMs = 8000
    this.diagnosticAttempt++
    this.diagnose('started')
    let socket: SocketLike
    try { socket = this.socketFactory(this.payload.wsUrl) } catch {
      this.diagnose('constructor_error')
      this.terminate('disconnected')
      return
    }
    this.socket = socket
    this.stage = 'connecting'
    this.waitForStage(8000, 'connect_timeout')
    this.attemptTimer = setTimeout(() => {
      this.diagnose('attempt_timeout', { timeoutMs: 15000 })
      this.terminate('disconnected')
    }, Math.max(0, this.deadline - Date.now()))
    this.emit({ kind: 'connectionStateChanged', connection: 'connecting' })

    socket.onopen = () => {
      if (generation !== this.generation) return
      this.stage = 'authenticating'
      this.waitForStage(4000, 'auth_timeout')
      this.diagnose('socket_open')
      this.emit({ kind: 'connectionStateChanged', connection: 'authenticating' })
      // hello 携带 sessionId：服务端按 envelope.sessionId 校验 pair token 的会话绑定。
      socket.send(
        envelope(this.wireVersion, 'hello', this.payload.sessionId, '', '', 0, {
          pairToken: this.payload.pairToken,
        }),
      )
    }
    socket.onmessage = (message) => {
      if (generation !== this.generation) return
      this.handleMessage(message.data)
    }
    socket.onerror = () => {
      if (generation !== this.generation) return
      this.diagnose('socket_error')
      this.terminate('disconnected')
    }
    socket.onclose = (event) => {
      if (generation !== this.generation) return
      this.socket = null
      this.clearTimers()
      this.generation++
      if (this.closedByUser) return
      this.diagnose('socket_closed', { ...this.closeDetails(event), readyState: socket.readyState })
      // 非正常 close 不区分原因（网络断开与服务端溢出关闭在 close 上不可区分）。
      // 断开状态由 onClose 通知，不进入 connectionStateChanged。
      this.onClose?.({ kind: 'disconnected', pendingCommandId: this.pendingCommandId })
      this.pendingCommandId = null
      this.pendingKind = null
    }
  }

  reconnect(): void {
    this.diagnose('manual_retry')
    this.close()
    this.connect()
  }

  close(): void {
    if (this.socket !== null) this.diagnose('cancelled')
    this.clearTimers()
    const socket = this.socket
    this.socket = null
    this.stage = 'connecting'
    this.pendingCommandId = null
    this.pendingKind = null
    if (socket === null) return
    this.closedByUser = true
    this.generation++
    socket.onopen = null
    socket.onmessage = null
    socket.onclose = null
    socket.onerror = null
    socket.close()
  }

  sendAttachments(text: string, attachments: Attachment[], commandId: string): SendResult {
    return this.sendCommand(commandId, { kind: 'send_attachments', text, attachments })
  }

  sendText(text: string, commandId: string): SendResult {
    return this.sendCommand(commandId, { kind: 'send_text', text })
  }

  // 审批或问答决议（B2 §5）：与文本命令共用同一个单在途门控。
  resolveAction(resolution: ActionResolution, commandId: string): SendResult {
    const payload: CommandPayload =
      'decision' in resolution
        ? { kind: 'resolve_action', actionId: resolution.actionId, decision: resolution.decision }
        : { kind: 'resolve_action', actionId: resolution.actionId, answers: resolution.answers }
    return this.sendCommand(commandId, payload)
  }

  cancelTurn(requestId: string, commandId: string): SendResult {
    return this.sendCommand(commandId, { kind: 'cancel_turn', requestId })
  }

  cancelQueued(requestId: string, commandId: string): SendResult {
    return this.sendCommand(commandId, { kind: 'cancel_queued', requestId })
  }

  setPermissionMode(permissionMode: PermissionMode, commandId: string): SendResult {
    return this.sendCommand(commandId, { kind: 'set_permission_mode', permissionMode })
  }

  createSession(commandId: string): SendResult {
    return this.sendManagement('create_session', commandId)
  }

  stopSession(commandId: string): SendResult {
    return this.sendManagement('stop_session', commandId)
  }

  private sendManagement(kind: 'create_session' | 'stop_session', commandId: string): SendResult {
    this.checkConnection()
    if (this.stage !== 'ready' || this.socket === null) { recordAction('not_ready'); return { ok: false, reason: 'notReady' } }
    if (this.pendingCommandId !== null) { recordAction('busy'); return { ok: false, reason: 'busy' } }
    this.pendingCommandId = commandId
    recordAction(kind)
    this.pendingKind = kind
    this.waitForCommand(15000)
    this.socket.send(envelope(this.wireVersion, kind, this.payload.sessionId, commandId, '', 0, {}))
    return { ok: true }
  }

  // 所有命令共用单在途门控：服务端按 commandId 单次应答，并行发送无法对应结果。
  private sendCommand(commandId: string, payload: CommandPayload): SendResult {
    this.checkConnection()
    if (this.stage !== 'ready' || this.socket === null) {
      recordAction('not_ready')
      return { ok: false, reason: 'notReady' }
    }
    if (this.pendingCommandId !== null) {
      recordAction('busy')
      return { ok: false, reason: 'busy' }
    }
    this.pendingCommandId = commandId
    recordAction(Object.hasOwn(actionText, payload.kind) ? payload.kind as keyof typeof actionText : 'command')
    this.pendingKind = 'command'
    this.waitForCommand(payload.kind === 'send_attachments' ? 30000 : 15000)
    this.socket.send(
      envelope(this.wireVersion, 'command', this.payload.sessionId, commandId, '', 0, payload),
    )
    return { ok: true }
  }

  private emit(event: ClientEvent): void {
    try {
      this.onEvent?.(event)
    } catch {
      // 观察者异常不得影响连接。
    }
  }

  private terminate(kind: CloseKind): void {
    this.clearTimers()
    if (kind === 'pairingInvalid') this.diagnose('pairing_invalid')
    if (kind === 'protocolError') this.diagnose('protocol_error')
    const socket = this.socket
    this.socket = null
    this.closedByUser = true
    this.generation++
    if (socket !== null) {
      socket.onopen = null
      socket.onmessage = null
      const details = this.diagnosticDetails()
      const stage = this.stage === 'ready' ? 'connected' : this.stage
      const startedAt = this.startedAt
      socket.onclose = (event) => {
        socket.onclose = null
        if (event === undefined) return
        recordNetworkDiagnostic(this.payload.sessionId, this.payload.wsUrl, stage, 'socket_closed', startedAt,
          { ...details, ...this.closeDetails(event), readyState: socket.readyState })
      }
      socket.onerror = null
      socket.close()
    }
    this.onClose?.({ kind, pendingCommandId: this.pendingCommandId })
    this.pendingCommandId = null
    this.pendingKind = null
  }

  private handleMessage(data: string): void {
    let message
    try {
      message = decodeServerMessage(data)
    } catch {
      this.terminate('protocolError')
      return
    }

    const legacyVersion = message.version === 2 || message.version === 3 || message.version === 4 || message.version === 5
    if (legacyVersion && message.version < this.wireVersion && this.stage === 'authenticating' && message.type === 'hello_ack') {
      this.wireVersion = message.version
    }
    const downgradeError = message.type === 'error' && this.stage === 'authenticating' && legacyVersion &&
      (message.payload.code === 'unauthorized' || (message.payload.code === 'unsupported_version' &&
        (message.payload.expectedVersion === 2 || message.payload.expectedVersion === 3 || message.payload.expectedVersion === 4 || message.payload.expectedVersion === 5) && message.payload.expectedVersion < this.wireVersion))
    if (message.version !== this.wireVersion && !downgradeError) {
      this.terminate('protocolError')
      return
    }

    if (
      (message.type === 'pong' || message.type === 'snapshot' ||
        message.type === 'event' ||
        message.type === 'unsupported' ||
        message.type === 'command_result' ||
        message.type === 'create_session_result' ||
        message.type === 'stop_session_result') &&
      message.sessionId !== this.payload.sessionId
    ) {
      this.terminate('protocolError')
      return
    }

    switch (message.type) {
      case 'pong':
        if (this.stage !== 'ready' || this.pongDeadline === 0) { this.terminate('protocolError'); return }
        if (Date.now() >= this.pongDeadline) { this.checkConnection(); return }
        this.pongDeadline = 0
        this.diagnose('heartbeat_received')
        this.nextProbeAt = Date.now() + 10000
        this.scheduleHeartbeat(10000)
        return
      case 'hello_ack':
        if (this.stage !== 'authenticating') {
          this.terminate('protocolError')
          return
        }
        this.stage = 'subscribing'
        this.waitForStage(4000, 'snapshot_timeout')
        this.diagnose('authenticated')
        this.emit({ kind: 'connectionStateChanged', connection: 'subscribing' })
        this.socket?.send(
          envelope(this.wireVersion, 'subscribe', this.payload.sessionId, '', '', 0, {}),
        )
        return
      case 'subscribed':
        if (this.stage !== 'subscribing') {
          this.terminate('protocolError')
          return
        }
        // 已订阅，等待 snapshot。
        return
      case 'snapshot':
        if (this.stage !== 'subscribing') {
          this.terminate('protocolError')
          return
        }
        this.clearTimers()
        this.stage = 'ready'
        this.phaseStartedAt = Date.now()
        this.phaseTimeoutMs = 0
        this.diagnose('ready')
        if (this.wireVersion >= 6) { this.nextProbeAt = Date.now() + 10000; this.scheduleHeartbeat(10000) }
        else this.diagnose('heartbeat_unavailable')
        this.emit({ kind: 'connectionStateChanged', connection: 'connected' })
        this.emit({ kind: 'snapshot', snapshot: message.payload.snapshot })
        return
      case 'event':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        this.emit({ kind: 'event', event: message.payload.event })
        return
      case 'command_result':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        if (message.commandId !== this.pendingCommandId || this.pendingKind !== 'command') {
          this.terminate('protocolError')
          return
        }
        if (this.commandTimer !== null) clearTimeout(this.commandTimer)
        this.commandTimer = null
        recordAction(message.payload.accepted ? 'accepted' : 'rejected')
        this.pendingCommandId = null
        this.pendingKind = null
        this.emit({
          kind: 'commandResult',
          commandId: message.commandId,
          requestId: message.requestId,
          accepted: message.payload.accepted,
          code: message.payload.code,
          message: message.payload.message,
        })
        return
      case 'create_session_result':
      case 'stop_session_result': {
        const operation = message.type === 'create_session_result' ? 'create_session' : 'stop_session'
        if (this.stage !== 'ready' || message.commandId !== this.pendingCommandId || this.pendingKind !== operation) {
          this.terminate('protocolError')
          return
        }
        if (this.commandTimer !== null) clearTimeout(this.commandTimer)
        this.commandTimer = null
        recordAction(message.payload.accepted ? 'accepted' : 'rejected')
        this.pendingCommandId = null
        this.pendingKind = null
        this.emit({
          kind: 'managementResult', operation, commandId: message.commandId,
          accepted: message.payload.accepted, code: message.payload.code,
          message: message.payload.message,
          ...(message.payload.pairing === undefined ? {} : { pairing: message.payload.pairing }),
        })
        return
      }
      case 'unsupported':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        // 通知而非错误：上报观察者后继续使用当前连接。seq 随通知上报，
        // 供 reducer 做与普通事件一致的连续性校验。
        this.emit({ kind: 'unsupported', code: message.payload.code, seq: message.seq })
        return
      case 'error':
        if (message.payload.code === 'unsupported_version' && this.stage === 'authenticating' &&
            (message.payload.expectedVersion === 2 || message.payload.expectedVersion === 3 || message.payload.expectedVersion === 4 || message.payload.expectedVersion === 5) && message.payload.expectedVersion < this.wireVersion) {
          this.wireVersion = message.payload.expectedVersion!
          this.diagnose('protocol_retry')
          this.close()
          this.connect(true)
          return
        }
        if (message.payload.code === 'unauthorized') {
          this.terminate('pairingInvalid')
          return
        }
        // 其他服务端错误：协议会话仍在，但按协议错误终止本连接最保守。
        this.terminate('protocolError')
        return
      case 'unsubscribed':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        return
    }
  }
}
