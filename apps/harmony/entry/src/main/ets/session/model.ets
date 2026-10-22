import type {
  Attachment,
  CommandKind,
  ContentBlock,
  ErrorInfo,
  InputSource,
  PairingPayload,
  PendingAction,
  PermissionMode,
  QueuedRequest,
  ResolvedAnswer,
  SessionStatus,
  UnsupportedCode,
} from '../protocol/types'

export enum ConnectionStage {
  CONNECTING = 'CONNECTING',
  AUTHENTICATING = 'AUTHENTICATING',
  SUBSCRIBING = 'SUBSCRIBING',
  SUBSCRIBED = 'SUBSCRIBED',
  READY = 'READY',
  DISCONNECTED = 'DISCONNECTED',
  PAIRING_INVALID = 'PAIRING_INVALID',
  PROTOCOL_ERROR = 'PROTOCOL_ERROR',
}

export type ClaudeMessageStatus = 'streaming' | 'done' | 'failed' | 'cancelled'
export type InputMessageStatus = 'accepted' | 'unknown' | ''

export interface ClaudeFeedMessage {
  kind: 'claude'
  requestId: string
  source: InputSource
  status: ClaudeMessageStatus
  text: string
  error: ErrorInfo | null
}

export interface InputFeedMessage {
  kind: 'input'
  attachments?: Array<Attachment>
  source: InputSource
  requestId: string
  text: string
  status: InputMessageStatus
}

export interface BlockFeedMessage {
  kind: 'block'
  requestId: string
  block: ContentBlock
}

export interface AnswerFeedMessage {
  kind: 'answer'
  requestId: string
  answers: Array<ResolvedAnswer>
}

export type FeedMessage =
  | ClaudeFeedMessage
  | InputFeedMessage
  | BlockFeedMessage
  | AnswerFeedMessage

export interface PendingCommand {
  commandId: string
  kind: CommandKind
  text: string
  targetId: string
}

export interface NoticeInfo {
  code: UnsupportedCode
  message: string
}

export interface HarmonySessionState {
  permissionMode: PermissionMode
  pairing: PairingPayload | null
  connection: ConnectionStage
  sessionId: string
  projectCwd?: string
  sessionTitle?: string
  createdAt?: string
  lastActivityAt?: string
  sessionStatus: SessionStatus | ''
  sessionError: ErrorInfo | null
  seqBase: number
  queueDepthHint: number
  queuedRequests: Array<QueuedRequest>
  pendingAction: PendingAction | null
  pendingCommand: PendingCommand | null
  messages: Array<FeedMessage>
  lastError: ErrorInfo | null
  lastNotice: NoticeInfo | null
  canSend: boolean
  canResolveAction: boolean
}

export function createInitialHarmonySessionState(): HarmonySessionState {
  return {
    permissionMode: 'default',
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
}
