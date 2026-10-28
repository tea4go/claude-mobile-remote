import { recordHostDiagnostic } from '../debug/log'
import {
  ProtocolClient,
  type ActionResolution,
  type ClientEvent,
  type CloseInfo,
  type SendResult,
  type SocketFactory,
} from '../protocol/client'
import type { PairingPayload } from '../protocol/pairing'
import type { Attachment, AttachmentSendResult, PermissionMode } from '../protocol/types'
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

import { ReminderPolicy, type ReminderKind } from '../native/experience'

interface RuntimeEntry {
  payloadKey: string
  generation: number
  client: ProtocolClient
}

interface PendingManagement {
  operation: 'create_session' | 'stop_session'
  commandId: string
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
    payload.capabilities,
  ])
}

export class SessionRuntimeManager {
  private readonly attachmentResults = new Map<string, { commandId: string; resolve: (result: AttachmentSendResult) => void; timer: ReturnType<typeof setTimeout> }>()
  private finishAttachment(sessionId: string, result: AttachmentSendResult, commandId?: string): void {
    const pending = this.attachmentResults.get(sessionId)
    if (pending === undefined || (commandId !== undefined && pending.commandId !== commandId)) return
    clearTimeout(pending.timer)
    this.attachmentResults.delete(sessionId)
    pending.resolve(result)
  }
  private readonly reminders = new ReminderPolicy()
  onReminder: ((sessionId: string, seq: number, kind: ReminderKind) => void) | null = null
  viewedSession: () => string | null = () => null
  private readonly entries = new Map<string, RuntimeEntry>()
  private readonly generations = new Map<string, number>()
  private readonly store: AppStore
  private readonly socketFactory: SocketFactory
  private readonly now: () => number
  private readonly pendingManagement = new Map<string, PendingManagement>()
  private onSessionCreated: ((payload: PairingPayload) => void) | null = null
  private onManagementResult: ((sessionId: string, operation: PendingManagement['operation'], accepted: boolean, code: string) => void) | null = null

  setManagementCallbacks(
    onSessionCreated: ((payload: PairingPayload) => void) | null,
    onManagementResult: ((sessionId: string, operation: PendingManagement['operation'], accepted: boolean, code: string) => void) | null,
  ): void {
    this.onSessionCreated = onSessionCreated
    this.onManagementResult = onManagementResult
  }

  setReminderCallbacks(
    onReminder: SessionRuntimeManager['onReminder'],
    viewedSession: () => string | null = () => null,
  ): void {
    this.onReminder = onReminder
    this.viewedSession = viewedSession
  }

  constructor(
    store: AppStore,
    socketFactory: SocketFactory,
    now: () => number,
  ) {
    this.store = store
    this.socketFactory = socketFactory
    this.now = now
  }

  ensureConnected(payload: PairingPayload, verified?: ProtocolClient): void {
    const key = payloadKey(payload)
    const existing = this.entries.get(payload.sessionId)
    if (existing?.payloadKey === key && verified === undefined) {
      existing.client.checkConnection(true)
      if (existing.client.connectionState() === 'disconnected') this.reconnect(payload.sessionId)
      return
    }
    if (existing !== undefined) this.closeClient(payload.sessionId, existing)

    const generation = (this.generations.get(payload.sessionId) ?? 0) + 1
    this.generations.set(payload.sessionId, generation)
    const client = verified ?? new ProtocolClient(payload, this.socketFactory)
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
    if (verified === undefined) client.connect()
  }

  adoptVerified(payload: PairingPayload, client: ProtocolClient, snapshot: import('../protocol/types').SessionSnapshot, bufferedEvents: ClientEvent[] = []): void {
    this.ensureConnected(payload, client)
    this.handleEvent(payload.sessionId, { kind: 'snapshot', snapshot })
    for (const event of bufferedEvents) this.handleEvent(payload.sessionId, event)
  }

  reconnect(sessionId: string): void {
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return
    const current = this.store.getState().sessions[sessionId] ?? null
    this.store.getState().dispatchSession(sessionId, sessionOpened(current))
    entry.client.reconnect()
  }

  closeSession(sessionId: string): void {
    this.reminders.remove(sessionId)
	this.pendingManagement.delete(sessionId)
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return
    this.closeClient(sessionId, entry)
  }

  closeAll(): void {
	this.pendingManagement.clear()
    for (const [sessionId, entry] of [...this.entries]) {
      this.closeClient(sessionId, entry)
    }
  }

  networkOffline(): void {
    for (const entry of this.entries.values()) entry.client.networkOffline()
  }

  refreshConnectionStates(): void {
    for (const [sessionId, entry] of this.entries) {
      entry.client.checkConnection(true)
      this.store.getState().dispatchSession(
        sessionId,
        connectionStateChanged(entry.client.connectionState()),
      )
    }
  }

  sendAttachments(sessionId: string, text: string, attachments: Attachment[]): Promise<AttachmentSendResult> {
    if (this.attachmentResults.has(sessionId)) return Promise.resolve('rejected')
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return Promise.resolve('rejected')
    const commandId = generateCommandId()
    return new Promise((resolve) => {
      const timer = setTimeout(() => this.finishAttachment(sessionId, 'unknown', commandId), 30000)
      this.attachmentResults.set(sessionId, { commandId, resolve, timer })
      try {
        const result = entry.client.sendAttachments(text, attachments, commandId)
        if (!result.ok) { this.finishAttachment(sessionId, 'rejected', commandId); return }
        this.store.getState().dispatchSession(sessionId, commandSent({ commandId, text, attachments }))
      } catch { this.finishAttachment(sessionId, 'unknown', commandId) }
    })
  }

  sendText(sessionId: string, text: string): boolean {
    return this.send(sessionId, (client, commandId) => client.sendText(text, commandId),
      (commandId) => commandSent({ commandId, text }))
  }

  createSession(sessionId: string): boolean {
    return this.sendManagement(sessionId, 'create_session')
  }

  stopSession(sessionId: string): boolean {
    return this.sendManagement(sessionId, 'stop_session')
  }

  private sendManagement(sessionId: string, operation: PendingManagement['operation']): boolean {
    if (this.pendingManagement.has(sessionId)) return false
    const entry = this.entries.get(sessionId)
    if (entry === undefined) return false
    const commandId = generateCommandId()
    const result = operation === 'create_session'
      ? entry.client.createSession(commandId)
      : entry.client.stopSession(commandId)
    if (result.ok) this.pendingManagement.set(sessionId, { operation, commandId })
    return result.ok
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
        this.reminders.snapshot(sessionId, event.snapshot.seq)
        this.store.getState().dispatchSession(sessionId, snapshotReceived(event.snapshot))
        this.store.getState().markPairingStatus(sessionId, 'connected', this.now())
        {
          const pending = this.pendingManagement.get(sessionId)
          const entry = this.entries.get(sessionId)
          if (pending !== undefined && entry !== undefined) {
            if (pending.operation === 'create_session') entry.client.createSession(pending.commandId)
            else entry.client.stopSession(pending.commandId)
          }
        }
        return
      case 'managementResult': {
        const pending = this.pendingManagement.get(sessionId)
        if (pending === undefined || pending.commandId !== event.commandId || pending.operation !== event.operation) return
        this.pendingManagement.delete(sessionId)
        if (event.accepted && event.operation === 'create_session' && event.pairing !== undefined) {
          this.onSessionCreated?.(event.pairing)
        }
        this.onManagementResult?.(sessionId, event.operation, event.accepted, event.code)
        return
      }
      case 'event': {
        const domain = event.event
        // Log only accepted sequence entries, with no session/request IDs or error text.
        const state = this.store.getState().sessions[sessionId]
        const context = this.entries.get(sessionId)?.client.debugContext()
        if (state && domain.seq === state.seqBase + 1) {
          if (domain.diagnostic) recordHostDiagnostic(domain.diagnostic, context)
          else if (domain.kind === 'turn_failed' || domain.kind === 'session_failed') recordHostDiagnostic({ code: 'turn_failed' }, context)
          else if (domain.kind === 'content_block' && domain.block?.kind === 'tool_result' && domain.block.isError) recordHostDiagnostic({ code: 'tool_failed' }, context)
        }
        const reminder = this.reminders.event(event.event, this.viewedSession())
        if (reminder !== null) this.onReminder?.(sessionId, event.event.seq, reminder)
        this.store.getState().dispatchSession(sessionId, eventReceived(event.event), true)
        return
      }
      case 'commandResult':
        this.finishAttachment(sessionId, event.accepted ? 'accepted' : 'rejected', event.commandId)
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
    const management = this.pendingManagement.get(sessionId)
    if (management !== undefined) this.onManagementResult?.(sessionId, management.operation, false, 'result_unknown')
    this.finishAttachment(sessionId, 'unknown')
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
    this.finishAttachment(sessionId, 'unknown')
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
