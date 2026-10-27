import { ConnectionStage, createInitialHarmonySessionState, type HarmonySessionState } from './model'
import type { ActionResolution } from '../protocol/client'
// Memory-only synthetic scenarios. No transport, credentials, persistence or native ports.
export class DemoController {
  state: HarmonySessionState = this.initial()
  private timer: number = -1
  private counter: number = 0
  private active: string = ''
  private startedAt: number = 0
  private changed: (state: HarmonySessionState) => void
  constructor(changed: (state: HarmonySessionState) => void) { this.changed = changed }
  private initial(): HarmonySessionState {
    const state = createInitialHarmonySessionState()
    state.sessionId = 'offline-demo'; state.connection = ConnectionStage.READY; state.sessionStatus = 'ready'; state.canSend = true
    return state
  }
  private emit(): void {
    // ArkUI must receive a fresh state object after each timer callback.
    const previous = this.state
    this.state = { permissionMode: previous.permissionMode, pairing: null, connection: previous.connection,
      sessionId: previous.sessionId, sessionStatus: previous.sessionStatus, sessionError: null,
      seqBase: 0, queueDepthHint: 0, queuedRequests: [], pendingAction: previous.pendingAction,
      pendingCommand: null, messages: previous.messages.slice(), lastError: null, lastNotice: null,
      canSend: previous.canSend, canResolveAction: previous.canResolveAction, progress: previous.progress }
    this.changed(this.state)
  }
  private clear(): void { if (this.timer !== -1) clearTimeout(this.timer); this.timer = -1 }
  dispose(): void { this.clear(); this.active = '' }
  reset(): void { this.dispose(); this.state = this.initial(); this.emit() }
  send(text: string): boolean {
    if (!this.state.canSend || text.trim().length === 0) return false
    const id = `demo-${++this.counter}`; this.active = id; this.startedAt = Date.now()
    this.state.canSend = false
    this.state.messages.push({ kind: 'input', source: 'mobile', requestId: id, text, status: 'accepted' })
    this.state.messages.push({ kind: 'claude', source: 'mobile', requestId: id, text: '', status: 'streaming', error: null })
    this.state.progress = { phase: 'thinking', startedAt: this.startedAt, updatedAt: this.startedAt, outputTokens: 0, tokensKnown: false, tokensEstimated: false }
    this.emit()
    this.timer = setTimeout(() => {
      if (this.active !== id) return
      if (text.trim() === '演示审批') {
        this.state.progress = undefined; this.state.canResolveAction = true
        this.state.pendingAction = { kind: 'tool_approval', actionId: id, requestId: id, toolName: 'Read', preview: '模拟读取 example.txt，不访问真实文件', allowSessionAvailable: false }; this.emit()
      } else if (text.trim() === '演示问答') {
        this.state.progress = undefined; this.state.canResolveAction = true
        this.state.pendingAction = { kind: 'ask_user_question', actionId: id, requestId: id, toolName: 'AskUserQuestion', allowSessionAvailable: false,
          questions: [{ questionId: 'demo-question', header: '演示选择', question: '请选择模拟回复语言', inputType: 'choice', multiSelect: false, options: [{ label: '中文', description: '中文示例' }, { label: 'English', description: 'English example' }] }] }; this.emit()
      } else this.stream('这是离线演示的预设回复。可以体验流式消息、审批、问答和停止。真实 Claude 操作需要扫码连接电脑。')
    }, 500)
    return true
  }
  resolve(resolution: ActionResolution): boolean {
    const action = this.state.pendingAction
    if (action === null || resolution.actionId !== action.actionId) return false
    if ((action.kind === 'tool_approval') !== (resolution.mode === 'decision')) return false
    if (resolution.mode === 'answers' && !resolution.answers.some((answer) => answer.questionId === 'demo-question' && answer.values.length > 0)) return false
    this.state.pendingAction = null; this.state.canResolveAction = false
    this.stream(resolution.mode === 'decision' ? (resolution.decision === 'deny' ? '模拟审批已拒绝，未执行操作。' : '模拟审批已允许。示例内容 Hello, demo! 未读取真实文件。') : '模拟答案已收到，未发送到电脑或 Claude。')
    return true
  }
  private stream(text: string): void {
    const id = this.active; let offset = 0
    const tick = (): void => {
      if (id.length === 0 || this.active !== id) return
      offset += 4; const done = offset >= text.length
      this.state.messages = this.state.messages.map((message) => {
        if (message.kind === 'claude' && message.requestId === id) { message.text = text.slice(0, offset); message.status = done ? 'done' : 'streaming' }
        return message
      })
      this.state.progress = done ? undefined : { phase: 'writing', startedAt: this.startedAt, updatedAt: Date.now(), outputTokens: 0, tokensKnown: false, tokensEstimated: false }
      this.state.canSend = done; this.emit()
      if (done) this.active = ''; else this.timer = setTimeout(tick, 80)
    }
    tick()
  }
  cancel(id: string): boolean {
    if (this.active.length === 0 || this.active !== id) return false
    this.dispose(); this.state.pendingAction = null; this.state.canResolveAction = false; this.state.canSend = true; this.state.progress = undefined
    this.state.messages.forEach((message) => { if (message.kind === 'claude' && message.requestId === id) message.status = 'cancelled' }); this.emit(); return true
  }
}
