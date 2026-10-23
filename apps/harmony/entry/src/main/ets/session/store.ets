import {
  ProtocolClient,
  type ActionResolution,
  type CloseInfo,
  type CommandResultInfo,
  type ManagementResultInfo,
  type ProtocolClientObserver,
} from '../protocol/client'
import { validateAttachments } from './attachments'
import { decodePairingPayload } from '../protocol/pairing'
import type {
  Attachment,
  CommandKind,
  DomainEvent,
  ErrorInfo,
  PairingPayload,
  PermissionMode,
  SessionSnapshot,
  UnsupportedCode,
} from '../protocol/types'
import {
  ConnectionStage,
  createInitialHarmonySessionState,
  type HarmonySessionState,
} from './model'
import type { SessionAction } from './reducer'
import {
  commandAccepted,
  commandRejected,
  commandStarted,
  commandUnknown,
  eventReceived,
  helloAck,
  localError,
  pairingInvalid,
  protocolError,
  reduceHarmonySessionState,
  snapshotReceived,
  socketClosed,
  socketOpened,
  startConnect,
  subscribedReceived,
  unsupportedReceived,
} from './reducer'

interface PendingManagement { operation: 'create_session' | 'stop_session'; commandId: string }

export interface HarmonySessionStore {
  connectFromRawPairing(raw: string): boolean
  connectWithPairing(pairing: PairingPayload): void
  disconnect(): void
  reset(): void
  reconnect(): boolean
  sendText(text: string): boolean
  setPermissionMode(mode: PermissionMode): boolean
  sendAttachments(text: string, attachments: Array<Attachment>): boolean
  restoreDisplay(state: HarmonySessionState): void
  setEventListener(listener: ((event: DomainEvent) => void) | null): void
  setSendListener(listener: ((accepted: boolean) => void) | null): void
  resolveAction(resolution: ActionResolution): boolean
  cancelTurn(requestId: string): boolean
  cancelQueued(requestId: string): boolean
  createSession(): boolean
  stopSession(): boolean
  setManagementListener(listener: ((result: ManagementResultInfo) => void) | null): void
  canSend(): boolean
  getState(): HarmonySessionState
  subscribe(listener: StateListener): UnsubscribeListener
}

export type StateListener = (state: HarmonySessionState) => void
export type UnsubscribeListener = () => void

let commandCounter = 0

function nextCommandId(): string {
  commandCounter += 1
  return `harmony-cmd-${Date.now()}-${commandCounter}`
}

function makeError(code: string, message: string): ErrorInfo {
  return { code, message }
}

class SessionStoreObserver implements ProtocolClientObserver {
  private readonly owner: SessionStore

  constructor(owner: SessionStore) {
    this.owner = owner
  }

  onSocketOpened(): void {
    this.owner.handleSocketOpened()
  }

  onHelloAck(): void {
    this.owner.handleHelloAck()
  }

  onSubscribed(): void {
    this.owner.handleSubscribed()
  }

  onSnapshot(snapshot: SessionSnapshot): void {
    this.owner.handleSnapshot(snapshot)
  }

  onEvent(event: DomainEvent): void {
    this.owner.handleEvent(event)
  }

  onUnsupported(code: UnsupportedCode, seq: number): void {
    this.owner.handleUnsupported(code, seq)
  }

  onCommandResult(result: CommandResultInfo): void {
    this.owner.handleCommandResult(result)
  }

  onManagementResult(result: ManagementResultInfo): void {
    this.owner.handleManagementResult(result)
  }

  onClosed(info: CloseInfo): void {
    this.owner.handleClosed(info)
  }
}

class SessionStore implements HarmonySessionStore {
  private state: HarmonySessionState = createInitialHarmonySessionState()
  private client: ProtocolClient | null = null
  private pendingAttachments: Array<Attachment> = []
  private eventListener: ((event: DomainEvent) => void) | null = null
  private sendListener: ((accepted: boolean) => void) | null = null
  private commandTimer: number = -1

  setEventListener(listener: ((event: DomainEvent) => void) | null): void { this.eventListener = listener }
  setSendListener(listener: ((accepted: boolean) => void) | null): void { this.sendListener = listener }
  restoreDisplay(state: HarmonySessionState): void {
    if (this.client !== null) return
    this.state = state
    this.emit()
  }

  sendAttachments(text: string, attachments: Array<Attachment>): boolean {
    if (!this.state.canSend || validateAttachments(attachments).length > 0) return false
    this.pendingAttachments = attachments.slice()
    return this.sendCommand('send_attachments', text, '', (commandId: string) =>
      this.client !== null && this.client.sendAttachments(text, attachments, commandId).ok)
  }

  private readonly observer: SessionStoreObserver
  private listeners: Array<StateListener> = []
  private pendingManagement: PendingManagement | null = null
  private managementListener: ((result: ManagementResultInfo) => void) | null = null

  constructor() {
    this.observer = new SessionStoreObserver(this)
  }

  connectFromRawPairing(raw: string): boolean {
    const result = decodePairingPayload(raw)
    if (!result.ok) {
      if (result.error !== null) {
        this.dispatch(localError(makeError(result.error.code, result.error.message)))
      } else {
        this.dispatch(localError(makeError('invalid', 'Invalid pairing payload')))
      }
      return false
    }
    if (result.payload === null) {
      this.dispatch(localError(makeError('invalid', 'Invalid pairing payload')))
      return false
    }
    this.connectWithPairing(result.payload)
    return true
  }

  connectWithPairing(pairing: PairingPayload): void {
    if (this.state.pairing !== null && this.state.pairing.sessionId !== pairing.sessionId) {
      this.abandonManagement()
    }
    this.start(pairing)
  }

  disconnect(): void {
    if (this.state.pendingCommand?.kind === 'send_text' || this.state.pendingCommand?.kind === 'send_attachments') this.sendListener?.(false)
    this.pendingAttachments = []
    this.abandonManagement()
    if (this.state.pendingCommand !== null) this.dispatch(commandUnknown(this.state.pendingCommand.commandId))
    this.detachClient()
    this.dispatch(socketClosed(makeError('closed', 'connection closed by user')))
  }

  reset(): void {
    this.abandonManagement()
    this.detachClient()
    this.state = createInitialHarmonySessionState()
    this.emit()
  }

  reconnect(): boolean {
    if (this.state.pairing === null) {
      this.dispatch(localError(makeError('not_paired', 'no pairing payload is loaded')))
      return false
    }
    this.start(this.state.pairing)
    return true
  }

  sendText(text: string): boolean {
    if (!this.state.canSend || text.trim().length === 0) return false
    this.pendingAttachments = []
    return this.sendCommand('send_text', text, '', (commandId: string) => {
      if (this.client === null) {
        return false
      }
      return this.client.sendText(text, commandId).ok
    })
  }

  setPermissionMode(mode: PermissionMode): boolean {
    if (this.state.connection !== ConnectionStage.READY || (this.state.sessionStatus !== 'ready' && this.state.sessionStatus !== 'starting')) return false
    return this.sendCommand('set_permission_mode', mode, '', (commandId: string) =>
      this.client !== null && this.client.setPermissionMode(mode, commandId).ok)
  }

  resolveAction(resolution: ActionResolution): boolean {
    return this.sendCommand('resolve_action', '', resolution.actionId, (commandId: string) => {
      if (this.client === null) {
        return false
      }
      return this.client.resolveAction(resolution, commandId).ok
    })
  }

  cancelTurn(requestId: string): boolean {
    return this.sendCommand('cancel_turn', '', requestId, (commandId: string) => {
      if (this.client === null) {
        return false
      }
      return this.client.cancelTurn(requestId, commandId).ok
    })
  }

  cancelQueued(requestId: string): boolean {
    return this.sendCommand('cancel_queued', '', requestId, (commandId: string) => {
      if (this.client === null) {
        return false
      }
      return this.client.cancelQueued(requestId, commandId).ok
    })
  }

  createSession(): boolean {
    return this.sendManagement('create_session')
  }

  stopSession(): boolean {
    return this.sendManagement('stop_session')
  }

  setManagementListener(listener: ((result: ManagementResultInfo) => void) | null): void {
    this.managementListener = listener
  }

  private abandonManagement(): void {
    const pending = this.pendingManagement
    this.pendingManagement = null
    if (pending !== null) this.managementListener?.({ operation: pending.operation, commandId: pending.commandId,
      accepted: false, code: 'result_unknown', message: '操作结果未知，请核对电脑端状态' })
  }

  private sendManagement(operation: 'create_session' | 'stop_session'): boolean {
    if (this.client === null || this.pendingManagement !== null) {
      return false
    }
    const commandId = nextCommandId()
    const sent = operation === 'create_session'
      ? this.client.createSession(commandId).ok
      : this.client.stopSession(commandId).ok
    if (sent) {
      this.pendingManagement = { operation, commandId }
    }
    return sent
  }

  canSend(): boolean {
    return this.state.canSend
  }

  getState(): HarmonySessionState {
    return this.state
  }

  subscribe(listener: StateListener): UnsubscribeListener {
    this.listeners.push(listener)
    return () => {
      this.unsubscribe(listener)
    }
  }

  handleSocketOpened(): void {
    this.dispatch(socketOpened())
  }

  handleHelloAck(): void {
    this.dispatch(helloAck())
  }

  handleSubscribed(): void {
    this.dispatch(subscribedReceived())
  }

  handleSnapshot(snapshot: SessionSnapshot): void {
    this.dispatch(snapshotReceived(snapshot))
    if (this.client !== null && this.pendingManagement !== null) {
      if (this.pendingManagement.operation === 'create_session') {
        this.client.createSession(this.pendingManagement.commandId)
      } else {
        this.client.stopSession(this.pendingManagement.commandId)
      }
    }
  }

  handleEvent(event: DomainEvent): void {
    this.dispatch(eventReceived(event))
    if (this.state.connection === ConnectionStage.READY) this.eventListener?.(event)
  }

  handleUnsupported(code: UnsupportedCode, seq: number): void {
    this.dispatch(unsupportedReceived(code, seq))
  }

  handleCommandResult(result: CommandResultInfo): void {
    if (this.commandTimer >= 0) clearTimeout(this.commandTimer)
    this.commandTimer = -1
    const isSend = this.state.pendingCommand?.kind === 'send_text' || this.state.pendingCommand?.kind === 'send_attachments'
    if (result.accepted) {
      this.dispatch(commandAccepted(result.commandId, result.requestId))
      if (isSend && this.pendingAttachments.length > 0) {
        this.state.messages.forEach((message) => {
          if (message.kind === 'input' && message.requestId === result.requestId) message.attachments = this.pendingAttachments.slice()
        })
        this.emit()
      }
    } else this.dispatch(commandRejected(result.commandId, result.code, result.message))
    this.pendingAttachments = []
    if (isSend) this.sendListener?.(result.accepted)
  }

  handleManagementResult(result: ManagementResultInfo): void {
    if (this.pendingManagement === null || result.commandId !== this.pendingManagement.commandId ||
      result.operation !== this.pendingManagement.operation) {
      return
    }
    this.pendingManagement = null
    if (this.managementListener !== null) {
      this.managementListener(result)
    }
  }

  handleClosed(info: CloseInfo): void {
    if (this.commandTimer >= 0) clearTimeout(this.commandTimer)
    this.commandTimer = -1
    if (this.state.pendingCommand?.kind === 'send_text' || this.state.pendingCommand?.kind === 'send_attachments') this.sendListener?.(false)
    this.pendingAttachments = []
    this.client = null
    if (info.pendingCommandId !== null && (this.pendingManagement === null || info.pendingCommandId !== this.pendingManagement.commandId)) {
      this.dispatch(commandUnknown(info.pendingCommandId))
    }
    if (info.reason === 'PAIRING_INVALID') {
      this.dispatch(pairingInvalid(info.error))
      return
    }
    if (info.reason === 'PROTOCOL_ERROR') {
      this.dispatch(protocolError(info.error))
      return
    }
    this.dispatch(socketClosed(info.error))
  }

  private sendCommand(
    kind: CommandKind,
    text: string,
    targetId: string,
    sender: (commandId: string) => boolean,
  ): boolean {
    // Do not replace the in-flight command before the protocol client rejects a second request.
    if (this.state.pendingCommand !== null || this.pendingManagement !== null) return false
    if (this.client === null) {
      this.dispatch(localError(makeError('not_ready', 'connection is not ready')))
      return false
    }
    const commandId = nextCommandId()
    this.dispatch(commandStarted(commandId, kind, text, targetId))
    const sent = sender(commandId)
    if (!sent) {
      this.dispatch(commandRejected(commandId, 'send_failed', `${kind} rejected locally`))
      return false
    }
    this.commandTimer = setTimeout(() => {
      this.commandTimer = -1
      if (this.state.pendingCommand?.commandId !== commandId) return
      this.dispatch(commandUnknown(commandId))
      this.detachClient()
      this.dispatch(socketClosed(makeError('timeout', '结果未知，请核对电脑端后再发送；草稿已保留')))
      this.pendingAttachments = []
      this.sendListener?.(false)
    }, 30000)
    return true
  }

  private unsubscribe(listener: StateListener): void {
    const next: Array<StateListener> = []
    for (let index = 0; index < this.listeners.length; index += 1) {
      if (this.listeners[index] !== listener) {
        next.push(this.listeners[index])
      }
    }
    this.listeners = next
  }

  private emit(): void {
    for (let index = 0; index < this.listeners.length; index += 1) {
      this.listeners[index](this.state)
    }
  }

  private dispatch(action: SessionAction): void {
    const pending = this.state.pendingCommand
    this.state = reduceHarmonySessionState(this.state, action)
    if (this.state.connection === ConnectionStage.PROTOCOL_ERROR && this.client !== null) {
      this.detachClient()
      this.pendingAttachments = []
      if (pending?.kind === 'send_text' || pending?.kind === 'send_attachments') this.sendListener?.(false)
      this.abandonManagement()
    }
    this.emit()
  }

  private detachClient(): void {
    if (this.commandTimer >= 0) clearTimeout(this.commandTimer)
    this.commandTimer = -1
    if (this.client === null) {
      return
    }
    this.client.setObserver(null)
    this.client.close()
    this.client = null
  }

  private attachClient(pairing: PairingPayload): void {
    const client = new ProtocolClient(pairing)
    client.setObserver(this.observer)
    this.client = client
    client.connect()
  }

  private start(pairing: PairingPayload): void {
    if (this.state.pendingCommand !== null) {
      const id = this.state.pendingCommand.commandId
      this.sendListener?.(false)
      this.dispatch(commandUnknown(id))
      this.pendingAttachments = []
    }
    this.detachClient()
    this.dispatch(startConnect(pairing))
    this.attachClient(pairing)
  }
}

export function createSessionStore(): HarmonySessionStore {
  return new SessionStore()
}
