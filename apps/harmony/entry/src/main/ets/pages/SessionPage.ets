import type {
  Question,
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
  @Prop state: HarmonySessionState
  @Prop questionAnswers: Array<QuestionAnswer> = []
  onBack: () => void = () => {}
  onReconnect: () => void = () => {}
  onDisconnect: () => void = () => {}
  onSend: (text: string) => void = () => {}
  onAllowOnce: () => void = () => {}
  onAllowSession: () => void = () => {}
  onDeny: () => void = () => {}
  onQuestionAnswerChange: (questionId: string, values: Array<string>) => void = () => {}
  onSubmitAnswers: () => void = () => {}
  onCancelTurn: (requestId: string) => void = () => {}
  onCancelQueued: (requestId: string) => void = () => {}

  private showReconnect(): boolean {
    return this.state.connection === ConnectionStage.DISCONNECTED ||
      this.state.connection === ConnectionStage.PROTOCOL_ERROR
  }

  private cancelTarget(): string {
    return currentCancelTarget(this.state)
  }

  private commandReady(): boolean {
    return this.state.connection === ConnectionStage.READY &&
      this.state.pendingCommand === null
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

  build() {
    Column() {
      Row({ space: 12 }) {
        Button('返回')
          .id('session-back-button')
          .onClick(() => {
            this.onBack()
          })

        Text(this.title)
          .fontSize(20)
          .fontWeight(FontWeight.Bold)
          .layoutWeight(1)
          .textAlign(TextAlign.Start)
      }
      .padding({ left: 16, right: 16, top: 12, bottom: 12 })
      .width('100%')
      .alignItems(VerticalAlign.Center)

      Scroll() {
        Column({ space: 16 }) {
          StatusBar({
            state: this.state,
          })

          if (this.state.lastNotice !== null) {
            Text(`提示：${this.state.lastNotice.message}`)
              .id('session-notice')
              .fontSize(12)
              .fontColor('#8a5a00')
              .width('100%')
          }

          QueuedSummary({
            queuedRequests: this.state.queuedRequests,
            cancelAllowed: this.commandReady(),
            onCancelQueued: (requestId: string) => {
              this.onCancelQueued(requestId)
            },
          })

          if (this.hasCancelTarget()) {
            Row({ space: 8 }) {
              Button('取消当前 Turn')
                .id('cancel-turn-button')
                .enabled(this.commandReady())
                .onClick(() => {
                  this.onCancelTurn(this.cancelTarget())
                })

              if (this.cancelRequested()) {
                Text('已请求取消')
                  .fontSize(12)
                  .opacity(0.7)
              }
            }
          }

          if (this.state.connection === ConnectionStage.PAIRING_INVALID) {
            Text('配对已失效，请返回列表后重新配对。')
              .fontColor('#c0392b')
              .width('100%')
          }

          if (this.state.connection === ConnectionStage.PROTOCOL_ERROR) {
            Text('当前状态已不可信，请重新连接获取最新状态。')
              .fontColor('#c0392b')
              .width('100%')
          }

          if (this.state.lastError !== null) {
            Text(`错误：${this.state.lastError.message}`)
              .fontColor('#c0392b')
              .width('100%')
          }

          if (this.showReconnect()) {
            Button('重新连接')
              .id('reconnect-button')
              .onClick(() => {
                this.onReconnect()
              })
          }

          Button('断开连接')
            .id('disconnect-button')
            .onClick(() => {
              this.onDisconnect()
            })

          MessageList({
            messages: this.state.messages,
          })

          if (this.state.pendingAction !== null) {
            PendingSummary({
              pendingAction: this.state.pendingAction,
              questionAnswers: this.questionAnswers,
              actionEnabled: this.state.canResolveAction,
              cancelAllowed: this.commandReady(),
              answersReady: this.answersReady(),
              onAllowOnce: () => {
                this.onAllowOnce()
              },
              onAllowSession: () => {
                this.onAllowSession()
              },
              onDeny: () => {
                this.onDeny()
              },
              onQuestionAnswerChange: (questionId: string, values: Array<string>) => {
                this.onQuestionAnswerChange(questionId, values)
              },
              onSubmitAnswers: () => {
                this.onSubmitAnswers()
              },
              onCancelTurn: (requestId: string) => {
                this.onCancelTurn(requestId)
              },
            })
          }
        }
        .padding(16)
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }
      .width('100%')
      .layoutWeight(1)

      Column() {
        MessageComposer({
          composerEnabled: this.state.canSend,
          onSend: (text: string) => {
            this.onSend(text)
          },
        })
      }
      .padding(16)
      .width('100%')
      .alignItems(HorizontalAlign.Start)
      .backgroundColor('#ffffff')
    }
    .width('100%')
    .height('100%')
    .backgroundColor('#f5f7fa')
  }
}
