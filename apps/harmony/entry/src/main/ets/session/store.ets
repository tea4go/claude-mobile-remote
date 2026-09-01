import {
  ProtocolClient,
  type ActionResolution,
  type CloseInfo,
  type CommandResultInfo,
  type ProtocolClientObserver,
} from '../protocol/client'
import { decodePairingPayload } from '../protocol/pairing'
import type {
  CommandKind,
  DomainEvent,
  ErrorInfo,
  PairingPayload,
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

export interface HarmonySessionStore {
  connectFromRawPairing(raw: string): boolean
  connectWithPairing(pairing: PairingPayload): void
  disconnect(): void
  reset(): void
  reconnect(): boolean
  sendText(text: string): boolean
  resolveAction(resolution: ActionResolution): boolean
  cancelTurn(requestId: string): boolean
  cancelQueued(requestId: string): boolean
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

  onClosed(info: CloseInfo): void {
    this.owner.handleClosed(info)
  }
}

class SessionStore implements HarmonySessionStore {
  private state: HarmonySessionState = createInitialHarmonySessionState()
  private client: ProtocolClient | null = null
  private readonly observer: SessionStoreObserver
  private listeners: Array<StateListener> = []

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
    this.start(pairing)
  }

  disconnect(): void {
    this.detachClient()
    this.dispatch(socketClosed(makeError('closed', 'connection closed by user')))
  }

  reset(): void {
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
    return this.sendCommand('send_text', text, '', (commandId: string) => {
      if (this.client === null) {
        return false
      }
      return this.client.sendText(text, commandId).ok
    })
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
  }

  handleEvent(event: DomainEvent): void {
    this.dispatch(eventReceived(event))
  }

  handleUnsupported(code: UnsupportedCode, seq: number): void {
    this.dispatch(unsupportedReceived(code, seq))
  }

  handleCommandResult(result: CommandResultInfo): void {
    if (result.accepted) {
      this.dispatch(commandAccepted(result.commandId, result.requestId))
      return
    }
    this.dispatch(commandRejected(result.commandId, result.code, result.message))
  }

  handleClosed(info: CloseInfo): void {
    this.client = null
    if (info.pendingCommandId !== null) {
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
    this.state = reduceHarmonySessionState(this.state, action)
    if (this.state.connection === ConnectionStage.PROTOCOL_ERROR && this.client !== null) {
      this.detachClient()
    }
    this.emit()
  }

  private detachClient(): void {
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
    this.detachClient()
    this.dispatch(startConnect(pairing))
    this.attachClient(pairing)
  }
}

export function createSessionStore(): HarmonySessionStore {
  return new SessionStore()
}
