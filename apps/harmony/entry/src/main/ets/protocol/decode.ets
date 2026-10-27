import {
  type ProgressPhase, type DiagnosticCode, type TurnProgress, type SessionDiagnostic,
  ACTION_KIND_VALUES,
  ASK_USER_QUESTION_TOOL_NAME,
  BLOCK_KIND_VALUES,
  EVENT_KIND_VALUES,
  INPUT_SOURCE_VALUES,
  PERMISSION_MODE_VALUES,
  type PermissionMode,
  MAX_TEXT_BYTES,
  PROTOCOL_VERSION,
  PREFERRED_PROTOCOL_VERSION,
  QUESTION_INPUT_TYPE_VALUES,
  SERVER_MESSAGE_TYPE_VALUES,
  SESSION_STATUS_VALUES,
  TOOL_CLASS_VALUES,
  TURN_STATUS_VALUES,
  UNSUPPORTED_CODE_VALUES,
  type AskUserQuestionPendingAction,
  type BlockKind,
  type CommandResultPayload,
  type ManagementResultPayload,
  type ContentBlock,
  type DomainEvent,
  type EmptyPayload,
  type ErrorInfo,
  type ErrorPayload,
  type EventKind,
  type EventPayload,
  type InputSource,
  type PendingAction,
  type PendingActionKind,
  type Question,
  type QuestionInputType,
  type QuestionOption,
  type QueuedRequest,
  type ResolvedAnswer,
  type ServerMessage,
  type ServerMessageType,
  type SessionSnapshot,
  type SessionStatus,
  type SnapshotPayload,
  type ThinkingContentBlock,
  type ToolApprovalPendingAction,
  type ToolClass,
  type ToolResultContentBlock,
  type ToolUseContentBlock,
  type TurnSnapshot,
  type TurnStatus,
  type UnsupportedCode,
  type UnsupportedPayload,
} from './types'
import { decodePairingPayload } from './pairing'

interface EnvelopeCandidate {
  version?: number
  type?: string
  sessionId?: string
  commandId?: string
  requestId?: string
  seq?: number
  payload?: Object | null
}

interface SnapshotPayloadCandidate {
  connectionStatus?: string
  snapshot?: Object | null
}

interface SessionSnapshotCandidate {
  permissionMode?: string
  sessionId?: string
  sessionStatus?: string
  sessionError?: Object | null
  seq?: number
  turn?: Object | null
  queueDepth?: number
  queuedRequests?: Array<Object> | null
  projectCwd?: string
  sessionTitle?: string
  createdAt?: string
  lastActivityAt?: string
}

interface TurnSnapshotCandidate {
  progress?: Object | null
  status?: string
  requestId?: string
  source?: string
  message?: string
  error?: Object | null
  pendingAction?: Object | null
}

interface ErrorInfoCandidate {
  code?: string
  message?: string
}

interface EventPayloadCandidate {
  event?: Object | null
}

interface EventCandidate {
  progress?: Object | null
  diagnostic?: Object | null
  permissionMode?: string
  seq?: number
  kind?: string
  sessionId?: string
  requestId?: string
  source?: string
  text?: string
  sessionStatus?: string
  turnStatus?: string
  error?: Object | null
  block?: Object | null
  pendingAction?: Object | null
  answers?: Array<Object> | null
  sessionTitle?: string
  lastActivityAt?: string
}

interface ThinkingBlockCandidate {
  id?: string
  kind?: string
  thinking?: string
}

interface ToolUseBlockCandidate {
  id?: string
  kind?: string
  toolUseId?: string
  toolClass?: string
  toolName?: string
  preview?: string
  mcpServer?: string
}

interface ToolResultBlockCandidate {
  id?: string
  kind?: string
  toolUseId?: string
  isError?: boolean
  output?: string
  truncated?: boolean
}

interface ToolApprovalCandidate {
  actionId?: string
  requestId?: string
  kind?: string
  toolName?: string
  preview?: string
  allowSessionAvailable?: boolean
}

interface AskUserQuestionCandidate {
  actionId?: string
  requestId?: string
  kind?: string
  toolName?: string
  allowSessionAvailable?: boolean
  questions?: Array<Object> | null
}

interface QuestionCandidate {
  questionId?: string
  header?: string
  question?: string
  inputType?: string
  multiSelect?: boolean
  options?: Array<Object> | null
}

interface QuestionOptionCandidate {
  label?: string
  description?: string
}

interface QueuedRequestCandidate {
  requestId?: string
  source?: string
  preview?: string
}

interface ResolvedAnswerCandidate {
  questionId?: string
  header?: string
  question?: string
  values?: Array<string> | null
}

interface CommandResultPayloadCandidate {
  accepted?: boolean
  code?: string
  message?: string
}

interface ManagementResultPayloadCandidate {
  accepted?: boolean
  code?: string
  message?: string
  pairing?: Object | null
}

interface ErrorPayloadCandidate {
  code?: string
  message?: string
  expectedVersion?: number
}

interface UnsupportedPayloadCandidate {
  code?: string
}

const ENVELOPE_KEYS: Array<string> = ['version', 'type', 'sessionId', 'commandId', 'requestId', 'seq', 'payload']
const SNAPSHOT_PAYLOAD_KEYS: Array<string> = ['connectionStatus', 'snapshot']
const SESSION_SNAPSHOT_KEYS: Array<string> = [
  'permissionMode',
  'sessionId',
  'sessionStatus',
  'sessionError',
  'seq',
  'turn',
  'queueDepth',
  'queuedRequests',
  'projectCwd', 'sessionTitle', 'createdAt', 'lastActivityAt',
]
const TURN_SNAPSHOT_KEYS: Array<string> = ['progress', 'status', 'requestId', 'source', 'message', 'error', 'pendingAction']
const ERROR_INFO_KEYS: Array<string> = ['code', 'message']
const EVENT_PAYLOAD_KEYS: Array<string> = ['event']
const EVENT_KEYS: Array<string> = [
  'progress', 'diagnostic',
  'permissionMode',
  'seq',
  'kind',
  'sessionId',
  'requestId',
  'source',
  'text',
  'sessionStatus',
  'turnStatus',
  'error',
  'block',
  'pendingAction',
  'answers',
  'sessionTitle', 'lastActivityAt',
]
const THINKING_BLOCK_KEYS: Array<string> = ['id', 'kind', 'thinking']
const TOOL_USE_BLOCK_KEYS: Array<string> = [
  'id',
  'kind',
  'toolUseId',
  'toolClass',
  'toolName',
  'preview',
  'mcpServer',
]
const TOOL_RESULT_BLOCK_KEYS: Array<string> = ['id', 'kind', 'toolUseId', 'isError', 'output', 'truncated']
const TOOL_APPROVAL_KEYS: Array<string> = [
  'actionId',
  'requestId',
  'kind',
  'toolName',
  'preview',
  'allowSessionAvailable',
]
const ASK_USER_QUESTION_KEYS: Array<string> = [
  'actionId',
  'requestId',
  'kind',
  'toolName',
  'allowSessionAvailable',
  'questions',
]
const QUESTION_KEYS: Array<string> = [
  'questionId',
  'header',
  'question',
  'inputType',
  'multiSelect',
  'options',
]
const QUESTION_OPTION_KEYS: Array<string> = ['label', 'description']
const QUEUED_REQUEST_KEYS: Array<string> = ['requestId', 'source', 'preview']
const RESOLVED_ANSWER_KEYS: Array<string> = ['questionId', 'header', 'question', 'values']
const COMMAND_RESULT_KEYS: Array<string> = ['accepted', 'code', 'message']
const MANAGEMENT_RESULT_KEYS: Array<string> = ['accepted', 'code', 'message', 'pairing']
const ERROR_PAYLOAD_KEYS: Array<string> = ['code', 'message', 'expectedVersion']
const UNSUPPORTED_KEYS: Array<string> = ['code']

function fail(reason: string): never {
  throw new Error(`protocol decode error: ${reason}`)
}

function isPlainObject(value: Object | null): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function isObjectArray(value: Object | null): boolean {
  return Array.isArray(value)
}

function hasAllowedKeys(value: Object, allowedKeys: Array<string>): boolean {
  const keys = Object.keys(value)
  for (let index = 0; index < keys.length; index += 1) {
    if (allowedKeys.indexOf(keys[index]) < 0) {
      return false
    }
  }
  return true
}

function requirePlainObject(value: Object | null, label: string): Object {
  if (!isPlainObject(value)) {
    fail(`${label} must be an object`)
  }
  return value as Object
}

function requireObjectArray(value: Object | null, label: string): Array<Object> {
  if (!isObjectArray(value)) {
    fail(`${label} must be an array`)
  }
  return value as Array<Object>
}

function requireAllowedObject(value: Object | null, label: string, allowedKeys: Array<string>): Object {
  const objectValue = requirePlainObject(value, label)
  if (!hasAllowedKeys(objectValue, allowedKeys)) {
    fail(`${label} has unexpected field`)
  }
  return objectValue
}

function readString(value: string | undefined, field: string): string {
  if (value === undefined) {
    fail(`field ${field} must be a string`)
  }
  return value
}

function readNonEmptyString(value: string | undefined, field: string): string {
  const result = readString(value, field)
  if (result.length === 0) {
    fail(`field ${field} must not be empty`)
  }
  return result
}

function readOptionalString(value: string | undefined, field: string): string {
  if (value === undefined) {
    return ''
  }
  return readString(value, field)
}

function readBoolean(value: boolean | undefined, field: string): boolean {
  if (value === undefined) {
    fail(`field ${field} must be a boolean`)
  }
  return value
}

function readNonNegativeInteger(value: number | undefined, field: string): number {
  if (value === undefined || !Number.isFinite(value) || Math.floor(value) !== value || value < 0) {
    fail(`field ${field} must be a non-negative integer`)
  }
  return value
}

function readInteger(value: number | undefined, field: string): number {
  if (value === undefined || !Number.isFinite(value) || Math.floor(value) !== value) {
    fail(`field ${field} must be an integer`)
  }
  return value
}

function readInputSource(value: string | undefined, field: string): InputSource {
  const result = readString(value, field)
  if (INPUT_SOURCE_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as InputSource
}

function readOptionalInputSource(value: string | undefined, field: string): InputSource | '' {
  const result = readOptionalString(value, field)
  if (result.length === 0) {
    return ''
  }
  if (INPUT_SOURCE_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as InputSource
}

function readSessionStatus(value: string | undefined, field: string): SessionStatus {
  const result = readString(value, field)
  if (SESSION_STATUS_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as SessionStatus
}

function readOptionalSessionStatus(value: string | undefined, field: string): SessionStatus | '' {
  const result = readOptionalString(value, field)
  if (result.length === 0) {
    return ''
  }
  if (SESSION_STATUS_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as SessionStatus
}

function readTurnStatus(value: string | undefined, field: string): TurnStatus {
  const result = readString(value, field)
  if (TURN_STATUS_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as TurnStatus
}

function readOptionalTurnStatus(value: string | undefined, field: string): TurnStatus | '' {
  const result = readOptionalString(value, field)
  if (result.length === 0) {
    return ''
  }
  if (TURN_STATUS_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as TurnStatus
}

function readEventKind(value: string | undefined, field: string): EventKind {
  const result = readString(value, field)
  if (EVENT_KIND_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as EventKind
}

function readBlockKind(value: string | undefined, field: string): BlockKind {
  const result = readString(value, field)
  if (BLOCK_KIND_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as BlockKind
}

function readToolClass(value: string | undefined, field: string): ToolClass {
  const result = readString(value, field)
  if (TOOL_CLASS_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as ToolClass
}

function readPendingActionKind(value: string | undefined, field: string): PendingActionKind {
  const result = readString(value, field)
  if (ACTION_KIND_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as PendingActionKind
}

function readQuestionInputType(value: string | undefined, field: string): QuestionInputType {
  const result = readString(value, field)
  if (QUESTION_INPUT_TYPE_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as QuestionInputType
}

function readUnsupportedCode(value: string | undefined, field: string): UnsupportedCode {
  const result = readString(value, field)
  if (UNSUPPORTED_CODE_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as UnsupportedCode
}

function readServerMessageType(value: string | undefined, field: string): ServerMessageType {
  const result = readString(value, field)
  if (SERVER_MESSAGE_TYPE_VALUES.indexOf(result) < 0) {
    fail(`field ${field} has unknown value`)
  }
  return result as ServerMessageType
}

function decodeErrorInfo(value: Object | null, field: string): ErrorInfo | null {
  if (value === null) {
    return null
  }
  const objectValue = requireAllowedObject(value, field, ERROR_INFO_KEYS) as ErrorInfoCandidate
  const info: ErrorInfo = {
    code: readString(objectValue.code, 'code'),
    message: readString(objectValue.message, 'message'),
  }
  return info
}

function utf8ByteLength(text: string): number {
  let total = 0
  for (let index = 0; index < text.length; index += 1) {
    const codeUnit = text.charCodeAt(index)
    if (codeUnit <= 0x7f) {
      total += 1
    } else if (codeUnit <= 0x7ff) {
      total += 2
    } else if (codeUnit >= 0xd800 && codeUnit <= 0xdbff && index + 1 < text.length) {
      const nextCodeUnit = text.charCodeAt(index + 1)
      if (nextCodeUnit >= 0xdc00 && nextCodeUnit <= 0xdfff) {
        total += 4
        index += 1
      } else {
        total += 3
      }
    } else {
      total += 3
    }
  }
  return total
}

function decodeContentBlock(value: Object | null): ContentBlock {
  const blockObject = requirePlainObject(value, 'block')
  const kind = readBlockKind((blockObject as ThinkingBlockCandidate).kind, 'kind')

  if (kind === 'thinking') {
    const candidate = requireAllowedObject(blockObject, 'thinking block', THINKING_BLOCK_KEYS) as ThinkingBlockCandidate
    const block: ThinkingContentBlock = {
      id: readNonEmptyString(candidate.id, 'id'),
      kind: 'thinking',
      thinking: readString(candidate.thinking, 'thinking'),
    }
    return block
  }

  if (kind === 'tool_use') {
    const candidate = requireAllowedObject(blockObject, 'tool_use block', TOOL_USE_BLOCK_KEYS) as ToolUseBlockCandidate
    const toolClass = readToolClass(candidate.toolClass, 'toolClass')
    const mcpServer = readOptionalString(candidate.mcpServer, 'mcpServer')
    if (toolClass === 'builtin' && mcpServer.length > 0) {
      fail('builtin tool_use must not carry mcpServer')
    }
    if (toolClass === 'mcp' && mcpServer.length === 0) {
      fail('mcp tool_use must carry mcpServer')
    }
    const preview = readOptionalString(candidate.preview, 'preview')
    const block: ToolUseContentBlock = {
      id: readNonEmptyString(candidate.id, 'id'),
      kind: 'tool_use',
      toolUseId: readNonEmptyString(candidate.toolUseId, 'toolUseId'),
      toolClass,
      toolName: readNonEmptyString(candidate.toolName, 'toolName'),
      mcpServer,
    }
    if (preview.length > 0) {
      block.preview = preview
    }
    return block
  }

  const resultCandidate = requireAllowedObject(blockObject, 'tool_result block', TOOL_RESULT_BLOCK_KEYS) as ToolResultBlockCandidate
  const result: ToolResultContentBlock = {
    id: readNonEmptyString(resultCandidate.id, 'id'),
    kind: 'tool_result',
    toolUseId: readNonEmptyString(resultCandidate.toolUseId, 'toolUseId'),
    isError: readBoolean(resultCandidate.isError, 'isError'),
  }
  const output = readOptionalString(resultCandidate.output, 'output')
  if (utf8ByteLength(output) > 16384) fail('tool output exceeds limit')
  if (output.length > 0) result.output = output
  if (resultCandidate.truncated !== undefined) result.truncated = readBoolean(resultCandidate.truncated, 'truncated')
  return result
}

function decodeQuestionOption(value: Object | null): QuestionOption {
  const candidate = requireAllowedObject(value, 'question option', QUESTION_OPTION_KEYS) as QuestionOptionCandidate
  const option: QuestionOption = {
    label: readNonEmptyString(candidate.label, 'label'),
    description: readOptionalString(candidate.description, 'description'),
  }
  return option
}

function decodeQuestions(value: Object | null): Array<Question> {
  const questionArray = requireObjectArray(value, 'questions')
  if (questionArray.length === 0) {
    fail('questions must be a non-empty array')
  }

  const questions: Array<Question> = []
  const seenQuestionIds: Array<string> = []
  for (let index = 0; index < questionArray.length; index += 1) {
    const candidate = requireAllowedObject(questionArray[index], 'question', QUESTION_KEYS) as QuestionCandidate
    const questionId = readNonEmptyString(candidate.questionId, 'questionId')
    if (seenQuestionIds.indexOf(questionId) >= 0) {
      fail('duplicate questionId')
    }
    seenQuestionIds.push(questionId)

    const inputType = readQuestionInputType(candidate.inputType, 'inputType')
    const multiSelect = readBoolean(candidate.multiSelect, 'multiSelect')
    const options: Array<QuestionOption> = []
    if (candidate.options !== undefined && candidate.options !== null) {
      const optionArray = requireObjectArray(candidate.options, 'options')
      for (let optionIndex = 0; optionIndex < optionArray.length; optionIndex += 1) {
        options.push(decodeQuestionOption(optionArray[optionIndex]))
      }
    }

    if (inputType === 'choice' && options.length === 0) {
      fail('choice question requires options')
    }
    if (inputType === 'text' && options.length > 0) {
      fail('text question must not carry options')
    }
    if (inputType === 'text' && multiSelect) {
      fail('text question must not be multi-select')
    }

    const question: Question = {
      questionId,
      header: readString(candidate.header, 'header'),
      question: readString(candidate.question, 'question'),
      inputType,
      multiSelect,
      options,
    }
    questions.push(question)
  }
  return questions
}

function decodePendingAction(value: Object | null, expectedRequestId: string): PendingAction {
  const actionObject = requirePlainObject(value, 'pendingAction')
  const kind = readPendingActionKind((actionObject as ToolApprovalCandidate).kind, 'kind')
  const requestId = readNonEmptyString((actionObject as ToolApprovalCandidate).requestId, 'requestId')
  if (requestId !== expectedRequestId) {
    fail('pendingAction requestId does not match')
  }

  if (kind === 'tool_approval') {
    const candidate = requireAllowedObject(actionObject, 'tool_approval action', TOOL_APPROVAL_KEYS) as ToolApprovalCandidate
    const action: ToolApprovalPendingAction = {
      actionId: readNonEmptyString(candidate.actionId, 'actionId'),
      requestId,
      kind: 'tool_approval',
      toolName: readNonEmptyString(candidate.toolName, 'toolName'),
      preview: readOptionalString(candidate.preview, 'preview'),
      allowSessionAvailable: readBoolean(candidate.allowSessionAvailable, 'allowSessionAvailable'),
    }
    return action
  }

  const questionCandidate = requireAllowedObject(actionObject, 'ask_user_question action', ASK_USER_QUESTION_KEYS) as AskUserQuestionCandidate
  if (readString(questionCandidate.toolName, 'toolName') !== ASK_USER_QUESTION_TOOL_NAME) {
    fail('ask_user_question action has unexpected toolName')
  }
  const questionAction: AskUserQuestionPendingAction = {
    actionId: readNonEmptyString(questionCandidate.actionId, 'actionId'),
    requestId,
    kind: 'ask_user_question',
    toolName: ASK_USER_QUESTION_TOOL_NAME,
    allowSessionAvailable: readBoolean(questionCandidate.allowSessionAvailable, 'allowSessionAvailable'),
    questions: decodeQuestions(questionCandidate.questions as Object | null),
  }
  return questionAction
}

function decodeResolvedAnswers(value: Object | null): Array<ResolvedAnswer> {
  if (value === null) {
    return []
  }

  const answerArray = requireObjectArray(value, 'answers')
  const answers: Array<ResolvedAnswer> = []
  const seenQuestionIds: Array<string> = []
  for (let index = 0; index < answerArray.length; index += 1) {
    const candidate = requireAllowedObject(answerArray[index], 'resolved answer', RESOLVED_ANSWER_KEYS) as ResolvedAnswerCandidate
    const questionId = readNonEmptyString(candidate.questionId, 'questionId')
    if (seenQuestionIds.indexOf(questionId) >= 0) {
      fail('duplicate answer questionId')
    }
    seenQuestionIds.push(questionId)

    const valueList: Array<string> = []
    if (candidate.values === undefined || candidate.values === null || candidate.values.length === 0) {
      fail('answer values must be a non-empty array')
    }
    for (let valueIndex = 0; valueIndex < candidate.values.length; valueIndex += 1) {
      const answerValue = candidate.values[valueIndex]
      if (answerValue.length === 0 || answerValue.trim().length === 0) {
        fail('answer value must be a non-empty string')
      }
      valueList.push(answerValue)
    }

    const answer: ResolvedAnswer = {
      questionId,
      header: readOptionalString(candidate.header, 'header'),
      question: readNonEmptyString(candidate.question, 'question'),
      values: valueList,
    }
    answers.push(answer)
  }
  return answers
}

function decodeQueuedRequests(value: Array<Object> | null | undefined): Array<QueuedRequest> {
  if (value === undefined || value === null) {
    return []
  }

  const requests: Array<QueuedRequest> = []
  for (let index = 0; index < value.length; index += 1) {
    const candidate = requireAllowedObject(value[index], 'queued request', QUEUED_REQUEST_KEYS) as QueuedRequestCandidate
    const request: QueuedRequest = {
      requestId: readNonEmptyString(candidate.requestId, 'requestId'),
      source: readInputSource(candidate.source, 'source'),
      preview: readOptionalString(candidate.preview, 'preview'),
    }
    requests.push(request)
  }
  return requests
}

interface ProgressCandidate { phase?: string; toolName?: string; startedAt?: number; updatedAt?: number; outputTokens?: number; tokensKnown?: boolean; tokensEstimated?: boolean }
interface DiagnosticCandidate { code?: string; httpStatus?: number; attempt?: number; maxAttempts?: number; retryDelayMs?: number }
function decodeProgress(value: Object | null): TurnProgress {
  const c = requireAllowedObject(value, 'progress', ['phase','toolName','startedAt','updatedAt','outputTokens','tokensKnown','tokensEstimated']) as ProgressCandidate
  const phase = readString(c.phase, 'phase')
  if (['waiting','working','thinking','writing','tool','subtask','compacting','requesting','retrying','waiting_followup','tool_result','subtask_result'].indexOf(phase) < 0) fail('invalid progress phase')
  const p: TurnProgress = { phase: phase as ProgressPhase, startedAt: readNonNegativeInteger(c.startedAt, 'startedAt'), updatedAt: readNonNegativeInteger(c.updatedAt, 'updatedAt'), outputTokens: readNonNegativeInteger(c.outputTokens, 'outputTokens'), tokensKnown: readBoolean(c.tokensKnown, 'tokensKnown'), tokensEstimated: readBoolean(c.tokensEstimated, 'tokensEstimated') }
  if (p.startedAt === 0 || p.updatedAt < p.startedAt || p.outputTokens > 1000000000) fail('invalid progress metadata')
  if (c.toolName !== undefined) { p.toolName = readString(c.toolName, 'toolName'); if (phase !== 'tool' || !/^[a-zA-Z0-9_:.@\/-]{1,128}$/.test(p.toolName)) fail('tool name too long') }
  return p
}
function decodeDiagnostic(value: Object | null): SessionDiagnostic {
  const c = requireAllowedObject(value, 'diagnostic', ['code','httpStatus','attempt','maxAttempts','retryDelayMs']) as DiagnosticCandidate
  const code = readString(c.code, 'code')
  if (['api_retry','query_failed','receive_failed','response_incomplete','assistant_error','result_error','turn_failed','tool_failed'].indexOf(code) < 0) fail('invalid diagnostic code')
  const d: SessionDiagnostic = { code: code as DiagnosticCode }
  if (c.httpStatus !== undefined) { d.httpStatus = readNonNegativeInteger(c.httpStatus, 'httpStatus'); if (d.httpStatus !== 0 && (d.httpStatus < 100 || d.httpStatus > 599)) fail('invalid httpStatus') }
  if (c.attempt !== undefined) { d.attempt = readNonNegativeInteger(c.attempt, 'attempt'); if (d.attempt > 1000000) fail('invalid attempt') }
  if (c.maxAttempts !== undefined) { d.maxAttempts = readNonNegativeInteger(c.maxAttempts, 'maxAttempts'); if (d.maxAttempts > 1000000) fail('invalid maxAttempts') }
  if (c.retryDelayMs !== undefined) { d.retryDelayMs = readNonNegativeInteger(c.retryDelayMs, 'retryDelayMs'); if (d.retryDelayMs > 86400000) fail('invalid delay') }
  return d
}

function decodeTurnSnapshot(value: Object | null, version: number): TurnSnapshot {
  const candidate = requireAllowedObject(value, 'turn', TURN_SNAPSHOT_KEYS) as TurnSnapshotCandidate
  const requestId = readString(candidate.requestId, 'requestId')
  const source = readOptionalInputSource(candidate.source, 'source')
  if (requestId.length > 0 && source === '') {
    fail('turn source is required once requestId exists')
  }

  let pendingAction: PendingAction | null = null
  if (candidate.pendingAction !== undefined && candidate.pendingAction !== null) {
    pendingAction = decodePendingAction(candidate.pendingAction, requestId)
  }

  const snapshot: TurnSnapshot = {
    status: readTurnStatus(candidate.status, 'status'),
    requestId,
    source,
    message: readString(candidate.message, 'message'),
    error: decodeErrorInfo(candidate.error !== undefined ? candidate.error : null, 'error'),
    pendingAction,
  }
  if (candidate.progress !== undefined) {
    if (version < 5 || requestId.length === 0 || ['running','waiting'].indexOf(snapshot.status) < 0) fail('unexpected snapshot progress')
    snapshot.progress = decodeProgress(candidate.progress)
  }
  if (snapshot.pendingAction !== null && snapshot.status !== 'waiting') {
    fail('pendingAction requires turnStatus waiting')
  }
  return snapshot
}

function displayField(value: string | undefined, field: string, version: number): string | undefined {
  if (value === undefined) return undefined
  if (typeof value !== 'string' || version < 3 || value.length === 0) fail(`invalid ${field} display field`)
  if (field === 'projectCwd' && value.length > 4096) fail('project cwd is too long')
  if (field === 'sessionTitle' && Array.from(value).length > 80) fail('session title is too long')
  if (field.endsWith('At') && (!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(value) || Number.isNaN(Date.parse(value)))) fail(`invalid ${field} timestamp`)
  return value
}

function permissionMode(value: string | undefined): PermissionMode {
  if (value === undefined || PERMISSION_MODE_VALUES.indexOf(value) < 0) fail('invalid permission mode')
  return value as PermissionMode
}

function decodeSessionSnapshot(value: Object | null, version: number): SessionSnapshot {
  const candidate = requireAllowedObject(value, 'snapshot', SESSION_SNAPSHOT_KEYS) as SessionSnapshotCandidate
  const queueDepth = readNonNegativeInteger(candidate.queueDepth, 'queueDepth')
  const queuedRequests = decodeQueuedRequests(candidate.queuedRequests)
  if (queueDepth !== queuedRequests.length) {
    fail('queueDepth does not match queuedRequests length')
  }

  const snapshot: SessionSnapshot = {
    sessionId: readString(candidate.sessionId, 'sessionId'),
    sessionStatus: readSessionStatus(candidate.sessionStatus, 'sessionStatus'),
    sessionError: decodeErrorInfo(candidate.sessionError !== undefined ? candidate.sessionError : null, 'sessionError'),
    seq: readNonNegativeInteger(candidate.seq, 'seq'),
    turn: decodeTurnSnapshot(candidate.turn !== undefined ? candidate.turn : null, version),
    queueDepth,
    queuedRequests,
  }
  snapshot.projectCwd = displayField(candidate.projectCwd, 'projectCwd', version)
  snapshot.sessionTitle = displayField(candidate.sessionTitle, 'sessionTitle', version)
  snapshot.createdAt = displayField(candidate.createdAt, 'createdAt', version)
  snapshot.lastActivityAt = displayField(candidate.lastActivityAt, 'lastActivityAt', version)
  if (candidate.permissionMode !== undefined) snapshot.permissionMode = permissionMode(candidate.permissionMode)
  return snapshot
}

function decodeSnapshotPayload(value: Object | null, version: number): SnapshotPayload {
  const candidate = requireAllowedObject(value, 'snapshot payload', SNAPSHOT_PAYLOAD_KEYS) as SnapshotPayloadCandidate
  const payload: SnapshotPayload = {
    connectionStatus: readString(candidate.connectionStatus, 'connectionStatus'),
    snapshot: decodeSessionSnapshot(candidate.snapshot !== undefined ? candidate.snapshot : null, version),
  }
  return payload
}

function validateEventSemantics(event: DomainEvent): void {
  if (event.kind === 'turn_started') {
    if (
      event.requestId.length === 0 ||
      event.source === '' ||
      event.text.length === 0 ||
      event.turnStatus !== 'running' ||
      utf8ByteLength(event.text) > MAX_TEXT_BYTES
    ) {
      fail('invalid turn_started event')
    }
  } else if (event.kind === 'text_delta') {
    if (event.text.length === 0) {
      fail('invalid text_delta event')
    }
  } else if (event.text.length > 0) {
    fail('unexpected event text')
  }

  if (event.kind === 'content_block') {
    if (event.requestId.length === 0 || event.block === undefined) {
      fail('content_block requires requestId and block')
    }
  } else if (event.block !== undefined) {
    fail('unexpected event block')
  }

  if (event.kind === 'action_required') {
    if (event.requestId.length === 0 || event.turnStatus !== 'waiting' || event.pendingAction === undefined) {
      fail('action_required requires waiting turn and pendingAction')
    }
  } else if (event.pendingAction !== undefined) {
    fail('unexpected event pendingAction')
  }

  if (event.kind !== 'action_resolved' && event.answers !== undefined) {
    fail('unexpected event answers')
  }
}

function decodeEventPayload(value: Object | null, version: number): EventPayload {
  const candidate = requireAllowedObject(value, 'event payload', EVENT_PAYLOAD_KEYS) as EventPayloadCandidate
  const eventCandidate = requireAllowedObject(candidate.event !== undefined ? candidate.event : null, 'event', EVENT_KEYS) as EventCandidate
  if (eventCandidate.error === undefined) fail('event error field is required')
  const requestId = readString(eventCandidate.requestId, 'requestId')

  const event: DomainEvent = {
    seq: readNonNegativeInteger(eventCandidate.seq, 'seq'),
    kind: readEventKind(eventCandidate.kind, 'kind'),
    sessionId: readString(eventCandidate.sessionId, 'sessionId'),
    requestId,
    source: readOptionalInputSource(eventCandidate.source, 'source'),
    text: readString(eventCandidate.text, 'text'),
    sessionStatus: readOptionalSessionStatus(eventCandidate.sessionStatus, 'sessionStatus'),
    turnStatus: readOptionalTurnStatus(eventCandidate.turnStatus, 'turnStatus'),
    error: decodeErrorInfo(eventCandidate.error !== undefined ? eventCandidate.error : null, 'error'),
  }
  event.sessionTitle = displayField(eventCandidate.sessionTitle, 'sessionTitle', version)
  event.lastActivityAt = displayField(eventCandidate.lastActivityAt, 'lastActivityAt', version)
  if (event.sessionTitle !== undefined && event.kind !== 'turn_started') fail('title requires turn_started')
  if (event.kind === 'permission_mode_changed') event.permissionMode = permissionMode(eventCandidate.permissionMode)
  else if (eventCandidate.permissionMode !== undefined) fail('unexpected permission mode')

  if (event.kind === 'content_block') {
    event.block = decodeContentBlock(eventCandidate.block !== undefined ? eventCandidate.block : null)
    if (version < 4 && event.block.kind === 'tool_result' && (event.block.output !== undefined || event.block.truncated === true)) fail('tool output requires v4')
  }
  if (event.kind === 'action_required') {
    event.pendingAction = decodePendingAction(
      eventCandidate.pendingAction !== undefined ? eventCandidate.pendingAction : null,
      requestId,
    )
  }
  if (event.kind === 'action_resolved') {
    const answers = decodeResolvedAnswers(eventCandidate.answers as Object | null)
    if (answers.length > 0) {
      event.answers = answers
    }
  }

  if (eventCandidate.progress !== undefined || event.kind === 'turn_progress') {
    if (version < 5 || event.kind !== 'turn_progress') fail('unexpected progress')
    event.progress = decodeProgress(eventCandidate.progress ?? null)
  }
  if (eventCandidate.diagnostic !== undefined || event.kind === 'session_diagnostic') {
    if (version < 5 || event.kind !== 'session_diagnostic') fail('unexpected diagnostic')
    event.diagnostic = decodeDiagnostic(eventCandidate.diagnostic ?? null)
  }
  if ((event.kind === 'turn_progress' || event.kind === 'session_diagnostic') && (requestId.length === 0 || event.turnStatus !== '' || event.sessionStatus !== '' || event.error !== null)) fail('invalid observation event')
  validateEventSemantics(event)
  const payload: EventPayload = { event }
  return payload
}

function decodeCommandResultPayload(value: Object | null): CommandResultPayload {
  const candidate = requireAllowedObject(value, 'command_result payload', COMMAND_RESULT_KEYS) as CommandResultPayloadCandidate
  const payload: CommandResultPayload = {
    accepted: readBoolean(candidate.accepted, 'accepted'),
    code: readString(candidate.code, 'code'),
    message: readString(candidate.message, 'message'),
  }
  return payload
}

function decodeManagementResultPayload(value: Object | null, create: boolean, sourceSessionId: string): ManagementResultPayload {
  const candidate = requireAllowedObject(value, 'management result payload', MANAGEMENT_RESULT_KEYS) as ManagementResultPayloadCandidate
  const accepted = readBoolean(candidate.accepted, 'accepted')
  const code = readString(candidate.code, 'code')
  const message = readString(candidate.message, 'message')
  const payload: ManagementResultPayload = { accepted, code, message }
  if (accepted) {
    if (code.length > 0 || message.length > 0) {
      fail('accepted management result must not carry error')
    }
    if (create) {
      if (candidate.pairing === undefined || candidate.pairing === null) {
        fail('create result missing pairing')
      }
      const pairing = decodePairingPayload(JSON.stringify(candidate.pairing), Date.now())
      if (!pairing.ok || pairing.payload === null || pairing.payload.sessionId === sourceSessionId) {
        fail('invalid created pairing')
      }
      payload.pairing = pairing.payload
    } else if (candidate.pairing !== undefined) {
      fail('stop result must not carry pairing')
    }
  } else if (code.length === 0 || candidate.pairing !== undefined) {
    fail('invalid rejected management result')
  }
  return payload
}

function decodeErrorPayload(value: Object | null): ErrorPayload {
  const candidate = requireAllowedObject(value, 'error payload', ERROR_PAYLOAD_KEYS) as ErrorPayloadCandidate
  const payload: ErrorPayload = {
    code: readString(candidate.code, 'code'),
    message: readString(candidate.message, 'message'),
  }
  if (candidate.expectedVersion !== undefined) {
    payload.expectedVersion = readInteger(candidate.expectedVersion, 'expectedVersion')
  }
  return payload
}

function decodeUnsupportedPayload(value: Object | null): UnsupportedPayload {
  const candidate = requireAllowedObject(value, 'unsupported payload', UNSUPPORTED_KEYS) as UnsupportedPayloadCandidate
  const payload: UnsupportedPayload = {
    code: readUnsupportedCode(candidate.code, 'code'),
  }
  return payload
}

function decodeEmptyPayload(value: Object | null): EmptyPayload {
  const objectValue = requirePlainObject(value, 'payload')
  if (Object.keys(objectValue).length > 0) {
    fail('payload must be empty')
  }
  const payload: EmptyPayload = {}
  return payload
}

export function decodeServerMessage(raw: string): ServerMessage {
  let parsedValue: Object | null = null
  try {
    parsedValue = JSON.parse(raw) as Object
  } catch (_error) {
    fail('message is not valid JSON')
  }

  const envelope = requireAllowedObject(parsedValue, 'message', ENVELOPE_KEYS) as EnvelopeCandidate
  const version = readInteger(envelope.version, 'version')
  if (version !== PROTOCOL_VERSION && version !== 3 && version !== 4 && version !== 5 && version !== PREFERRED_PROTOCOL_VERSION) {
    fail('unsupported protocol version')
  }

  const type = readServerMessageType(envelope.type, 'type')
  const sessionId = readString(envelope.sessionId, 'sessionId')
  const commandId = readString(envelope.commandId, 'commandId')
  const requestId = readString(envelope.requestId, 'requestId')
  const seq = readNonNegativeInteger(envelope.seq, 'seq')
  if (envelope.payload === undefined) {
    fail('field payload must be an object')
  }
  const payloadValue = envelope.payload

  if (type === 'pong' && (version < 6 || commandId !== '' || requestId !== '' || seq !== 0)) fail('invalid heartbeat')
  if (type === 'pong' || type === 'hello_ack' || type === 'subscribed' || type === 'unsubscribed') {
    const emptyPayload = decodeEmptyPayload(payloadValue)
    const message = {
      version,
      type,
      sessionId,
      commandId,
      requestId,
      seq,
      payload: emptyPayload,
    } as ServerMessage
    return message
  }

  if (type === 'snapshot') {
    const snapshotPayload = decodeSnapshotPayload(payloadValue, version)
    if (snapshotPayload.snapshot.sessionId !== sessionId || snapshotPayload.snapshot.seq !== seq) {
      fail('snapshot identity does not match envelope')
    }
    const message = {
      version,
      type,
      sessionId,
      commandId,
      requestId,
      seq,
      payload: snapshotPayload,
    } as ServerMessage
    return message
  }

  if (type === 'event') {
    const eventPayload = decodeEventPayload(payloadValue, version)
    if (
      eventPayload.event.sessionId !== sessionId ||
      eventPayload.event.requestId !== requestId ||
      eventPayload.event.seq !== seq
    ) {
      fail('event identity does not match envelope')
    }
    const message = {
      version,
      type,
      sessionId,
      commandId,
      requestId,
      seq,
      payload: eventPayload,
    } as ServerMessage
    return message
  }

  if (type === 'unsupported') {
    const unsupportedPayload = decodeUnsupportedPayload(payloadValue)
    const message = {
      version,
      type,
      sessionId,
      commandId,
      requestId,
      seq,
      payload: unsupportedPayload,
    } as ServerMessage
    return message
  }

  if (type === 'command_result') {
    const commandResultPayload = decodeCommandResultPayload(payloadValue)
    const message = {
      version,
      type,
      sessionId,
      commandId,
      requestId,
      seq,
      payload: commandResultPayload,
    } as ServerMessage
    return message
  }

  if (type === 'create_session_result' || type === 'stop_session_result') {
    if (commandId.length === 0 || requestId.length > 0 || seq !== 0) {
      fail('invalid management correlation fields')
    }
    const managementPayload = decodeManagementResultPayload(payloadValue, type === 'create_session_result', sessionId)
    const message = {
      version,
      type,
      sessionId,
      commandId,
      requestId,
      seq,
      payload: managementPayload,
    } as ServerMessage
    return message
  }

  const errorPayload = decodeErrorPayload(payloadValue)
  const message = {
    version,
    type,
    sessionId,
    commandId,
    requestId,
    seq,
    payload: errorPayload,
  } as ServerMessage
  return message
}
