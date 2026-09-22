import {
  ProtocolClient,
  type ActionResolution,
  type ClientEvent,
  type CloseInfo,
  type SendResult,
  type SocketFactory,
} from '../protocol/client'
import type { PairingPayload } from '../protocol/pairing'
import type { PermissionMode } from '../protocol/types'
import {
  actionResolutionSent,
  commandAccepted,
  commandRejected,
  commandSent,
  commandUnknown,
  connectionFailed,
  connectionStateChanged,
  eventReceived,
  noticeReceived,
  permissionModeSent,
  queuedCancelSent,
  sessionOpened,
  snapshotReceived,
  turnCancelSent,
  type Action,
} from './reducer'
import type { AppStore } from './appStore'

interface RuntimeEntry {
  payloadKey: string
  generation: number
  client: ProtocolClient
}

let commandCounter = 0
function generateCommandId(): string {
  if (globalThis.crypto?.randomUUID !== undefined) return globalThis.crypto.randomUUID()
  commandCounter += 1
  return `cmd-${commandCounter}`
}

function payloadKey(payload: PairingPayload): string {
  return JSON.stringify([
    payload.version,
    payload.wsUrl,
    payload.sessionId,
    payload.pairToken,
    payload.expiresAt,
  ])
}

export class SessionRuntimeManager {
  private readonly entries = new Map<string, RuntimeEntry>()
  private readonly generations = new Map<string, number>()
  private readonly store: AppStore
  private readonly socketFactory: SocketFactory
  private readonly now: () => number

  constructor(
    store: AppStore,
    socketFactory: SocketFactory,
    now: () => number,
  ) {
    this.store = store
    this.socketFactory = socketFactory
    this.now = now
  }

  ensureConnected(payload: PairingPayload): void {
    const key = payloadKey(payload)
    const existing = this.entries.get(payload.sessionId)
    if (existing?.payloadKey === key) return
    if (existing !== undefined) this.closeClient(payload.sessionId, existing)

    const generation = (this.generations.get(payload.sessionId) ?? 0) + 1
    this.generations.set(payload.sessionId, generation)
    const client = new ProtocolClient(payload, this.socketFactory)
    const entry: RuntimeEntry = { payloadKey: key, generation, client }
    this.entries.set(payload.sessionId, entry)
    const current = this.store.getState().sessions[payload.sessionId] ?? null
    this.store.getState().dispatchSession(payload.sessionId, sessionOpened(current))
    this.store.getState().setPairingInvalid(payload.sessionId, false)

    client.onEvent = (event) => {
      if (!this.isCurrent(payload.sessionId, generation, client)) return
      this.handleEvent(payload.sessionId, event)
    }
    client.onClose = (info) => {
      if (!this.isCurrent(payload.sessionId, generation, client)) return
      this.handleClose(payload.sessionId, info)
    }
    client.connect()
  }

  reconnect(sessionId: string): void {
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return
    const current = this.store.getState().sessions[sessionId] ?? null
    this.store.getState().dispatchSession(sessionId, sessionOpened(current))
    entry.client.reconnect()
  }

  closeSession(sessionId: string): void {
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return
    this.closeClient(sessionId, entry)
  }

  closeAll(): void {
    for (const [sessionId, entry] of [...this.entries]) {
      this.closeClient(sessionId, entry)
    }
  }

  refreshConnectionStates(): void {
    for (const [sessionId, entry] of this.entries) {
      this.store.getState().dispatchSession(
        sessionId,
        connectionStateChanged(entry.client.connectionState()),
      )
    }
  }

  sendText(sessionId: string, text: string): boolean {
    return this.send(sessionId, (client, commandId) => client.sendText(text, commandId),
      (commandId) => commandSent({ commandId, text }))
  }

  setPermissionMode(sessionId: string, permissionMode: PermissionMode): boolean {
    return this.send(
      sessionId,
      (client, commandId) => client.setPermissionMode(permissionMode, commandId),
      (commandId) => permissionModeSent({ commandId, permissionMode }),
    )
  }

  resolveAction(sessionId: string, resolution: ActionResolution): boolean {
    return this.send(
      sessionId,
      (client, commandId) => client.resolveAction(resolution, commandId),
      (commandId) => actionResolutionSent({ commandId, actionId: resolution.actionId }),
    )
  }

  cancelTurn(sessionId: string, requestId: string): boolean {
    return this.send(
      sessionId,
      (client, commandId) => client.cancelTurn(requestId, commandId),
      (commandId) => turnCancelSent({ commandId, requestId }),
    )
  }

  cancelQueued(sessionId: string, requestId: string): boolean {
    return this.send(
      sessionId,
      (client, commandId) => client.cancelQueued(requestId, commandId),
      (commandId) => queuedCancelSent({ commandId, requestId }),
    )
  }

  private send(
    sessionId: string,
    transmit: (client: ProtocolClient, commandId: string) => SendResult,
    action: (commandId: string) => Action,
  ): boolean {
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return false
    const commandId = generateCommandId()
    const result = transmit(entry.client, commandId)
    if (result.ok) this.store.getState().dispatchSession(sessionId, action(commandId))
    return result.ok
  }

  private handleEvent(sessionId: string, event: ClientEvent): void {
    switch (event.kind) {
      case 'snapshot':
        this.store.getState().dispatchSession(sessionId, snapshotReceived(event.snapshot))
        this.store.getState().markPairingStatus(sessionId, 'connected', this.now())
        return
      case 'event':
        this.store.getState().dispatchSession(sessionId, eventReceived(event.event), true)
        return
      case 'commandResult':
        if (event.accepted && event.requestId !== '') {
          this.store.getState().dispatchSession(sessionId, commandAccepted({
            commandId: event.commandId,
            requestId: event.requestId,
          }))
        } else if (event.accepted) {
          this.store.getState().dispatchSession(sessionId, connectionFailed({ kind: 'protocolError' }))
        } else {
          this.store.getState().dispatchSession(sessionId, commandRejected({
            commandId: event.commandId,
            code: event.code,
            message: event.message,
          }))
        }
        return
      case 'connectionStateChanged':
        this.store.getState().dispatchSession(sessionId, connectionStateChanged(event.connection))
        return
      case 'unsupported':
        this.store.getState().dispatchSession(sessionId, noticeReceived(event.code, event.seq), true)
    }
  }

  private handleClose(sessionId: string, info: CloseInfo): void {
    const kind = info.kind === 'protocolError' ? 'protocolError' : 'disconnected'
    this.store.getState().dispatchSession(sessionId, connectionFailed({ kind }))
    if (info.pendingCommandId !== null) {
      this.store.getState().dispatchSession(sessionId, commandUnknown({ commandId: info.pendingCommandId }))
    }
    if (info.kind === 'pairingInvalid') {
      this.store.getState().setPairingInvalid(sessionId, true)
      this.store.getState().markPairingStatus(sessionId, 'needsPairing')
    } else {
      this.store.getState().markPairingStatus(sessionId, 'disconnected')
    }
  }

  private closeClient(sessionId: string, entry: RuntimeEntry): void {
    if (this.entries.get(sessionId) !== entry) return
    this.entries.delete(sessionId)
    this.generations.set(sessionId, entry.generation + 1)
    entry.client.onEvent = null
    entry.client.onClose = null
    entry.client.close()
  }

  private isCurrent(sessionId: string, generation: number, client: ProtocolClient): boolean {
    const entry = this.entries.get(sessionId)
    return entry?.generation === generation && entry.client === client
  }
}
