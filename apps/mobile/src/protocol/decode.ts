import type {
  CommandResultPayload,
  DomainEvent,
  ErrorInfo,
  EventKind,
  EventPayload,
  InputSource,
  MessageType,
  PendingAction,
  ServerMessage,
  SessionSnapshot,
  SessionStatus,
  SnapshotPayload,
  TurnSnapshot,
  TurnStatus,
} from './types'
import { MAX_TEXT_BYTES, PROTOCOL_VERSION, SERVER_TO_CLIENT_TYPES } from './types'

// 不可信网络 JSON 的运行时严格解码。TypeScript 类型不能替代校验：
// 未知版本、未知类型、方向不符或错误结构一律抛错，由调用方终止当前连接。

const SESSION_STATUSES: ReadonlySet<string> = new Set([
  'starting',
  'ready',
  'stopped',
  'failed',
])

const TURN_STATUSES: ReadonlySet<string> = new Set([
  'idle',
  'running',
  'waiting',
  'completed',
  'cancelled',
  'failed',
])

const INPUT_SOURCES: ReadonlySet<string> = new Set(['terminal', 'mobile'])

const EVENT_KINDS: ReadonlySet<string> = new Set([
  'session_started',
  'turn_started',
  'text_delta',
  'turn_completed',
  'turn_failed',
  'session_failed',
  'session_stopped',
])

class DecodeError extends Error {}

function fail(reason: string): never {
  throw new DecodeError(`protocol decode error: ${reason}`)
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function requireString(container: Record<string, unknown>, key: string): string {
  const value = container[key]
  if (typeof value !== 'string') fail(`field ${key} must be a string`)
  return value
}

function requireNonNegativeInteger(
  container: Record<string, unknown>,
  key: string,
): number {
  const value = container[key]
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    fail(`field ${key} must be a non-negative integer`)
  }
  return value
}

// 事件按 kind 部分携带字段：text_delta 只带 text、session_started 只带
// sessionStatus 等，其余枚举字段在线上为空字符串（Go 零值）。
// requireEnum 允许空；requireNonEmptyEnum 用于 kind 等必须存在的枚举。
function requireEnum<T extends string>(
  container: Record<string, unknown>,
  key: string,
  allowed: ReadonlySet<string>,
): T | '' {
  const value = requireString(container, key)
  if (value !== '' && !allowed.has(value)) fail(`field ${key} has unknown value`)
  return value as T | ''
}

function requireNonEmptyEnum<T extends string>(
  container: Record<string, unknown>,
  key: string,
  allowed: ReadonlySet<string>,
): T {
  const value = requireString(container, key)
  if (value === '' || !allowed.has(value)) fail(`field ${key} has unknown value`)
  return value as T
}

function requireObject(
  container: Record<string, unknown>,
  key: string,
): Record<string, unknown> {
  const value = container[key]
  if (!isObject(value)) fail(`field ${key} must be an object`)
  return value
}

function decodeErrorInfo(value: unknown): ErrorInfo | null {
  if (value === null) return null
  if (!isObject(value)) fail('error must be an object or null')
  return {
    code: requireString(value, 'code'),
    message: requireString(value, 'message'),
  }
}

function decodePendingAction(value: unknown): PendingAction | null {
  if (value === null) return null
  if (!isObject(value)) fail('pendingAction must be an object or null')
  for (const key of Object.keys(value)) {
    if (key !== undefined) fail('pendingAction must be an empty object')
  }
  return {}
}

function decodeTurnSnapshot(value: unknown): TurnSnapshot {
  if (!isObject(value)) fail('turn must be an object')
  const turn: TurnSnapshot = {
    status: requireNonEmptyEnum<TurnStatus>(value, 'status', TURN_STATUSES),
    requestId: requireString(value, 'requestId'),
    // A5.2：首轮前尚无任何 Turn，source 在线上为空串（Go 零值）。
    source: requireEnum<InputSource>(value, 'source', INPUT_SOURCES),
    message: requireString(value, 'message'),
    error: decodeErrorInfo(value.error),
    pendingAction: decodePendingAction(value.pendingAction),
  }
  // 服务端对 Turn 同时赋值 requestId 与 source（internal/session
  // applyNextInput），因此已有 Turn 时 source 不得为空。
  if (turn.source === '' && turn.requestId !== '') {
    fail('turn source is required once a turn exists')
  }
  return turn
}

function decodeSessionSnapshot(value: unknown): SessionSnapshot {
  if (!isObject(value)) fail('snapshot must be an object')
  return {
    sessionId: requireString(value, 'sessionId'),
    sessionStatus: requireNonEmptyEnum<SessionStatus>(value, 'sessionStatus', SESSION_STATUSES),
    sessionError: decodeErrorInfo(value.sessionError),
    seq: requireNonNegativeInteger(value, 'seq'),
    turn: decodeTurnSnapshot(value.turn),
    queueDepth: requireNonNegativeInteger(value, 'queueDepth'),
  }
}

function decodeSnapshotPayload(value: unknown): SnapshotPayload {
  if (!isObject(value)) fail('snapshot payload must be an object')
  return {
    connectionStatus: requireString(value, 'connectionStatus'),
    snapshot: decodeSessionSnapshot(value.snapshot),
  }
}

function decodeEventPayload(value: unknown): EventPayload {
  if (!isObject(value)) fail('event payload must be an object')
  const event = requireObject(value, 'event')
  // kind 必须非空且合法；其余枚举按 kind 语义可为空（服务端零值序列化）。
  const domainEvent: DomainEvent = {
    seq: requireNonNegativeInteger(event, 'seq'),
    kind: requireNonEmptyEnum<EventKind>(event, 'kind', EVENT_KINDS),
    sessionId: requireString(event, 'sessionId'),
    requestId: requireString(event, 'requestId'),
    source: requireEnum<InputSource>(event, 'source', INPUT_SOURCES),
    text: requireString(event, 'text'),
    sessionStatus: requireEnum<SessionStatus>(event, 'sessionStatus', SESSION_STATUSES),
    turnStatus: requireEnum<TurnStatus>(event, 'turnStatus', TURN_STATUSES),
    error: decodeErrorInfo(event.error),
  }
  validateEventSemantics(domainEvent)
  return { event: domainEvent }
}

// kind 级事件语义（A5.1 设计 §4.1/§9）：turn_started 必须携带非空、不超
// MaxPromptBytes 的输入正文和 running 状态；text_delta 必须有增量；其他
// kind 不得携带正文。错误信息不包含正文本身。
function validateEventSemantics(event: DomainEvent): void {
  if (event.kind === 'turn_started') {
    if (
      event.requestId === '' ||
      event.source === '' ||
      event.text === '' ||
      event.turnStatus !== 'running' ||
      new TextEncoder().encode(event.text).length > MAX_TEXT_BYTES
    ) {
      fail('invalid turn_started event')
    }
    return
  }
  if (event.kind === 'text_delta') {
    if (event.text === '') fail('invalid text_delta event')
    return
  }
  if (event.text !== '') fail('unexpected event text')
}

function decodeCommandResultPayload(value: unknown): CommandResultPayload {
  if (!isObject(value)) fail('command_result payload must be an object')
  const accepted = value.accepted
  if (typeof accepted !== 'boolean') fail('field accepted must be a boolean')
  return {
    accepted,
    code: requireString(value, 'code'),
    message: requireString(value, 'message'),
  }
}

function decodeErrorPayload(value: unknown): ErrorInfo {
  if (!isObject(value)) fail('error payload must be an object')
  return {
    code: requireString(value, 'code'),
    message: requireString(value, 'message'),
  }
}

// 解析服务端→客户端消息。任何不符合版本 1 契约的内容都抛 DecodeError。
export function decodeServerMessage(text: string): ServerMessage {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    fail('message is not valid JSON')
  }
  if (!isObject(parsed)) fail('message must be an object')

  const version = parsed.version
  if (typeof version !== 'number' || !Number.isSafeInteger(version)) {
    fail('field version must be an integer')
  }
  if (version !== PROTOCOL_VERSION) fail('unsupported protocol version')

  const type = requireString(parsed, 'type') as MessageType
  if (!SERVER_TO_CLIENT_TYPES.has(type)) {
    fail('unknown message type or wrong direction')
  }

  const payload = parsed.payload
  if (!isObject(payload)) fail('field payload must be an object')

  const envelope = {
    sessionId: requireString(parsed, 'sessionId'),
    commandId: requireString(parsed, 'commandId'),
    requestId: requireString(parsed, 'requestId'),
    seq: requireNonNegativeInteger(parsed, 'seq'),
  }

  switch (type) {
    case 'hello_ack':
    case 'subscribed':
    case 'unsubscribed':
      if (Object.keys(payload).length > 0) fail('payload must be empty')
      return { ...envelope, type, payload: {} }
    case 'snapshot':
      {
        const decoded = decodeSnapshotPayload(payload)
        if (
          decoded.snapshot.sessionId !== envelope.sessionId ||
          decoded.snapshot.seq !== envelope.seq
        ) {
          fail('snapshot identity does not match envelope')
        }
        return { ...envelope, type, payload: decoded }
      }
    case 'event':
      {
        const decoded = decodeEventPayload(payload)
        if (
          decoded.event.sessionId !== envelope.sessionId ||
          decoded.event.requestId !== envelope.requestId ||
          decoded.event.seq !== envelope.seq
        ) {
          fail('event identity does not match envelope')
        }
        return { ...envelope, type, payload: decoded }
      }
    case 'command_result':
      return { ...envelope, type, payload: decodeCommandResultPayload(payload) }
    case 'error':
      return { ...envelope, type, payload: decodeErrorPayload(payload) }
    default:
      fail('unknown message type')
  }
}
