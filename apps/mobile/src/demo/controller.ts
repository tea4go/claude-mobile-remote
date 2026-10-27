import type { ActionResolution } from '../protocol/client'
import { createInitialState, type MobileState } from '../session/reducer'

export function initialDemoState(): MobileState {
  return { ...createInitialState(), sessionId: 'offline-demo', phase: 'connected', connection: 'connected', sessionStatus: 'ready', canSend: true }
}

// Independent, memory-only synthetic scenario. No transport, storage or native ports.
export class DemoController {
  state = initialDemoState()
  private timer: ReturnType<typeof setTimeout> | undefined
  private counter = 0
  private active = ''
  private readonly changed: (state: MobileState) => void
  constructor(changed: (state: MobileState) => void) { this.changed = changed }
  private update(patch: Partial<MobileState>) {
    this.state = { ...this.state, ...patch }
    this.changed(this.state)
  }
  private clear() { clearTimeout(this.timer); this.timer = undefined }
  dispose() { this.clear(); this.active = '' }
  reset() { this.dispose(); this.update(initialDemoState()) }
  send(text: string) {
    if (!this.state.canSend || !text.trim()) return
    const id = `demo-${++this.counter}`
    this.active = id
    this.update({ canSend: false, messages: [...this.state.messages,
      { kind: 'input', source: 'mobile', requestId: id, text, status: 'accepted' },
      { kind: 'claude', source: 'mobile', requestId: id, status: 'streaming', text: '', error: null }],
      progress: { phase: 'thinking', startedAt: Date.now(), updatedAt: Date.now(), outputTokens: 0, tokensKnown: false, tokensEstimated: false } })
    this.timer = setTimeout(() => {
      if (this.active !== id) return
      if (text.trim() === '演示审批') {
        this.update({ progress: null, canResolveAction: true, pendingAction: { kind: 'tool_approval', actionId: id, requestId: id, toolName: 'Read', preview: '模拟读取 example.txt，不访问真实文件', allowSessionAvailable: false } })
      } else if (text.trim() === '演示问答') {
        this.update({ progress: null, canResolveAction: true, pendingAction: { kind: 'ask_user_question', actionId: id, requestId: id, toolName: 'AskUserQuestion', allowSessionAvailable: false,
          questions: [{ questionId: 'demo-question', header: '演示选择', question: '请选择模拟回复的语言', inputType: 'choice', multiSelect: false, options: [{ label: '中文', description: '中文示例' }, { label: 'English', description: 'English example' }] }] } })
      } else this.stream('这是离线演示的预设回复。\n\n你可以体验流式消息、审批、问答和停止。真实 Claude 操作需要返回后扫码连接电脑。')
    }, 500)
  }
  resolve(resolution: ActionResolution): boolean {
    const action = this.state.pendingAction
    if (!action || resolution.actionId !== action.actionId) return false
    this.update({ pendingAction: null, canResolveAction: false })
    this.stream('decision' in resolution
      ? resolution.decision === 'deny' ? '模拟审批已拒绝，未执行任何操作。' : '模拟审批已允许。示例文件内容：Hello, demo!（未读取真实文件）'
      : '模拟答案已收到，问答流程完成。未发送到电脑或 Claude。')
    return true
  }
  private stream(text: string) {
    const id = this.active
    let offset = 0
    const tick = () => {
      if (!id || this.active !== id) return
      offset += 4
      const done = offset >= text.length
      this.update({ messages: this.state.messages.map(m => m.kind === 'claude' && m.requestId === id ? { ...m, text: text.slice(0, offset), status: done ? 'done' : 'streaming' } : m),
        canSend: done, progress: done ? null : { phase: 'writing', startedAt: Date.now(), updatedAt: Date.now(), outputTokens: 0, tokensKnown: false, tokensEstimated: false } })
      if (done) this.active = ''
      else this.timer = setTimeout(tick, 80)
    }
    tick()
  }
  cancel(id: string): boolean {
    if (!this.active || id !== this.active) return false
    this.clear(); this.active = ''
    this.update({ canSend: true, canResolveAction: false, pendingAction: null, progress: null,
      messages: this.state.messages.map(m => m.kind === 'claude' && m.requestId === id ? { ...m, status: 'cancelled' } : m) })
    return true
  }
}
