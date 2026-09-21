import type {
  CommandResultPayload,
  ContentBlock,
  DomainEvent,
  ErrorInfo,
  ErrorPayload,
  EventKind,
  EventPayload,
  InputSource,
  MessageType,
  PendingAction,
  Question,
  QuestionInputType,
  QuestionOption,
  QueuedRequest,
  ResolvedAnswer,
  ServerMessage,
  SessionSnapshot,
  SessionStatus,
  SnapshotPayload,
  ToolClass,
  TurnSnapshot,
  TurnStatus,
  PermissionMode,
  UnsupportedCode,
  UnsupportedPayload,
} from './types'
import {
  ASK_USER_QUESTION_TOOL_NAME,
  MAX_TEXT_BYTES,
  PROTOCOL_VERSION,
  SERVER_TO_CLIENT_TYPES,
} from './types'

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
const PERMISSION_MODES: ReadonlySet<string> = new Set(['default', 'acceptEdits', 'plan', 'bypassPermissions'])

const INPUT_SOURCES: ReadonlySet<string> = new Set(['terminal', 'mobile'])

const EVENT_KINDS: ReadonlySet<string> = new Set([
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
])

// unsupported 不是第四类 block，也不是事件 kind：它走独立 envelope（B2 §4.4）。
const BLOCK_KINDS: ReadonlySet<string> = new Set(['thinking', 'tool_use', 'tool_result'])

const TOOL_CLASSES: ReadonlySet<string> = new Set(['builtin', 'mcp'])

const ACTION_KINDS: ReadonlySet<string> = new Set(['tool_approval', 'ask_user_question'])

const QUESTION_INPUT_TYPES: ReadonlySet<string> = new Set(['choice', 'text'])

const UNSUPPORTED_CODES: ReadonlySet<string> = new Set([
  'unknown_block',
  'unknown_subtype',
  'concurrent_action',
  'text_input_disabled',
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

function requireNonEmptyString(container: Record<string, unknown>, key: string): string {
  const value = requireString(container, key)
  if (value === '') fail(`field ${key} must not be empty`)
  return value
}

function requireBoolean(container: Record<string, unknown>, key: string): boolean {
  const value = container[key]
  if (typeof value !== 'boolean') fail(`field ${key} must be a boolean`)
  return value
}

// Go 侧对空字符串字段使用 omitempty，缺省即空串；但字段存在时类型必须正确。
function optionalString(container: Record<string, unknown>, key: string): string {
  const value = container[key]
  if (value === undefined) return ''
  if (typeof value !== 'string') fail(`field ${key} must be a string`)
  return value
}

// 结构化子对象按 kind 携带互不相交的字段子集：多余字段说明对端契约不一致，
// 或试图透传越界数据（如工具原始 input），一律拒绝而不是静默丢弃。
function rejectUnknownKeys(
  value: Record<string, unknown>,
  allowed: ReadonlySet<string>,
  label: string,
): void {
  for (const key of Object.keys(value)) {
    if (!allowed.has(key)) fail(`${label} has unexpected field`)
  }
}

function decodeErrorInfo(value: unknown): ErrorInfo | null {
  if (value === null) return null
  if (!isObject(value)) fail('error must be an object or null')
  return {
    code: requireString(value, 'code'),
    message: requireString(value, 'message'),
  }
}

const THINKING_BLOCK_KEYS: ReadonlySet<string> = new Set(['id', 'kind', 'thinking'])
const TOOL_USE_BLOCK_KEYS: ReadonlySet<string> = new Set([
  'id',
  'kind',
  'toolUseId',
  'toolClass',
  'toolName',
  'mcpServer',
  'preview',
])
const TOOL_RESULT_BLOCK_KEYS: ReadonlySet<string> = new Set([
  'id',
  'kind',
  'toolUseId',
  'isError',
])

// 结构化内容块（B2 §4.1）：只解码展示所需字段，不接受工具参数或结果正文。
function decodeContentBlock(value: unknown): ContentBlock {
  if (!isObject(value)) fail('block must be an object')
  const id = requireNonEmptyString(value, 'id')
  const kind = requireNonEmptyEnum<ContentBlock['kind']>(value, 'kind', BLOCK_KINDS)
  switch (kind) {
    case 'thinking':
      rejectUnknownKeys(value, THINKING_BLOCK_KEYS, 'thinking block')
      return { id, kind, thinking: requireString(value, 'thinking') }
    case 'tool_use': {
      rejectUnknownKeys(value, TOOL_USE_BLOCK_KEYS, 'tool_use block')
      const toolClass = requireNonEmptyEnum<ToolClass>(value, 'toolClass', TOOL_CLASSES)
      const mcpServer = optionalString(value, 'mcpServer')
      // 内置工具没有 MCP 服务器；MCP 工具必须标明来源，否则界面无法区分。
      if (toolClass === 'builtin' && mcpServer !== '') fail('builtin tool must not carry mcpServer')
      if (toolClass === 'mcp' && mcpServer === '') fail('mcp tool must carry mcpServer')
      const preview = optionalString(value, 'preview')
      return {
        id,
        kind,
        toolUseId: requireNonEmptyString(value, 'toolUseId'),
        toolClass,
        toolName: requireNonEmptyString(value, 'toolName'),
        mcpServer,
        ...(preview === '' ? {} : { preview }),
      }
    }
    case 'tool_result':
      rejectUnknownKeys(value, TOOL_RESULT_BLOCK_KEYS, 'tool_result block')
      return {
        id,
        kind,
        toolUseId: requireNonEmptyString(value, 'toolUseId'),
        isError: requireBoolean(value, 'isError'),
      }
  }
}

const OPTION_KEYS: ReadonlySet<string> = new Set(['label', 'description'])
const QUESTION_KEYS: ReadonlySet<string> = new Set([
  'questionId',
  'header',
  'question',
  'inputType',
  'multiSelect',
  'options',
])

function decodeQuestionOption(value: unknown): QuestionOption {
  if (!isObject(value)) fail('question option must be an object')
  rejectUnknownKeys(value, OPTION_KEYS, 'question option')
  return {
    // label 是提交答案时的取值，空标签无法回传。
    label: requireNonEmptyString(value, 'label'),
    description: optionalString(value, 'description'),
  }
}

function decodeQuestions(value: unknown): Question[] {
  if (!Array.isArray(value) || value.length === 0) fail('questions must be a non-empty array')
  const questions: Question[] = []
  const seen = new Set<string>()
  for (const item of value) {
    if (!isObject(item)) fail('question must be an object')
    rejectUnknownKeys(item, QUESTION_KEYS, 'question')
    const questionId = requireNonEmptyString(item, 'questionId')
    // 答案按 questionId 寻址，重复 ID 无法回传。
    if (seen.has(questionId)) fail('duplicate questionId')
    seen.add(questionId)
    const inputType = requireNonEmptyEnum<QuestionInputType>(
      item,
      'inputType',
      QUESTION_INPUT_TYPES,
    )
    const multiSelect = requireBoolean(item, 'multiSelect')
    const rawOptions = item.options
    let options: QuestionOption[] = []
    if (rawOptions !== undefined && rawOptions !== null) {
      if (!Array.isArray(rawOptions)) fail('options must be an array')
      options = rawOptions.map(decodeQuestionOption)
    }
    if (inputType === 'choice' && options.length === 0) fail('choice question requires options')
    if (inputType === 'text' && options.length > 0) fail('text question must not carry options')
    if (inputType === 'text' && multiSelect) fail('text question must not be multi-select')
    questions.push({
      questionId,
      header: requireString(item, 'header'),
      question: requireString(item, 'question'),
      inputType,
      multiSelect,
      options,
    })
  }
  return questions
}

const APPROVAL_KEYS: ReadonlySet<string> = new Set([
  'actionId',
  'requestId',
  'kind',
  'toolName',
  'preview',
  'allowSessionAvailable',
])
const QUESTION_ACTION_KEYS: ReadonlySet<string> = new Set([
  'actionId',
  'requestId',
  'kind',
  'toolName',
  'allowSessionAvailable',
  'questions',
])

// 待处理事项（B2 §4.2）：两种形态的字段子集互不相交，且必须归属同一个 Turn。
function decodePendingAction(value: unknown, requestId: string): PendingAction {
  if (!isObject(value)) fail('pendingAction must be an object')
  const kind = requireNonEmptyEnum<PendingAction['kind']>(value, 'kind', ACTION_KINDS)
  const actionId = requireNonEmptyString(value, 'actionId')
  const actionRequestId = requireNonEmptyString(value, 'requestId')
  if (actionRequestId !== requestId) fail('pendingAction requestId does not match the turn')
  const allowSessionAvailable = requireBoolean(value, 'allowSessionAvailable')
  if (kind === 'tool_approval') {
    rejectUnknownKeys(value, APPROVAL_KEYS, 'tool_approval action')
    return {
      actionId,
      requestId: actionRequestId,
      kind,
      toolName: requireNonEmptyString(value, 'toolName'),
      // 白名单未命中时服务端不下发预览，客户端不得自行拼接。
      preview: optionalString(value, 'preview'),
      allowSessionAvailable,
    }
  }
  rejectUnknownKeys(value, QUESTION_ACTION_KEYS, 'ask_user_question action')
  if (requireString(value, 'toolName') !== ASK_USER_QUESTION_TOOL_NAME) {
    fail('ask_user_question action has unexpected toolName')
  }
  return {
    actionId,
    requestId: actionRequestId,
    kind,
    toolName: ASK_USER_QUESTION_TOOL_NAME,
    allowSessionAvailable,
    questions: decodeQuestions(value.questions),
  }
}

function decodeOptionalPendingAction(
  value: unknown,
  requestId: string,
): PendingAction | null {
  if (value === null || value === undefined) return null
  return decodePendingAction(value, requestId)
}

const QUEUED_REQUEST_KEYS: ReadonlySet<string> = new Set(['requestId', 'source', 'preview'])

const RESOLVED_ANSWER_KEYS: ReadonlySet<string> = new Set([
  'questionId',
  'header',
  'question',
  'values',
])

// action_resolved 回带的已校验答案（D52）：服务端只在回答型事项上携带它，
// 权限决议不带。每项必须能定位到具体问题并带至少一个非空白取值。
function decodeResolvedAnswers(value: unknown): ResolvedAnswer[] {
  if (value === undefined || value === null) return []
  if (!Array.isArray(value)) fail('answers must be an array')
  const answers: ResolvedAnswer[] = []
  const seen = new Set<string>()
  for (const item of value) {
    if (!isObject(item)) fail('answer must be an object')
    rejectUnknownKeys(item, RESOLVED_ANSWER_KEYS, 'resolved answer')
    const questionId = requireNonEmptyString(item, 'questionId')
    if (seen.has(questionId)) fail('duplicate answered questionId')
    seen.add(questionId)
    const rawValues = item.values
    if (!Array.isArray(rawValues) || rawValues.length === 0) {
      fail('answer values must be a non-empty array')
    }
    const values = rawValues.map((entry) => {
      if (typeof entry !== 'string' || entry.trim() === '') fail('blank answer value')
      return entry
    })
    answers.push({
      questionId,
      header: optionalString(item, 'header'),
      question: requireNonEmptyString(item, 'question'),
      values,
    })
  }
  return answers
}

// 排队输入（B2 §4.3）：Go 侧 QueuedRequests 无 omitempty，空队列在线上是 null，
// 但字段本身必须存在，缺失说明对端不是 B2 服务端。
function decodeQueuedRequests(container: Record<string, unknown>): QueuedRequest[] {
  if (!('queuedRequests' in container)) fail('field queuedRequests is required')
  const value = container.queuedRequests
  if (value === null) return []
  if (!Array.isArray(value)) fail('field queuedRequests must be an array or null')
  return value.map((item) => {
    if (!isObject(item)) fail('queued request must be an object')
    rejectUnknownKeys(item, QUEUED_REQUEST_KEYS, 'queued request')
    return {
      requestId: requireNonEmptyString(item, 'requestId'),
      source: requireNonEmptyEnum<InputSource>(item, 'source', INPUT_SOURCES),
      preview: optionalString(item, 'preview'),
    }
  })
}

function decodeTurnSnapshot(value: unknown): TurnSnapshot {
  if (!isObject(value)) fail('turn must be an object')
  const requestId = requireString(value, 'requestId')
  const turn: TurnSnapshot = {
    status: requireNonEmptyEnum<TurnStatus>(value, 'status', TURN_STATUSES),
    requestId,
    // A5.2：首轮前尚无任何 Turn，source 在线上为空串（Go 零值）。
    source: requireEnum<InputSource>(value, 'source', INPUT_SOURCES),
    message: requireString(value, 'message'),
    error: decodeErrorInfo(value.error),
    pendingAction: decodeOptionalPendingAction(value.pendingAction, requestId),
  }
  // 服务端对 Turn 同时赋值 requestId 与 source（internal/session
  // applyNextInput），因此已有 Turn 时 source 不得为空。
  if (turn.source === '' && turn.requestId !== '') {
    fail('turn source is required once a turn exists')
  }
  // 不变量（B2 §4.2）：有待处理事项的 Turn 必然处于 waiting。
  if (turn.pendingAction !== null && turn.status !== 'waiting') {
    fail('pendingAction requires a waiting turn')
  }
  return turn
}

function decodeSessionSnapshot(value: unknown): SessionSnapshot {
  if (!isObject(value)) fail('snapshot must be an object')
  const snapshot: SessionSnapshot = {
    sessionId: requireString(value, 'sessionId'),
    sessionStatus: requireNonEmptyEnum<SessionStatus>(value, 'sessionStatus', SESSION_STATUSES),
    sessionError: decodeErrorInfo(value.sessionError),
    seq: requireNonNegativeInteger(value, 'seq'),
    turn: decodeTurnSnapshot(value.turn),
    queueDepth: requireNonNegativeInteger(value, 'queueDepth'),
    queuedRequests: decodeQueuedRequests(value),
    permissionMode: (() => {
      const raw = optionalString(value, 'permissionMode')
      if (raw === '') return 'default'
      if (!PERMISSION_MODES.has(raw)) fail('field permissionMode has unknown value')
      return raw as PermissionMode
    })(),
  }
  // queueDepth 与列表长度恒等（B2 §4.3）：不一致说明快照自相矛盾。
  if (snapshot.queueDepth !== snapshot.queuedRequests.length) {
    fail('queueDepth does not match queuedRequests')
  }
  return snapshot
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
  const kind = requireNonEmptyEnum<EventKind>(event, 'kind', EVENT_KINDS)
  const requestId = requireString(event, 'requestId')
  // unsupported 只走自己的 envelope（B2 §4.4）：事件不得携带通知码。
  if (optionalString(event, 'code') !== '') fail('event must not carry an unsupported code')

  // block 只属于 content_block，pendingAction 只属于 action_required；互相串位
  // 意味着对端实现与契约不一致。
  const rawBlock = event.block
  const rawAction = event.pendingAction
  if (kind === 'content_block') {
    if (rawBlock === undefined || rawBlock === null) fail('content_block event requires block')
  } else if (rawBlock !== undefined && rawBlock !== null) {
    fail('unexpected event block')
  }
  if (kind === 'action_required') {
    if (rawAction === undefined || rawAction === null) {
      fail('action_required event requires pendingAction')
    }
  } else if (rawAction !== undefined && rawAction !== null) {
    fail('unexpected event pendingAction')
  }
  // 答案只属于 action_resolved（D52）。
  const rawAnswers = event.answers
  if (kind !== 'action_resolved' && rawAnswers !== undefined && rawAnswers !== null) {
    fail('unexpected event answers')
  }

  // kind 必须非空且合法；其余枚举按 kind 语义可为空（服务端零值序列化）。
  const domainEvent: DomainEvent = {
    seq: requireNonNegativeInteger(event, 'seq'),
    kind,
    sessionId: requireString(event, 'sessionId'),
    requestId,
    source: requireEnum<InputSource>(event, 'source', INPUT_SOURCES),
    text: requireString(event, 'text'),
    sessionStatus: requireEnum<SessionStatus>(event, 'sessionStatus', SESSION_STATUSES),
    turnStatus: requireEnum<TurnStatus>(event, 'turnStatus', TURN_STATUSES),
    error: decodeErrorInfo(event.error),
  }
  if (kind === 'content_block') {
    domainEvent.block = decodeContentBlock(rawBlock)
  }
  if (kind === 'action_required') {
    domainEvent.pendingAction = decodePendingAction(rawAction, requestId)
  }
  if (kind === 'action_resolved') {
    const answers = decodeResolvedAnswers(rawAnswers)
    if (answers.length > 0) domainEvent.answers = answers
  }
  if (kind === 'permission_mode_changed') {
    const rawMode = requireNonEmptyString(event, 'permissionMode')
    if (!PERMISSION_MODES.has(rawMode)) fail('invalid permission mode event')
    domainEvent.permissionMode = rawMode as PermissionMode
  }
  validateEventSemantics(domainEvent)
  return { event: domainEvent }
}

// kind 级事件语义（A5.1 设计 §4.1/§9、B2 §4.1/§4.2）：turn_started 必须携带
// 非空、不超 MaxPromptBytes 的输入正文和 running 状态；text_delta 必须有增量；
// content_block / action_required 必须归属具体 Turn；其他 kind 不得携带正文。
// 错误信息不包含正文本身。
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
  if (event.kind === 'content_block') {
    if (event.requestId === '') fail('invalid content_block event')
  }
  if (event.kind === 'permission_mode_changed' && event.permissionMode === undefined) {
    fail('permission mode event requires permissionMode')
  }
  if (event.kind === 'action_required') {
    // 等待待处理事项的 Turn 必然处于 waiting（B2 §4.2）。
    if (event.requestId === '' || event.turnStatus !== 'waiting') {
      fail('invalid action_required event')
    }
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

function decodeErrorPayload(value: unknown): ErrorPayload {
  if (!isObject(value)) fail('error payload must be an object')
  const payload: ErrorPayload = {
    code: requireString(value, 'code'),
    message: requireString(value, 'message'),
  }
  // 仅版本不匹配时携带（B2 §4.5）：客户端据此提示用户升级。
  if (value.expectedVersion !== undefined) {
    payload.expectedVersion = requireNonNegativeInteger(value, 'expectedVersion')
  }
  return payload
}

const UNSUPPORTED_PAYLOAD_KEYS: ReadonlySet<string> = new Set(['code'])

// 不支持通知（B2 §4.4）：只允许固定安全码，不透传任何原始 SDK 数据。
function decodeUnsupportedPayload(value: unknown): UnsupportedPayload {
  if (!isObject(value)) fail('unsupported payload must be an object')
  rejectUnknownKeys(value, UNSUPPORTED_PAYLOAD_KEYS, 'unsupported payload')
  return {
    code: requireNonEmptyEnum<UnsupportedCode>(value, 'code', UNSUPPORTED_CODES),
  }
}

// 解析服务端→客户端消息。任何不符合当前版本契约的内容都抛 DecodeError。
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
    case 'unsupported':
      return { ...envelope, type, payload: decodeUnsupportedPayload(payload) }
    case 'error':
      return { ...envelope, type, payload: decodeErrorPayload(payload) }
    default:
      fail('unknown message type')
  }
}
