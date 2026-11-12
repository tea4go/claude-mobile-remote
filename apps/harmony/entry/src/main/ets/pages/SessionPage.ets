import { PAGE_BACKGROUND, PRIMARY, SECONDARY } from '../ui/styles'
import { ActionMenu, type ActionMenuItem } from '../components/ActionMenu'
import { curves } from '@kit.ArkUI'
import { pageFade } from '../ui/transitions'
import { nativeLifecycle } from '../native/lifecycle'
import { messageSelection } from '../ui/messageSelection'
import { type ComposerDraft, emptyDraft } from '../session/attachments'
import type {
  Question,
  ProgressPhase,
  PermissionMode,
  QuestionAnswer,
} from '../protocol/types'
import type { HarmonySessionState } from '../session/model'
import { ConnectionStage } from '../session/model'
import { MessageComposer } from '../components/MessageComposer'
import { MessageList, ActivityOverlay } from '../components/MessageList'
import { PendingSummary } from '../components/PendingSummary'
import { QueuedSummary } from '../components/QueuedSummary'
import { type ActivityFeedGroup } from '../ui/feedDataSource'
import { StatusBar } from '../components/StatusBar'

function progressLabel(phase: ProgressPhase): string {
  switch (phase) {
    case 'waiting': return '等待响应'
    case 'working': return '处理中'
    case 'thinking': return '思考中'
    case 'writing': return '生成回复'
    case 'tool': return '调用工具'
    case 'subtask': return '执行子任务'
    case 'compacting': return '压缩上下文'
    case 'requesting': return '请求模型'
    case 'retrying': return '重试请求'
    case 'waiting_followup': return '等待后续响应'
    case 'tool_result': return '处理工具结果'
    case 'subtask_result': return '处理子任务结果'
  }
}

function findStreamingRequestId(state: HarmonySessionState): string {
  for (let index = state.messages.length - 1; index >= 0; index -= 1) {
    const message = state.messages[index]
    if (message.kind === 'claude' && message.status === 'streaming') {
      return message.requestId
    }
  }
  return ''
}

function currentCancelTarget(state: HarmonySessionState): string {
  if (state.pendingAction !== null) {
    return state.pendingAction.requestId
  }
  return findStreamingRequestId(state)
}

function hasAnswer(questionAnswers: Array<QuestionAnswer>, question: Question): boolean {
  for (let index = 0; index < questionAnswers.length; index += 1) {
    const answer = questionAnswers[index]
    if (answer.questionId !== question.questionId) {
      continue
    }
    if (question.inputType === 'text') {
      return answer.values.length > 0 && answer.values[0].trim().length > 0
    }
    return answer.values.length > 0
  }
  return false
}

@Component
export struct SessionPage {
  @Prop textOnly: boolean = false
  @State private previewImage: string = ''
  @State private now: number = Date.now()
  private clock: number = -1
  @Prop title: string = 'ccBuddy'
  @Prop @Watch('stateChanged') state: HarmonySessionState
  @Prop draft: ComposerDraft = emptyDraft()
  @Prop speechActive: boolean = false
  @Prop speechFinishing: boolean = false
  @Prop debugEnabled: boolean = false
  onDebugOpen: () => void = () => {}
  @Prop selecting: boolean = false
  onDraft: (text: string) => void = () => {}
  onSelect: (kind: string) => void = () => {}
  onRemoveAttachment: (index: number) => void = () => {}
  onClearAttachments: () => void = () => {}
  onSpeech: () => void = () => {}
  onStopSpeech: () => void = () => {}
  onCancelSpeech: () => void = () => {}
  @State private selectedActivity: ActivityFeedGroup | null = null
  @State private selectedStep: number = -1
  @State private sheet: string = ''
  @State private pageHeight: number = 720
  private menuController: CustomDialogController = new CustomDialogController({
    builder: ActionMenu({ title: '会话操作', items: this.menuItems(), onClose: (): void => { this.menuController.close() } }),
    customStyle: true, alignment: DialogAlignment.Bottom, autoCancel: true,
    openAnimation: { curve: curves.springMotion(0.25, 1) },
    closeAnimation: { curve: curves.springMotion(0.25, 1) },
  })
  private trackedAction: string = ''
  private readonly scroller: Scroller = new Scroller()
  @State private atBottom: boolean = true
  @Prop questionAnswers: Array<QuestionAnswer> = []
  @Prop capabilities: Array<string> = []
  @Prop managementBusy: boolean = false
  @Prop managementFeedback: string = ''
  onBack: () => void = () => {}
  onReconnect: () => void = () => {}
  onDisconnect: () => void = () => {}
  onSend: () => void = () => {}
  onPermissionMode: (mode: PermissionMode) => void = () => {}
  onAllowOnce: () => void = () => {}
  onAllowSession: () => void = () => {}
  onDeny: () => void = () => {}
  onCancelQuestion: () => void = () => {}
  onQuestionAnswerChange: (questionId: string, values: Array<string>) => void = () => {}
  onSubmitAnswers: () => void = () => {}
  onCancelTurn: (requestId: string) => void = () => {}
  onCancelQueued: (requestId: string) => void = () => {}
  onSteerQueued: (requestId: string) => void = () => {}
  onCreateSession: () => void = () => {}
  onStopSession: () => void = () => {}

  private menuItems(): Array<ActionMenuItem> {
    const items: Array<ActionMenuItem> = [
      { title: '重新连接', action: () => this.onReconnect() },
      { title: '断开连接', action: () => this.onDisconnect() },
    ]
    if (this.capabilities.indexOf('stop_session') >= 0) {
      items.push({ title: '终止 Claude 会话', danger: true, enabled: this.commandReady(), action: () => {
        if (this.commandReady()) this.onStopSession()
      } })
    }
    return items
  }

  private showReconnect(): boolean {
    return this.state.connection === ConnectionStage.DISCONNECTED ||
      this.state.connection === ConnectionStage.PROTOCOL_ERROR
  }

  private cancelTarget(): string {
    return currentCancelTarget(this.state)
  }

  private commandReady(): boolean {
    return this.state.connection === ConnectionStage.READY &&
      this.state.pendingCommand === null && !this.managementBusy
  }

  private cancelRequested(): boolean {
    return this.state.pendingCommand !== null && this.state.pendingCommand.kind === 'cancel_turn'
  }

  private answersReady(): boolean {
    const action = this.state.pendingAction
    if (action === null || action.kind !== 'ask_user_question') {
      return false
    }
    for (let index = 0; index < action.questions.length; index += 1) {
      if (!hasAnswer(this.questionAnswers, action.questions[index])) {
        return false
      }
    }
    return true
  }

  private hasCancelTarget(): boolean {
    return this.cancelTarget().length > 0
  }

  aboutToAppear(): void {
    this.stateChanged()
    setTimeout(() => { this.scroller.scrollEdge(Edge.Bottom); this.atBottom = true }, 150)
    nativeLifecycle.detailBack = this.backHandler
  }
  private readonly backHandler: () => boolean = () => {
      if (this.previewImage.length > 0) { this.previewImage = ''; return true }
      if (this.sheet.length === 0 && nativeLifecycle.messageBack?.()) return true
      if (this.sheet.length === 0) return false
      this.closeSheet(); return true
  }
  aboutToDisappear(): void {
    this.menuController.close()
    if (this.clock >= 0) clearInterval(this.clock)
    this.clock = -1
    if (nativeLifecycle.detailBack === this.backHandler) nativeLifecycle.detailBack = null
  }
  private stateChanged(): void {
    if (this.state.progress !== undefined && this.clock < 0) {
      this.now = Date.now()
      this.clock = setInterval(() => { this.now = Date.now() }, 1000)
    } else if (this.state.progress === undefined && this.clock >= 0) {
      clearInterval(this.clock)
      this.clock = -1
    }
    const id = this.state.pendingAction?.actionId ?? ''
    if (id.length > 0 && id !== this.trackedAction) this.sheet = 'action'
    if (id.length === 0 && this.sheet === 'action') this.sheet = ''
    if (this.sheet === 'queue' && this.state.queuedRequests.length === 0) this.sheet = ''
    this.trackedAction = id
  }
  private closeSheet(): void {
    if (this.sheet === 'action' && this.state.pendingAction !== null) {
      if (this.commandReady()) {
        if (this.state.pendingAction.kind === 'ask_user_question') this.onCancelQuestion()
        else this.onCancelTurn(this.state.pendingAction.requestId)
      }
      return
    }
    this.sheet = ''
  }
  @Builder
  private drawer() {
    Column() {
      Blank().width(36).height(5).backgroundColor('#d5deea').borderRadius(3).margin({ top: 10, bottom: 4 })
      Row() {
          Text(this.sheet === 'action' ? (this.state.pendingAction?.kind === 'ask_user_question' ? '需要你的回答' : '待处理事项') : `排队队列（${this.state.queuedRequests.length}）`).fontSize(18).layoutWeight(1)
        Button(this.sheet === 'action' ? (this.state.pendingAction?.kind === 'ask_user_question' ? '取消提问' : '取消本轮') : '关闭', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true)
          .enabled(this.sheet !== 'action' || this.commandReady()).onClick(() => this.closeSheet())
      }.padding(12)
      Scroll() {
        Column() {
          if (this.sheet === 'action' && this.state.pendingAction !== null) {
            PendingSummary({ pendingAction: this.state.pendingAction, questionAnswers: this.questionAnswers,
              actionEnabled: this.state.canResolveAction, cancelAllowed: this.commandReady(), answersReady: this.answersReady(),
              onAllowOnce: this.onAllowOnce, onAllowSession: this.onAllowSession, onDeny: this.onDeny,
              onQuestionAnswerChange: this.onQuestionAnswerChange, onSubmitAnswers: this.onSubmitAnswers,
              onCancelTurn: this.onCancelTurn, showSubmit: false })
          } else {
            QueuedSummary({ queuedRequests: this.state.queuedRequests, cancelAllowed: this.commandReady(),
              steerAllowed: this.state.canSteerQueue === true && this.hasCancelTarget() && this.state.pendingAction === null,
              onCancelQueued: this.onCancelQueued, onSteerQueued: this.onSteerQueued })
          }
        }.padding(16).width('100%')
      }.constraintSize({ maxHeight: Math.max(80, this.pageHeight * 0.75 -
        (this.sheet === 'action' && this.state.pendingAction?.kind === 'ask_user_question' ? 136 : 68)) }).width('100%').align(Alignment.TopStart)
      if (this.sheet === 'action' && this.state.pendingAction?.kind === 'ask_user_question') {
        Row() { Button('提交', { type: ButtonType.Normal }).height(52).width('100%').fontSize(16).fontColor('#ffffff').backgroundColor(PRIMARY).borderRadius(16)
          .enabled(this.state.canResolveAction && this.answersReady()).onClick(() => this.onSubmitAnswers()) }.width('100%').padding({ left: 16, right: 16, top: 8, bottom: 12 })
      }
    }.width('100%').backgroundColor('#ffffff').borderRadius({ topLeft: 20, topRight: 20 }).transition(pageFade())
  }
  build() {
    Stack({ alignContent: Alignment.Bottom }) {
      Column() {
        Row({ space: 8 }) {
          Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(24).height(24) }.width(44).height(44).padding(0).fontSize(30).fontColor(SECONDARY).backgroundColor('#ffffff').borderRadius(22).hoverEffect(HoverEffect.Highlight).clip(true).accessibilityText('返回会话列表').onClick(() => this.onBack())
          Column({ space: 3 }) {
            Text(this.title).fontSize(17).fontWeight(FontWeight.Medium).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%').textAlign(TextAlign.Center)
            StatusBar({ state: this.state })
          }.layoutWeight(1)
          if (this.showReconnect()) { Button('重连', { type: ButtonType.Normal }).height(44).padding(4).fontSize(12).fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).onClick(() => this.onReconnect()) }
          if (!this.textOnly) {
          Button('⋯', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(24).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).accessibilityText('更多会话操作').onClick(() => {
            this.menuController.open()
          })
          }
        }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%').backgroundColor(PAGE_BACKGROUND)
        if (this.state.connection !== ConnectionStage.READY && this.state.messages.length > 0) {
          Text('本机已收到的消息 · 当前状态待同步').fontSize(12).padding(6)
        }
        if (this.managementFeedback.length > 0) { Text(this.managementFeedback).fontSize(12).padding(6) }
        Row({ space: 8 }) {
          if (this.state.pendingAction !== null) { Button('待处理事项', { type: ButtonType.Normal }).height(44).fontSize(13).fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).onClick(() => { this.sheet = 'action' }) }
        }
        MessageList({ selectedActivity: $selectedActivity, selectedStep: $selectedStep, messages: this.state.messages, scroller: this.scroller, debugEnabled: this.debugEnabled, onDebugOpen: this.onDebugOpen,
          onAtBottom: (value: boolean) => { this.atBottom = value }, onPreviewImage: (value: string) => { this.previewImage = value } })
          .layoutWeight(1).width('100%')
        Row() {
          Blank()
          Button(this.atBottom ? '已到最新消息' : '最新消息 ↓', { type: ButtonType.Normal }).height(32).fontSize(12).fontColor(PRIMARY)
            .backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).enabled(!this.atBottom).onClick(() => this.scroller.scrollEdge(Edge.Bottom))
        }.padding({ right: 12 })
        Column({ space: 4 }) {
          if (this.state.progress !== undefined) {
            Row({ space: 6 }) {
              LoadingProgress().width(14).height(14).color(PRIMARY)
              Text(`${progressLabel(this.state.progress.phase)}${this.state.progress.toolName ? ' · ' + this.state.progress.toolName : ''} · ${Math.max(0, Math.floor((this.now - this.state.progress.startedAt) / 1000))}秒${this.state.progress.tokensKnown ? ' · ' + (this.state.progress.tokensEstimated ? '约' : '') + this.state.progress.outputTokens + ' tokens' : ''}`)
                .fontSize(12).fontColor(SECONDARY).layoutWeight(1)
            }.width('100%').padding({ left: 8, right: 8 })
          }
          MessageComposer({ textOnly: this.textOnly, draft: this.draft, composerEnabled: this.state.canSend,
            queueCount: this.state.canSteerQueue ? this.state.queuedRequests.length : 0,
            onQueue: () => { this.sheet = 'queue' },
            onPreviewImage: (value: string) => { this.previewImage = value },
            permissionMode: this.state.permissionMode,
            permissionEnabled: this.commandReady() && (this.state.sessionStatus === 'ready' || this.state.sessionStatus === 'starting'),
            onPermissionMode: this.onPermissionMode,
            createSessionAvailable: false,
            createSessionEnabled: false, running: this.hasCancelTarget(), cancelEnabled: this.commandReady(),
            speechActive: this.speechActive, speechFinishing: this.speechFinishing, selecting: this.selecting,
            onDraft: this.onDraft, onSelect: this.onSelect, onRemoveAttachment: this.onRemoveAttachment, onClearAttachments: this.onClearAttachments,
            onSpeech: this.onSpeech, onStopSpeech: this.onStopSpeech, onCancelSpeech: this.onCancelSpeech,
            onSend: this.onSend, onCreateSession: this.onCreateSession, onCancel: () => this.onCancelTurn(this.cancelTarget()) })
        }.padding({ left: 12, right: 12, top: 6, bottom: 12 }).width('100%')
      }.width('100%').height('100%').backgroundColor(PAGE_BACKGROUND)
      if (this.selectedActivity !== null) {
        ActivityOverlay({ selectedActivity: $selectedActivity, selectedStep: $selectedStep, messages: this.state.messages, pageHeight: this.pageHeight, debugEnabled: this.debugEnabled, onDebugOpen: this.onDebugOpen })
      }
      if (this.sheet.length > 0) {
        Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => { if (this.sheet !== 'action') this.closeSheet() }).transition(pageFade())
        this.drawer()
      }
      if (this.previewImage.length > 0) {
        Column() {
          Row() {
            Blank()
            Button('关闭', { type: ButtonType.Normal }).height(48).fontColor('#ffffff').backgroundColor(Color.Transparent)
              .onClick(() => { this.previewImage = '' })
          }.width('100%')
          Image(this.previewImage).width('100%').layoutWeight(1).objectFit(ImageFit.Contain)
        }.width('100%').height('100%').padding(12).backgroundColor('#ee000000')
      }
    }.width('100%').height('100%')
      .onTouch((event: TouchEvent) => {
        if (event.type === TouchType.Down && event.touches.length > 0) {
          messageSelection.outside(event.touches[0].windowX, event.touches[0].windowY)
        }
      })
      .onAreaChange((_old: Area, area: Area) => { this.pageHeight = Number(area.height) })
  }
}
