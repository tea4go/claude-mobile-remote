// 版本 1 协议类型，字段与 A4 internal/transport/protocol.go 逐字段对应。
// 组件不得复制消息定义，一律从本文件导入。

export const PROTOCOL_VERSION = 1

export const MAX_TEXT_BYTES = 1024 * 1024

// ---- 领域枚举（A3 internal/session/model.go、event.go）----

export type SessionStatus = 'starting' | 'ready' | 'stopped' | 'failed'

export type TurnStatus =
  | 'idle'
  | 'running'
  | 'waiting'
  | 'completed'
  | 'cancelled'
  | 'failed'

export type InputSource = 'terminal' | 'mobile'

export type EventKind =
  | 'session_started'
  | 'turn_started'
  | 'text_delta'
  | 'turn_completed'
  | 'turn_failed'
  | 'session_failed'
  | 'session_stopped'

// ---- envelope（A4 internal/transport/protocol.go）----

export type MessageType =
  | 'hello'
  | 'hello_ack'
  | 'subscribe'
  | 'subscribed'
  | 'unsubscribe'
  | 'unsubscribed'
  | 'snapshot'
  | 'event'
  | 'command'
  | 'command_result'
  | 'error'

// 消息方向表（A4 设计 §5）：客户端→服务端、服务端→客户端。
export const CLIENT_TO_SERVER_TYPES: ReadonlySet<string> = new Set([
  'hello',
  'subscribe',
  'unsubscribe',
  'command',
])

export const SERVER_TO_CLIENT_TYPES: ReadonlySet<string> = new Set([
  'hello_ack',
  'subscribed',
  'unsubscribed',
  'snapshot',
  'event',
  'command_result',
  'error',
])

export interface ErrorInfo {
  code: string
  message: string
}

export interface PendingAction {
  // A3 的 PendingAction 目前为空对象；保留字段以匹配 JSON 形状。
  [key: string]: never
}

export interface TurnSnapshot {
  status: TurnStatus
  requestId: string
  // 首轮前（A5.2）尚无 Turn，source 为空串；一旦 requestId 非空则必非空。
  source: InputSource | ''
  message: string
  error: ErrorInfo | null
  pendingAction: PendingAction | null
}

export interface SessionSnapshot {
  sessionId: string
  sessionStatus: SessionStatus
  sessionError: ErrorInfo | null
  seq: number
  turn: TurnSnapshot
  queueDepth: number
}

// 领域事件：按 kind 部分携带字段，其余枚举在线上为空字符串（Go 零值
// 序列化）。kind 语义校验由 decoder 的 validateEventSemantics 负责。
export interface DomainEvent {
  seq: number
  kind: EventKind
  sessionId: string
  requestId: string
  source: InputSource | ''
  text: string
  sessionStatus: SessionStatus | ''
  turnStatus: TurnStatus | ''
  error: ErrorInfo | null
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

export interface ErrorPayload {
  code: string
  message: string
}

export type CommandKind = 'send_text'

export interface CommandPayload {
  kind: CommandKind
  text: string
}

// 服务端可发送给客户端的消息：envelope 关键字段与已验证的 payload 绑定。
export type ServerMessage = {
  sessionId: string
  commandId: string
  requestId: string
  seq: number
} & (
  | { type: 'hello_ack' | 'subscribed' | 'unsubscribed'; payload: Record<string, never> }
  | { type: 'snapshot'; payload: SnapshotPayload }
  | { type: 'event'; payload: EventPayload }
  | { type: 'command_result'; payload: CommandResultPayload }
  | { type: 'error'; payload: ErrorPayload }
)
