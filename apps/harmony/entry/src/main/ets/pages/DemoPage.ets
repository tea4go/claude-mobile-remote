import { DemoController } from '../session/demo'
import { createInitialHarmonySessionState, type HarmonySessionState } from '../session/model'
import { emptyDraft, type ComposerDraft } from '../session/attachments'
import { createDecisionResolution, createAnswersResolution, type ActionDecision } from '../protocol/client'
import type { QuestionAnswer } from '../protocol/types'
import { SessionPage } from './SessionPage'
@Component
export struct DemoPage {
  onExit: () => void = () => {}
  @State private state: HarmonySessionState = createInitialHarmonySessionState()
  @State private draft: ComposerDraft = emptyDraft()
  @State private answers: Array<QuestionAnswer> = []
  private demo: DemoController = new DemoController((state: HarmonySessionState) => { this.state = state })
  aboutToAppear(): void { this.state = this.demo.state }
  aboutToDisappear(): void { this.demo.dispose() }
  private send(text: string): void { if (this.demo.send(text)) { this.draft = emptyDraft(); this.answers = [] } }
  private reset(): void { this.demo.reset(); this.draft = emptyDraft(); this.answers = [] }
  private exit(): void { this.demo.dispose(); this.onExit() }
  private resolve(decision: ActionDecision): void {
    const action = this.state.pendingAction
    if (action !== null) this.demo.resolve(createDecisionResolution(action.actionId, decision))
  }
  build() {
    Column() {
      Text('离线演示 · 模拟数据，不连接电脑或 Claude').fontSize(13).fontColor('#614800').padding(8).width('100%').backgroundColor('#fff3ca')
      Text('发送任意文本体验预设流式回复；附件/语音需真实配对。').fontSize(12).padding(6)
      Flex({ wrap: FlexWrap.Wrap }) {
        Button('演示审批').fontSize(12).height(36).enabled(this.state.canSend).onClick(() => this.send('演示审批'))
        Button('演示问答').fontSize(12).height(36).enabled(this.state.canSend).onClick(() => this.send('演示问答'))
        Button('重新开始').fontSize(12).height(36).onClick(() => this.reset())
        Button('退出演示').fontSize(12).height(36).onClick(() => this.exit())
      }.padding(4)
      SessionPage({ title: '离线演示', state: this.state, draft: this.draft, textOnly: true,
        questionAnswers: this.answers, onBack: () => this.exit(),
        onDraft: (text: string) => { this.draft = { text, attachments: [], sending: false, error: '' } },
        onSend: () => this.send(this.draft.text), onCancelTurn: (id: string) => { this.demo.cancel(id); this.answers = [] },
        onAllowOnce: () => this.resolve('allow_once'), onAllowSession: () => this.resolve('allow_session'), onDeny: () => this.resolve('deny'),
        onQuestionAnswerChange: (id: string, values: Array<string>) => { this.answers = this.answers.filter((answer: QuestionAnswer) => answer.questionId !== id).concat([{ questionId: id, values }]) },
        onSubmitAnswers: () => { const action = this.state.pendingAction; if (action !== null) this.demo.resolve(createAnswersResolution(action.actionId, this.answers)); this.answers = [] },
      }).layoutWeight(1)
    }.width('100%').height('100%')
  }
}
