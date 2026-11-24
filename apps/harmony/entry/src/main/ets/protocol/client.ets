import { debugLog, type LogCode } from '../debug/log'
import { BusinessError } from '@kit.BasicServicesKit'
import { webSocket } from '@kit.NetworkKit'
import { decodeServerMessage } from './decode'
import type {
  Attachment,
  ImageResultPayload,
  ToolDetail, ToolDetailGetPayload,
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
import { PROTOCOL_VERSION, PREFERRED_PROTOCOL_VERSION, TOOL_DETAIL_PROTOCOL_VERSION } from './types'

export type ActionDecision = 'allow_once' | 'allow_session' | 'deny'

interface ImageGetPayload { ref: string }
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

export interface SteerQueuedCommandPayload {
  kind: 'steer_queued'
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
  | SteerQueuedCommandPayload

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
  imageRefs?: Array<string>
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
  onImageResult?(result: ImageResultPayload): void
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

interface PendingToolDetail {
  requestId: string
  toolUseId: string
  timer: number
  resolve: (detail: ToolDetail) => void
  reject: (error: Error) => void
}

export class ProtocolClient {
  private toolDetailSerial: number = 0
  private readonly toolDetails: Map<string, PendingToolDetail> = new Map()

  private readonly pairing: PairingPayload
  private socket: webSocket.WebSocket | null = null
  private stage: ClientStage = 'IDLE'
  private pendingCommandId: string | null = null
  private pendingKind: 'command' | 'create_session' | 'stop_session' | null = null
  private generation = 0
  private connectionId: number = 0
  private connectionStartedAt: number = 0
  private deadline: number = -1
  private heartbeat: number = -1
  private pongAt: number = 0
  private onProbePong: (() => void) | undefined = undefined
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
    this.connectionId = debugLog.beginConnection()
    this.connectionStartedAt = Date.now()
    this.logConnection('connect')
    this.armDeadline('connect_timeout', 10000)
    this.userClosed = false
    this.pendingCommandId = null
    this.pendingKind = null

    socket.on('open', (err: BusinessError, _value: Object) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      if (err !== null && err !== undefined) {
        this.logConnection('socket_open_error', err.code, err?.message ?? '')
        this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket open failed'))
        return
      }
      this.stage = 'AUTHENTICATING'
      this.logConnection('socket_open')
      this.armDeadline('auth_timeout', 8000)
      if (this.observer?.onSocketOpened !== undefined) {
        this.observer.onSocketOpened()
      }
      const payload = createHelloPayload(this.pairing.pairToken)
      this.sendFrame(encodeEnvelope(this.wireVersion, 'hello', this.pairing.sessionId, '', '', 0, payload as Object), 'hello')
    })

    socket.on('message', (err: BusinessError, value: string | ArrayBuffer) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      if (this.stage === 'AUTHENTICATING') {
        if (err !== null && err !== undefined) {
          this.logConnection('hello_response_error', err.code, err?.message ?? '')
        } else {
          this.logConnection('hello_response', typeof value === 'string' ? 1 : 0)
        }
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
      this.logConnection('closed', value.code, `关闭原因=${value.reason} ${err?.message ?? ''}`.trim())
      this.socket = null
      if (this.userClosed) {
        this.stage = 'IDLE'
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
      this.logConnection('socket_event_error', err?.code ?? 0, err?.message ?? '')
      this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket error'))
    })

    const address = this.pairing.wsUrl
    const url = this.wireVersion >= TOOL_DETAIL_PROTOCOL_VERSION && !/[?&]toolTitles=/.test(address)
      ? address + (address.includes('?') ? '&' : '?') + 'toolTitles=1' : address
    socket.connect(url, (err: BusinessError, value: boolean) => {
      if (generation !== this.generation || this.socket !== socket) {
        return
      }
      this.logConnection('socket_connect_result', err?.code ?? (value === true ? 1 : 0), err?.message ?? '')
      if ((err !== null && err !== undefined) || value !== true) {
        this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket connect request failed'))
      }
    })
  }

  private logConnection(code: LogCode, value: number = 0, detail: string = ''): void {
    if (this.connectionId === 0) return
    const token = this.pairing.pairToken
    const safeDetail = (token.length > 0 ? detail.split(token).join('[配对凭证]') : detail)
      .replace(/[\r\n]/g, ' ').slice(0, 500)
    debugLog.connection(this.connectionId, this.pairing.wsUrl, this.stage,
      Date.now() - this.connectionStartedAt, code, value, safeDetail)
  }

  private clearTimers(): void {
    if (this.deadline !== -1) clearTimeout(this.deadline)
    if (this.heartbeat !== -1) clearTimeout(this.heartbeat)
    this.deadline = -1; this.heartbeat = -1; this.pongAt = 0
    this.onProbePong = undefined
  }
  private armDeadline(code: LogCode, ms: number): void {
    if (this.deadline !== -1) clearTimeout(this.deadline)
    this.deadline = setTimeout(() => {
      this.logConnection(code, ms)
      this.terminate('DISCONNECTED', makeError(code, '连接阶段超时，请检查网络、VPN或电脑端服务'))
    }, ms)
  }
  networkLost(): void {
    this.logConnection('network_lost')
    if (this.socket !== null) this.terminate('DISCONNECTED', makeError('network_lost', '网络连接已断开；配对与消息保留，请恢复网络后重连'))
  }
  checkConnection(onPong?: () => void): void {
    if (this.stage !== 'READY' || this.wireVersion < 6 || this.socket === null) {
      onPong?.()
      return
    }
    if (onPong !== undefined) this.onProbePong = onPong
    if (this.pongAt > 0) {
      if (Date.now() - this.pongAt >= 5000) { this.logConnection('heartbeat_timeout', 5000); this.terminate('DISCONNECTED', makeError('heartbeat_timeout', '连接探活超时，操作结果未知，请核对电脑端状态')) }
      return
    }
    if (this.heartbeat !== -1) clearTimeout(this.heartbeat)
    this.heartbeat = -1
    this.pongAt = Date.now(); this.logConnection('ping', 5000)
    this.sendFrame(encodeEnvelope(this.wireVersion, 'ping', this.pairing.sessionId, '', '', 0, createEmptyPayload() as Object), 'ping')
    if (this.stage !== 'READY') return
    this.heartbeat = setTimeout(() => this.checkConnection(), 5000)
  }
  private scheduleHeartbeat(): void {
    if (this.heartbeat !== -1) clearTimeout(this.heartbeat)
    this.heartbeat = setTimeout(() => this.checkConnection(), 10000)
  }

  reconnect(): void {
    this.logConnection('reconnect')
    this.close()
    this.connect()
  }

  close(): void {
    this.clearTimers()
    this.logConnection('closed')
    const socket = this.socket
    this.userClosed = true
    this.generation += 1
    this.socket = null
    this.stage = 'IDLE'
    this.clearToolDetails()
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

  steerQueued(requestId: string, commandId: string): SendResult {
    if (this.wireVersion < 8) return { ok: false, reason: 'notReady' }
    const payload: SteerQueuedCommandPayload = { kind: 'steer_queued', requestId }
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

  private sendFrame(frame: string, diagnostic?: 'hello' | 'ping'): void {
    if (this.socket === null || this.stage === 'CONNECTING') {
      this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'send called before socket open'))
      return
    }
    const generation = this.generation
    const socket = this.socket
    socket.send(frame).then((value: boolean) => {
      if (generation !== this.generation || this.socket !== socket) return
      if (diagnostic !== undefined) this.logConnection(diagnostic === 'hello' ? 'hello_send_result' : 'ping_send_result', value ? 1 : 0)
      if (!value) {
        this.terminate('DISCONNECTED', makeError('socket_error', 'socket send request failed'))
      }
    }).catch((err: BusinessError) => {
      if (generation !== this.generation || this.socket !== socket) return
      if (diagnostic !== undefined) this.logConnection(diagnostic === 'hello' ? 'hello_send_error' : 'ping_send_error', err?.code ?? 0, err?.message ?? '')
      this.terminate('DISCONNECTED', businessErrorToInfo(err, 'socket send failed'))
    })
  }

  getToolDetail(requestId: string, toolUseId: string): Promise<ToolDetail> {
    if (this.stage !== 'READY' || this.wireVersion < TOOL_DETAIL_PROTOCOL_VERSION || this.socket === null) {
      return Promise.reject(new Error('电脑未连接或不支持按需工具详情'))
    }
    if (this.toolDetails.size >= 16) return Promise.reject(new Error('工具详情请求过多，请稍后重试'))
    this.toolDetailSerial += 1
    const commandId = `detail-${this.generation}-${this.toolDetailSerial}`
    return new Promise<ToolDetail>((resolve: (detail: ToolDetail) => void, reject: (error: Error) => void) => {
      const timer = setTimeout(() => {
        this.toolDetails.delete(commandId)
        reject(new Error('读取工具详情超时，请重试'))
      }, 8000)
      this.toolDetails.set(commandId, { requestId, toolUseId, timer, resolve, reject })
      const payload: ToolDetailGetPayload = { toolUseId }
      this.sendFrame(encodeEnvelope(this.wireVersion, 'tool_detail_get', this.pairing.sessionId, commandId, requestId, 0, payload as Object))
    })
  }

  private clearToolDetails(): void {
    this.toolDetails.forEach((pending: PendingToolDetail) => {
      clearTimeout(pending.timer)
      pending.reject(new Error('连接已断开，请重连后重试'))
    })
    this.toolDetails.clear()
  }

  getImage(ref: string): boolean {
    if (this.stage !== 'READY' || this.wireVersion < 10 || this.socket === null || ref.length === 0 || ref.length > 128) return false
    const payload: ImageGetPayload = { ref }
    this.sendFrame(encodeEnvelope(this.wireVersion, 'image_get', this.pairing.sessionId, '', '', 0, payload as Object))
    return true
  }

  private pairingDisconnectMessage(error: ErrorInfo | null): string {
    const target = debugLog.target(this.pairing.wsUrl)
    if (error?.code === 'network_lost') return '手机网络已断开，请恢复网络后重新配对'
    if (this.stage === 'AUTHENTICATING') {
      return `已连接电脑（${target}），但配对验证未完成。请确认远程服务仍开启，刷新二维码后重试`
    }
    if (this.stage === 'SUBSCRIBING' || this.stage === 'SUBSCRIBED') {
      return `配对已验证，但未能完成会话同步（${target}）。请确认电脑上的会话仍可用后重试`
    }
    const reason = error?.code === 'connect_timeout' ? '连接电脑超时' : '连接失败'
    return `${reason}（${target}）。请确认电脑已开启远程服务、二维码地址正确，且手机可通过局域网或 VPN 访问电脑`
  }

  private notifyClosed(
    reason: 'DISCONNECTED' | 'PAIRING_INVALID' | 'PROTOCOL_ERROR',
    error: ErrorInfo | null,
  ): void {
    this.logConnection(reason === 'PROTOCOL_ERROR' ? 'protocol_error' : reason === 'PAIRING_INVALID' ? 'pairing_invalid' : 'socket_error',
      reason === 'DISCONNECTED' && error !== null && Number.isFinite(Number(error.code)) ? Number(error.code) : 0,
      error === null ? '' : `错误=${error.code}`)
    const displayedError = reason === 'DISCONNECTED' && this.stage !== 'READY'
      ? makeError(error?.code ?? 'socket_closed', this.pairingDisconnectMessage(error))
      : error
    this.stage = 'IDLE'
    this.clearToolDetails()
    const pendingCommandId = this.pendingCommandId
    this.pendingCommandId = null
    this.pendingKind = null
    const info: CloseInfo = {
      reason,
      pendingCommandId,
      error: displayedError,
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

    const legacy = message.version >= 2 && message.version < PREFERRED_PROTOCOL_VERSION
    if (legacy && message.version < this.wireVersion && this.stage === 'AUTHENTICATING' && message.type === 'hello_ack') {
      this.wireVersion = message.version
    }
    const fallbackError = message.type === 'error' && this.stage === 'AUTHENTICATING' && legacy &&
      message.payload.code === 'unsupported_version' &&
      (message.payload.expectedVersion !== undefined && message.payload.expectedVersion >= 2 && message.payload.expectedVersion < PREFERRED_PROTOCOL_VERSION) &&
      message.payload.expectedVersion < this.wireVersion
    const legacyUnauthorized = message.type === 'error' && this.stage === 'AUTHENTICATING' && legacy &&
      message.payload.code === 'unauthorized'
    if (message.version !== this.wireVersion && !fallbackError && !legacyUnauthorized) {
      this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'message version changed'))
      return
    }

    if (
      (message.type === 'tool_detail_result' || message.type === 'image_result' || message.type === 'pong' || message.type === 'snapshot' ||
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
      case 'tool_detail_result': {
        const pending = this.toolDetails.get(message.commandId)
        if (this.stage !== 'READY' || pending === undefined) return
        if (pending.requestId !== message.requestId || pending.toolUseId !== message.payload.toolUseId) {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'tool detail does not match request'))
          return
        }
        clearTimeout(pending.timer)
        this.toolDetails.delete(message.commandId)
        if (message.payload.detail === undefined) pending.reject(new Error('电脑端没有可读取的工具详情，结果可能已过期'))
        else pending.resolve(message.payload.detail)
        return
      }
      case 'image_result':
        if (this.stage !== 'READY') { this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'unexpected image result')); return }
        this.observer?.onImageResult?.(message.payload)
        return
      case 'pong':
        if (this.stage !== 'READY' || this.pongAt === 0) { this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'unexpected pong')); return }
        if (Date.now() - this.pongAt >= 5000) { this.logConnection('heartbeat_timeout', 5000); this.terminate('DISCONNECTED', makeError('heartbeat_timeout', '连接探活响应超过5秒，操作结果未知')); return }
        this.logConnection('pong', Date.now() - this.pongAt)
        this.pongAt = 0
        this.scheduleHeartbeat()
        const onPong = this.onProbePong
        this.onProbePong = undefined
        onPong?.()
        return
      case 'hello_ack': {
        if (this.stage !== 'AUTHENTICATING') {
          this.terminate('PROTOCOL_ERROR', makeError('protocol_error', 'hello_ack arrived before hello'))
          return
        }
        this.stage = 'SUBSCRIBING'
        this.logConnection('authenticated')
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
        this.logConnection('subscribed')
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
        this.logConnection('ready')
        if (this.wireVersion >= 6) this.scheduleHeartbeat()
        else this.logConnection('legacy', this.wireVersion)
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
          imageRefs: message.payload.imageRefs,
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
          this.logConnection('legacy', message.payload.expectedVersion ?? 2)
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
