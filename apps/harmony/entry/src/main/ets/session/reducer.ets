import type {
  ActionDecision,
  AskUserQuestionPendingAction,
  CommandKind,
  ContentBlock,
  DomainEvent,
  ErrorInfo,
  PairingPayload,
  PendingAction,
  Question,
  QuestionAnswer,
  QuestionOption,
  QueuedRequest,
  ResolvedAnswer,
  SessionSnapshot,
  ToolUseContentBlock,
  UnsupportedCode,
} from '../protocol/types'
import {
  type AnswerFeedMessage,
  type BlockFeedMessage,
  type ClaudeFeedMessage,
  ConnectionStage,
  type FeedMessage,
  type HarmonySessionState,
  type InputFeedMessage,
  type NoticeInfo,
  type PendingCommand,
} from './model'

export interface StartConnectAction {
  type: 'startConnect'
  pairing: PairingPayload
}

export interface SocketOpenedAction {
  type: 'socketOpened'
}

export interface HelloAckAction {
  type: 'helloAck'
}

export interface SubscribedReceivedAction {
  type: 'subscribedReceived'
}

export interface SnapshotReceivedAction {
  type: 'snapshotReceived'
  snapshot: SessionSnapshot
}

export interface EventReceivedAction {
  type: 'eventReceived'
  event: DomainEvent
}

export interface CommandStartedAction {
  type: 'commandStarted'
  commandId: string
  kind: CommandKind
  text: string
  targetId: string
}

export interface CommandAcceptedAction {
  type: 'commandAccepted'
  commandId: string
  requestId: string
}

export interface CommandRejectedAction {
  type: 'commandRejected'
  commandId: string
  code: string
  message: string
}

export interface CommandUnknownAction {
  type: 'commandUnknown'
  commandId: string
}

export interface UnsupportedReceivedAction {
  type: 'unsupportedReceived'
  code: UnsupportedCode
  seq: number
}

export interface SocketClosedAction {
  type: 'socketClosed'
  error: ErrorInfo | null
}

export interface PairingInvalidAction {
  type: 'pairingInvalid'
  error: ErrorInfo | null
}

export interface ProtocolErrorAction {
  type: 'protocolError'
  error: ErrorInfo | null
}

export interface LocalErrorAction {
  type: 'localError'
  error: ErrorInfo
}

export type SessionAction =
  | StartConnectAction
  | SocketOpenedAction
  | HelloAckAction
  | SubscribedReceivedAction
  | SnapshotReceivedAction
  | EventReceivedAction
  | CommandStartedAction
  | CommandAcceptedAction
  | CommandRejectedAction
  | CommandUnknownAction
  | UnsupportedReceivedAction
  | SocketClosedAction
  | PairingInvalidAction
  | ProtocolErrorAction
  | LocalErrorAction

const REJECTION_ACTION_NOT_PENDING = 'action_not_pending'
const REJECTION_ACTION_TIMEOUT = 'action_timeout'
const REJECTION_INVALID_ANSWER = 'invalid_answer'
const REJECTION_TURN_NOT_CANCELLABLE = 'turn_not_cancellable'
const REJECTION_REQUEST_NOT_QUEUED = 'request_not_queued'

function makeError(code: string, message: string): ErrorInfo {
  const error: ErrorInfo = { code, message }
  return error
}

function clonePairingPayload(pairing: PairingPayload | null): PairingPayload | null {
  if (pairing === null) {
    return null
  }
  return {
    version: pairing.version,
    wsUrl: pairing.wsUrl,
    sessionId: pairing.sessionId,
    pairToken: pairing.pairToken,
    expiresAt: pairing.expiresAt,
    capabilities: pairing.capabilities === undefined ? undefined : pairing.capabilities.slice(),
    pcId: pairing.pcId,
    pcName: pairing.pcName,
  }
}

function cloneErrorInfo(error: ErrorInfo | null): ErrorInfo | null {
  if (error === null) {
    return null
  }
  return makeError(error.code, error.message)
}

function cloneQuestionOptions(options: Array<QuestionOption>): Array<QuestionOption> {
  const next: Array<QuestionOption> = []
  for (let index = 0; index < options.length; index += 1) {
    next.push({
      label: options[index].label,
      description: options[index].description,
    })
  }
  return next
}

function cloneQuestions(questions: Array<Question>): Array<Question> {
  const next: Array<Question> = []
  for (let index = 0; index < questions.length; index += 1) {
    next.push({
      questionId: questions[index].questionId,
      header: questions[index].header,
      question: questions[index].question,
      inputType: questions[index].inputType,
      multiSelect: questions[index].multiSelect,
      options: cloneQuestionOptions(questions[index].options),
    })
  }
  return next
}

function cloneQuestionAnswers(answers: Array<QuestionAnswer>): Array<QuestionAnswer> {
  const next: Array<QuestionAnswer> = []
  for (let index = 0; index < answers.length; index += 1) {
    const values: Array<string> = []
    for (let valueIndex = 0; valueIndex < answers[index].values.length; valueIndex += 1) {
      values.push(answers[index].values[valueIndex])
    }
    next.push({
      questionId: answers[index].questionId,
      values,
    })
  }
  return next
}

function cloneResolvedAnswers(answers: Array<ResolvedAnswer>): Array<ResolvedAnswer> {
  const next: Array<ResolvedAnswer> = []
  for (let index = 0; index < answers.length; index += 1) {
    const values: Array<string> = []
    for (let valueIndex = 0; valueIndex < answers[index].values.length; valueIndex += 1) {
      values.push(answers[index].values[valueIndex])
    }
    next.push({
      questionId: answers[index].questionId,
      header: answers[index].header,
      question: answers[index].question,
      values,
    })
  }
  return next
}

function clonePendingAction(action: PendingAction | null): PendingAction | null {
  if (action === null) {
    return null
  }
  if (action.kind === 'tool_approval') {
    return {
      actionId: action.actionId,
      requestId: action.requestId,
      kind: 'tool_approval',
      toolName: action.toolName,
      preview: action.preview,
      allowSessionAvailable: action.allowSessionAvailable,
    }
  }
  const questionAction: AskUserQuestionPendingAction = {
    actionId: action.actionId,
    requestId: action.requestId,
    kind: 'ask_user_question',
    toolName: 'AskUserQuestion',
    allowSessionAvailable: action.allowSessionAvailable,
    questions: cloneQuestions(action.questions),
  }
  return questionAction
}

function cloneContentBlock(block: ContentBlock): ContentBlock {
  if (block.kind === 'thinking') {
    return {
      id: block.id,
      kind: 'thinking',
      thinking: block.thinking,
    }
  }
  if (block.kind === 'tool_use') {
    const next: ToolUseContentBlock = {
      id: block.id,
      kind: 'tool_use',
      toolUseId: block.toolUseId,
      toolClass: block.toolClass,
      toolName: block.toolName,
      mcpServer: block.mcpServer,
    }
    if (block.preview !== undefined) {
      next.preview = block.preview
    }
    return next
  }
  return {
    id: block.id,
    kind: 'tool_result',
    toolUseId: block.toolUseId,
    isError: block.isError,
    output: block.output,
    truncated: block.truncated,
  }
}

function cloneFeedMessage(message: FeedMessage): FeedMessage {
  if (message.kind === 'input') {
    const next: InputFeedMessage = {
      kind: 'input',
      source: message.source,
      requestId: message.requestId,
      text: message.text,
      status: message.status,
      attachments: message.attachments,
    }
    return next
  }
  if (message.kind === 'claude') {
    const next: ClaudeFeedMessage = {
      kind: 'claude',
      requestId: message.requestId,
      source: message.source,
      status: message.status,
      text: message.text,
      error: cloneErrorInfo(message.error),
    }
    return next
  }
  if (message.kind === 'block') {
    const next: BlockFeedMessage = {
      kind: 'block',
      requestId: message.requestId,
      block: cloneContentBlock(message.block),
    }
    return next
  }
  const answer: AnswerFeedMessage = {
    kind: 'answer',
    requestId: message.requestId,
    answers: cloneResolvedAnswers(message.answers),
  }
  return answer
}

function cloneMessages(messages: Array<FeedMessage>): Array<FeedMessage> {
  const next: Array<FeedMessage> = []
  for (let index = 0; index < messages.length; index += 1) {
    next.push(cloneFeedMessage(messages[index]))
  }
  return next
}

function cloneQueuedRequests(items: Array<QueuedRequest>): Array<QueuedRequest> {
  const next: Array<QueuedRequest> = []
  for (let index = 0; index < items.length; index += 1) {
    next.push({
      requestId: items[index].requestId,
      source: items[index].source,
      preview: items[index].preview,
    })
  }
  return next
}

function clonePendingCommand(command: PendingCommand | null): PendingCommand | null {
  if (command === null) {
    return null
  }
  return {
    commandId: command.commandId,
    kind: command.kind,
    text: command.text,
    targetId: command.targetId,
  }
}

function cloneNoticeInfo(notice: NoticeInfo | null): NoticeInfo | null {
  if (notice === null) {
    return null
  }
  return {
    code: notice.code,
    message: notice.message,
  }
}

function cloneState(state: HarmonySessionState): HarmonySessionState {
  return {
    permissionMode: state.permissionMode,
    pairing: clonePairingPayload(state.pairing),
    connection: state.connection,
    sessionId: state.sessionId,
    projectCwd: state.projectCwd,
    sessionTitle: state.sessionTitle,
    createdAt: state.createdAt,
    lastActivityAt: state.lastActivityAt,
    sessionStatus: state.sessionStatus,
    sessionError: cloneErrorInfo(state.sessionError),
    seqBase: state.seqBase,
    queueDepthHint: state.queueDepthHint,
    queuedRequests: cloneQueuedRequests(state.queuedRequests),
    pendingAction: clonePendingAction(state.pendingAction),
    pendingCommand: clonePendingCommand(state.pendingCommand),
    messages: cloneMessages(state.messages),
    lastError: cloneErrorInfo(state.lastError),
    lastNotice: cloneNoticeInfo(state.lastNotice),
    canSend: state.canSend,
    canResolveAction: state.canResolveAction,
  }
}

function canSendNow(state: HarmonySessionState): boolean {
  if (state.connection !== ConnectionStage.READY) {
    return false
  }
  if (state.pendingCommand !== null || state.pendingAction !== null) {
    return false
  }
  return state.sessionStatus === 'ready' || state.sessionStatus === 'starting'
}

function canResolveActionNow(state: HarmonySessionState): boolean {
  return state.connection === ConnectionStage.READY && state.pendingAction !== null && state.pendingCommand === null
}

function applyCapabilityFlags(state: HarmonySessionState): HarmonySessionState {
  const next = cloneState(state)
  next.canSend = canSendNow(next)
  next.canResolveAction = canResolveActionNow(next)
  return next
}

function createResetState(pairing: PairingPayload): HarmonySessionState {
  const next: HarmonySessionState = {
    permissionMode: 'default',
    pairing: clonePairingPayload(pairing),
    connection: ConnectionStage.CONNECTING,
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
  return applyCapabilityFlags(next)
}

function withoutQueued(state: HarmonySessionState, requestId: string): HarmonySessionState {
  const next = cloneState(state)
  const filtered: Array<QueuedRequest> = []
  for (let index = 0; index < next.queuedRequests.length; index += 1) {
    if (next.queuedRequests[index].requestId !== requestId) {
      filtered.push(next.queuedRequests[index])
    }
  }
  next.queuedRequests = filtered
  next.queueDepthHint = filtered.length
  return next
}

function markPendingUnknown(state: HarmonySessionState): HarmonySessionState {
  const pending = state.pendingCommand
  if (pending === null) {
    return cloneState(state)
  }
  const next = cloneState(state)
  if ((pending.kind === 'send_text' || pending.kind === 'send_attachments')) {
    const message: InputFeedMessage = {
      kind: 'input',
      source: 'mobile',
      requestId: '',
      text: pending.text,
      status: 'unknown',
    }
    next.messages.push(message)
  }
  next.pendingCommand = null
  return next
}

function createNotice(code: UnsupportedCode): NoticeInfo {
  if (code === 'unknown_block') {
    return { code, message: '本版本暂不支持展示该内容块，请在电脑端查看' }
  }
  if (code === 'unknown_subtype') {
    return { code, message: '本版本暂不支持该待处理事项，请在电脑端处理' }
  }
  if (code === 'concurrent_action') {
    return { code, message: '该待处理事项已由其他客户端处理' }
  }
  return { code, message: '当前需要先完成待处理事项，暂不接收新输入' }
}

function createRejectedMessage(code: string, message: string): string {
  if (code === REJECTION_ACTION_NOT_PENDING) {
    return '该待处理事项已由其他客户端处理'
  }
  if (code === REJECTION_ACTION_TIMEOUT) {
    return '待处理事项已超时取消，请等待新的请求'
  }
  if (code === REJECTION_INVALID_ANSWER) {
    return '回答不符合要求，请重新选择后提交'
  }
  if (code === REJECTION_TURN_NOT_CANCELLABLE) {
    return '当前 Turn 已无法取消'
  }
  if (code === REJECTION_REQUEST_NOT_QUEUED) {
    return '该排队输入已不在队列中'
  }
  return message.length > 0 ? message : '操作被拒绝'
}

function createAcceptedInputMessage(requestId: string, text: string): InputFeedMessage {
  return {
    kind: 'input',
    source: 'mobile',
    requestId,
    text,
    status: 'accepted',
  }
}

function createClaudeMessage(
  requestId: string,
  source: 'terminal' | 'mobile',
  text: string,
  status: 'streaming' | 'done' | 'failed' | 'cancelled',
  error: ErrorInfo | null,
): ClaudeFeedMessage {
  return {
    kind: 'claude',
    requestId,
    source,
    text,
    status,
    error: cloneErrorInfo(error),
  }
}

function createBlockMessage(requestId: string, block: ContentBlock): BlockFeedMessage {
  return {
    kind: 'block',
    requestId,
    block: cloneContentBlock(block),
  }
}

function createAnswerMessage(requestId: string, answers: Array<ResolvedAnswer>): AnswerFeedMessage {
  return {
    kind: 'answer',
    requestId,
    answers: cloneResolvedAnswers(answers),
  }
}

function findFirstMessageIndex(messages: Array<FeedMessage>, requestId: string): number {
  for (let index = 0; index < messages.length; index += 1) {
    if (messages[index].requestId === requestId) {
      return index
    }
  }
  return -1
}

function findLastClaudeIndex(messages: Array<FeedMessage>, requestId: string): number {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if (message.kind === 'claude' && message.requestId === requestId) {
      return index
    }
    if (message.requestId === requestId) {
      break
    }
  }
  return -1
}

function hasClaudeForRequest(messages: Array<FeedMessage>, requestId: string): boolean {
  for (let index = 0; index < messages.length; index += 1) {
    const message = messages[index]
    if (message.kind === 'claude' && message.requestId === requestId) {
      return true
    }
  }
  return false
}

function replaceMessage(messages: Array<FeedMessage>, index: number, replacement: FeedMessage): Array<FeedMessage> {
  const next = cloneMessages(messages)
  next[index] = replacement
  return next
}

function mergeTurnStarted(state: HarmonySessionState, event: DomainEvent): HarmonySessionState | null {
  if (event.source === '') {
    return null
  }
  if (hasClaudeForRequest(state.messages, event.requestId)) {
    return null
  }

  const next = cloneState(state)
  const inputIndex = findFirstMessageIndex(next.messages, event.requestId)
  if (inputIndex >= 0) {
    const existing = next.messages[inputIndex]
    if (existing.kind !== 'input') {
      return null
    }
    if (existing.source !== event.source || (existing.text !== event.text && existing.attachments === undefined)) {
      return null
    }
  } else {
    const input: InputFeedMessage = {
      kind: 'input',
      source: event.source,
      requestId: event.requestId,
      text: event.text,
      status: '',
    }
    next.messages.push(input)
  }

  next.messages.push(createClaudeMessage(event.requestId, event.source, '', 'streaming', null))
  return withoutQueued(next, event.requestId)
}

function applySnapshot(state: HarmonySessionState, snapshot: SessionSnapshot): HarmonySessionState {
  const next = cloneState(state)
  next.permissionMode = snapshot.permissionMode ?? 'default'
  next.connection = ConnectionStage.READY
  next.sessionId = snapshot.sessionId
  next.projectCwd = snapshot.projectCwd
  next.sessionTitle = snapshot.sessionTitle
  next.createdAt = snapshot.createdAt
  next.lastActivityAt = snapshot.lastActivityAt
  next.sessionStatus = snapshot.sessionStatus
  next.sessionError = cloneErrorInfo(snapshot.sessionError)
  next.seqBase = snapshot.seq
  next.queueDepthHint = snapshot.queueDepth
  next.queuedRequests = cloneQueuedRequests(snapshot.queuedRequests)
  next.pendingAction = clonePendingAction(snapshot.turn.pendingAction)
  next.pendingCommand = null
  next.messages = next.messages.map((message: FeedMessage): FeedMessage => {
    if (message.kind === 'claude' && message.status === 'streaming') message.status = 'done'
    return message
  })
  next.lastError = null
  next.lastNotice = null

  const turnActive = snapshot.turn.status === 'running' || snapshot.turn.status === 'waiting'
  if (snapshot.turn.source !== '' && (snapshot.turn.message.length > 0 || turnActive)) {
    const status = snapshot.turn.status === 'failed'
      ? 'failed'
      : snapshot.turn.status === 'cancelled'
        ? 'cancelled'
      : turnActive
        ? 'streaming'
        : 'done'
    next.messages = next.messages.filter((message: FeedMessage) => !(message.kind === 'claude' && message.requestId === snapshot.turn.requestId))
    next.messages.push(
      createClaudeMessage(
        snapshot.turn.requestId,
        snapshot.turn.source,
        snapshot.turn.message,
        status,
        snapshot.turn.error,
      ),
    )
  }

  return applyCapabilityFlags(next)
}

function applyEvent(state: HarmonySessionState, event: DomainEvent): HarmonySessionState {
  if (event.seq !== state.seqBase + 1) {
    const failed = markPendingUnknown(state)
    failed.connection = ConnectionStage.PROTOCOL_ERROR
    failed.lastError = makeError('protocol_error', 'event sequence is inconsistent')
    return applyCapabilityFlags(failed)
  }

  let next = cloneState(state)
  next.seqBase = event.seq
  if (event.sessionTitle !== undefined && next.sessionTitle === undefined) next.sessionTitle = event.sessionTitle
  if (event.lastActivityAt !== undefined) next.lastActivityAt = event.lastActivityAt

  switch (event.kind) {
    case 'permission_mode_changed':
      if (event.permissionMode !== undefined) next.permissionMode = event.permissionMode
      break
    case 'session_started':
      next.sessionId = event.sessionId
      next.sessionStatus = event.sessionStatus
      next.sessionError = null
      break
    case 'session_failed':
      next.messages.forEach((message: FeedMessage) => { if (message.kind === 'claude' && message.status === 'streaming') { message.status = 'failed'; message.error = cloneErrorInfo(event.error) } })
      next.sessionStatus = event.sessionStatus
      next.sessionError = cloneErrorInfo(event.error)
      next.pendingAction = null
      next.queuedRequests = []
      next.queueDepthHint = 0
      break
    case 'session_stopped':
      next.messages.forEach((message: FeedMessage) => { if (message.kind === 'claude' && message.status === 'streaming') message.status = 'cancelled' })
      next.sessionStatus = event.sessionStatus
      next.pendingAction = null
      next.queuedRequests = []
      next.queueDepthHint = 0
      break
    case 'turn_started': {
      const merged = mergeTurnStarted(next, event)
      if (merged === null) {
        const failed = markPendingUnknown(next)
        failed.connection = ConnectionStage.PROTOCOL_ERROR
        failed.lastError = makeError('protocol_error', 'turn_started is inconsistent with current feed')
        return applyCapabilityFlags(failed)
      }
      next = merged
      break
    }
    case 'text_delta': {
      const lastClaudeIndex = findLastClaudeIndex(next.messages, event.requestId)
      if (lastClaudeIndex >= 0 && lastClaudeIndex === next.messages.length - 1) {
        const last = next.messages[lastClaudeIndex] as ClaudeFeedMessage
        next.messages = replaceMessage(
          next.messages,
          lastClaudeIndex,
          createClaudeMessage(last.requestId, last.source, `${last.text}${event.text}`, last.status, last.error),
        )
      } else if (hasClaudeForRequest(next.messages, event.requestId) && event.source !== '') {
        next.messages.push(createClaudeMessage(event.requestId, event.source, event.text, 'streaming', null))
      }
      break
    }
    case 'turn_completed':
    case 'turn_failed': {
      const updated: Array<FeedMessage> = []
      for (let index = 0; index < next.messages.length; index += 1) {
        const message = next.messages[index]
        if (message.kind === 'claude' && message.requestId === event.requestId) {
          let status: 'streaming' | 'done' | 'failed' | 'cancelled' = 'done'
          if (event.kind === 'turn_failed') {
            status = 'failed'
          } else if (event.turnStatus === 'cancelled') {
            status = 'cancelled'
          }
          updated.push(createClaudeMessage(message.requestId, message.source, message.text, status, event.error))
        } else {
          updated.push(cloneFeedMessage(message))
        }
      }
      next.messages = updated
      next.pendingAction = null
      break
    }
    case 'content_block':
      if (event.block !== undefined) {
        next.messages.push(createBlockMessage(event.requestId, event.block))
      }
      break
    case 'action_required':
      next.pendingAction = event.pendingAction !== undefined ? clonePendingAction(event.pendingAction) : null
      break
    case 'action_resolved':
      next.pendingAction = null
      if (event.answers !== undefined && event.answers.length > 0) {
        next.messages.push(createAnswerMessage(event.requestId, event.answers))
      }
      break
  }

  return applyCapabilityFlags(next)
}

export function startConnect(pairing: PairingPayload): SessionAction {
  return { type: 'startConnect', pairing }
}

export function socketOpened(): SessionAction {
  return { type: 'socketOpened' }
}

export function helloAck(): SessionAction {
  return { type: 'helloAck' }
}

export function subscribedReceived(): SessionAction {
  return { type: 'subscribedReceived' }
}

export function snapshotReceived(snapshot: SessionSnapshot): SessionAction {
  return { type: 'snapshotReceived', snapshot }
}

export function eventReceived(event: DomainEvent): SessionAction {
  return { type: 'eventReceived', event }
}

export function commandStarted(
  commandId: string,
  kind: CommandKind,
  text: string,
  targetId: string,
): SessionAction {
  return { type: 'commandStarted', commandId, kind, text, targetId }
}

export function commandAccepted(commandId: string, requestId: string): SessionAction {
  return { type: 'commandAccepted', commandId, requestId }
}

export function commandRejected(commandId: string, code: string, message: string): SessionAction {
  return { type: 'commandRejected', commandId, code, message }
}

export function commandUnknown(commandId: string): SessionAction {
  return { type: 'commandUnknown', commandId }
}

export function unsupportedReceived(code: UnsupportedCode, seq: number): SessionAction {
  return { type: 'unsupportedReceived', code, seq }
}

export function socketClosed(error: ErrorInfo | null = null): SessionAction {
  return { type: 'socketClosed', error }
}

export function pairingInvalid(error: ErrorInfo | null = null): SessionAction {
  return { type: 'pairingInvalid', error }
}

export function protocolError(error: ErrorInfo | null = null): SessionAction {
  return { type: 'protocolError', error }
}

export function localError(error: ErrorInfo): SessionAction {
  return { type: 'localError', error }
}

export function reduceHarmonySessionState(
  state: HarmonySessionState,
  action: SessionAction,
): HarmonySessionState {
  switch (action.type) {
    case 'startConnect': {
      const next = createResetState(action.pairing)
      if (state.sessionId === action.pairing.sessionId || state.pairing?.sessionId === action.pairing.sessionId) {
        next.sessionId = action.pairing.sessionId
        next.messages = cloneMessages(state.messages)
        next.projectCwd = state.projectCwd
        next.sessionTitle = state.sessionTitle
        next.createdAt = state.createdAt
        next.lastActivityAt = state.lastActivityAt
      }
      return next
    }
    case 'socketOpened': {
      const next = cloneState(state)
      next.connection = ConnectionStage.AUTHENTICATING
      next.lastError = null
      return applyCapabilityFlags(next)
    }
    case 'helloAck': {
      const next = cloneState(state)
      next.connection = ConnectionStage.SUBSCRIBING
      next.lastError = null
      return applyCapabilityFlags(next)
    }
    case 'subscribedReceived': {
      const next = cloneState(state)
      next.connection = ConnectionStage.SUBSCRIBED
      next.lastError = null
      return applyCapabilityFlags(next)
    }
    case 'snapshotReceived':
      return applySnapshot(state, action.snapshot)
    case 'eventReceived':
      return applyEvent(state, action.event)
    case 'commandStarted': {
      const next = cloneState(state)
      next.pendingCommand = {
        commandId: action.commandId,
        kind: action.kind,
        text: action.text,
        targetId: action.targetId,
      }
      next.lastError = null
      return applyCapabilityFlags(next)
    }
    case 'commandAccepted': {
      const pending = state.pendingCommand
      if (pending === null || pending.commandId !== action.commandId) {
        return applyCapabilityFlags(cloneState(state))
      }
      let next = cloneState(state)
      next.pendingCommand = null
      if ((pending.kind === 'send_text' || pending.kind === 'send_attachments')) {
        const existing = next.messages.find((message: FeedMessage) => message.kind === 'input' && message.requestId === action.requestId)
        if (existing !== undefined && existing.kind === 'input') {
          existing.status = 'accepted'
          if (pending.kind === 'send_attachments') existing.text = pending.text
        } else next.messages.push(createAcceptedInputMessage(action.requestId, pending.text))
      } else if (pending.kind === 'resolve_action') {
        next.pendingAction = null
      } else if (pending.kind === 'cancel_queued') {
        next = withoutQueued(next, pending.targetId)
      }
      return applyCapabilityFlags(next)
    }
    case 'commandRejected': {
      const pending = state.pendingCommand
      if (pending === null || pending.commandId !== action.commandId) {
        return applyCapabilityFlags(cloneState(state))
      }
      let next = cloneState(state)
      next.pendingCommand = null
      next.lastError = makeError(action.code, createRejectedMessage(action.code, action.message))
      if (action.code === REJECTION_ACTION_NOT_PENDING || action.code === REJECTION_ACTION_TIMEOUT) {
        next.pendingAction = null
      } else if (action.code === REJECTION_REQUEST_NOT_QUEUED) {
        next = withoutQueued(next, pending.targetId)
      }
      return applyCapabilityFlags(next)
    }
    case 'commandUnknown': {
      const pending = state.pendingCommand
      if (pending === null || pending.commandId !== action.commandId) {
        return applyCapabilityFlags(cloneState(state))
      }
      return applyCapabilityFlags(markPendingUnknown(state))
    }
    case 'unsupportedReceived': {
      if (action.seq !== state.seqBase + 1) {
        const failed = markPendingUnknown(state)
        failed.connection = ConnectionStage.PROTOCOL_ERROR
        failed.lastError = makeError('protocol_error', 'unsupported sequence is inconsistent')
        return applyCapabilityFlags(failed)
      }
      const next = cloneState(state)
      next.seqBase = action.seq
      next.lastNotice = createNotice(action.code)
      return applyCapabilityFlags(next)
    }
    case 'socketClosed': {
      const next = cloneState(state)
      next.connection = ConnectionStage.DISCONNECTED
      next.lastError = cloneErrorInfo(action.error)
      return applyCapabilityFlags(next)
    }
    case 'pairingInvalid': {
      const next = cloneState(state)
      next.connection = ConnectionStage.PAIRING_INVALID
      next.lastError = action.error !== null
        ? cloneErrorInfo(action.error)
        : makeError('unauthorized', 'pairing is no longer valid')
      return applyCapabilityFlags(next)
    }
    case 'protocolError': {
      const next = cloneState(state)
      next.connection = ConnectionStage.PROTOCOL_ERROR
      next.lastError = action.error !== null
        ? cloneErrorInfo(action.error)
        : makeError('protocol_error', 'protocol state is not trustworthy')
      return applyCapabilityFlags(next)
    }
    case 'localError': {
      const next = cloneState(state)
      next.lastError = cloneErrorInfo(action.error)
      return applyCapabilityFlags(next)
    }
  }
}
