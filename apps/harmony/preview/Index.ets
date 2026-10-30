import type { QuestionAnswer } from '../protocol/types'
// Copied only into the isolated preview project; never an application entry.
import { SessionListPage } from './SessionListPage'
import { SessionPage } from './SessionPage'
import { createInitialHarmonySessionState, ConnectionStage, type HarmonySessionState } from '../session/model'
import type { SessionRecord } from '../session/records'
import { emptyDraft, type ComposerDraft } from '../session/attachments'
import { MARKDOWN_SAMPLE } from '../session/markdownSample'
import { ActionMenu, type ActionMenuItem } from '../components/ActionMenu'
import { curves } from '@kit.ArkUI'
import { pageFade } from '../ui/transitions'

function records(): Array<SessionRecord> {
  return [
    { sessionId: 'preview-session', wsUrl: 'ws://preview.invalid', pairToken: '', expiresAt: '', pcId: 'preview-mac',
      name: '检查移动端界面', pcName: 'MacBook Pro', projectCwd: '/demo/claude-mobile-remote', preview: '请检查列表、聊天和审批布局',
      runtimeStatus: '等待处理', unread: true, lastActivityAt: '2026-10-23T08:42:00Z', lastConnectedAt: 0, hasConnectedAt: false, status: 'connected' },
    { sessionId: 'preview-second', wsUrl: 'ws://preview.invalid', pairToken: '', expiresAt: '', pcId: 'preview-mac',
      name: '这是一个用于检查省略显示的非常长的会话标题', pcName: 'MacBook Pro', projectCwd: '/demo/work/projects/a-very-long-directory/feature-preview',
      preview: '已完成布局调整，等待你确认效果', runtimeStatus: '已完成', lastActivityAt: '2026-10-23T08:30:00Z', lastConnectedAt: 0, hasConnectedAt: false, status: 'connected' },
    { sessionId: 'preview-third', wsUrl: 'ws://preview-pc.invalid', pairToken: '', expiresAt: '', pcId: 'preview-pc',
      name: '后端接口联调', pcName: '开发电脑', projectCwd: '/demo/server', preview: '上次收到的消息保存在本机',
      runtimeStatus: '空闲', lastActivityAt: '2026-10-23T07:12:00Z', lastConnectedAt: 0, hasConnectedAt: false, status: 'disconnected' },
  ]
}
function initialState(): HarmonySessionState {
  const state = createInitialHarmonySessionState()
  state.sessionId = 'preview-session'
  state.connection = ConnectionStage.READY
  state.sessionStatus = 'ready'
  state.canSend = true
  state.canResolveAction = true
  state.messages = [
    { kind: 'input', source: 'mobile', requestId: 'preview-request', text: '请检查界面布局', status: 'accepted' },
    { kind: 'claude', source: 'mobile', requestId: 'preview-request', status: 'done',
      text: '## 界面预览\n\n这是虚构数据，可检查列表、聊天、设置和审批抽屉。\n\n- 不连接真实电脑\n- 不保存配对信息\n\n```ts\nconst preview = true\n```', error: null },
  ]
  state.pendingAction = { actionId: 'preview-action', requestId: 'preview-request', kind: 'tool_approval',
    toolName: 'Read', preview: '/demo/project/README.md', allowSessionAvailable: true }
  return state
}
@Entry
@Component
struct Index {
  @State private detail: boolean = false
  @State private speech: boolean = false
  @State private answers: Array<QuestionAnswer> = []
  @State private state: HarmonySessionState = initialState()
  @State private draft: ComposerDraft = emptyDraft()
  @State private sessions: Array<SessionRecord> = records()
  @State private recentlyViewed: string = 'preview-second'
  @State private title: string = '检查移动端界面'
  @State private feedback: string = ''
  @State private notifications: boolean = false
  private sceneController: CustomDialogController = new CustomDialogController({
    builder: ActionMenu({ title: '预览场景（仅模拟数据）', items: this.sceneItems(),
      onClose: (): void => { this.sceneController.close() } }),
    customStyle: true, alignment: DialogAlignment.Bottom, autoCancel: true,
    openAnimation: { curve: curves.springMotion(0.25, 1) },
    closeAnimation: { curve: curves.springMotion(0.25, 1) },
  })
  aboutToDisappear(): void { this.sceneController.close() }
  private sceneItems(): Array<ActionMenuItem> {
    return [
      { title: '会话列表', action: () => this.scene('list') },
      { title: 'Markdown 排版', action: () => this.scene('markdown') },
      { title: '工具审批', action: () => this.scene('approval') },
      { title: '问题回答', action: () => this.scene('question') },
      { title: '运行中', action: () => this.scene('running') },
      { title: '录音中', action: () => this.scene('speech') },
      { title: '断开连接', action: () => this.scene('disconnected') },
      { title: '长标题', action: () => this.scene('long') },
    ]
  }
  private resolve(): void {
    const next = initialState()
    next.pendingAction = null
    this.state = next
    this.feedback = '预览操作已完成（未发送到电脑）'
  }
  private scene(kind: string): void {
    const next = initialState()
    this.speech = kind === 'speech'
    this.feedback = ''
    this.answers = []
    this.title = kind === 'long' ? '这是一个用于检查省略显示的非常长的会话标题' : '检查移动端界面'
    if (kind !== 'approval' && kind !== 'question') next.pendingAction = null
    if (kind === 'running' || kind === 'speech') {
      next.messages = [{ kind: 'claude', source: 'mobile', requestId: 'preview-request', status: 'streaming', text: '正在检查界面布局…', error: null }]
      next.permissionMode = 'acceptEdits'
    }
    if (kind === 'disconnected') { next.connection = ConnectionStage.DISCONNECTED; next.canSend = false; next.canResolveAction = false }
    if (kind === 'markdown') next.messages = [{ kind: 'claude', source: 'mobile', requestId: 'preview-markdown', status: 'done', text: MARKDOWN_SAMPLE, error: null }]
    if (kind === 'question') {
      next.pendingAction = { actionId: 'preview-question', requestId: 'preview-request', kind: 'ask_user_question', toolName: 'AskUserQuestion', allowSessionAvailable: false,
        questions: [{ questionId: 'preview-choice', header: '预览选项', question: '选择要检查的布局', inputType: 'choice', multiSelect: false,
          options: [{ label: '紧凑布局', description: '检查较长的选项说明是否正常换行，不挤压操作区域。' }, { label: '标准布局', description: '' }] }] }
    }
    this.state = next
    this.detail = kind !== 'list'
  }
  build() {
    Column() {
      Text('ArkUI 预览 · 模拟数据 · 点此切换场景').fontSize(12).padding(8).width('100%').backgroundColor('#fff3cd')
        .onClick(() => {
          this.sceneController.open()
        })
      Stack() {
      if (this.detail) {
        SessionPage({ title: this.title, questionAnswers: this.answers,
          onQuestionAnswerChange: (id: string, values: Array<string>) => { this.answers = [{ questionId: id, values: values }] },
          onSubmitAnswers: () => this.resolve(), speechActive: this.speech, capabilities: ['create_session'], state: this.state, draft: this.draft, managementFeedback: this.feedback,
          onBack: () => { this.speech = false; this.detail = false },
          onStopSpeech: () => { this.speech = false }, onCancelSpeech: () => { this.speech = false },
          onCancelTurn: () => this.resolve(),
          onDraft: (text: string) => { this.draft = { text, attachments: [], sending: false, error: '' } },
          onSend: () => { this.feedback = '预览草稿已提交（未联网）'; this.draft = emptyDraft() },
          onAllowOnce: () => this.resolve(), onAllowSession: () => this.resolve(), onDeny: () => this.resolve(),
          onSelect: () => { this.feedback = '附件选择需要设备验证' },
          onSpeech: () => { this.feedback = '语音需要设备验证' } }).transition(pageFade())
      } else {
        SessionListPage({ sessions: this.sessions, recentlyViewedId: this.recentlyViewed, notificationsEnabled: this.notifications,
          onOpen: (id: string) => { this.recentlyViewed = id; this.title = this.sessions.find((item: SessionRecord) => item.sessionId === id)?.name ?? '会话'; this.state = initialState(); this.feedback = ''; this.detail = true },
          onAdd: () => { this.detail = true; this.feedback = '扫码需要设备验证' },
          onNotifications: () => { this.notifications = !this.notifications },
          onClearCache: () => {},
          onRemove: (id: string) => { this.sessions = this.sessions.filter((item: SessionRecord) => item.sessionId !== id) },
          onRemovePC: (key: string) => { this.sessions = this.sessions.filter((item: SessionRecord) => (item.pcId ?? item.wsUrl) !== key) },
          onRename: (id: string, name: string) => { this.sessions = this.sessions.map((item: SessionRecord) => { if (item.sessionId === id) { item.name = name; item.customName = true }; return item }) } }).transition(pageFade())
      }
      }.width('100%').layoutWeight(1)
    }.width('100%').height('100%')
  }
}
