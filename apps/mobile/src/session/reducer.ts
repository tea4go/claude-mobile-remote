// 单连接 UI 状态归并（D32/D33/D36）。纯函数，不依赖 React 运行时。

import type { DomainEvent, ErrorInfo, SessionSnapshot } from '../protocol/types'

export type ConnectionState =
  | 'connecting'
  | 'authenticating'
  | 'subscribing'
  | 'connected'
  | 'disconnected'

export type Phase =
  | 'unpaired'
  | 'connecting'
  | 'connected'
  | 'disconnected'
  | 'protocolError'

export interface ClaudeMessage {
  kind: 'claude'
  requestId: string
  source: 'terminal' | 'mobile'
  text: string
  status: 'streaming' | 'done' | 'failed'
  error: ErrorInfo | null
}

export interface MobileMessage {
  kind: 'mobile'
  requestId: string | null
  text: string
  status: 'accepted' | 'unknown'
}

export type Message = ClaudeMessage | MobileMessage

export interface PendingCommand {
  commandId: string
  text: string
}

export interface MobileState {
  phase: Phase
  connection: ConnectionState
  sessionId: string
  sessionStatus: 'starting' | 'ready' | 'stopped' | 'failed' | null
  sessionError: ErrorInfo | null
  seqBase: number
  queueDepthHint: number
  messages: Message[]
  pendingCommand: PendingCommand | null
  lastError: ErrorInfo | null
  canSend: boolean
}

export function createInitialState(): MobileState {
  return {
    phase: 'unpaired',
    connection: 'connecting',
    sessionId: '',
    sessionStatus: null,
    sessionError: null,
    seqBase: 0,
    queueDepthHint: 0,
    messages: [],
    pendingCommand: null,
    lastError: null,
    canSend: false,
  }
}

// ---- Action 构造器 ----

export function pairLoaded() {
  return { type: 'pairLoaded' } as const
}

export function snapshotReceived(snapshot: SessionSnapshot) {
  return { type: 'snapshotReceived', snapshot } as const
}

export function eventReceived(event: DomainEvent) {
  return { type: 'eventReceived', event } as const
}

export function connectionStateChanged(connection: ConnectionState) {
  return { type: 'connectionStateChanged', connection } as const
}

export function commandSent(info: { commandId: string; text: string }) {
  return { type: 'commandSent', ...info } as const
}

export function commandAccepted(info: { commandId: string; requestId: string }) {
  return { type: 'commandAccepted', ...info } as const
}

export function commandRejected(info: { commandId: string; code: string; message: string }) {
  return { type: 'commandRejected', ...info } as const
}

export function commandUnknown(info: { commandId: string }) {
  return { type: 'commandUnknown', ...info } as const
}

export function connectionFailed(info: { kind: 'disconnected' | 'protocolError' }) {
  return { type: 'connectionFailed', ...info } as const
}

export function reset() {
  return { type: 'reset' } as const
}

export type Action =
  | ReturnType<typeof pairLoaded>
  | ReturnType<typeof snapshotReceived>
  | ReturnType<typeof eventReceived>
  | ReturnType<typeof connectionStateChanged>
  | ReturnType<typeof commandSent>
  | ReturnType<typeof commandAccepted>
  | ReturnType<typeof commandRejected>
  | ReturnType<typeof commandUnknown>
  | ReturnType<typeof connectionFailed>
  | ReturnType<typeof reset>

// ---- 归并逻辑 ----

function canSendNow(state: MobileState): boolean {
  return (
    state.phase === 'connected' &&
    (state.sessionStatus === 'ready' || state.sessionStatus === 'starting') &&
    state.pendingCommand === null
  )
}

function applyCanSend(state: MobileState): MobileState {
  return { ...state, canSend: canSendNow(state) }
}

function markPendingUnknown(state: MobileState): MobileState {
  if (state.pendingCommand === null) return state
  const unknown: MobileMessage = {
    kind: 'mobile',
    requestId: null,
    text: state.pendingCommand.text,
    status: 'unknown',
  }
  return {
    ...state,
    pendingCommand: null,
    messages: [...state.messages, unknown],
  }
}

function applySnapshot(state: MobileState, snapshot: SessionSnapshot): MobileState {
  const next: MobileState = {
    ...state,
    phase: 'connected',
    connection: 'connected',
    sessionId: snapshot.sessionId,
    sessionStatus: snapshot.sessionStatus,
    sessionError: snapshot.sessionError,
    seqBase: snapshot.seq,
    queueDepthHint: snapshot.queueDepth,
    // 重连/刷新以新 snapshot 重新初始化，旧易失消息清空（D32）。
    messages: [],
    pendingCommand: null,
    lastError: null,
  }
  if (snapshot.turn.message !== '') {
    next.messages = [
      {
        kind: 'claude',
        requestId: snapshot.turn.requestId,
        source: snapshot.turn.source,
        text: snapshot.turn.message,
        status:
          snapshot.turn.status === 'failed'
            ? 'failed'
            : snapshot.turn.status === 'running' || snapshot.turn.status === 'waiting'
              ? 'streaming'
              : 'done',
        error: snapshot.turn.error,
      },
    ]
  }
  return applyCanSend(next)
}

function applyEvent(state: MobileState, event: DomainEvent): MobileState {
  if (event.seq !== state.seqBase + 1) {
    // 重复、倒退或缺口：状态不可信，按协议错误处理（§6.2）。
    return { ...markPendingUnknown(state), phase: 'protocolError', canSend: false }
  }
  let next: MobileState = { ...state, seqBase: event.seq }

  switch (event.kind) {
    case 'session_started':
      next.sessionStatus = event.sessionStatus
      next.sessionId = event.sessionId
      break
    case 'session_failed':
      next.sessionStatus = event.sessionStatus
      next.sessionError = event.error
      break
    case 'session_stopped':
      next.sessionStatus = event.sessionStatus
      break
    case 'turn_started':
      next.messages = [
        ...next.messages,
        {
          kind: 'claude',
          requestId: event.requestId,
          source: event.source,
          text: '',
          status: 'streaming',
          error: null,
        },
      ]
      break
    case 'text_delta': {
      const index = next.messages.findIndex(
        (m) => m.kind === 'claude' && m.requestId === event.requestId,
      )
      if (index >= 0) {
        const target = next.messages[index] as ClaudeMessage
        next.messages = [
          ...next.messages.slice(0, index),
          { ...target, text: target.text + event.text },
          ...next.messages.slice(index + 1),
        ]
      }
      break
    }
    case 'turn_completed':
    case 'turn_failed': {
      const index = next.messages.findIndex(
        (m) => m.kind === 'claude' && m.requestId === event.requestId,
      )
      if (index >= 0) {
        const target = next.messages[index] as ClaudeMessage
        next.messages = [
          ...next.messages.slice(0, index),
          {
            ...target,
            status: event.kind === 'turn_completed' ? 'done' : 'failed',
            error: event.error,
          },
          ...next.messages.slice(index + 1),
        ]
      }
      break
    }
  }
  return applyCanSend(next)
}

export function reducer(state: MobileState, action: Action): MobileState {
  switch (action.type) {
    case 'pairLoaded':
      return applyCanSend({
        ...state,
        phase: 'connecting',
        connection: 'connecting',
        messages: [],
        pendingCommand: null,
        lastError: null,
      })
    case 'snapshotReceived':
      return applySnapshot(state, action.snapshot)
    case 'eventReceived':
      return applyEvent(state, action.event)
    case 'connectionStateChanged':
      return { ...state, connection: action.connection }
    case 'commandSent':
      return applyCanSend({
        ...state,
        pendingCommand: { commandId: action.commandId, text: action.text },
      })
    case 'commandAccepted': {
      if (
        state.pendingCommand === null ||
        state.pendingCommand.commandId !== action.commandId
      ) {
        return state
      }
      const accepted: MobileMessage = {
        kind: 'mobile',
        requestId: action.requestId,
        text: state.pendingCommand.text,
        status: 'accepted',
      }
      return applyCanSend({
        ...state,
        pendingCommand: null,
        messages: [...state.messages, accepted],
      })
    }
    case 'commandRejected': {
      if (
        state.pendingCommand === null ||
        state.pendingCommand.commandId !== action.commandId
      ) {
        return state
      }
      return applyCanSend({
        ...state,
        pendingCommand: null,
        lastError: { code: action.code, message: action.message },
      })
    }
    case 'commandUnknown':
      if (
        state.pendingCommand === null ||
        state.pendingCommand.commandId !== action.commandId
      ) {
        return state
      }
      return applyCanSend(markPendingUnknown(state))
    case 'connectionFailed': {
      const marked = markPendingUnknown(state)
      if (action.kind === 'protocolError') {
        return { ...marked, phase: 'protocolError', canSend: false }
      }
      return applyCanSend({ ...marked, phase: 'disconnected', connection: 'disconnected' })
    }
    case 'reset':
      return createInitialState()
  }
}
