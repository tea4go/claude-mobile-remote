export const PROTOCOL_VERSION = 2
export const PREFERRED_PROTOCOL_VERSION = 4
export const PAIRING_PAYLOAD_VERSION = 1
export const ASK_USER_QUESTION_TOOL_NAME = 'AskUserQuestion'
export const MAX_TEXT_BYTES = 1024 * 1024

export const SESSION_STATUS_VALUES: string[] = ['starting', 'ready', 'stopped', 'failed']
export const TURN_STATUS_VALUES: string[] = [
  'idle',
  'running',
  'waiting',
  'completed',
  'cancelled',
  'failed',
]
export const INPUT_SOURCE_VALUES: string[] = ['terminal', 'mobile']
export const PERMISSION_MODE_VALUES: string[] = ['default', 'acceptEdits', 'plan', 'bypassPermissions']
export type PermissionMode = 'default' | 'acceptEdits' | 'plan' | 'bypassPermissions'
export const EVENT_KIND_VALUES: string[] = [
  'session_started',
  'turn_started',
  'text_delta',
  'turn_completed',
  'turn_failed',
  'session_failed',
  'session_stopped',
  'content_block',
  'action_required',
  'action_resolved',
  'permission_mode_changed',
]
export const BLOCK_KIND_VALUES: string[] = ['thinking', 'tool_use', 'tool_result']
export const TOOL_CLASS_VALUES: string[] = ['builtin', 'mcp']
export const ACTION_KIND_VALUES: string[] = ['tool_approval', 'ask_user_question']
export const QUESTION_INPUT_TYPE_VALUES: string[] = ['choice', 'text']
export const MESSAGE_TYPE_VALUES: string[] = [
  'hello',
  'hello_ack',
  'subscribe',
  'subscribed',
  'unsubscribe',
  'unsubscribed',
  'snapshot',
  'event',
  'unsupported',
  'command',
  'command_result',
  'create_session',
  'create_session_result',
  'stop_session',
  'stop_session_result',
  'error',
]
export const SERVER_MESSAGE_TYPE_VALUES: string[] = [
  'hello_ack',
  'subscribed',
  'unsubscribed',
  'snapshot',
  'event',
  'unsupported',
  'command_result',
  'create_session_result',
  'stop_session_result',
  'error',
]
export const UNSUPPORTED_CODE_VALUES: string[] = [
  'unknown_block',
  'unknown_subtype',
  'concurrent_action',
  'text_input_disabled',
]

export type SessionStatus = 'starting' | 'ready' | 'stopped' | 'failed'
export type TurnStatus = 'idle' | 'running' | 'waiting' | 'completed' | 'cancelled' | 'failed'
export type InputSource = 'terminal' | 'mobile'
export type EventKind =
  | 'session_started'
  | 'turn_started'
  | 'text_delta'
  | 'turn_completed'
  | 'turn_failed'
  | 'session_failed'
  | 'session_stopped'
  | 'content_block'
  | 'action_required'
  | 'action_resolved'
  | 'permission_mode_changed'
export type BlockKind = 'thinking' | 'tool_use' | 'tool_result'
export type ToolClass = 'builtin' | 'mcp'
export type PendingActionKind = 'tool_approval' | 'ask_user_question'
export type QuestionInputType = 'choice' | 'text'
export type MessageType =
  | 'hello'
  | 'hello_ack'
  | 'subscribe'
  | 'subscribed'
  | 'unsubscribe'
  | 'unsubscribed'
  | 'snapshot'
  | 'event'
  | 'unsupported'
  | 'command'
  | 'command_result'
  | 'create_session'
  | 'create_session_result'
  | 'stop_session'
  | 'stop_session_result'
  | 'error'
export type ServerMessageType =
  | 'hello_ack'
  | 'subscribed'
  | 'unsubscribed'
  | 'snapshot'
  | 'event'
  | 'unsupported'
  | 'command_result'
  | 'create_session_result'
  | 'stop_session_result'
  | 'error'
export type UnsupportedCode =
  | 'unknown_block'
  | 'unknown_subtype'
  | 'concurrent_action'
  | 'text_input_disabled'
export type ActionDecision = 'allow_once' | 'allow_session' | 'deny'
export type CommandKind = 'send_attachments' | 'send_text' | 'resolve_action' | 'cancel_turn' | 'cancel_queued' | 'set_permission_mode'

export interface PairingPayload {
  version: number
  wsUrl: string
  sessionId: string
  pairToken: string
  expiresAt: string
  capabilities?: string[]
  pcId?: string
  pcName?: string
}

export interface ErrorInfo {
  code: string
  message: string
}

export interface QuestionOption {
  label: string
  description: string
}

export interface Question {
  questionId: string
  header: string
  question: string
  inputType: QuestionInputType
  multiSelect: boolean
  options: QuestionOption[]
}

export interface QuestionAnswer {
  questionId: string
  values: string[]
}

export interface ToolApprovalPendingAction {
  actionId: string
  requestId: string
  kind: 'tool_approval'
  toolName: string
  preview: string
  allowSessionAvailable: boolean
}

export interface AskUserQuestionPendingAction {
  actionId: string
  requestId: string
  kind: 'ask_user_question'
  toolName: 'AskUserQuestion'
  allowSessionAvailable: boolean
  questions: Question[]
}

export type PendingAction = ToolApprovalPendingAction | AskUserQuestionPendingAction

export interface ResolvedAnswer {
  questionId: string
  header: string
  question: string
  values: string[]
}

export interface QueuedRequest {
  requestId: string
  source: InputSource
  preview: string
}

export interface UnsupportedPayload {
  code: UnsupportedCode
}

export interface ThinkingContentBlock {
  id: string
  kind: 'thinking'
  thinking: string
}

export interface ToolUseContentBlock {
  id: string
  kind: 'tool_use'
  toolUseId: string
  toolClass: ToolClass
  toolName: string
  preview?: string
  mcpServer: string
}

export interface ToolResultContentBlock {
  id: string
  kind: 'tool_result'
  output?: string
  truncated?: boolean
  toolUseId: string
  isError: boolean
}

export type ContentBlock = ThinkingContentBlock | ToolUseContentBlock | ToolResultContentBlock

export interface TurnSnapshot {
  status: TurnStatus
  requestId: string
  source: InputSource | ''
  message: string
  error: ErrorInfo | null
  pendingAction: PendingAction | null
}

export interface SessionSnapshot {
  permissionMode?: PermissionMode
  sessionId: string
  sessionStatus: SessionStatus
  sessionError: ErrorInfo | null
  seq: number
  turn: TurnSnapshot
  queueDepth: number
  queuedRequests: QueuedRequest[]
  projectCwd?: string
  sessionTitle?: string
  createdAt?: string
  lastActivityAt?: string
}

export interface DomainEvent {
  permissionMode?: PermissionMode
  seq: number
  kind: EventKind
  sessionId: string
  requestId: string
  source: InputSource | ''
  text: string
  sessionStatus: SessionStatus | ''
  turnStatus: TurnStatus | ''
  error: ErrorInfo | null
  block?: ContentBlock
  pendingAction?: PendingAction
  answers?: ResolvedAnswer[]
  sessionTitle?: string
  lastActivityAt?: string
}

export interface EmptyPayload {}

export interface HelloPayload {
  pairToken: string
}

export interface SnapshotPayload {
  connectionStatus: string
  snapshot: SessionSnapshot
}

export interface EventPayload {
  event: DomainEvent
}

export interface CommandResultPayload {
  accepted: boolean
  code: string
  message: string
}

export interface ManagementResultPayload {
  accepted: boolean
  code: string
  message: string
  pairing?: PairingPayload
}

export interface ErrorPayload {
  code: string
  message: string
  expectedVersion?: number
}

export interface BaseServerMessage {
  version: number
  type: ServerMessageType
  sessionId: string
  commandId: string
  requestId: string
  seq: number
}

export interface HelloAckServerMessage extends BaseServerMessage {
  type: 'hello_ack'
  payload: EmptyPayload
}

export interface SubscribedServerMessage extends BaseServerMessage {
  type: 'subscribed'
  payload: EmptyPayload
}

export interface UnsubscribedServerMessage extends BaseServerMessage {
  type: 'unsubscribed'
  payload: EmptyPayload
}

export interface SnapshotServerMessage extends BaseServerMessage {
  type: 'snapshot'
  payload: SnapshotPayload
}

export interface EventServerMessage extends BaseServerMessage {
  type: 'event'
  payload: EventPayload
}

export interface UnsupportedServerMessage extends BaseServerMessage {
  type: 'unsupported'
  payload: UnsupportedPayload
}

export interface CommandResultServerMessage extends BaseServerMessage {
  type: 'command_result'
  payload: CommandResultPayload
}

export interface CreateSessionResultServerMessage extends BaseServerMessage {
  type: 'create_session_result'
  payload: ManagementResultPayload
}

export interface StopSessionResultServerMessage extends BaseServerMessage {
  type: 'stop_session_result'
  payload: ManagementResultPayload
}

export interface ErrorServerMessage extends BaseServerMessage {
  type: 'error'
  payload: ErrorPayload
}

export type ServerMessage =
  | HelloAckServerMessage
  | SubscribedServerMessage
  | UnsubscribedServerMessage
  | SnapshotServerMessage
  | EventServerMessage
  | UnsupportedServerMessage
  | CommandResultServerMessage
  | CreateSessionResultServerMessage
  | StopSessionResultServerMessage
  | ErrorServerMessage

export interface Attachment {
  name: string
  mediaType: string
  data: string
}
