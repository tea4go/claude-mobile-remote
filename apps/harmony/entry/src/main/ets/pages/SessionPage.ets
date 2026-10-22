import { nativeLifecycle } from '../native/lifecycle'
import { type ComposerDraft, emptyDraft } from '../session/attachments'
import type {
  Question,
  PermissionMode,
  QuestionAnswer,
} from '../protocol/types'
import type { HarmonySessionState } from '../session/model'
import { ConnectionStage } from '../session/model'
import { MessageComposer } from '../components/MessageComposer'
import { MessageList } from '../components/MessageList'
import { PendingSummary } from '../components/PendingSummary'
import { QueuedSummary } from '../components/QueuedSummary'
import { StatusBar } from '../components/StatusBar'

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
  @Prop title: string = 'Claude Mobile Remote'
  @Prop @Watch('stateChanged') state: HarmonySessionState
  @Prop draft: ComposerDraft = emptyDraft()
  @Prop speechActive: boolean = false
  @Prop selecting: boolean = false
  onDraft: (text: string) => void = () => {}
  onSelect: (kind: string) => void = () => {}
  onRemoveAttachment: (index: number) => void = () => {}
  onSpeech: () => void = () => {}
  onStopSpeech: () => void = () => {}
  onCancelSpeech: () => void = () => {}
  @State private sheet: string = ''
  private trackedAction: string = ''
  private readonly scroller: Scroller = new Scroller()
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
  onQuestionAnswerChange: (questionId: string, values: Array<string>) => void = () => {}
  onSubmitAnswers: () => void = () => {}
  onCancelTurn: (requestId: string) => void = () => {}
  onCancelQueued: (requestId: string) => void = () => {}
  onCreateSession: () => void = () => {}
  onStopSession: () => void = () => {}

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
    nativeLifecycle.detailBack = () => {
      if (this.sheet.length === 0) return false
      this.closeSheet(); return true
    }
  }
  aboutToDisappear(): void { nativeLifecycle.detailBack = null }
  private stateChanged(): void {
    const id = this.state.pendingAction?.actionId ?? ''
    if (id.length > 0 && id !== this.trackedAction) this.sheet = 'action'
    if (id.length === 0 && this.sheet === 'action') this.sheet = ''
    this.trackedAction = id
  }
  private closeSheet(): void {
    if (this.sheet === 'action' && this.state.pendingAction !== null) {
      if (this.commandReady()) this.onCancelTurn(this.state.pendingAction.requestId)
      return
    }
    this.sheet = ''
  }
  @Builder
  private drawer() {
    Column({ space: 8 }) {
      Row() {
        Text(this.sheet === 'action' ? '待处理事项' : '排队消息').fontSize(18).layoutWeight(1)
        Button(this.sheet === 'action' ? '取消本轮' : '关闭').height(44)
          .enabled(this.sheet !== 'action' || this.commandReady()).onClick(() => this.closeSheet())
      }.padding(12)
      Scroll() {
        Column() {
          if (this.sheet === 'action' && this.state.pendingAction !== null) {
            PendingSummary({ pendingAction: this.state.pendingAction, questionAnswers: this.questionAnswers,
              actionEnabled: this.state.canResolveAction, cancelAllowed: this.commandReady(), answersReady: this.answersReady(),
              onAllowOnce: this.onAllowOnce, onAllowSession: this.onAllowSession, onDeny: this.onDeny,
              onQuestionAnswerChange: this.onQuestionAnswerChange, onSubmitAnswers: this.onSubmitAnswers,
              onCancelTurn: this.onCancelTurn })
          } else {
            QueuedSummary({ queuedRequests: this.state.queuedRequests, cancelAllowed: this.commandReady(), onCancelQueued: this.onCancelQueued })
          }
        }.padding(16).width('100%')
      }.layoutWeight(1).width('100%')
    }.height('70%').width('100%').backgroundColor('#ffffff').borderRadius({ topLeft: 20, topRight: 20 })
  }
  build() {
    Stack({ alignContent: Alignment.Bottom }) {
      Column() {
        Row({ space: 8 }) {
          Button('‹').width(44).height(44).accessibilityText('返回会话列表').onClick(() => this.onBack())
          Text(this.title).fontSize(18).fontWeight(FontWeight.Bold).maxLines(1).layoutWeight(1)
          Button('更多').height(44).onClick(() => {
            ActionSheet.show({ title: '会话操作', sheets: [
              { title: '重新连接', action: () => this.onReconnect() },
              { title: '断开连接', action: () => this.onDisconnect() },
              { title: this.capabilities.indexOf('stop_session') >= 0 ? '终止 Claude 会话' : '关闭', action: () => {
                if (this.capabilities.indexOf('stop_session') >= 0 && this.commandReady()) this.onStopSession()
              } },
            ] })
          })
        }.padding(12).width('100%')
        StatusBar({ state: this.state })
        if (this.state.connection !== ConnectionStage.READY && this.state.messages.length > 0) {
          Text('本机已收到的消息 · 当前状态待同步').fontSize(12).padding(6)
        }
        if (this.managementFeedback.length > 0) { Text(this.managementFeedback).fontSize(12).padding(6) }
        if (this.state.lastError !== null) { Text(this.state.lastError.message).fontSize(12).fontColor('#b3261e').padding(6) }
        if (this.state.lastNotice !== null) { Text(this.state.lastNotice.message).fontSize(12).padding(6) }
        if (this.state.connection === ConnectionStage.PAIRING_INVALID) { Text('配对已失效，请重新扫码').fontColor('#b3261e') }
        Row({ space: 8 }) {
          if (this.state.pendingAction !== null) { Button('待处理事项').height(44).onClick(() => { this.sheet = 'action' }) }
          if (this.state.queuedRequests.length > 0) { Button(`排队 ${this.state.queuedRequests.length}`).height(44).onClick(() => { this.sheet = 'queue' }) }
          if (this.showReconnect()) { Button('重新连接').height(44).onClick(() => this.onReconnect()) }
        }
        Scroll(this.scroller) {
          MessageList({ messages: this.state.messages })
        }.layoutWeight(1).width('100%').padding(12)
        Row() {
          Blank()
          Button('最新消息 ↓').height(32).fontSize(12).onClick(() => this.scroller.scrollEdge(Edge.Bottom))
        }.padding({ right: 12 })
        Column() {
          MessageComposer({ draft: this.draft, composerEnabled: this.state.canSend,
            permissionMode: this.state.permissionMode,
            permissionEnabled: this.commandReady() && (this.state.sessionStatus === 'ready' || this.state.sessionStatus === 'starting'),
            onPermissionMode: this.onPermissionMode,
            createSessionAvailable: this.capabilities.indexOf('create_session') >= 0,
            createSessionEnabled: this.commandReady(), running: this.hasCancelTarget(), cancelEnabled: this.commandReady(),
            speechActive: this.speechActive, selecting: this.selecting,
            onDraft: this.onDraft, onSelect: this.onSelect, onRemoveAttachment: this.onRemoveAttachment,
            onSpeech: this.onSpeech, onStopSpeech: this.onStopSpeech, onCancelSpeech: this.onCancelSpeech,
            onSend: this.onSend, onCreateSession: this.onCreateSession, onCancel: () => this.onCancelTurn(this.cancelTarget()) })
        }.padding(12).width('100%').backgroundColor('#ffffff')
      }.width('100%').height('100%').backgroundColor('#f5f7fa')
      if (this.sheet.length > 0) {
        Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => this.closeSheet())
        this.drawer()
      }
    }.width('100%').height('100%')
  }
}
