// 单连接 UI 状态归并（D32/D33/D36）。纯函数，不依赖 React 运行时。

import type {
  CommandKind,
  ContentBlock,
  DomainEvent,
  ErrorInfo,
  PendingAction,
  PermissionMode,
  QueuedRequest,
  ResolvedAnswer,
  SessionSnapshot,
  UnsupportedCode,
} from '../protocol/types'

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
  // cancelled 来自 turn_completed + turnStatus=cancelled（B2 §4.3）：
  // 服务端不为取消单独发事件，只靠 turnStatus 区分。
  status: 'streaming' | 'done' | 'failed' | 'cancelled'
  text: string
  error: ErrorInfo | null
}

// 通用输入消息（A5.1/D37）：终端与本机手机输入都走这里。终端输入由
// turn_started 创建；本机手机输入由 command_result 创建；其他手机连接
// 的输入由 turn_started 观察创建。
export interface InputMessage {
  kind: 'input'
  source: 'terminal' | 'mobile'
  requestId: string | null
  text: string
  status: 'accepted' | 'unknown' | null
}

// 结构化内容块消息（B2 §4.1）：按 content_block 到达顺序追加，与 Claude
// 文本共用一条消息流，重连后不由 snapshot 补造历史块。
export interface BlockMessage {
  kind: 'block'
  requestId: string
  block: ContentBlock
}

// 已回答事项的答案回显（D52）：只来自 action_resolved 携带的服务端已校验
// 答案，因此哪一端答的都能看到；重连后不由 snapshot 补造。
export interface AnswerMessage {
  kind: 'answer'
  requestId: string
  answers: ResolvedAnswer[]
}

export type Message = ClaudeMessage | InputMessage | BlockMessage | AnswerMessage

// 单在途命令（B2 §5）：所有 kind 共用一个槽位。targetId 按 kind 指向
// actionId 或 requestId，text 只对 send_text 有意义。
export interface PendingCommand {
  commandId: string
  kind: CommandKind
  text: string
  targetId: string
}

// 不支持通知（B2 §4.4）：只提示，不影响连接阶段与可发送状态。
export interface Notice {
  code: UnsupportedCode
  message: string
}

export interface MobileState {
  phase: Phase
  connection: ConnectionState
  sessionId: string
  sessionStatus: 'starting' | 'ready' | 'stopped' | 'failed' | null
  sessionError: ErrorInfo | null
  permissionMode: PermissionMode
  seqBase: number
  queueDepthHint: number
  queuedRequests: QueuedRequest[]
  messages: Message[]
  pendingAction: PendingAction | null
  pendingCommand: PendingCommand | null
  lastError: ErrorInfo | null
  lastNotice: Notice | null
  canSend: boolean
  canResolveAction: boolean
}

export function createInitialState(): MobileState {
  return {
    phase: 'unpaired',
    connection: 'connecting',
    sessionId: '',
    sessionStatus: null,
    sessionError: null,
    permissionMode: 'default',
    seqBase: 0,
    queueDepthHint: 0,
    queuedRequests: [],
    messages: [],
    pendingAction: null,
    pendingCommand: null,
    lastError: null,
    lastNotice: null,
    canSend: false,
    canResolveAction: false,
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

// 四个命令构造器共用一个 action 类型：单在途门控与结果归并只区分 kind。
interface CommandSentAction {
  type: 'commandSent'
  commandId: string
  kind: CommandKind
  text: string
  targetId: string
}

export function commandSent(info: { commandId: string; text: string }): CommandSentAction {
  return {
    type: 'commandSent',
    commandId: info.commandId,
    kind: 'send_text',
    text: info.text,
    targetId: '',
  }
}

export function actionResolutionSent(info: {
  commandId: string
  actionId: string
}): CommandSentAction {
  return {
    type: 'commandSent',
    commandId: info.commandId,
    kind: 'resolve_action',
    text: '',
    targetId: info.actionId,
  }
}

export function turnCancelSent(info: {
  commandId: string
  requestId: string
}): CommandSentAction {
  return {
    type: 'commandSent',
    commandId: info.commandId,
    kind: 'cancel_turn',
    text: '',
    targetId: info.requestId,
  }
}

export function queuedCancelSent(info: {
  commandId: string
  requestId: string
}): CommandSentAction {
  return {
    type: 'commandSent',
    commandId: info.commandId,
    kind: 'cancel_queued',
    text: '',
    targetId: info.requestId,
  }
}

export function permissionModeSent(info: { commandId: string; permissionMode: PermissionMode }): CommandSentAction {
  return {
    type: 'commandSent',
    commandId: info.commandId,
    kind: 'set_permission_mode',
    text: info.permissionMode,
    targetId: '',
  }
}

export function noticeReceived(code: UnsupportedCode, seq: number) {
  return { type: 'noticeReceived', code, seq } as const
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
  | CommandSentAction
  | ReturnType<typeof noticeReceived>
  | ReturnType<typeof commandAccepted>
  | ReturnType<typeof commandRejected>
  | ReturnType<typeof commandUnknown>
  | ReturnType<typeof connectionFailed>
  | ReturnType<typeof reset>

// ---- 归并逻辑 ----

// 不支持通知文案（B2 §4.4）：服务端只给类别码，文案在客户端固定。
const NOTICE_MESSAGES: Record<UnsupportedCode, string> = {
  unknown_block: '本版本暂不支持展示该内容块，请在电脑端查看',
  unknown_subtype: '本版本暂不支持该待处理事项，请在电脑端处理',
  concurrent_action: '该待处理事项已由其他客户端处理',
  text_input_disabled: '当前需要先完成待处理事项，暂不接收新输入',
}

// 业务拒绝码文案（internal/transport/protocol.go）：需要引导用户下一步的
// 类别在客户端统一文案；其余类别直接用服务端描述。
const REJECTION_MESSAGES: Record<string, string> = {
  action_not_pending: '该待处理事项已由其他客户端处理',
  action_timeout: '待处理事项已超时取消，请等待新的请求',
  invalid_answer: '回答不符合要求，请重新选择后提交',
  turn_not_cancellable: '当前 Turn 已无法取消',
  request_not_queued: '该排队输入已不在队列中',
}

function canSendNow(state: MobileState): boolean {
  return (
    state.phase === 'connected' &&
    (state.sessionStatus === 'ready' || state.sessionStatus === 'starting') &&
    // 待处理事项未完成时服务端拒收自由文本（B2 §6）。
    state.pendingAction === null &&
    state.pendingCommand === null
  )
}

function canResolveActionNow(state: MobileState): boolean {
  return (
    state.phase === 'connected' &&
    state.pendingAction !== null &&
    state.pendingCommand === null
  )
}

function applyCanSend(state: MobileState): MobileState {
  return {
    ...state,
    canSend: canSendNow(state),
    canResolveAction: canResolveActionNow(state),
  }
}

// 队列只在本地维护（B2 §4.3）：queueDepthHint 始终跟随列表长度。
function withoutQueued(state: MobileState, requestId: string): MobileState {
  const queuedRequests = state.queuedRequests.filter((q) => q.requestId !== requestId)
  if (queuedRequests.length === state.queuedRequests.length) return state
  return { ...state, queuedRequests, queueDepthHint: queuedRequests.length }
}

function markPendingUnknown(state: MobileState): MobileState {
  if (state.pendingCommand === null) return state
  // 只有自由文本有正文需要回显；决议与取消不入消息流。
  if (state.pendingCommand.kind !== 'send_text') {
    return { ...state, pendingCommand: null }
  }
  const unknown: InputMessage = {
    kind: 'input',
    source: 'mobile',
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
    permissionMode: snapshot.permissionMode ?? 'default',
    seqBase: snapshot.seq,
    queueDepthHint: snapshot.queueDepth,
    queuedRequests: snapshot.queuedRequests,
    // 重连/刷新以新 snapshot 重新初始化，旧易失消息清空（D32）。
    // 结构化内容块不在 snapshot 中，不得伪造历史块（B2 §4.1）。
    messages: [],
    // waiting 的 snapshot 携带待处理事项，重连后仍需禁用文本输入。
    pendingAction: snapshot.turn.pendingAction,
    pendingCommand: null,
    lastError: null,
    lastNotice: null,
  }
  // 运行/等待中的 Turn 即使尚无文本也创建占位消息：用户可能在 AI 回复
  // 前配对，订阅后的 text_delta 需要归并目标（A5.1 验收回归）。
  // source 为空即首轮前尚无 Turn（A5.2），此时不创建任何消息。
  const turnActive =
    snapshot.turn.status === 'running' || snapshot.turn.status === 'waiting'
  const turnSource = snapshot.turn.source
  if (turnSource !== '' && (snapshot.turn.message !== '' || turnActive)) {
    next.messages = [
      {
        kind: 'claude',
        requestId: snapshot.turn.requestId,
        source: turnSource,
        text: snapshot.turn.message,
        status:
          snapshot.turn.status === 'failed'
            ? 'failed'
            : turnActive
              ? 'streaming'
              : 'done',
        error: snapshot.turn.error,
      },
    ]
  }
  return applyCanSend(next)
}

// turn_started 归并（A5.1 §7.2）：同一 requestId 已有 Claude 回复是重复
// Turn；已有输入必须是本连接 accepted 且 source/text 一致；否则按事件
// 正文创建输入。最后只创建一个 Claude 回复。返回 null 表示协议错误。
function mergeTurnStarted(
  state: MobileState,
  event: DomainEvent,
): MobileState | null {
  const claudeIndex = state.messages.findIndex(
    (m) => m.kind === 'claude' && m.requestId === event.requestId,
  )
  if (claudeIndex >= 0) return null

  const inputIndex = state.messages.findIndex(
    (m) => m.kind === 'input' && m.requestId === event.requestId,
  )
  let messages = state.messages
  // decoder 已校验 turn_started.source 非空合法；此处收窄给消息模型。
  const source = event.source as InputMessage['source']
  if (inputIndex >= 0) {
    const existing = state.messages[inputIndex] as InputMessage
    if (existing.source !== event.source || existing.text !== event.text) {
      return null
    }
  } else {
    const input: InputMessage = {
      kind: 'input',
      source,
      requestId: event.requestId,
      text: event.text,
      status: null,
    }
    messages = [...state.messages, input]
  }
  messages = [
    ...messages,
    {
      kind: 'claude',
      requestId: event.requestId,
      source,
      text: '',
      status: 'streaming',
      error: null,
    },
  ]
  return { ...state, messages }
}

function applyEvent(state: MobileState, event: DomainEvent): MobileState {
  if (event.seq !== state.seqBase + 1) {
    // 重复、倒退或缺口：状态不可信，按协议错误处理（§6.2）。
    return { ...markPendingUnknown(state), phase: 'protocolError', canSend: false }
  }
  let next: MobileState = { ...state, seqBase: event.seq }

  switch (event.kind) {
    // session_* 事件按协议携带非空 sessionStatus（decoder 语义校验入口）。
    case 'session_started':
      next.sessionStatus = event.sessionStatus as MobileState['sessionStatus']
      next.sessionId = event.sessionId
      break
    case 'session_failed':
      next.sessionStatus = event.sessionStatus as MobileState['sessionStatus']
      next.sessionError = event.error
      // Session 终止后待处理事项与队列都不再可操作。
      next.pendingAction = null
      next.queuedRequests = []
      next.queueDepthHint = 0
      break
    case 'session_stopped':
      next.sessionStatus = event.sessionStatus as MobileState['sessionStatus']
      next.pendingAction = null
      next.queuedRequests = []
      next.queueDepthHint = 0
      break
    case 'turn_started': {
      const merged = mergeTurnStarted(next, event)
      if (merged === null) {
        return { ...markPendingUnknown(next), phase: 'protocolError', canSend: false }
      }
      // 排队项转为运行中 Turn 后移出列表。
      next = withoutQueued(merged, event.requestId)
      break
    }
    case 'text_delta': {
      // WhaleTerm PC UI 的 ${toolKey} 仅是内部工具卡片占位符，不是 Claude 正文。
      // 服务端已过滤；这里再做一次边界保护，避免旧 Host 把占位符显示给用户。
      if (/^\$\{[^{}]+\}$/.test(event.text.trim())) {
        return applyCanSend(next)
      }
      // 文本必须和结构化 block 共用一条有序流。若上一个条目仍是同一
      // Turn 的 Claude 文本，则继续合并流式增量；若中间已经出现 tool/
      // thinking，则新建文本片段，避免把后续文本搬到所有工具之后。
      let lastIndex = -1
      for (let i = next.messages.length - 1; i >= 0; i -= 1) {
        if (next.messages[i]?.kind === 'claude' && next.messages[i]?.requestId === event.requestId) {
          lastIndex = i
          break
        }
        if (next.messages[i]?.requestId === event.requestId) break
      }
      const last = lastIndex >= 0 ? next.messages[lastIndex] : undefined
      if (last !== undefined && last.kind === 'claude' && lastIndex === next.messages.length - 1) {
        next.messages = [
          ...next.messages.slice(0, lastIndex),
          { ...last, text: last.text + event.text },
          ...next.messages.slice(lastIndex + 1),
        ]
      } else if (last !== undefined || next.messages.some((m) => m.kind === 'claude' && m.requestId === event.requestId)) {
        next.messages = [
          ...next.messages,
          {
            kind: 'claude',
            requestId: event.requestId,
            source: event.source as ClaudeMessage['source'],
            text: event.text,
            status: 'streaming',
            error: null,
          },
        ]
      }
      break
    }
    case 'turn_completed':
    case 'turn_failed': {
      const status: ClaudeMessage['status'] =
        event.kind === 'turn_failed'
          ? 'failed'
          : event.turnStatus === 'cancelled'
            ? 'cancelled'
            : 'done'
      next.messages = next.messages.map((message) =>
        message.kind === 'claude' && message.requestId === event.requestId
          ? { ...message, status, error: event.error }
          : message,
      )
      // Turn 结束则待处理事项已无意义（服务端不保证补发 action_resolved）。
      next.pendingAction = null
      break
    }
    case 'content_block':
      // decoder 已保证 content_block 必携 block；此处只做类型收窄。
      if (event.block !== undefined) {
        next.messages = [
          ...next.messages,
          { kind: 'block', requestId: event.requestId, block: event.block },
        ]
      }
      break
    case 'action_required':
      next.pendingAction = event.pendingAction ?? null
      break
    case 'action_resolved':
      next.pendingAction = null
      // 回答型事项回带已校验答案（D52）；权限决议不带，不记一条空回显。
      if (event.answers !== undefined && event.answers.length > 0) {
        next.messages = [
          ...next.messages,
          { kind: 'answer', requestId: event.requestId, answers: event.answers },
        ]
      }
      break
    case 'permission_mode_changed':
      if (event.permissionMode === undefined) {
        return { ...next, phase: 'protocolError', canSend: false }
      }
      console.info('[remote] permission mode changed', {
        sessionId: event.sessionId,
        seq: event.seq,
        permissionMode: event.permissionMode,
      })
      next.permissionMode = event.permissionMode
      break
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
        pendingAction: null,
        queuedRequests: [],
        queueDepthHint: 0,
        pendingCommand: null,
        lastError: null,
        lastNotice: null,
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
        pendingCommand: {
          commandId: action.commandId,
          kind: action.kind,
          text: action.text,
          targetId: action.targetId,
        },
      })
    case 'noticeReceived':
      // 不支持通知也经服务端 publish 递增并占用一个 seq（controller.publish +
      // connection.outboundEvent 以 event.Seq 下行）：必须像普通事件一样做连续
      // 性校验并推进 seqBase，否则其后事件会在 applyEvent 处被误判为缺口而
      // 断连（真实 Claude 复测缺陷）。通知不改变连接阶段与可发送状态。
      if (action.seq !== state.seqBase + 1) {
        return { ...markPendingUnknown(state), phase: 'protocolError', canSend: false }
      }
      return {
        ...state,
        seqBase: action.seq,
        lastNotice: { code: action.code, message: NOTICE_MESSAGES[action.code] },
      }
    case 'commandAccepted': {
      const pending = state.pendingCommand
      if (pending === null || pending.commandId !== action.commandId) {
        return state
      }
      const next: MobileState = { ...state, pendingCommand: null }
      switch (pending.kind) {
        case 'send_text': {
          const accepted: InputMessage = {
            kind: 'input',
            source: 'mobile',
            requestId: action.requestId,
            text: pending.text,
            status: 'accepted',
          }
          next.messages = [...state.messages, accepted]
          break
        }
        case 'resolve_action':
          // 决议被接受即事项已处理，不等 action_resolved 就解除等待。
          next.pendingAction = null
          break
        case 'cancel_queued':
          // 取消排队不发领域事件，只能按 command_result 本地移除。
          return applyCanSend(withoutQueued(next, pending.targetId))
        case 'cancel_turn':
          // 取消 Turn 的可见变化由 turn_completed(cancelled) 带来。
          break
      }
      return applyCanSend(next)
    }
    case 'commandRejected': {
      const pending = state.pendingCommand
      if (pending === null || pending.commandId !== action.commandId) {
        return state
      }
      const next: MobileState = {
        ...state,
        pendingCommand: null,
        lastError: {
          code: action.code,
          message:
            REJECTION_MESSAGES[action.code] ??
            (action.message !== '' ? action.message : '操作被拒绝'),
        },
      }
      switch (action.code) {
        // 事项已不在待处理（双客户端竞态或超时）：清除本地待处理事项。
        case 'action_not_pending':
        case 'action_timeout':
          next.pendingAction = null
          break
        // 回答无效：事项仍待处理，保留供用户重填。
        case 'invalid_answer':
          break
        case 'request_not_queued':
          return applyCanSend(withoutQueued(next, pending.targetId))
      }
      return applyCanSend(next)
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
