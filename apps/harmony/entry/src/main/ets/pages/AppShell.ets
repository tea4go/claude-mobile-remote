import { NetworkMonitor } from '../native/network'
import { KeyboardAvoidMode, promptAction } from '@kit.ArkUI'
import { FeedbackFrame } from '../ui/feedbackFrame'
import { pageFade } from '../ui/transitions'
import { nativeLifecycle } from '../native/lifecycle'
import { NativeStorage } from '../native/storage'
import { SpeechDraft } from '../native/speech'
import { LocalNotifications } from '../native/notifications'
import { scanPairing, selectAttachments } from '../native/selection'
import { encodeCache, CacheWriteGate } from '../session/cache'
import { emptyDraft, validateAttachments, utf8Bytes, type ComposerDraft } from '../session/attachments'
import type { Attachment, PermissionMode } from '../protocol/types'
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
  markSessionRecordRead,
  pairingPayloadFromRecord,
  removeSessionRecord,
  renameSessionRecord,
  type SessionRecord,
  upsertSessionRecord,
  updateDisplayRecord,
} from '../session/records'
import { PairingPage } from './PairingPage'
import { SessionListPage } from './SessionListPage'
import { DemoPage } from './DemoPage'
import { DebugPanel } from '../components/DebugPanel'
import { debugLog } from '../debug/log'
import { SessionPage } from './SessionPage'
import { groupSessions, sessionPCKey, sessionPreview } from '../session/projectGroups'
import { projectMatches, createdPairingAddress } from '../session/projectPairing'
import { commitPairingRecords } from '../session/pairingCommit'

type AppView = 'list' | 'pairing' | 'pairingSettings' | 'detail'
interface DisplayPreferences { selectedPC: string | null; debugEnabled: boolean }
interface ProjectTarget { pcId: string; cwd: string }

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
  private entryStore: HarmonySessionStore = createSessionStore()
  private store: HarmonySessionStore = this.entryStore
  private readonly stores: Map<string, HarmonySessionStore> = new Map()
  private readonly storeSubscriptions: Map<string, () => void> = new Map()
  private readonly states: Map<string, HarmonySessionState> = new Map()
  private readonly managementPendingSessions: Set<string> = new Set()
  private unsubscribe: (() => void) | null = null

  private storage: NativeStorage | null = null
  @State private pushStatus: string = '远程通知已关闭'
  @State private pushConsent: boolean = false
  private notifications: LocalNotifications | null = null
  private readonly speech: SpeechDraft = new SpeechDraft()
  private readonly drafts: Map<string, ComposerDraft> = new Map()
  private readonly dirty: Set<string> = new Set()
  private readonly cacheGate: CacheWriteGate = new CacheWriteGate()
  private network: NetworkMonitor = new NetworkMonitor()
  private cacheTimer: number = -1
  private lifecycleUnsubscribe: (() => void) | null = null
  private initialized: boolean = false
  private nativeGeneration: number = 0
  private alive: boolean = false
  private foreground: boolean = true
  @State private demoOpen: boolean = false
  @State private demoOpening: boolean = false
  private demoGeneration: number = 0
  @State private debugOpen: boolean = false
  @State private debugEnabled: boolean = false
  @State private selectedPC: string = 'all'
  private preferencesReady: boolean = false
  private selectedPCConfigured: boolean = false
  @State private pairingTarget: string = ''
  private pendingPairing: PairingPayload | null = null
  @State private pairingSaving: boolean = false
  private projectTarget: ProjectTarget | null = null
  private createdSource: string = ''
  private debugUnsubscribe: (() => void) | null = null
  @State private nativeReady: boolean = false
  @State private nativeError: string = ''
  @State private notificationsEnabled: boolean = false
  @State private draft: ComposerDraft = emptyDraft()
  @State private speechActive: boolean = false
  @State private speechFinishing: boolean = false
  @State private selecting: boolean = false
  @State private scanning: boolean = false
  @State private clearingCache: boolean = false

  @State private sessionState: HarmonySessionState = createInitialHarmonySessionState()
  @State private sessions: Array<SessionRecord> = []
  @State private view: AppView = 'pairing'
  @State private currentSessionId: string = ''
  @State private recentlyViewedId: string = ''
  private listOffset: number = 0
  private listOrder: Array<string> = []
  @State private pairingInput: string = ''
  private pairingPreviewTimer: number = -1
  @State private questionAnswers: Array<QuestionAnswer> = []
  @State private trackedPendingActionId: string = ''
  @State private selfChecks: string[] = ['self-check-pending']
  @State private selfCheckError: string = ''
  @State private managementBusy: boolean = false
  @State private projectCreating: boolean = false
  @State private managementFeedback: string = ''

  aboutToAppear(): void {
    this.alive = true
    this.network.start((available: boolean) => {
      if (!this.alive) return
      if (available) { this.entryStore.checkConnection(); this.stores.forEach((store: HarmonySessionStore) => store.checkConnection()) }
      else { this.entryStore.networkLost(); this.stores.forEach((store: HarmonySessionStore) => store.networkLost()) }
    })
    this.debugEnabled = debugLog.enabled
    this.debugUnsubscribe = debugLog.subscribe(() => { if (this.debugEnabled !== debugLog.enabled) this.debugEnabled = debugLog.enabled })
    this.getUIContext().setKeyboardAvoidMode(KeyboardAvoidMode.RESIZE)
    nativeLifecycle.onBack = () => {
      if (this.debugOpen) { this.debugOpen = false; return true }
      if (this.demoOpening) { this.demoGeneration += 1; this.demoOpening = false; return true }
      if (this.demoOpen) { if (nativeLifecycle.detailBack?.()) return true; this.demoOpen = false; return true }
      if (nativeLifecycle.detailBack?.()) return true
      if (this.view === 'detail') { this.handleBackFromDetail(); return true }
      if (this.view === 'pairing' && this.sessions.length > 0) { this.openListPage(); return true }
      return false
    }
    this.sessionState = this.store.getState()
    this.view = defaultView(this.sessions)
    if (!this.initialized) { this.initialized = true; void this.initializeNative() }
    this.watchEntry()

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

  private watchEntry(): void {
    if (this.unsubscribe === null) {
      this.unsubscribe = this.entryStore.subscribe((nextState: HarmonySessionState) => {
        if (this.view === 'pairing') this.sessionState = nextState
        const pairing = this.pendingPairing
        if (pairing !== null && this.createdSource.length > 0 && nextState.lastError !== null &&
          (nextState.connection === ConnectionStage.DISCONNECTED || nextState.connection === ConnectionStage.PAIRING_INVALID || nextState.connection === ConnectionStage.PROTOCOL_ERROR)) {
          this.pendingPairing = null; this.createdSource = ''; this.projectTarget = null; this.projectCreating = false
          promptAction.showToast({ message: '电脑可能已创建会话，但手机连接失败；请在电脑检查后重新配对' })
          return
        }
        if (pairing !== null && nextState.connection === ConnectionStage.READY && this.foreground) {
          if (this.projectTarget !== null && !projectMatches(this.projectTarget.pcId, this.projectTarget.cwd, pairing, nextState.projectCwd)) {
            this.pendingPairing = null; this.projectCreating = false; this.entryStore.disconnect()
            this.managementFeedback = '无法验证目标电脑或工作目录，请检查后重新扫码'
            promptAction.showToast({ message: this.managementFeedback })
            return
          }
          if (!this.pairingSaving) { void this.completePairing(pairing) }
        }
      })
    }
  }

  private async completePairing(pairing: PairingPayload): Promise<void> {
    if (this.storage === null || this.pairingSaving) return
    this.pairingSaving = true
    const verified = this.entryStore
    const candidate = upsertSessionRecord(this.sessions, pairing)
    try {
      const result = await commitPairingRecords(this.storage, candidate, this.sessions, () =>
        this.alive && this.foreground && this.pendingPairing === pairing && this.entryStore === verified && verified.getState().connection === ConnectionStage.READY)
      if (result !== 'saved') {
        if (this.pendingPairing === pairing) this.pendingPairing = null
        verified.disconnect()
        this.nativeError = result === 'rollback_failed' ? '配对保存失败且恢复未完成，请核对本机配对后重试' :
          result === 'cancelled' ? '连接已中断，配对未保存，请重新扫码' : '配对保存失败，请重试；电脑可能已创建会话'
        if (this.alive) promptAction.showToast({ message: this.nativeError })
        return
      }
      this.projectTarget = null; this.createdSource = ''; this.pendingPairing = null
      this.sessions = upsertSessionRecord(this.sessions, pairing)
      if (!this.selectedPCConfigured) this.reconcileSelectedPC()
      if (this.selectedPC !== 'all') {
        const record = this.sessions.find((item: SessionRecord) => item.sessionId === pairing.sessionId)
        if (record !== undefined) { this.selectedPC = sessionPCKey(record); this.saveDisplayPreferences() }
      }
      this.currentSessionId = pairing.sessionId
      this.managementPendingSessions.delete(pairing.sessionId); this.managementBusy = false; this.managementFeedback = ''
      this.unsubscribe?.(); this.unsubscribe = null
      const previous = this.stores.get(pairing.sessionId)
      this.storeSubscriptions.get(pairing.sessionId)?.()
      previous?.setManagementListener(null); previous?.setEventListener(null); previous?.setSendListener(null); previous?.disconnect()
      this.stores.delete(pairing.sessionId)
      this.store = this.prepareStore(pairing.sessionId, verified)
      this.entryStore = createSessionStore(); this.watchEntry()
      this.handleStoreState(pairing.sessionId, verified.getState())
      this.sessionState = this.store.getState()
      this.view = 'detail'; this.pairingInput = ''; this.pairingTarget = ''; this.activateDraft()
    } catch (_) {
      if (this.pendingPairing === pairing) { this.pendingPairing = null; verified.disconnect() }
      this.nativeError = '配对保存失败，请检查后重试；电脑可能已创建会话'
      if (this.alive) promptAction.showToast({ message: this.nativeError })
    } finally { this.pairingSaving = false; this.projectCreating = false }
  }

  aboutToDisappear(): void {
    this.alive = false
    this.demoGeneration += 1
    this.demoOpening = false
    this.scanning = false
    if (this.pairingPreviewTimer >= 0) clearTimeout(this.pairingPreviewTimer)
    this.pairingPreviewTimer = -1
    this.network.stop()
    this.debugUnsubscribe?.(); this.debugUnsubscribe = null
    this.pendingPairing = null
    this.entryStore.disconnect()
    this.nativeGeneration += 1
    this.initialized = false
    this.nativeReady = false
    nativeLifecycle.onBack = null
    this.flushCache()
    this.lifecycleUnsubscribe?.(); this.lifecycleUnsubscribe = null
    void this.speech.cancel()

    if (this.unsubscribe !== null) {
      this.unsubscribe()
      this.unsubscribe = null
    }
    this.storeSubscriptions.forEach((unsubscribe: () => void) => unsubscribe())
    this.storeSubscriptions.clear()
    this.stores.forEach((store: HarmonySessionStore) => {
      store.setManagementListener(null)
      store.setEventListener(null)
      store.setSendListener(null)
      store.disconnect()
    })
    this.stores.clear()
    this.states.clear()
    this.managementPendingSessions.clear()
    this.store = this.entryStore
    this.currentSessionId = ''
    this.managementBusy = false
  }

  private async initializeNative(): Promise<void> {
    const generation = ++this.nativeGeneration
    const context = nativeLifecycle.context
    if (context === null) { this.nativeError = '原生环境尚未就绪'; return }
    try {
      this.storage = new NativeStorage(context)
      this.notifications = new LocalNotifications(context)
      this.notificationsEnabled = await this.storage.readNotificationsEnabled()
      this.notifications.enabled = this.notificationsEnabled
      await this.notifications.clear().catch(() => {})
      const records = await this.storage.loadRecords()
      if (!this.alive || generation !== this.nativeGeneration) return
      this.sessions = records
      try {
        const raw = await this.storage.readDisplayPreferences()
        if (raw !== null) {
          const prefs = JSON.parse(raw) as DisplayPreferences
          if ((prefs.selectedPC !== null && typeof prefs.selectedPC !== 'string') || typeof prefs.debugEnabled !== 'boolean') throw new Error('invalid preferences')
          this.selectedPC = prefs.selectedPC ?? 'all'; this.selectedPCConfigured = true
          debugLog.setEnabled(prefs.debugEnabled)
        }
        this.preferencesReady = true
        this.reconcileSelectedPC()
      } catch (_) { this.nativeError = '设置读取失败，本次使用默认设置' }
      // Team package ships before remote push is configured.
      this.pushStatus = '当前版本暂不支持后台推送'
      this.pushConsent = false
      for (const record of this.sessions) {
        const cached = await this.storage.readCache(record.sessionId)
        if (!this.alive || generation !== this.nativeGeneration) return
        const store = this.prepareStore(record.sessionId)
        if (cached !== null) store.restoreDisplay(cached)
      }
      this.view = defaultView(this.sessions)
      this.nativeReady = true
      this.lifecycleUnsubscribe = nativeLifecycle.subscribe(() => this.handleLifecycle())
      this.foreground = false
      this.handleLifecycle()
    } catch (_) {
      if (!this.alive || generation !== this.nativeGeneration) return
      this.nativeError = '读取本机数据失败，可重新扫码；原有记录未被覆盖'
      // Keep persisted records intact until the user explicitly chooses a new pairing.
      this.nativeReady = true
      if (this.lifecycleUnsubscribe === null) this.lifecycleUnsubscribe = nativeLifecycle.subscribe(() => this.handleLifecycle())
    }
  }

  private prepareStore(id: string, verified: HarmonySessionStore | null = null): HarmonySessionStore {
    const existing = this.stores.get(id)
    if (existing !== undefined) return existing
    const store = verified ?? createSessionStore()
    this.stores.set(id, store)
    this.storeSubscriptions.set(id, store.subscribe((state: HarmonySessionState) => this.handleStoreState(id, state)))
    store.setManagementListener((result: ManagementResultInfo) => this.handleManagementResult(id, result))
    // edo is the only notification source; no duplicate WebSocket local alert.
    store.setSendListener((accepted: boolean) => {
      const draft = this.drafts.get(id) ?? emptyDraft()
      const next: ComposerDraft = accepted ? emptyDraft() : { text: draft.text, attachments: draft.attachments, sending: false, error: '发送未确认，草稿已保留；请核对电脑端后再发送' }
      this.drafts.set(id, next)
      if (id === this.currentSessionId) this.draft = next
    })
    return store
  }

  private handleLifecycle(): void {
    const foreground = nativeLifecycle.foreground
    if (this.foreground !== foreground) {
      this.foreground = foreground
      if (this.notifications !== null) this.notifications.foreground = foreground
      if (!foreground) {
        void this.speech.cancel(); this.speechActive = false
        this.flushCache()
        debugLog.record('background')
        this.pendingPairing = null; this.projectCreating = false
        this.entryStore.disconnect(); this.entryStore.reset()
        this.stores.forEach((store: HarmonySessionStore) => store.disconnect())
        this.drafts.forEach((draft: ComposerDraft) => { draft.sending = false })
      } else {
        for (const record of this.sessions) {
          if (record.pairToken.length > 0 && Date.parse(record.expiresAt) > Date.now()) this.connectSession(pairingPayloadFromRecord(record), true)
          else this.sessions = markSessionRecordStatus(this.sessions, record.sessionId, 'needsPairing')
        }
      }
    }
    if (foreground) this.stores.forEach((store: HarmonySessionStore) => store.checkConnection())
    if (foreground && nativeLifecycle.pendingTopic.length > 0) {
      const topic = nativeLifecycle.pendingTopic; nativeLifecycle.pendingTopic = ''
      const target = ''
      if (target.length > 0) nativeLifecycle.pendingSessionId = target
      else this.nativeError = '通知对应的会话已移除或过期，请重新扫码'
    }
    const id = nativeLifecycle.pendingSessionId
    if (foreground && id.length > 0) {
      nativeLifecycle.pendingSessionId = ''
      if (this.sessions.some((record: SessionRecord) => record.sessionId === id)) this.handleOpenSession(id)
      else this.nativeError = '通知对应的会话已移除，请重新扫码'
    }
  }

  private async persistRecords(): Promise<void> {
    if (this.storage === null) throw new Error('secure storage unavailable')
    await this.storage.saveRecords(this.sessions)
  }
  private saveRecords(): void { void this.persistRecords().catch(() => { this.nativeError = '保存配对失败，请重试；当前连接仍可使用' }) }
  private scheduleCache(id: string): void {
    if (!this.nativeReady || this.storage === null) return
    this.dirty.add(id)
    if (this.cacheTimer < 0) this.cacheTimer = setTimeout(() => this.flushCache(), 1000)
  }
  private flushCache(): void {
    if (this.cacheTimer >= 0) clearTimeout(this.cacheTimer)
    this.cacheTimer = -1
    for (const id of this.dirty) {
      const state = this.states.get(id)
      if (state !== undefined && this.sessions.some((item: SessionRecord) => item.sessionId === id)) {
        const data = encodeCache(id, state, Date.now())
        if (this.cacheGate.changed(id, data)) void this.storage?.writeCache(id, data).catch(() => {
          this.cacheGate.forget(id); this.nativeError = '消息缓存写入失败'
        })
      }
    }
    this.dirty.clear()
  }
  private async clearCache(): Promise<void> {
    if (this.clearingCache || this.storage === null) return
    this.clearingCache = true
    this.nativeError = '正在清除本地消息缓存…'
    if (this.cacheTimer >= 0) clearTimeout(this.cacheTimer)
    this.cacheTimer = -1; this.dirty.clear()
    this.states.forEach((state: HarmonySessionState, id: string) => { this.cacheGate.changed(id, encodeCache(id, state, Date.now())) })
    try { await this.storage?.clearCache(); this.nativeError = '本地消息缓存已清除' }
    catch (_) { this.nativeError = '清除缓存失败，请重试' }
    finally { this.clearingCache = false }
  }
  private async toggleNotifications(): Promise<void> {
    if (this.notifications === null || this.storage === null) return
    try {
      if (this.notificationsEnabled) { this.notifications.enabled = false; await this.notifications.clear() }
      else { await this.notifications.enable() }
      this.pushConsent = await this.storage.readPushConsent()
      this.notificationsEnabled = this.notifications.enabled
      await this.storage.setNotificationsEnabled(this.notificationsEnabled)
    } catch (_) { this.nativeError = '通知未开启，请在系统设置中允许通知' }
  }
  private activateDraft(): void {
    void this.speech.cancel(); this.speechActive = false
    this.draft = this.drafts.get(this.currentSessionId) ?? emptyDraft()
    this.drafts.set(this.currentSessionId, this.draft)
    if (this.notifications !== null) this.notifications.currentId = this.view === 'detail' ? this.currentSessionId : ''
    if (this.view === 'detail') void this.notifications?.dismiss(this.currentSessionId).catch(() => {})
  }
  private updateDraft(text: string, attachments: Array<Attachment>, error: string = ''): void {
    const next: ComposerDraft = { text, attachments, sending: this.draft.sending, error }
    this.drafts.set(this.currentSessionId, next); this.draft = next
  }
  private async selectAttachment(kind: string): Promise<void> {
    const context = nativeLifecycle.context
    if (context === null || this.selecting || this.draft.sending) return
    const id = this.currentSessionId
    this.selecting = true
    try {
      const items = await selectAttachments(context, kind, 4 - this.draft.attachments.length)
      if (!this.alive || !this.sessions.some((item: SessionRecord) => item.sessionId === id)) return
      const draft = this.drafts.get(id) ?? emptyDraft()
      const attachments = draft.attachments.concat(items)
      const error = attachments.length > 0 ? validateAttachments(attachments) : ''
      const next: ComposerDraft = { text: draft.text, attachments: error.length === 0 ? attachments : draft.attachments, sending: false, error }
      this.drafts.set(id, next)
      if (id === this.currentSessionId) this.draft = next
    } catch (_) { if (id === this.currentSessionId) this.updateDraft(this.draft.text, this.draft.attachments, '未添加附件：选择已取消、文件超限或格式不可读取') }
    finally { this.selecting = false }
  }
  private startSpeech(): void {
    const context = nativeLifecycle.context
    if (context === null || this.draft.sending) return
    const id = this.currentSessionId
    const base = this.draft.text
    void this.speech.start(context, (text: string, active: boolean, error: string) => {
      if (!this.alive || id !== this.currentSessionId || this.view !== 'detail') return
      this.speechActive = active
      if (!active) this.speechFinishing = false
      this.updateDraft(base + (text.length > 0 ? (base.length > 0 ? ' ' : '') + text : ''), this.draft.attachments, error)
    })
  }
  private async scan(): Promise<void> {
    const context = nativeLifecycle.context
    if (context === null || !this.nativeReady || this.scanning) return
    this.scanning = true
    const generation = this.nativeGeneration
    try {
      await new Promise<void>((resolve: () => void) => this.getUIContext().postFrameCallback(new FeedbackFrame(resolve)))
      if (!this.alive || generation !== this.nativeGeneration || !this.foreground || this.view !== 'pairing') return
      const raw = await scanPairing(context)
      if (this.alive && generation === this.nativeGeneration && this.view === 'pairing' && !this.demoOpen && !this.demoOpening) {
        this.pairingInput = raw; this.handleConnect()
      }
    } catch (_) {
      if (this.alive && generation === this.nativeGeneration) this.nativeError = '扫码已取消或不可用，可粘贴配对信息'
    } finally { if (generation === this.nativeGeneration) this.scanning = false }
  }

  private openDemo(): void {
    if (this.demoOpen || this.demoOpening || this.scanning) return
    const generation = ++this.demoGeneration
    this.demoOpening = true
    this.getUIContext().postFrameCallback(new FeedbackFrame(() => {
      if (!this.alive || generation !== this.demoGeneration) return
      this.demoOpening = false
      if (this.foreground) this.demoOpen = true
    }))
  }

  private handleStoreState(sessionId: string, nextState: HarmonySessionState): void {
    const previousState = this.states.get(sessionId) ?? createInitialHarmonySessionState()
    this.states.set(sessionId, nextState)
    const queueCommand = previousState.pendingCommand?.kind === 'cancel_queued' || previousState.pendingCommand?.kind === 'steer_queued'
    if (sessionId === this.currentSessionId && this.view === 'detail' && queueCommand && nextState.pendingCommand === null && nextState.lastError !== null) {
      promptAction.showToast({ message: nextState.lastError.message })
    }
    if (nextState.messages !== previousState.messages || nextState.projectCwd !== previousState.projectCwd || nextState.sessionTitle !== previousState.sessionTitle) this.scheduleCache(sessionId)
    if (nextState.connection === ConnectionStage.READY && previousState.connection !== ConnectionStage.READY) this.notifications?.seed(sessionId, nextState.seqBase)
    const previousRecord = this.sessions.find((record: SessionRecord) => record.sessionId === sessionId)
    let runtimeStatus = '空闲'
    if (nextState.sessionStatus === 'failed') runtimeStatus = '会话失败'
    else if (nextState.sessionStatus === 'stopped') runtimeStatus = '已结束'
    else if (nextState.pendingAction !== null) runtimeStatus = '等待处理'
    else if (nextState.messages.some((message) => message.kind === 'claude' && message.status === 'streaming')) runtimeStatus = '进行中'
    else if (nextState.messages.some((message) => message.kind === 'claude' && message.status === 'done')) runtimeStatus = '已完成'
    const currentLast = nextState.messages[nextState.messages.length - 1]
    const previousLast = previousState.messages[previousState.messages.length - 1]
    let changedText = false
    if (currentLast !== undefined && previousLast !== undefined && currentLast.kind === 'claude' && previousLast.kind === 'claude') {
      changedText = currentLast.text !== previousLast.text
    }
    const newContent = nextState.messages.length > previousState.messages.length || changedText
    const unread = (previousRecord?.unread ?? false) ||
      (newContent && !(this.view === 'detail' && this.currentSessionId === sessionId))
    let preview = ''
    for (let index = nextState.messages.length - 1; index >= 0; index -= 1) {
      const message = nextState.messages[index]
      if (message.kind === 'input' || message.kind === 'claude') {
        preview = sessionPreview(message.text)
        if (preview.length > 0) break
      }
    }
    this.sessions = updateDisplayRecord(this.sessions, sessionId, nextState.projectCwd,
      nextState.sessionTitle, nextState.createdAt, nextState.lastActivityAt, preview, runtimeStatus, unread)
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

  private connectSession(pairing: PairingPayload, force: boolean = false): HarmonySessionStore {
    const existing = this.stores.has(pairing.sessionId)
    const store = this.prepareStore(pairing.sessionId)
    if ((!existing || force || store.getState().pairing === null) && this.foreground && pairing.pairToken.length > 0 && Date.parse(pairing.expiresAt) > Date.now()) store.connectWithPairing(pairing)
    return store
  }

  private handleManagementResult(sourceSessionId: string, result: ManagementResultInfo): void {
    this.managementPendingSessions.delete(sourceSessionId)
    if (sourceSessionId === this.currentSessionId) {
      this.managementBusy = false
      this.managementFeedback = result.accepted
        ? (result.operation === 'stop_session' ? 'Claude 会话已终止；配对入口仍保留' : '')
        : (result.code === 'session_capacity' ? '活跃会话已达上限' : result.code === 'result_unknown' ? '操作结果未知，请核对电脑端状态' : '操作失败，请重试')
    }
    if (result.accepted && result.operation === 'create_session' && result.pairing === undefined) {
      this.projectCreating = false
      promptAction.showToast({ message: '电脑可能已创建会话，但未返回有效配对信息；请在电脑检查后重新配对' })
    }
    if (!result.accepted && result.operation === 'create_session') this.projectCreating = false
    if (!result.accepted && result.operation === 'create_session' && this.view === 'list') {
      const message = result.code === 'result_unknown' ? '操作结果未知，请在电脑检查后再试' : result.code === 'session_capacity' ? '活跃会话已达上限' : '新建失败，请检查连接后重试'
      promptAction.showToast({ message })
    }
    if (result.accepted && result.operation === 'create_session' && result.pairing !== undefined) {
      const source = this.sessions.find((record: SessionRecord) => record.sessionId === sourceSessionId)
      const child = result.pairing
      const address = source === undefined ? null : createdPairingAddress(source.wsUrl, child.wsUrl)
      if (source === undefined || address === null || !source.pcId || !source.projectCwd || child.pcId !== source.pcId || this.pendingPairing !== null) {
        this.projectCreating = false
        this.managementFeedback = '电脑可能已创建会话，但无法验证连接；请在电脑检查后重新配对'
        promptAction.showToast({ message: this.managementFeedback })
        return
      }
      const candidate: PairingPayload = { version: child.version, wsUrl: address, sessionId: child.sessionId, pairToken: child.pairToken, expiresAt: child.expiresAt, capabilities: child.capabilities, pcId: child.pcId, pcName: child.pcName }
      const parsed = decodePairingPayload(JSON.stringify(candidate))
      if (!parsed.ok || parsed.payload === null) {
        this.projectCreating = false
        this.managementFeedback = '电脑可能已创建会话，但连接地址无效；请在电脑检查后重新配对'
        promptAction.showToast({ message: this.managementFeedback })
        return
      }
      this.entryStore.reset()
      this.projectTarget = { pcId: source.pcId, cwd: source.projectCwd }
      this.createdSource = sourceSessionId
      this.pendingPairing = parsed.payload
      this.entryStore.connectWithPairing(parsed.payload)
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

  private saveDisplayPreferences(): void {
    if (!this.preferencesReady || this.storage === null) return
    const prefs: DisplayPreferences = { selectedPC: this.selectedPC === 'all' ? null : this.selectedPC, debugEnabled: this.debugEnabled }
    void this.storage.writeDisplayPreferences(JSON.stringify(prefs)).catch(() => { this.nativeError = '设置保存失败' })
  }
  private reconcileSelectedPC(): void {
    const groups = groupSessions(this.sessions)
    const missing = this.selectedPC !== 'all' && !groups.some((pc) => pc.key === this.selectedPC)
    if (!this.selectedPCConfigured || missing) {
      this.selectedPC = groups.length === 1 ? groups[0].key : 'all'
      if (groups.length > 0) { this.selectedPCConfigured = true; this.saveDisplayPreferences() }
    }
  }
  private sortedSessions(): Array<SessionRecord> {
    const sorted = listSessionRecords(this.sessions)
    if (this.listOrder.length === 0) return sorted
    const positions: Map<string, number> = new Map()
    this.listOrder.forEach((id: string, index: number) => positions.set(id, index))
    sorted.sort((left, right) => {
      const leftIndex = positions.get(left.sessionId) ?? -1
      const rightIndex = positions.get(right.sessionId) ?? -1
      if (leftIndex < 0 && rightIndex < 0) return 0
      if (leftIndex < 0) return 1
      if (rightIndex < 0) return -1
      return leftIndex - rightIndex
    })
    return sorted
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
    if (this.pairingSaving) return
    this.projectTarget = null; this.createdSource = ''
    this.pendingPairing = null
    this.entryStore.reset()
    this.pairingTarget = ''
    this.sessionState = this.entryStore.getState()
    this.pairingInput = ''
    this.view = 'pairing'
    this.activateDraft()
  }

  private openListPage(): void {
    if (this.pairingSaving) return
    this.projectTarget = null; this.createdSource = ''
    this.pendingPairing = null; this.entryStore.disconnect()
    this.recentlyViewedId = this.currentSessionId
    this.listOrder = listSessionRecords(this.sessions).map((record: SessionRecord) => record.sessionId)
    this.pairingInput = ''
    this.view = 'list'
    this.activateDraft()
  }

  private handleConnect(): void {
    if (!this.nativeReady || this.pairingSaving) return
    this.pendingPairing = null
    this.entryStore.reset()
    const result = decodePairingPayload(this.pairingInput)
    if (!result.ok || result.payload === null) {
      this.entryStore.connectFromRawPairing(this.pairingInput)
      this.sessionState = this.entryStore.getState()
      return
    }
    const existing = this.stores.get(result.payload.sessionId)
    if (existing !== undefined) this.entryStore.restoreDisplay(existing.getState())
    this.pendingPairing = result.payload
    this.pairingTarget = debugLog.target(result.payload.wsUrl)
    this.entryStore.connectWithPairing(result.payload)
    this.sessionState = this.entryStore.getState()
  }

  private handleOpenSession(sessionId: string): void {
    for (let index = 0; index < this.sessions.length; index += 1) {
      const record = this.sessions[index]
      if (record.sessionId === sessionId) {
        this.sessions = markSessionRecordRead(this.sessions, sessionId)
        this.currentSessionId = sessionId
        this.view = 'detail'
        this.activateDraft()
        this.store = this.connectSession(pairingPayloadFromRecord(record))
        this.sessionState = this.store.getState()
        this.syncPendingAnswers(this.sessionState)
        this.managementBusy = this.managementPendingSessions.has(sessionId)
        this.managementFeedback = ''
        if (this.sessionState.connection === ConnectionStage.DISCONNECTED && record.status !== 'needsPairing') {
          this.store.reconnect()
        }
        return
      }
    }
  }

  private handleRemoveSession(sessionId: string): void {
    if (this.pairingSaving) return
    this.cacheGate.forget(sessionId)
    this.dirty.delete(sessionId); this.drafts.delete(sessionId)
    void this.notifications?.remove(sessionId).catch(() => {})
    void this.storage?.removeCache(sessionId).catch(() => { this.nativeError = '删除缓存失败，请清除全部缓存' })

    if (sessionId === this.currentSessionId) {
      this.currentSessionId = ''
      this.store = this.entryStore
      this.sessionState = createInitialHarmonySessionState()
    }
    const removed = this.stores.get(sessionId)
    if (removed !== undefined) {
      removed.setManagementListener(null)
      removed.setEventListener(null)
      removed.setSendListener(null)
      this.storeSubscriptions.get(sessionId)?.()
      removed.disconnect()
      this.stores.delete(sessionId)
      this.storeSubscriptions.delete(sessionId)
      this.states.delete(sessionId)
      this.managementPendingSessions.delete(sessionId)
    }
    this.sessions = removeSessionRecord(this.sessions, sessionId)
    this.reconcileSelectedPC()
    this.saveRecords()
    if (this.sessions.length === 0) {
      this.view = 'pairing'
    }
  }

  private handleRenameSession(sessionId: string, name: string): void {
    if (this.pairingSaving) return
    this.sessions = renameSessionRecord(this.sessions, sessionId, name)
    this.saveRecords()
  }

  private handleBackFromDetail(): void {
    this.recentlyViewedId = this.currentSessionId
    this.listOrder = listSessionRecords(this.sessions).map((record: SessionRecord) => record.sessionId)
    this.currentSessionId = ''
    this.questionAnswers = []
    this.trackedPendingActionId = ''
    this.managementBusy = false
    this.managementFeedback = ''
    this.view = defaultView(this.sessions)
    this.activateDraft()
  }

  private handleCreateSession(): void {
    if (this.store.createSession()) {
      this.managementPendingSessions.add(this.currentSessionId)
      this.managementBusy = true
      this.managementFeedback = ''
    }
  }

  private handleStopSession(): void {
    const id = this.currentSessionId
    const store = this.store
    AlertDialog.show({
      title: '终止此 Claude 会话？',
      message: '将关闭对应的 Claude Client，之后无法在原会话继续输入。配对入口和终态会保留，其他会话不受影响。',
      primaryButton: {
        value: '终止会话',
        action: () => {
          if (this.stores.get(id) !== store) return
          if (store.stopSession()) {
            this.managementPendingSessions.add(id)
            if (this.currentSessionId === id) {
              this.managementBusy = true
              this.managementFeedback = ''
            }
          }
        },
      },
      secondaryButton: { value: '取消', action: () => {} },
    })
  }

  private handleSend(): void {
    if (!this.sessionState.canSend || this.draft.sending || this.speechActive || this.selecting) return
    const text = this.draft.text.trim()
    if (utf8Bytes(text) > 1024 * 1024 || (text.length === 0 && this.draft.attachments.length === 0)) return
    const next: ComposerDraft = { text: this.draft.text, attachments: this.draft.attachments, sending: true, error: '' }
    this.drafts.set(this.currentSessionId, next); this.draft = next
    const sent = this.draft.attachments.length > 0 ? this.store.sendAttachments(text, this.draft.attachments) : this.store.sendText(text)
    if (!sent) { this.draft.sending = false; this.updateDraft(this.draft.text, this.draft.attachments, '发送失败，草稿已保留') }
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
      promptAction.showToast({ message: '无法取消排队，请检查连接后重试' })
    }
  }

  private handleSteerQueued(requestId: string): void {
    if (!this.store.steerQueued(requestId)) promptAction.showToast({ message: '无法引导队列，请检查连接后重试' })
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
    return record !== null ? record.name : 'Whale Remote Control'
  }

  @Builder
  private buildDetailPage(): void {
    SessionPage({
      title: this.activeRecordTitle(),
      state: this.sessionState,
      draft: this.draft,
      speechActive: this.speechActive,
      speechFinishing: this.speechFinishing,
      debugEnabled: this.debugEnabled,
      onDebugOpen: () => { this.debugOpen = true },
      selecting: this.selecting,
      onDraft: (text: string) => this.updateDraft(text, this.draft.attachments),
      onSelect: (kind: string) => { void this.selectAttachment(kind) },
      onRemoveAttachment: (index: number) => this.updateDraft(this.draft.text, this.draft.attachments.filter((_item: Attachment, itemIndex: number) => itemIndex !== index)),
      onClearAttachments: () => this.updateDraft(this.draft.text, []),
      onSpeech: () => this.startSpeech(),
      onStopSpeech: () => { if (!this.speechFinishing) { this.speechFinishing = true; void this.speech.stop() } },
      onCancelSpeech: () => { void this.speech.cancel(); this.speechActive = false; this.speechFinishing = false },
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
      onSend: () => this.handleSend(),
      onPermissionMode: (mode: PermissionMode) => { this.store.setPermissionMode(mode) },
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
      onSteerQueued: (requestId: string) => this.handleSteerQueued(requestId),
    })
  }

  @Builder
  private buildPairingView(): void {
    PairingPage({
      onDemo: () => this.openDemo(),
      onSettings: () => { this.view = 'pairingSettings' },
      projectCwd: this.projectTarget?.cwd ?? '',
      saving: this.pairingSaving,
      onCancel: () => { if (!this.pairingSaving) { this.pendingPairing = null; this.entryStore.disconnect(); this.pairingTarget = '' } },
      target: this.pairingTarget,
      pairingInput: this.pairingInput,
      nativeReady: this.nativeReady && !this.pairingSaving,
      scanning: this.scanning,
      onScan: () => { void this.scan() },
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
        if (this.pairingPreviewTimer >= 0) clearTimeout(this.pairingPreviewTimer)
        this.pairingTarget = ''
        if (value.length <= 16384) this.pairingPreviewTimer = setTimeout(() => {
          if (!this.alive || this.view !== 'pairing' || this.pairingInput !== value) return
          const parsed = decodePairingPayload(value)
          this.pairingTarget = parsed.ok && parsed.payload !== null ? debugLog.target(parsed.payload.wsUrl) : ''
        }, 200)
      },
      onConnect: () => {
        this.handleConnect()
      },
    })
  }

  @Builder
  private buildListView(): void {
    SessionListPage({
      initialSettings: this.view === 'pairingSettings',
      onExitSettings: () => { this.view = 'pairing' },
      onDemo: () => this.openDemo(),
      debugEnabled: this.debugEnabled,
      onDebug: (value: boolean) => { debugLog.setEnabled(value); this.saveDisplayPreferences() },
      selectedPC: this.selectedPC,
      projectCreating: this.projectCreating,
      onSelectPC: (key: string) => { this.selectedPC = key; this.selectedPCConfigured = true; this.saveDisplayPreferences() },
      onCreateProject: (pcKey: string, cwd: string, sourceId: string) => {
        if (cwd.length === 0 || !/^[0-9a-f]{32}$/.test(pcKey)) {
          promptAction.showToast({ message: '无法验证电脑或工作目录，请重新配对后重试' }); return
        }
        if (sourceId.length === 0) {
          const hasConnected = this.sessions.some((record: SessionRecord) => sessionPCKey(record) === pcKey && record.projectCwd === cwd && record.status === 'connected')
          if (hasConnected) { promptAction.showToast({ message: '当前会话不支持新建' }); return }
          this.openPairingPage(); this.projectTarget = { pcId: pcKey, cwd }
          promptAction.showToast({ message: '请扫描目标电脑和目录的新会话配对码' }); return
        }
        const source = this.stores.get(sourceId)
        if (source === undefined || this.projectCreating || this.pendingPairing !== null || this.managementPendingSessions.has(sourceId) || !source.createSession()) {
          promptAction.showToast({ message: '暂时无法新建会话，请检查连接后重试' }); return
        }
        this.managementPendingSessions.add(sourceId); this.projectCreating = true
      },
      onDebugOpen: () => { this.debugOpen = true },
      sessions: this.sortedSessions(),
      scrollOffset: this.listOffset,
      onScrollOffset: (value: number) => { this.listOffset = value },
      notificationsEnabled: this.notificationsEnabled,
      pushStatus: this.pushStatus,
      pushConsent: this.pushConsent,
      onEnablePush: () => {
        AlertDialog.show({ title: '后台通知', message: '当前版本暂不支持后台推送。', confirm: { value: '知道了', action: () => {} } })
      },
      onNotifications: () => { void this.toggleNotifications() },
      onClearCache: () => { void this.clearCache() },
      onRemovePC: (pcKey: string) => {
        const ids = this.sessions.filter((record: SessionRecord) => (record.pcId ?? record.wsUrl) === pcKey).map((record: SessionRecord) => record.sessionId)
        ids.forEach((id: string) => this.handleRemoveSession(id))
      },
      recentlyViewedId: this.recentlyViewedId,
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
      Column() { this.buildDetailPage() }.width('100%').height('100%').transition(pageFade())
    } else if (this.view === 'pairing') {
      Column() { this.buildPairingView() }.width('100%').height('100%').transition(pageFade())
    } else {
      Column() { this.buildListView() }.width('100%').height('100%').transition(pageFade())
    }
  }

  build() {
    Stack({ alignContent: Alignment.Bottom }) {
    Column() {
      if (this.nativeError.length > 0) {
        Text(this.nativeError).fontSize(13).fontColor('#a54b00').padding(8).onClick(() => { this.nativeError = '' })
      }
      if (!this.nativeReady) { LoadingProgress().width(32).height(32) }
      Stack() {
      if (this.demoOpening) {
        Column({ space: 12 }) {
          LoadingProgress().width(32).height(32)
          Text('正在打开离线演示…').fontSize(14)
        }.height('100%').width('100%').justifyContent(FlexAlign.Center).transition(pageFade())
      } else if (this.demoOpen) {
        DemoPage({ onExit: () => { this.demoOpen = false } }).width('100%').height('100%').transition(pageFade())
      } else { Stack() { this.buildCurrentView() }.width('100%').height('100%').transition(pageFade()) }
      }.layoutWeight(1).width('100%').clip(true)
    }
    .width('100%')
    .height('100%')
    if (this.debugEnabled && !this.debugOpen && !this.demoOpen && !this.demoOpening) {
      Row() { Blank(); Button('⌘', { type: ButtonType.Normal }).width(36).height(36).padding(0).fontSize(18).accessibilityText('查看调试日志').onClick(() => { this.debugOpen = true }) }.width('100%').position({ x: 0, y: '60%' })
    }
    if (this.debugOpen) {
      Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => { this.debugOpen = false }).transition(pageFade())
      DebugPanel({ onClose: () => { this.debugOpen = false }, onEnabledChange: (value: boolean) => { debugLog.setEnabled(value); this.saveDisplayPreferences() } }).transition(pageFade())
    }
    }.width('100%').height('100%')
  }
}
