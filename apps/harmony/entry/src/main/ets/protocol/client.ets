import { debugLog, type LogCode } from '../debug/log'
import { BusinessError } from '@kit.BasicServicesKit'
import { webSocket } from '@kit.NetworkKit'
import { decodeServerMessage } from './decode'
import type {
  Attachment,
  DomainEvent,
  EmptyPayload,
  ErrorInfo,
  HelloPayload,
  PairingPayload,
  PermissionMode,
  QuestionAnswer,
  ServerMessage,
  SessionSnapshot,
  UnsupportedCode,
} from './types'
import { PROTOCOL_VERSION, PREFERRED_PROTOCOL_VERSION } from './types'

export type ActionDecision = 'allow_once' | 'allow_session' | 'deny'

export interface SendTextCommandPayload {
  kind: 'send_text'
  text: string
}

export interface ResolveActionDecisionPayload {
  kind: 'resolve_action'
  actionId: string
  decision: ActionDecision
}

export interface ResolveActionAnswersPayload {
  kind: 'resolve_action'
  actionId: string
  answers: Array<QuestionAnswer>
}

export interface CancelTurnCommandPayload {
  kind: 'cancel_turn'
  requestId: string
}

export interface CancelQueuedCommandPayload {
  kind: 'cancel_queued'
  requestId: string
}

export interface SendAttachmentsPayload {
  kind: 'send_attachments'
  text: string
  attachments: Array<Attachment>
}

export interface SetPermissionModePayload {
  kind: 'set_permission_mode'
  permissionMode: PermissionMode
}

export type CommandPayload =
  | SetPermissionModePayload
  | SendAttachmentsPayload
  | SendTextCommandPayload
  | ResolveActionDecisionPayload
  | ResolveActionAnswersPayload
  | CancelTurnCommandPayload
  | CancelQueuedCommandPayload

export interface ResolveActionDecision {
  mode: 'decision'
  actionId: string
  decision: ActionDecision
}

export interface ResolveActionAnswers {
  mode: 'answers'
  actionId: string
  answers: Array<QuestionAnswer>
}

export type ActionResolution = ResolveActionDecision | ResolveActionAnswers

export interface SendResult {
  ok: boolean
  reason?: 'notReady' | 'busy'
}

export interface CommandResultInfo {
  commandId: string
  requestId: string
  accepted: boolean
  code: string
  message: string
}

export interface ManagementResultInfo {
  operation: 'create_session' | 'stop_session'
  commandId: string
  accepted: boolean
  code: string
  message: string
  pairing?: PairingPayload
}

export interface CloseInfo {
  reason: 'DISCONNECTED' | 'PAIRING_INVALID' | 'PROTOCOL_ERROR'
  pendingCommandId: string | null
  error: ErrorInfo | null
}

export interface ProtocolClientObserver {
  onSocketOpened?(): void
  onHelloAck?(): void
  onSubscribed?(): void
  onSnapshot?(snapshot: SessionSnapshot): void
  onEvent?(event: DomainEvent): void
  onUnsupported?(code: UnsupportedCode, seq: number): void
  onCommandResult?(result: CommandResultInfo): void
  onManagementResult?(result: ManagementResultInfo): void
  onClosed?(info: CloseInfo): void
}

type ClientStage =
  | 'IDLE'
  | 'CONNECTING'
  | 'AUTHENTICATING'
  | 'SUBSCRIBING'
  | 'SUBSCRIBED'
  | 'READY'

interface ClientEnvelope {
  version: number
  type: string
  sessionId: string
  commandId: string
  requestId: string
  seq: number
  payload: Object
}

function makeError(code: string, message: string): ErrorInfo {
  const error: ErrorInfo = { code, message }
  return error
}

function createHelloPayload(pairToken: string): HelloPayload {
  const payload: HelloPayload = { pairToken }
  return payload
}

function createEmptyPayload(): EmptyPayload {
  const payload: EmptyPayload = {}
  return payload
}

function createResolveActionDecisionPayload(
  actionId: string,
  decision: ActionDecision,
): ResolveActionDecisionPayload {
  const payload: ResolveActionDecisionPayload = {
    kind: 'resolve_action',
    actionId,
    decision,
  }
  return payload
}

function createResolveActionAnswersPayload(
  actionId: string,
  answers: Array<QuestionAnswer>,
): ResolveActionAnswersPayload {
  const payload: ResolveActionAnswersPayload = {
    kind: 'resolve_action',
    actionId,
    answers,
  }
  return payload
}

function createEnvelope(
  version: number,
  type: string,
  sessionId: string,
  commandId: string,
  requestId: string,
  seq: number,
  payload: Object,
): ClientEnvelope {
  const envelope: ClientEnvelope = {
    version,
    type,
    sessionId,
    commandId,
    requestId,
    seq,
    payload,
  }
  return envelope
}

function encodeEnvelope(
  version: number,
  type: string,
  sessionId: string,
  commandId: string,
  requestId: string,
  seq: number,
  payload: Object,
): string {
  return JSON.stringify(createEnvelope(version, type, sessionId, commandId, requestId, seq, payload))
}

function businessErrorToInfo(err: BusinessError | null | undefined, fallback: string): ErrorInfo | null {
  if (err === null || err === undefined) {
    if (fallback.length === 0) {
      return null
    }
    return makeError('socket_error', fallback)
  }
  const message = typeof err.message === 'string' && err.message.length > 0 ? err.message : fallback
  return makeError(`${err.code}`, message)
}

export function createDecisionResolution(actionId: string, decision: ActionDecision): ActionResolution {
  const resolution: ResolveActionDecision = {
    mode: 'decision',
    actionId,
    decision,
  }
  return resolution
}

export function createAnswersResolution(actionId: string, answers: Array<QuestionAnswer>): ActionResolution {
  const resolution: ResolveActionAnswers = {
    mode: 'answers',
    actionId,
    answers,
  }
  return resolution
}

export class ProtocolClient {
  private readonly pairing: PairingPayload
  private socket: webSocket.WebSocket | null = null
  private stage: ClientStage = 'IDLE'
  private pendingCommandId: string | null = null
  private pendingKind: 'command' | 'create_session' | 'stop_session' | null = null
  private generation = 0
  private deadline: number = -1
  private heartbeat: number = -1
  private pongAt: number = 0
  private userClosed = false
  private wireVersion: number = PREFERRED_PROTOCOL_VERSION
  private observer: ProtocolClientObserver | null = null

  constructor(pairing: PairingPayload) {
    this.pairing = pairing
  }

  setObserver(observer: ProtocolClientObserver | null): void {
    this.observer = observer
  }

  isReady(): boolean {
    return this.stage === 'READY'
  }

  connect(): void {
    if (this.socket !== null) {
      return
    }

    const socket = webSocket.createWebSocket()
    const generation = this.generation + 1
    this.generation = generation
    this.socket = socket
    this.stage = 'CONNECTING'
    debugLog.record('connect')
    debugLog.connectionTarget(this.pairing.wsUrl)
    this.armDeadline('connect_timeout', 10000)
    this.userClosed = false
    this.pendingCommandId = null
    this.pendingKind = null

    socket.on('open', (err: BusinessError, _value: Object) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      if (err !== null && err !== undefined) {
        this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket open failed'))
        return
      }
      this.stage = 'AUTHENTICATING'
      debugLog.record('socket_open')
      this.armDeadline('auth_timeout', 8000)
      if (this.observer?.onSocketOpened !== undefined) {
        this.observer.onSocketOpened()
      }
      const payload = createHelloPayload(this.pairing.pairToken)
      this.sendFrame(encodeEnvelope(this.wireVersion, 'hello', this.pairing.sessionId, '', '', 0, payload as Object))
    })

    socket.on('message', (err: BusinessError, value: string | ArrayBuffer) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      if (err !== null && err !== undefined) {
        this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket message failed'))
        return
      }
      if (typeof value !== 'string') {
        this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'server message must be JSON text'))
        return
      }
      this.handleServerMessage(value)
    })

    socket.on('close', (err: BusinessError, value: webSocket.CloseResult) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      this.clearTimers()
      debugLog.record('closed', value.code)
      this.socket = null
      this.stage = 'IDLE'
      if (this.userClosed) {
        return
      }
      let error: ErrorInfo | null = null
      if (err !== null && err !== undefined) {
        error = businessErrorToInfo(err, 'socket closed')
      } else if (value.reason.length > 0 && value.code !== 1000) {
        error = makeError(`${value.code}`, value.reason)
      }
      this.notifyClosed('DISCONNECTED', error)
    })

    socket.on('error', (err: BusinessError) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket error'))
    })

    socket.connect(this.pairing.wsUrl, (err: BusinessError, value: boolean) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      if ((err !== null && err !== undefined) || value !== true) {
        this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket connect request failed'))
      }
    })
  }

  private clearTimers(): void {
    if (this.deadline !== -1) clearTimeout(this.deadline)
    if (this.heartbeat !== -1) clearTimeout(this.heartbeat)
    this.deadline = -1; this.heartbeat = -1; this.pongAt = 0
  }
  private armDeadline(code: LogCode, ms: number): void {
    if (this.deadline !== -1) clearTimeout(this.deadline)
    this.deadline = setTimeout(() => {
      debugLog.record(code, ms)
      this.terminate('DISCONNECTED', makeError(code, '连接阶段超时，请检查网络、VPN或电脑端服务'))
    }, ms)
  }
  networkLost(): void {
    debugLog.record('network_lost')
    if (this.socket !== null) this.terminate('DISCONNECTED', makeError('network_lost', '网络连接已断开；配对与消息保留，请恢复网络后重连'))
  }
  checkConnection(): void {
    if (this.stage !== 'READY' || this.wireVersion < 6 || this.socket === null) return
    if (this.pongAt > 0) {
      if (Date.now() - this.pongAt >= 5000) { debugLog.record('heartbeat_timeout', 5000); this.terminate('DISCONNECTED', makeError('heartbeat_timeout', '连接探活超时，操作结果未知，请核对电脑端状态')) }
      return
    }
    if (this.heartbeat !== -1) clearTimeout(this.heartbeat)
    this.heartbeat = -1
    this.pongAt = Date.now(); debugLog.record('ping', 5000)
    this.sendFrame(encodeEnvelope(this.wireVersion, 'ping', this.pairing.sessionId, '', '', 0, createEmptyPayload() as Object))
    if (this.stage !== 'READY') return
    this.heartbeat = setTimeout(() => this.checkConnection(), 5000)
  }
  private scheduleHeartbeat(): void {
    if (this.heartbeat !== -1) clearTimeout(this.heartbeat)
    this.heartbeat = setTimeout(() => this.checkConnection(), 10000)
  }

  reconnect(): void {
    debugLog.record('reconnect')
    this.close()
    this.connect()
  }

  close(): void {
    this.clearTimers()
    debugLog.record('closed')
    const socket = this.socket
    this.userClosed = true
    this.generation += 1
    this.socket = null
    this.stage = 'IDLE'
    this.pendingCommandId = null
    this.pendingKind = null
    if (socket === null) {
      return
    }
    socket.close().catch((_err: BusinessError) => {})
  }

  sendAttachments(text: string, attachments: Array<Attachment>, commandId: string): SendResult {
    const payload: SendAttachmentsPayload = { kind: 'send_attachments', text, attachments }
    return this.sendCommand(commandId, payload)
  }

  setPermissionMode(permissionMode: PermissionMode, commandId: string): SendResult {
    const payload: SetPermissionModePayload = { kind: 'set_permission_mode', permissionMode }
    return this.sendCommand(commandId, payload)
  }

  sendText(text: string, commandId: string): SendResult {
    const payload: SendTextCommandPayload = { kind: 'send_text', text }
    return this.sendCommand(commandId, payload)
  }

  resolveAction(resolution: ActionResolution, commandId: string): SendResult {
    let payload: CommandPayload
    if (resolution.mode === 'decision') {
      payload = createResolveActionDecisionPayload(resolution.actionId, resolution.decision)
    } else {
      payload = createResolveActionAnswersPayload(resolution.actionId, resolution.answers)
    }
    return this.sendCommand(commandId, payload)
  }

  cancelTurn(requestId: string, commandId: string): SendResult {
    const payload: CancelTurnCommandPayload = { kind: 'cancel_turn', requestId }
    return this.sendCommand(commandId, payload)
  }

  cancelQueued(requestId: string, commandId: string): SendResult {
    const payload: CancelQueuedCommandPayload = { kind: 'cancel_queued', requestId }
    return this.sendCommand(commandId, payload)
  }

  createSession(commandId: string): SendResult {
    return this.sendManagement('create_session', commandId)
  }

  stopSession(commandId: string): SendResult {
    return this.sendManagement('stop_session', commandId)
  }

  private sendManagement(operation: 'create_session' | 'stop_session', commandId: string): SendResult {
    if (this.stage !== 'READY' || this.socket === null) {
      return { ok: false, reason: 'notReady' }
    }
    if (this.pendingCommandId !== null) {
      return { ok: false, reason: 'busy' }
    }
    this.pendingCommandId = commandId
    this.pendingKind = operation
    const payload = createEmptyPayload()
    this.sendFrame(encodeEnvelope(this.wireVersion, operation, this.pairing.sessionId, commandId, '', 0, payload as Object))
    return { ok: true }
  }

  private sendCommand(commandId: string, payload: CommandPayload): SendResult {
    if (this.stage !== 'READY' || this.socket === null) {
      return { ok: false, reason: 'notReady' }
    }
    if (this.pendingCommandId !== null) {
      return { ok: false, reason: 'busy' }
    }
    this.pendingCommandId = commandId
    this.pendingKind = 'command'
    this.sendFrame(
      encodeEnvelope(this.wireVersion, 'command', this.pairing.sessionId, commandId, '', 0, payload as Object),
    )
    const result: SendResult = { ok: true }
    return result
  }

  private sendFrame(frame: string): void {
    if (this.socket === null || this.stage === 'CONNECTING') {
      this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'send called before socket open'))
      return
    }
    const generation = this.generation
    const socket = this.socket
    socket.send(frame).then((value: boolean) => {
      if (generation !== this.generation || this.socket !== socket) return
      if (!value) {
        this.terminate('DISCONNECTED', makeError('socket_error', 'socket send request failed'))
      }
    }).catch((err: BusinessError) => {
      if (generation !== this.generation || this.socket !== socket) return
      this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket send failed'))
    })
  }

  private notifyClosed(
    reason: 'DISCONNECTED' | 'PAIRING_INVALID' | 'PROTOCOL_ERROR',
    error: ErrorInfo | null,
  ): void {
    debugLog.record(reason === 'PROTOCOL_ERROR' ? 'protocol_error' : reason === 'PAIRING_INVALID' ? 'pairing_invalid' : 'socket_error')
    const pendingCommandId = this.pendingCommandId
    this.pendingCommandId = null
    this.pendingKind = null
    const info: CloseInfo = {
      reason,
      pendingCommandId,
      error,
    }
    if (this.observer?.onClosed !== undefined) {
      this.observer.onClosed(info)
    }
  }

  private terminate(
    reason: 'DISCONNECTED' | 'PAIRING_INVALID' | 'PROTOCOL_ERROR',
    error: ErrorInfo | null,
  ): void {
    const socket = this.socket
    this.clearTimers()
    this.userClosed = true
    this.generation += 1
    this.socket = null
    this.stage = 'IDLE'
    if (socket !== null) {
      socket.close().catch((_err: BusinessError) => {})
    }
    this.notifyClosed(reason, error)
  }

  private handleServerMessage(raw: string): void {
    let message: ServerMessage
    try {
      message = decodeServerMessage(raw)
    } catch (_error) {
      this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'failed to decode server message'))
      return
    }

    const legacy = message.version >= 2 && message.version <= 5
    if (legacy && message.version < this.wireVersion && this.stage === 'AUTHENTICATING' && message.type === 'hello_ack') {
      this.wireVersion = message.version
    }
    const fallbackError = message.type === 'error' && this.stage === 'AUTHENTICATING' && legacy &&
      message.payload.code === 'unsupported_version' &&
      (message.payload.expectedVersion !== undefined && message.payload.expectedVersion >= 2 && message.payload.expectedVersion <= 5) &&
      message.payload.expectedVersion < this.wireVersion
    const legacyUnauthorized = message.type === 'error' && this.stage === 'AUTHENTICATING' && legacy &&
      message.payload.code === 'unauthorized'
    if (message.version !== this.wireVersion && !fallbackError && !legacyUnauthorized) {
      this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'message version changed'))
      return
    }

    if (
      (message.type === 'pong' || message.type === 'snapshot' ||
        message.type === 'event' ||
        message.type === 'unsupported' ||
        message.type === 'command_result' ||
        message.type === 'create_session_result' ||
        message.type === 'stop_session_result') &&
      message.sessionId !== this.pairing.sessionId
    ) {
      this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'message session does not match pairing'))
      return
    }

    switch (message.type) {
      case 'pong':
        if (this.stage !== 'READY' || this.pongAt === 0) { this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'unexpected pong')); return }
        if (Date.now() - this.pongAt >= 5000) { debugLog.record('heartbeat_timeout', 5000); this.terminate('DISCONNECTED', makeError('heartbeat_timeout', '连接探活响应超过5秒，操作结果未知')); return }
        debugLog.record('pong', Date.now() - this.pongAt)
        this.pongAt = 0
        this.scheduleHeartbeat()
        return
      case 'hello_ack': {
        if (this.stage !== 'AUTHENTICATING') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'hello_ack arrived before hello'))
          return
        }
        this.stage = 'SUBSCRIBING'
        debugLog.record('authenticated')
        this.armDeadline('subscribe_timeout', 8000)
        if (this.observer?.onHelloAck !== undefined) {
          this.observer.onHelloAck()
        }
        const payload = createEmptyPayload()
        this.sendFrame(encodeEnvelope(this.wireVersion, 'subscribe', this.pairing.sessionId, '', '', 0, payload as Object))
        return
      }
      case 'subscribed':
        if (this.stage !== 'SUBSCRIBING') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'subscribed arrived before subscribe'))
          return
        }
        this.stage = 'SUBSCRIBED'
        debugLog.record('subscribed')
        this.armDeadline('snapshot_timeout', 10000)
        if (this.observer?.onSubscribed !== undefined) {
          this.observer.onSubscribed()
        }
        return
      case 'snapshot':
        if (this.stage !== 'SUBSCRIBED') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'snapshot arrived before subscribed state'))
          return
        }
        this.stage = 'READY'
        this.clearTimers()
        debugLog.record('ready')
        if (this.wireVersion >= 6) this.scheduleHeartbeat()
        else debugLog.record('legacy', this.wireVersion)
        this.pendingCommandId = null
        this.pendingKind = null
        if (this.observer?.onSnapshot !== undefined) {
          this.observer.onSnapshot(message.payload.snapshot)
        }
        return
      case 'event':
        if (this.stage !== 'READY') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'event arrived before snapshot'))
          return
        }
        if (this.observer?.onEvent !== undefined) {
          this.observer.onEvent(message.payload.event)
        }
        return
      case 'unsupported':
        if (this.stage !== 'READY') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'unsupported arrived before snapshot'))
          return
        }
        if (this.observer?.onUnsupported !== undefined) {
          this.observer.onUnsupported(message.payload.code, message.seq)
        }
        return
      case 'command_result': {
        if (this.stage !== 'READY') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'command_result arrived before snapshot'))
          return
        }
        if (message.commandId !== this.pendingCommandId || this.pendingKind !== 'command') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'command_result does not match pending command'))
          return
        }
        if (message.payload.accepted && message.requestId.length === 0) {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'accepted command_result must carry requestId'))
          return
        }
        this.pendingCommandId = null
        this.pendingKind = null
        const result: CommandResultInfo = {
          commandId: message.commandId,
          requestId: message.requestId,
          accepted: message.payload.accepted,
          code: message.payload.code,
          message: message.payload.message,
        }
        if (this.observer?.onCommandResult !== undefined) {
          this.observer.onCommandResult(result)
        }
        return
      }
      case 'create_session_result':
      case 'stop_session_result': {
        const operation = message.type === 'create_session_result' ? 'create_session' : 'stop_session'
        if (this.stage !== 'READY' || message.commandId !== this.pendingCommandId || this.pendingKind !== operation) {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'management result does not match pending request'))
          return
        }
        this.pendingCommandId = null
        this.pendingKind = null
        const result: ManagementResultInfo = {
          operation,
          commandId: message.commandId,
          accepted: message.payload.accepted,
          code: message.payload.code,
          message: message.payload.message,
        }
        if (message.payload.pairing !== undefined) {
          result.pairing = message.payload.pairing
        }
        if (this.observer?.onManagementResult !== undefined) {
          this.observer.onManagementResult(result)
        }
        return
      }
      case 'error':
        if (fallbackError) {
          debugLog.record('legacy', message.payload.expectedVersion ?? 2)
          this.close()
          this.wireVersion = message.payload.expectedVersion ?? PROTOCOL_VERSION
          this.connect()
          return
        }
        if (message.payload.code === 'unauthorized') {
          this.terminate('PAIRING_INVALID', makeError(message.payload.code, message.payload.message))
          return
        }
        this.terminate('PROTOCOL_ERROR', makeError(message.payload.code, message.payload.message))
        return
      case 'unsubscribed':
        this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'unexpected unsubscribed message'))
        return
    }
  }
}
