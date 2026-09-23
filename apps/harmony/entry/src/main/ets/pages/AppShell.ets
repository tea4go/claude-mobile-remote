import { runSelfChecks } from '../debug/selfCheck'
import {
  createAnswersResolution,
  createDecisionResolution,
  type ActionDecision,
  type ManagementResultInfo,
} from '../protocol/client'
import { decodePairingPayload } from '../protocol/pairing'
import type { PairingPayload, QuestionAnswer } from '../protocol/types'
import {
  createSessionStore,
  type HarmonySessionStore,
} from '../session/store'
import {
  ConnectionStage,
  createInitialHarmonySessionState,
  type HarmonySessionState,
} from '../session/model'
import {
  listSessionRecords,
  markSessionRecordStatus,
  pairingPayloadFromRecord,
  removeSessionRecord,
  renameSessionRecord,
  type SessionRecord,
  upsertSessionRecord,
} from '../session/records'
import { PairingPage } from './PairingPage'
import { SessionListPage } from './SessionListPage'
import { SessionPage } from './SessionPage'

type AppView = 'list' | 'pairing' | 'detail'

function defaultView(records: Array<SessionRecord>): AppView {
  return records.length > 0 ? 'list' : 'pairing'
}

function cloneAnswers(answers: Array<QuestionAnswer>): Array<QuestionAnswer> {
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

function findAnswerIndex(answers: Array<QuestionAnswer>, questionId: string): number {
  for (let index = 0; index < answers.length; index += 1) {
    if (answers[index].questionId === questionId) {
      return index
    }
  }
  return -1
}

function upsertAnswer(
  answers: Array<QuestionAnswer>,
  questionId: string,
  values: Array<string>,
): Array<QuestionAnswer> {
  const nextValues: Array<string> = []
  for (let index = 0; index < values.length; index += 1) {
    const trimmed = values[index].trim()
    if (trimmed.length > 0) {
      nextValues.push(trimmed)
    }
  }

  const index = findAnswerIndex(answers, questionId)
  const next = cloneAnswers(answers)
  if (nextValues.length === 0) {
    if (index >= 0) {
      next.splice(index, 1)
    }
    return next
  }

  const answer: QuestionAnswer = { questionId, values: nextValues }
  if (index >= 0) {
    next[index] = answer
    return next
  }
  next.push(answer)
  return next
}

@Component
export struct AppShell {
  private readonly entryStore: HarmonySessionStore = createSessionStore()
  private store: HarmonySessionStore = this.entryStore
  private readonly stores: Map<string, HarmonySessionStore> = new Map()
  private readonly storeSubscriptions: Map<string, () => void> = new Map()
  private readonly states: Map<string, HarmonySessionState> = new Map()
  private readonly managementPendingSessions: Set<string> = new Set()
  private unsubscribe: (() => void) | null = null

  @State private sessionState: HarmonySessionState = createInitialHarmonySessionState()
  @State private sessions: Array<SessionRecord> = []
  @State private view: AppView = 'pairing'
  @State private currentSessionId: string = ''
  @State private pairingInput: string = ''
  @State private questionAnswers: Array<QuestionAnswer> = []
  @State private trackedPendingActionId: string = ''
  @State private selfChecks: string[] = ['self-check-pending']
  @State private selfCheckError: string = ''
  @State private managementBusy: boolean = false
  @State private managementFeedback: string = ''

  aboutToAppear(): void {
    this.sessionState = this.store.getState()
    this.view = defaultView(this.sessions)
    if (this.unsubscribe === null) {
      this.unsubscribe = this.entryStore.subscribe((nextState: HarmonySessionState) => {
        if (this.view === 'pairing') {
          this.sessionState = nextState
        }
      })
    }

    try {
      const results = runSelfChecks()
      this.selfChecks = results.length > 0 ? results : ['self-check-empty']
      this.selfCheckError = ''
    } catch (error) {
      const message = error instanceof Error ? error.message : 'self-check failed'
      this.selfChecks = []
      this.selfCheckError = message
    }
  }

  aboutToDisappear(): void {
    if (this.unsubscribe !== null) {
      this.unsubscribe()
      this.unsubscribe = null
    }
    this.storeSubscriptions.forEach((unsubscribe: () => void) => unsubscribe())
    this.storeSubscriptions.clear()
    this.stores.forEach((store: HarmonySessionStore) => {
      store.setManagementListener(null)
      store.disconnect()
    })
    this.stores.clear()
    this.states.clear()
    this.managementPendingSessions.clear()
    this.store = this.entryStore
    this.currentSessionId = ''
    this.managementBusy = false
  }

  private handleStoreState(sessionId: string, nextState: HarmonySessionState): void {
    const previousState = this.states.get(sessionId) ?? createInitialHarmonySessionState()
    this.states.set(sessionId, nextState)
    if (sessionId === this.currentSessionId) {
      this.sessionState = nextState
      this.syncPendingAnswers(nextState)
    }

    if (
      nextState.connection === ConnectionStage.READY &&
      previousState.connection !== ConnectionStage.READY
    ) {
      this.sessions = markSessionRecordStatus(
        this.sessions,
        sessionId,
        'connected',
        Date.now(),
      )
      return
    }

    if (
      nextState.connection === ConnectionStage.PAIRING_INVALID &&
      previousState.connection !== ConnectionStage.PAIRING_INVALID
    ) {
      this.sessions = markSessionRecordStatus(this.sessions, sessionId, 'needsPairing')
      return
    }

    if (
      (nextState.connection === ConnectionStage.DISCONNECTED ||
        nextState.connection === ConnectionStage.PROTOCOL_ERROR) &&
      previousState.connection !== nextState.connection
    ) {
      this.sessions = markSessionRecordStatus(this.sessions, sessionId, 'disconnected')
    }
  }

  private connectSession(pairing: PairingPayload): HarmonySessionStore {
    let store = this.stores.get(pairing.sessionId)
    if (store === undefined) {
      store = createSessionStore()
      const sessionId = pairing.sessionId
      this.stores.set(sessionId, store)
      const unsubscribe = store.subscribe((state: HarmonySessionState) => this.handleStoreState(sessionId, state))
      this.storeSubscriptions.set(sessionId, unsubscribe)
      store.setManagementListener((result: ManagementResultInfo) => this.handleManagementResult(sessionId, result))
      store.connectWithPairing(pairing)
    }
    return store
  }

  private handleManagementResult(sourceSessionId: string, result: ManagementResultInfo): void {
    this.managementPendingSessions.delete(sourceSessionId)
    if (sourceSessionId === this.currentSessionId) {
      this.managementBusy = false
      this.managementFeedback = result.accepted
        ? (result.operation === 'stop_session' ? 'Claude 会话已终止；配对入口仍保留' : '')
        : (result.code === 'session_capacity' ? '活跃会话已达上限' : '操作失败，请重试')
    }
    if (result.accepted && result.operation === 'create_session' && result.pairing !== undefined) {
      this.sessions = upsertSessionRecord(this.sessions, result.pairing)
      this.currentSessionId = result.pairing.sessionId
      this.store = this.connectSession(result.pairing)
      this.sessionState = this.store.getState()
      this.managementBusy = this.managementPendingSessions.has(this.currentSessionId)
      this.managementFeedback = ''
      this.view = 'detail'
    }
  }

  private syncPendingAnswers(state: HarmonySessionState): void {
    if (state.pendingAction === null) {
      this.trackedPendingActionId = ''
      this.questionAnswers = []
      return
    }
    if (state.pendingAction.actionId !== this.trackedPendingActionId) {
      this.trackedPendingActionId = state.pendingAction.actionId
      this.questionAnswers = []
    }
  }

  private sortedSessions(): Array<SessionRecord> {
    return listSessionRecords(this.sessions)
  }

  private activeRecord(): SessionRecord | null {
    if (this.currentSessionId.length === 0) {
      return null
    }
    for (let index = 0; index < this.sessions.length; index += 1) {
      if (this.sessions[index].sessionId === this.currentSessionId) {
        return this.sessions[index]
      }
    }
    return null
  }

  private openPairingPage(): void {
    this.pairingInput = ''
    this.view = 'pairing'
  }

  private openListPage(): void {
    this.pairingInput = ''
    this.view = 'list'
  }

  private handleConnect(): void {
    const result = decodePairingPayload(this.pairingInput)
    if (!result.ok || result.payload === null) {
      this.entryStore.connectFromRawPairing(this.pairingInput)
      this.sessionState = this.entryStore.getState()
      return
    }
    this.sessions = upsertSessionRecord(this.sessions, result.payload)
    this.currentSessionId = result.payload.sessionId
    const existing = this.stores.has(result.payload.sessionId)
    this.store = this.connectSession(result.payload)
    if (existing) {
      this.store.connectWithPairing(result.payload)
    }
    this.sessionState = this.store.getState()
    this.managementBusy = this.managementPendingSessions.has(this.currentSessionId)
    this.managementFeedback = ''
    this.view = 'detail'
  }

  private handleOpenSession(sessionId: string): void {
    for (let index = 0; index < this.sessions.length; index += 1) {
      const record = this.sessions[index]
      if (record.sessionId === sessionId) {
        this.currentSessionId = sessionId
        this.view = 'detail'
        this.store = this.connectSession(pairingPayloadFromRecord(record))
        this.sessionState = this.store.getState()
        this.managementBusy = this.managementPendingSessions.has(sessionId)
        this.managementFeedback = ''
        if (this.sessionState.connection === ConnectionStage.DISCONNECTED) {
          this.store.reconnect()
        }
        return
      }
    }
  }

  private handleRemoveSession(sessionId: string): void {
    if (sessionId === this.currentSessionId) {
      this.currentSessionId = ''
      this.store = this.entryStore
      this.sessionState = createInitialHarmonySessionState()
    }
    const removed = this.stores.get(sessionId)
    if (removed !== undefined) {
      removed.setManagementListener(null)
      this.storeSubscriptions.get(sessionId)?.()
      removed.disconnect()
      this.stores.delete(sessionId)
      this.storeSubscriptions.delete(sessionId)
      this.states.delete(sessionId)
      this.managementPendingSessions.delete(sessionId)
    }
    this.sessions = removeSessionRecord(this.sessions, sessionId)
    if (this.sessions.length === 0) {
      this.view = 'pairing'
    }
  }

  private handleRenameSession(sessionId: string, name: string): void {
    this.sessions = renameSessionRecord(this.sessions, sessionId, name)
  }

  private handleBackFromDetail(): void {
    this.currentSessionId = ''
    this.questionAnswers = []
    this.trackedPendingActionId = ''
    this.managementBusy = false
    this.managementFeedback = ''
    this.view = defaultView(this.sessions)
  }

  private handleCreateSession(): void {
    if (this.store.createSession()) {
      this.managementPendingSessions.add(this.currentSessionId)
      this.managementBusy = true
      this.managementFeedback = ''
    }
  }

  private handleStopSession(): void {
    AlertDialog.show({
      title: '终止此 Claude 会话？',
      message: '将关闭对应的 Claude Client，之后无法在原会话继续输入。配对入口和终态会保留，其他会话不受影响。',
      primaryButton: {
        value: '终止会话',
        action: () => {
          if (this.store.stopSession()) {
            this.managementPendingSessions.add(this.currentSessionId)
            this.managementBusy = true
            this.managementFeedback = ''
          }
        },
      },
      secondaryButton: { value: '取消', action: () => {} },
    })
  }

  private handleSend(text: string): void {
    if (!this.store.sendText(text)) {
      this.sessionState = this.store.getState()
    }
  }

  private handleReconnect(): void {
    this.store.reconnect()
    this.sessionState = this.store.getState()
  }

  private handleDisconnect(): void {
    this.store.disconnect()
    this.sessionState = this.store.getState()
  }

  private handleCancelTurn(requestId: string): void {
    if (requestId.length === 0) {
      return
    }
    if (!this.store.cancelTurn(requestId)) {
      this.sessionState = this.store.getState()
    }
  }

  private handleCancelQueued(requestId: string): void {
    if (requestId.length === 0) {
      return
    }
    if (!this.store.cancelQueued(requestId)) {
      this.sessionState = this.store.getState()
    }
  }

  private handleResolveToolApproval(decision: ActionDecision): void {
    const action = this.sessionState.pendingAction
    if (action === null || action.kind !== 'tool_approval') {
      return
    }
    if (!this.store.resolveAction(createDecisionResolution(action.actionId, decision))) {
      this.sessionState = this.store.getState()
    }
  }

  private handleQuestionAnswerChange(questionId: string, values: Array<string>): void {
    this.questionAnswers = upsertAnswer(this.questionAnswers, questionId, values)
  }

  private handleSubmitQuestionAnswers(): void {
    const action = this.sessionState.pendingAction
    if (action === null || action.kind !== 'ask_user_question') {
      return
    }
    if (!this.store.resolveAction(createAnswersResolution(action.actionId, this.questionAnswers))) {
      this.sessionState = this.store.getState()
    }
  }

  private canBackFromPairing(): boolean {
    return this.sessions.length > 0
  }

  private hasActiveRecord(): boolean {
    return this.activeRecord() !== null
  }

  private activeRecordTitle(): string {
    const record = this.activeRecord()
    return record !== null ? record.name : 'Claude Mobile Remote'
  }

  @Builder
  private buildDetailPage(): void {
    SessionPage({
      title: this.activeRecordTitle(),
      state: this.sessionState,
      questionAnswers: this.questionAnswers,
      capabilities: this.activeRecord()?.capabilities ?? [],
      managementBusy: this.managementBusy,
      managementFeedback: this.managementFeedback,
      onCreateSession: () => this.handleCreateSession(),
      onStopSession: () => this.handleStopSession(),
      onBack: () => {
        this.handleBackFromDetail()
      },
      onReconnect: () => {
        this.handleReconnect()
      },
      onDisconnect: () => {
        this.handleDisconnect()
      },
      onSend: (text: string) => {
        this.handleSend(text)
      },
      onAllowOnce: () => {
        this.handleResolveToolApproval('allow_once')
      },
      onAllowSession: () => {
        this.handleResolveToolApproval('allow_session')
      },
      onDeny: () => {
        this.handleResolveToolApproval('deny')
      },
      onQuestionAnswerChange: (questionId: string, values: Array<string>) => {
        this.handleQuestionAnswerChange(questionId, values)
      },
      onSubmitAnswers: () => {
        this.handleSubmitQuestionAnswers()
      },
      onCancelTurn: (requestId: string) => {
        this.handleCancelTurn(requestId)
      },
      onCancelQueued: (requestId: string) => {
        this.handleCancelQueued(requestId)
      },
    })
  }

  @Builder
  private buildPairingView(): void {
    PairingPage({
      pairingInput: this.pairingInput,
      connection: this.sessionState.connection,
      lastError: this.sessionState.lastError,
      selfChecks: this.selfChecks,
      selfCheckError: this.selfCheckError,
      canBack: this.canBackFromPairing(),
      onBack: () => {
        this.openListPage()
      },
      onPairingInputChange: (value: string) => {
        this.pairingInput = value
      },
      onConnect: () => {
        this.handleConnect()
      },
    })
  }

  @Builder
  private buildListView(): void {
    SessionListPage({
      sessions: this.sortedSessions(),
      selfChecks: this.selfChecks,
      selfCheckError: this.selfCheckError,
      onAdd: () => {
        this.openPairingPage()
      },
      onOpen: (sessionId: string) => {
        this.handleOpenSession(sessionId)
      },
      onRemove: (sessionId: string) => {
        this.handleRemoveSession(sessionId)
      },
      onRename: (sessionId: string, name: string) => {
        this.handleRenameSession(sessionId, name)
      },
    })
  }

  @Builder
  private buildCurrentView(): void {
    if (this.view === 'detail' && this.hasActiveRecord()) {
      this.buildDetailPage()
    } else if (this.view === 'pairing') {
      this.buildPairingView()
    } else {
      this.buildListView()
    }
  }

  build() {
    Column() {
      this.buildCurrentView()
    }
    .width('100%')
    .height('100%')
  }
}
