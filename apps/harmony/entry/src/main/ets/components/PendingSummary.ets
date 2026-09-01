import type {
  AskUserQuestionPendingAction,
  Question,
  QuestionAnswer,
  QuestionOption,
  PendingAction,
  ToolApprovalPendingAction,
} from '../protocol/types'

function findAnswerValues(
  questionAnswers: Array<QuestionAnswer>,
  questionId: string,
): Array<string> {
  for (let index = 0; index < questionAnswers.length; index += 1) {
    if (questionAnswers[index].questionId === questionId) {
      return questionAnswers[index].values
    }
  }
  return []
}

function isSelected(values: Array<string>, label: string): boolean {
  for (let index = 0; index < values.length; index += 1) {
    if (values[index] === label) {
      return true
    }
  }
  return false
}

function toggleMultiSelect(values: Array<string>, label: string): Array<string> {
  const next: Array<string> = []
  let found = false
  for (let index = 0; index < values.length; index += 1) {
    if (values[index] === label) {
      found = true
      continue
    }
    next.push(values[index])
  }
  if (!found) {
    next.push(label)
  }
  return next
}

function optionBackground(selected: boolean): string {
  return selected ? '#e6f4ff' : '#ffffff'
}

@Component
export struct PendingSummary {
  @Prop pendingAction: PendingAction | null = null
  @Prop questionAnswers: Array<QuestionAnswer> = []
  @Prop actionEnabled: boolean = false
  @Prop cancelAllowed: boolean = false
  @Prop answersReady: boolean = false
  onAllowOnce: () => void = () => {}
  onAllowSession: () => void = () => {}
  onDeny: () => void = () => {}
  onQuestionAnswerChange: (questionId: string, values: Array<string>) => void = () => {}
  onSubmitAnswers: () => void = () => {}
  onCancelTurn: (requestId: string) => void = () => {}

  private toolApprovalAction(): ToolApprovalPendingAction | null {
    if (this.pendingAction !== null && this.pendingAction.kind === 'tool_approval') {
      return this.pendingAction
    }
    return null
  }

  private questionAction(): AskUserQuestionPendingAction | null {
    if (this.pendingAction !== null && this.pendingAction.kind === 'ask_user_question') {
      return this.pendingAction
    }
    return null
  }

  private actionTitle(): string {
    return this.toolApprovalAction() !== null ? '待处理事项' : '需要你的回答'
  }

  private cancelRequestId(): string {
    if (this.pendingAction === null) {
      return ''
    }
    return this.pendingAction.requestId
  }

  private hasToolApprovalAction(): boolean {
    return this.toolApprovalAction() !== null
  }

  private questionValues(questionId: string): Array<string> {
    return findAnswerValues(this.questionAnswers, questionId)
  }

  private optionSelected(questionId: string, label: string): boolean {
    return isSelected(this.questionValues(questionId), label)
  }

  @Builder
  private buildToolApproval(action: ToolApprovalPendingAction): void {
    Column({ space: 10 }) {
      Text('Claude 请求使用工具')
        .fontSize(14)
        .width('100%')

      Text(action.toolName)
        .fontSize(16)
        .fontWeight(FontWeight.Medium)
        .width('100%')

      if (action.preview.length > 0) {
        Text(action.preview)
          .fontSize(13)
          .width('100%')
          .padding(10)
          .backgroundColor('#ffffff')
          .borderRadius(8)
      }

      Row({ space: 8 }) {
        Button('本次允许')
          .enabled(this.actionEnabled)
          .onClick(() => {
            this.onAllowOnce()
          })

        if (action.allowSessionAvailable) {
          Button('本会话允许')
            .enabled(this.actionEnabled)
            .onClick(() => {
              this.onAllowSession()
            })
        }

        Button('拒绝')
          .enabled(this.actionEnabled)
          .onClick(() => {
            this.onDeny()
          })
      }
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
  }

  @Builder
  private buildQuestion(question: Question): void {
    Column({ space: 8 }) {
      Text(question.header)
        .fontSize(12)
        .opacity(0.7)
        .width('100%')

      Text(question.question)
        .fontSize(14)
        .width('100%')

      if (question.inputType === 'text') {
        TextInput({
          text: this.questionValues(question.questionId).length > 0 ? this.questionValues(question.questionId)[0] : '',
          placeholder: '输入你的回答',
        })
          .enabled(this.actionEnabled)
          .onChange((value: string) => {
            this.onQuestionAnswerChange(question.questionId, [value])
          })
          .width('100%')
      } else {
        ForEach(question.options, (option: QuestionOption) => {
          Column({ space: 4 }) {
            Text(option.label)
              .fontSize(14)
              .fontWeight(this.optionSelected(question.questionId, option.label) ? FontWeight.Medium : FontWeight.Regular)
              .width('100%')
            if (option.description.length > 0) {
              Text(option.description)
                .fontSize(12)
                .opacity(0.7)
                .width('100%')
            }
          }
          .padding(10)
          .width('100%')
          .alignItems(HorizontalAlign.Start)
          .backgroundColor(optionBackground(this.optionSelected(question.questionId, option.label)))
          .borderRadius(8)
          .onClick(() => {
            if (!this.actionEnabled) {
              return
            }
            if (question.multiSelect) {
              this.onQuestionAnswerChange(
                question.questionId,
                toggleMultiSelect(this.questionValues(question.questionId), option.label),
              )
              return
            }
            this.onQuestionAnswerChange(question.questionId, [option.label])
          })
        }, (option: QuestionOption) => option.label)
      }
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
  }

  build() {
    Column({ space: 12 }) {
      if (this.pendingAction !== null) {
        Row({ space: 8 }) {
          Text(this.actionTitle())
            .fontSize(18)
            .fontWeight(FontWeight.Medium)
            .layoutWeight(1)
            .textAlign(TextAlign.Start)

          Button('关闭并取消')
            .enabled(this.cancelAllowed)
            .onClick(() => {
              this.onCancelTurn(this.cancelRequestId())
            })
        }

        Text('关闭此面板等价于取消当前 Turn。')
          .fontSize(12)
          .opacity(0.7)
          .width('100%')

        if (this.hasToolApprovalAction()) {
          this.buildToolApproval(this.toolApprovalAction() as ToolApprovalPendingAction)
        } else {
          ForEach((this.questionAction() as AskUserQuestionPendingAction).questions, (question: Question) => {
            this.buildQuestion(question)
          }, (question: Question) => question.questionId)

          Button('提交回答')
            .enabled(this.actionEnabled && this.answersReady)
            .onClick(() => {
              this.onSubmitAnswers()
            })
        }
      }
    }
    .id('pending-summary')
    .padding(16)
    .width('100%')
    .alignItems(HorizontalAlign.Start)
    .backgroundColor('#fdf7f0')
    .borderRadius(12)
  }
}
