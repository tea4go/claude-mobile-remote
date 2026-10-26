// v2–v6 协议类型，字段与 internal/transport/protocol.go 对应。
// 组件不得复制消息定义，一律从本文件导入。

export const PROTOCOL_VERSION = 2
export const PREFERRED_PROTOCOL_VERSION = 6

export const MAX_TEXT_BYTES = 1024 * 1024

// AskUserQuestion 是唯一以问答形态出现的工具名（internal/session/model.go
// 的 AskUserQuestionToolName）。
export const ASK_USER_QUESTION_TOOL_NAME = 'AskUserQuestion'

// ---- 领域枚举（B2 internal/session/model.go、event.go）----

export type SessionStatus = 'starting' | 'ready' | 'stopped' | 'failed'

export type TurnStatus =
  | 'idle'
  | 'running'
  | 'waiting'
  | 'completed'
  | 'cancelled'
  | 'failed'

export type PermissionMode = 'default' | 'acceptEdits' | 'plan' | 'bypassPermissions'

export type InputSource = 'terminal' | 'mobile'

// unsupported 不是事件 kind：它走独立的 type=unsupported envelope。
export type EventKind =
  | 'turn_progress'
  | 'session_diagnostic'
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

// ---- 结构化内容块（B2 §4.1）----

export type BlockKind = 'thinking' | 'tool_use' | 'tool_result'

export type ToolClass = 'builtin' | 'mcp'

// 按 kind 携带不相交的字段子集；服务端对多余字段一律拒绝。
export type ContentBlock =
  | { id: string; kind: 'thinking'; thinking: string }
  | {
      id: string
      kind: 'tool_use'
      toolUseId: string
      toolClass: ToolClass
      toolName: string
      // 服务端按工具白名单抽取的有限长度展示摘要，不是原始 input。
      preview?: string
      // builtin 工具必须为空串，mcp 工具必须非空。
      mcpServer: string
    }
  | { id: string; kind: 'tool_result'; toolUseId: string; isError: boolean; output?: string; truncated?: boolean }

// ---- 待处理事项（B2 §4.2）----

export type ActionKind = 'tool_approval' | 'ask_user_question'

export type QuestionInputType = 'choice' | 'text'

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
  // choice 必须非空；text 必须为空。
  options: QuestionOption[]
}

// pendingAction 只携带展示所需字段：不含工具原始 input、toolClass 或 toolUseId。
export type PendingAction =
  | {
      actionId: string
      requestId: string
      kind: 'tool_approval'
      toolName: string
      // 白名单抽取的单行预览；未命中白名单时为空串，客户端不得自行拼接。
      preview: string
      allowSessionAvailable: boolean
    }
  | {
      actionId: string
      requestId: string
      kind: 'ask_user_question'
      toolName: typeof ASK_USER_QUESTION_TOOL_NAME
      allowSessionAvailable: boolean
      questions: Question[]
    }

// action_resolved 回带的已校验答案（D52）：服务端把交给 SDK 的同一份答案原样回
// 显，任何端都能看到答了什么（包括别的端答的）。它不是工具结果，也不进快照。
export interface ResolvedAnswer {
  questionId: string
  header: string
  question: string
  values: string[]
}

// ---- 排队输入（B2 §4.3）----

export interface QueuedRequest {
  requestId: string
  source: InputSource
  preview: string
}

// ---- 不支持通知（B2 §4.4）----

export type UnsupportedCode =
  | 'unknown_block'
  | 'unknown_subtype'
  | 'concurrent_action'
  | 'text_input_disabled'

export interface UnsupportedPayload {
  code: UnsupportedCode
}

// ---- envelope（B2 internal/transport/protocol.go）----

export type MessageType =
  | 'ping'
  | 'pong'
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

// 消息方向表（A4 设计 §5、B2 §4.4）：客户端→服务端、服务端→客户端。
export const CLIENT_TO_SERVER_TYPES: ReadonlySet<string> = new Set([
  'ping',
  'hello',
  'subscribe',
  'unsubscribe',
  'command',
  'create_session',
  'stop_session',
])

export const SERVER_TO_CLIENT_TYPES: ReadonlySet<string> = new Set([
  'pong',
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
])

export interface ErrorInfo {
  code: string
  message: string
}

export interface TurnProgress {
 phase: 'waiting' | 'working' | 'thinking' | 'writing' | 'tool' | 'subtask' | 'compacting' | 'requesting' | 'retrying' | 'waiting_followup' | 'tool_result' | 'subtask_result'
 toolName?: string
 startedAt: number; updatedAt: number; outputTokens: number; tokensKnown: boolean; tokensEstimated: boolean
}
export interface SessionDiagnostic {
 code: 'api_retry' | 'query_failed' | 'receive_failed' | 'response_incomplete' | 'assistant_error' | 'result_error' | 'turn_failed' | 'tool_failed'
 httpStatus?: number; attempt?: number; maxAttempts?: number; retryDelayMs?: number
}
export interface TurnSnapshot {
 progress?: TurnProgress
  status: TurnStatus
  requestId: string
  // 首轮前（A5.2）尚无 Turn，source 为空串；一旦 requestId 非空则必非空。
  source: InputSource | ''
  message: string
  error: ErrorInfo | null
  // 非空时 status 必为 waiting，且 requestId 与 Turn 一致。
  pendingAction: PendingAction | null
}

export interface SessionSnapshot {
  sessionId: string
  sessionStatus: SessionStatus
  sessionError: ErrorInfo | null
  seq: number
  turn: TurnSnapshot
  // 与 queuedRequests.length 恒等；服务端空队列在线上是 null。
  queueDepth: number
  queuedRequests: QueuedRequest[]
  permissionMode?: PermissionMode
  projectCwd?: string
  sessionTitle?: string
  createdAt?: string
  lastActivityAt?: string
}

// 领域事件：按 kind 部分携带字段，其余枚举在线上为空字符串（Go 零值
// 序列化）。kind 语义校验由 decoder 的 validateEventSemantics 负责。
export interface DomainEvent {
 progress?: TurnProgress
 diagnostic?: SessionDiagnostic
  seq: number
  kind: EventKind
  sessionId: string
  requestId: string
  source: InputSource | ''
  text: string
  sessionStatus: SessionStatus | ''
  turnStatus: TurnStatus | ''
  error: ErrorInfo | null
  // 只有 content_block 携带 block，只有 action_required 携带 pendingAction，
  // 只有 action_resolved 携带 answers。
  block?: ContentBlock
  pendingAction?: PendingAction
  answers?: ResolvedAnswer[]
  permissionMode?: PermissionMode
  sessionTitle?: string
  lastActivityAt?: string
}

// 传输层 envelope。payload 在解码时按 type 严格校验，这里是未验证的原始值。
export type Envelope = {
  version: number
  type: MessageType
  sessionId: string
  commandId: string
  requestId: string
  seq: number
  payload: unknown
}

// ---- payload ----

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
  pairing?: import('./pairing').PairingPayload
}

export interface ErrorPayload {
  code: string
  message: string
  // 仅版本不匹配时携带，用于提示客户端需要的版本（B2 §4.5）。
  expectedVersion?: number
}

// ---- 命令（B2 §5）----

export type CommandKind =
  | 'send_text'
  | 'send_attachments'
  | 'resolve_action'
  | 'cancel_turn'
  | 'cancel_queued'
  | 'set_permission_mode'

export type ActionDecision = 'allow_once' | 'allow_session' | 'deny'

export interface QuestionAnswer {
  questionId: string
  values: string[]
}

// 每个 kind 的字段子集互不相交，与 Go validateCommand 一致。
export type CommandPayload =
  | { kind: 'send_text'; text: string }
  | { kind: 'send_attachments'; text: string; attachments: Attachment[] }
  | { kind: 'resolve_action'; actionId: string; decision: ActionDecision }
  | { kind: 'resolve_action'; actionId: string; answers: QuestionAnswer[] }
  | { kind: 'cancel_turn'; requestId: string }
  | { kind: 'cancel_queued'; requestId: string }
  | { kind: 'set_permission_mode'; permissionMode: PermissionMode }

// 服务端可发送给客户端的消息：envelope 关键字段与已验证的 payload 绑定。
export type ServerMessage = {
  version: number
  sessionId: string
  commandId: string
  requestId: string
  seq: number
} & (
  | { type: 'pong' | 'hello_ack' | 'subscribed' | 'unsubscribed'; payload: Record<string, never> }
  | { type: 'snapshot'; payload: SnapshotPayload }
  | { type: 'event'; payload: EventPayload }
  | { type: 'unsupported'; payload: UnsupportedPayload }
  | { type: 'command_result'; payload: CommandResultPayload }
  | { type: 'create_session_result' | 'stop_session_result'; payload: ManagementResultPayload }
  | { type: 'error'; payload: ErrorPayload }
)

export interface Attachment { name: string; mediaType: string; data: string }
export type AttachmentSendResult = 'accepted' | 'rejected' | 'unknown'
