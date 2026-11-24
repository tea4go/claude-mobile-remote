import { copyVisibleText } from '../native/clipboard'
import { pageSlide } from '../ui/transitions'
import { nativeLifecycle } from '../native/lifecycle'
import { MarkdownText } from './MarkdownText'
import { FeedDataSource, feedRenderKey, projectFeed, type FeedRow, type ActivityFeedGroup } from '../ui/feedDataSource'
import { toolNamesByTurn, toolNameForResult, toolStatusesByTurn } from '../session/stepIndex'
import { messageSelection } from '../ui/messageSelection'
import { inputDisplayText } from '../session/inputDisplay'
import type {
  Attachment,
  ContentBlock,
  ResolvedAnswer, SubagentTask, ToolDetail, ToolResultContentBlock,
} from '../protocol/types'
import type { FeedMessage, BlockFeedMessage } from '../session/model'

function blockTitle(block: ContentBlock, requestId: string, tools: Map<string, string>): string {
  if (block.kind === 'thinking') return '思考过程'
  if (block.kind === 'tool_use') {
    if (block.toolClass === 'mcp') return `MCP：${block.mcpServer} / ${block.toolName}`
    const name = block.toolName === 'Agent' || block.toolName === 'Task' ? '子代理' : block.toolName
    const summary = block.titleSummary ?? block.preview ?? ''
    return summary.length > 0 ? `${name} · ${summary.split('\n')[0]}` : name
  }
  const toolName = toolNameForResult(tools, requestId, block.toolUseId)
  return toolName.length > 0 ? `工具结果 · ${toolName}` : '工具结果'
}

function visibleSteps(group: ActivityFeedGroup): Array<BlockFeedMessage> {
  const toolIds: Set<string> = new Set()
  group.blocks.forEach((item: BlockFeedMessage) => { if (item.block.kind === 'tool_use') toolIds.add(item.block.toolUseId) })
  return group.blocks.filter((item: BlockFeedMessage): boolean => item.block.kind !== 'tool_result' || !toolIds.has(item.block.toolUseId))
}

function stepSummary(group: ActivityFeedGroup): string {
  const steps = visibleSteps(group)
  const count = steps.filter((item: BlockFeedMessage): boolean => item.block.kind !== 'thinking').length
  return `查看 ${steps.length} 个步骤${count > 0 ? `，运行 ${count} 个工具` : ''}`
}

function inputStatusLabel(message: FeedMessage, connected: boolean): string {
  if (message.kind !== 'input') return ''
  if (message.status === 'unknown') return '结果未知（连接已断开）'
  if (connected && message.status === 'accepted' && message.processingStarted !== true) return '等待处理'
  return ''
}

function answerTitle(answer: ResolvedAnswer): string {
  if (answer.header.length === 0) {
    return answer.question
  }
  return `${answer.header} · ${answer.question}`
}

@Component
export struct MessageList {
  @Prop @Watch('messagesChanged') messages: Array<FeedMessage> = []
  scroller: Scroller = new Scroller()
  onAtBottom: (value: boolean) => void = () => {}
  onPreviewImage: (value: string) => void = () => {}
  @State private atTop: boolean = true
  @State private atEnd: boolean = true
  @Prop debugEnabled: boolean = false
  @Prop connected: boolean = true
  onDebugOpen: () => void = () => {}
  private readonly source: FeedDataSource = new FeedDataSource()
  @Link selectedActivity: ActivityFeedGroup | null
  @Link selectedStep: number
  private toolNames: Map<string, string> = new Map()
  private toolStatuses: Map<string, string> = new Map()
  private previousMessages: Array<FeedMessage> = []
  private followTimer: number = -1

  aboutToAppear(): void {
    this.messagesChanged(); nativeLifecycle.messageBack = this.backHandler
    setTimeout(() => {
      if (this.source.totalCount() > 0) this.scroller.scrollToIndex(this.source.totalCount() - 1, false, ScrollAlign.END)
    }, 500)
  }
  aboutToDisappear(): void { if (this.followTimer !== -1) clearTimeout(this.followTimer); this.followTimer = -1; messageSelection.close(); if (nativeLifecycle.messageBack === this.backHandler) nativeLifecycle.messageBack = null }
  private readonly backHandler: () => boolean = () => {
    if (messageSelection.close()) return true
    if (this.selectedActivity !== null) {
      if (this.selectedStep >= 0) this.selectedStep = -1
      else this.selectedActivity = null
      return true
    }
    return false
  }
  private messagesChanged(): void {
    const previous = this.previousMessages
    this.previousMessages = this.messages
    if (this.atEnd && this.followTimer === -1) {
      this.followTimer = setTimeout(() => {
        this.followTimer = -1
        if (this.atEnd && this.source.totalCount() > 0) this.scroller.scrollToIndex(this.source.totalCount() - 1, false, ScrollAlign.END)
      }, 50)
    }
    // A text delta cannot change the tool index or an open activity group.
    if (previous.length > 0 && previous.length === this.messages.length) {
      const oldLast = previous[previous.length - 1]
      const newLast = this.messages[this.messages.length - 1]
      if (oldLast.kind === 'claude' && newLast.kind === 'claude' && oldLast.requestId === newLast.requestId &&
        oldLast.status === newLast.status) {
        let samePrefix = true
        for (let index = 0; index < previous.length - 1; index += 1) {
          if (previous[index] !== this.messages[index]) { samePrefix = false; break }
        }
        if (samePrefix) { this.source.replace(this.messages); return }
      }
    }
    this.toolNames = toolNamesByTurn(this.messages)
    this.toolStatuses = toolStatusesByTurn(this.messages)
    if (this.selectedActivity !== null) {
      const selected = this.selectedActivity
      const firstId = selected.blocks[0]?.block.id ?? ''
      const current = projectFeed(this.messages).find((row: FeedRow) => row.kind === 'activity' && row.requestId === selected.requestId && row.blocks[0]?.block.id === firstId)
      if (current !== undefined && current.kind === 'activity') this.selectedActivity = current
    }
    this.source.replace(this.messages)
  }

  private inputBubbleWidth(text: string, hasAttachments: boolean): Length {
    if (hasAttachments) return '100%'
    const measure = this.getUIContext().getMeasureUtils()
    const width = Math.max(...text.split('\n').map((line: string) =>
      this.getUIContext().px2vp(measure.measureText({ textContent: line, fontSize: 16 }))))
    return Math.ceil(width) + 24
  }

  @Builder
  private scrollButton(top: boolean): void {
    Button({ type: ButtonType.Circle }) {
      Image(top ? $r('app.media.scroll_top') : $r('app.media.scroll_bottom')).width(21).height(21)
        .objectFit(ImageFit.Contain).draggable(false)
    }.width(44).height(44).padding(0)
      .backgroundColor($r('app.color.surface')).border({ width: 1, color: '#dbe4ef' })
      .shadow({ radius: 8, offsetY: 2, color: '#1f0f172a' })
      .accessibilityText(top ? '到顶部' : '到底部')
      .onClick(() => this.scroller.scrollEdge(top ? Edge.Top : Edge.Bottom))
  }

  build() {
    Stack({ alignContent: Alignment.BottomEnd }) {
    List({ space: 10, scroller: this.scroller }) {
      if (this.messages.length > 0) {
        LazyForEach(this.source, (message: FeedRow, index: number) => {
          ListItem() {
          if (message.kind === 'input') {
            Row() {
              Blank()
              Column({ space: 6 }) {
                Column({ space: 6 }) {
                  if (message.attachments !== undefined) {
                    ForEach(message.attachments, (attachment: Attachment) => {
                      if (attachment.mediaType.startsWith('image/') && attachment.data.length > 0) {
                        Image(`data:${attachment.mediaType};base64,${attachment.data}`).height(140).width('100%').objectFit(ImageFit.Contain)
                          .onClick(() => this.onPreviewImage(`data:${attachment.mediaType};base64,${attachment.data}`))
                      }
                      Text(attachment.name).fontSize(12).fontColor(Color.White)
                    }, (attachment: Attachment, itemIndex: number) => `${itemIndex}-${attachment.name}`)
                  }
                  MarkdownText({ text: inputDisplayText(message.text), onAccent: true, scrollController: this.scroller })
                }.padding({ left: 12, right: 12, top: 8, bottom: 8 })
                  .width(this.inputBubbleWidth(inputDisplayText(message.text), (message.attachments?.length ?? 0) > 0))
                  .constraintSize({ maxWidth: '100%' }).backgroundColor($r('app.color.primary_fill')).borderRadius(16)
                if (inputStatusLabel(message, this.connected).length > 0) {
                  Text(inputStatusLabel(message, this.connected)).fontSize(12).fontColor($r('app.color.text_secondary')).width('100%').textAlign(TextAlign.End)
                }
              }.width('80%').alignItems(HorizontalAlign.End)
            }.width('100%')
          } else if (message.kind === 'answer') {
            Column({ space: 8 }) {
              Row({ space: 6 }) { Image($r('app.media.claude_mark')).width(18).height(18); Text('Claude').fontSize(12).fontColor($r('app.color.text_secondary')) }.width('100%')

              ForEach(message.answers, (answer: ResolvedAnswer) => {
                Column({ space: 4 }) {
                  Text(answerTitle(answer)).copyOption(CopyOptions.LocalDevice)
                    .fontSize(13)
                    .fontWeight(FontWeight.Medium)
                    .width('100%')

                  ForEach(answer.values, (value: string) => {
                    Row({ space: 8 }) {
                      Text('✓').fontSize(13).fontColor(Color.White).width(20).height(20).textAlign(TextAlign.Center).backgroundColor($r('app.color.badge_success')).borderRadius(5)
                      Text(value).copyOption(CopyOptions.LocalDevice).fontSize(14).fontColor($r('app.color.text')).layoutWeight(1)
                    }.width('100%').padding({ top: 6, bottom: 6 })
                  }, (value: string) => `${answer.questionId}-${value}`)
                }
                .width('100%')
                .alignItems(HorizontalAlign.Start)
              }, (answer: ResolvedAnswer) => answer.questionId)
            }
            .padding(12)
            .width('100%')
            .alignItems(HorizontalAlign.Start)
            .backgroundColor($r('app.color.surface'))
            .borderRadius(12)
          } else if (message.kind === 'collapsed') {
            Button(message.expanded ? '收起执行过程' : '展开执行过程', { type: ButtonType.Normal }).width('100%').height(44).fontColor($r('app.color.primary')).backgroundColor($r('app.color.surface')).onClick(() => this.source.toggleTurn(message.requestId))
          } else if (message.kind === 'activity') {
            Button({ type: ButtonType.Normal }) {
              Row() {
                Text(`☷  ${stepSummary(message)}`).fontSize(14).fontColor($r('app.color.text')).layoutWeight(1).textAlign(TextAlign.Start)
                Text('›').fontSize(20).fontColor($r('app.color.text_secondary'))
              }.width('100%')
            }.width('100%').height(48).padding({ left: 14, right: 14 }).backgroundColor($r('app.color.surface')).borderRadius(14)
              .onClick(() => { this.selectedActivity = message; this.selectedStep = -1 })
          } else if (message.kind === 'block') {
            Text('步骤').fontSize(13)
          } else if (message.text.trim().length > 0 || message.status === 'streaming' || message.status === 'failed' || message.status === 'cancelled') {
            Column({ space: 8 }) {
              Row({ space: 6 }) { Image($r('app.media.claude_mark')).width(18).height(18); Text(message.model ? 'Claude · ' + message.model : 'Claude').fontSize(12).fontColor($r('app.color.text_secondary')) }.width('100%')

              if (message.text.trim().length > 0) { Column() { MarkdownText({ text: message.text, scrollController: this.scroller }) }.padding(14).width('100%').backgroundColor($r('app.color.surface')).borderRadius(16) }

              if (message.status === 'streaming') { LoadingProgress().width(20).height(20) }
              if (message.status === 'cancelled') {
                Text('已取消')
                  .fontColor($r('app.color.warning')).fontSize(12)
                  .padding({ left: 8, right: 8, top: 4, bottom: 4 })
                  .backgroundColor($r('app.color.surface_warning'))
                  .borderRadius(999)
              }

              if (message.status === 'failed' && message.error !== null) {
                Text(`Turn 失败：${message.error.message}`)
                  .fontSize(12)
                  .fontColor($r('app.color.danger'))
                  .width('100%')
              }
            }
            .padding({ top: 2, bottom: 2 })
            .width('100%')
            .alignItems(HorizontalAlign.Start)
            .backgroundColor(Color.Transparent)
            .borderRadius(12)
          }
          }.padding({ left: 12, right: 12 })
        }, (message: FeedRow, index: number) => feedRenderKey(message, index))
      }
    }
    .id('message-list')
    .width('100%')
    .height('100%')
    .cachedCount(2)
    .clip(true)
    .onScroll(() => { messageSelection.refresh(); this.atTop = this.scroller.currentOffset().yOffset <= 1; this.atEnd = this.scroller.isAtEnd(); this.onAtBottom(this.atEnd) })
    .onReachEnd(() => { this.atEnd = true; this.onAtBottom(true) })
    .onReachStart(() => { this.atTop = true })
    Column({ space: 8 }) {
      if (!this.atTop) {
        this.scrollButton(true)
      }
      if (!this.atEnd) {
        this.scrollButton(false)
      }
    }.padding({ right: 16, bottom: 12 })
    }.width('100%').height('100%')
  }
}

export type ToolDetailLoader = (requestId: string, toolUseId: string) => Promise<ToolDetail>

@Component
struct ToolDetailContent {
  @Prop @Watch('syncDetail') item: BlockFeedMessage
  @Prop @Watch('syncDetail') result: ToolResultContentBlock | null = null
  @Prop answers: string = ''
  @Prop hideEmpty: boolean = false
  load: ToolDetailLoader = (_requestId: string, _toolUseId: string) => Promise.reject(new Error('请先连接电脑'))
  @State private detail: ToolDetail | null = null
  @State private loading: boolean = false
  @State private error: string = ''
  private generation: number = 0
  private detailKey: string = ''

  aboutToAppear(): void { this.syncDetail() }
  aboutToDisappear(): void { this.generation += 1 }

  private syncDetail(): void {
    const block = this.item.block
    const key = block.kind === 'thinking' ? block.id : JSON.stringify([this.item.requestId, block.toolUseId,
      block.detailAvailable, this.result?.id, this.result?.detailAvailable])
    if (key !== this.detailKey) {
      this.detailKey = key
      this.readDetail()
    }
  }

  private readDetail(): void {
    const generation = ++this.generation
    const block = this.item.block
    this.detail = null
    this.error = ''
    this.loading = false
    if (block.kind === 'thinking' || (block.detailAvailable !== true && this.result?.detailAvailable !== true)) return
    this.loading = true
    this.load(this.item.requestId, block.toolUseId).then((detail: ToolDetail) => {
      if (generation !== this.generation) return
      this.loading = false
      this.detail = detail
    }).catch((error: Error) => {
      if (generation !== this.generation) return
      this.loading = false
      this.error = error.message
    })
  }

  private shell(): boolean { return this.item.block.kind === 'tool_use' && this.item.block.toolName === 'Bash' }
  private input(): string {
    return this.detail?.input ?? (this.item.block.kind === 'tool_use' ? this.item.block.preview ?? '' : '')
  }
  private output(): string {
    return this.detail?.output ?? this.result?.output ?? (this.item.block.kind === 'tool_result' ? this.item.block.output ?? '' : '')
  }

  @Builder
  private code(label: string, text: string, copy: boolean, truncated: boolean): void {
    Row() {
      Text(label).fontSize(13).fontColor($r('app.color.text_secondary')).layoutWeight(1)
      if (copy) {
        Button(label === '命令' ? '复制命令' : '复制输出', { type: ButtonType.Normal }).height(36)
          .backgroundColor(Color.Transparent).fontColor($r('app.color.primary')).onClick(() => { void copyVisibleText(text) })
      }
    }.width('100%')
    Scroll() {
      Text(text).fontSize(14).fontFamily('monospace').copyOption(CopyOptions.LocalDevice)
    }.scrollable(ScrollDirection.Horizontal).width('100%').align(Alignment.TopStart)
      .padding(10).backgroundColor($r('app.color.surface_subtle')).borderRadius(8)
    if (truncated) Text('内容过长，已截断，请在电脑端查看完整内容').fontSize(12).fontColor($r('app.color.text_secondary'))
  }

  build() {
    Column({ space: 8 }) {
      if (this.item.block.kind === 'thinking') {
        MarkdownText({ text: this.item.block.thinking })
      } else if (this.loading) {
        Row({ space: 8 }) { LoadingProgress().width(20).height(20); Text('正在从电脑加载工具详情') }.height(52)
      } else if (this.error.length > 0) {
        Text(this.error).fontSize(14)
        Button('重试').onClick(() => this.readDetail())
      } else {
        if (this.input().length > 0) this.code(this.shell() ? '命令' : '输入', this.input(), this.shell() && !this.detail?.inputTruncated, this.detail?.inputTruncated === true)
        if (this.answers.length > 0) this.code('已提交的答案', this.answers, false, false)
        if (this.output().length > 0 && this.output().trim() !== this.answers.trim()) this.code('结果', this.output(), this.shell(), this.detail?.outputTruncated === true || this.result?.truncated === true)
        else if (this.answers.length === 0) {
          if (this.result !== null || this.item.block.kind === 'tool_result') Text('工具已返回，结果没有可显示的文本。').fontSize(13)
          else if (this.input().length === 0 && !this.hideEmpty) Text('暂无可显示的工具详情').fontSize(13)
        }
      }
    }.width('100%').alignItems(HorizontalAlign.Start)
  }
}

@Component
export struct ActivityOverlay {
  @Link selectedActivity: ActivityFeedGroup | null
  @Link selectedStep: number
  @Prop messages: Array<FeedMessage> = []
  @Prop tasks: Array<SubagentTask> = []
  @Prop debugEnabled: boolean = false
  @Prop pageHeight: number = 720
  @State private expandedTool: string = ''
  load: ToolDetailLoader = (_requestId: string, _toolUseId: string) => Promise.reject(new Error('请先连接电脑'))
  onDebugOpen: () => void = () => {}

  private task(item: BlockFeedMessage): SubagentTask | undefined {
    const block = item.block
    return block.kind !== 'thinking' ? this.tasks.find((task: SubagentTask) =>
      task.requestId === item.requestId && task.toolUseId === block.toolUseId) : undefined
  }

  private title(item: BlockFeedMessage): string {
    if (item.block.kind === 'tool_use' && !item.block.titleSummary && !item.block.preview) {
      const summary = this.task(item)?.summary
      if (summary) return `子代理 · ${summary}`
    }
    return blockTitle(item.block, item.requestId, toolNamesByTurn(this.messages))
  }

  private status(item: BlockFeedMessage): string {
    const task = this.task(item)
    if (task !== undefined) {
      if (task.status === 'completed') return '成功'
      if (task.status === 'failed') return '失败'
      if (task.status === 'cancelled') return '已取消'
      return '执行中'
    }
    if (item.block.kind === 'thinking') return ''
    const result = this.result(item)
    if (result !== null) return result.isError ? '失败' : '成功'
    const parent = item.block.parentToolUseId
    const owner = parent === undefined ? undefined : this.tasks.find((candidate: SubagentTask) =>
      candidate.requestId === item.requestId && candidate.toolUseId === parent)
    if (owner !== undefined) return owner.status === 'running' ? '执行中' : '结果未收到'
    return toolStatusesByTurn(this.messages).get(JSON.stringify([item.requestId, item.block.toolUseId])) ?? '执行中'
  }

  private result(item: BlockFeedMessage): ToolResultContentBlock | null {
    const block = item.block
    if (block.kind === 'thinking') return null
    const result = this.messages.find((entry: FeedMessage) =>
      entry.kind === 'block' && entry.requestId === item.requestId && entry.block.kind === 'tool_result' &&
      entry.block.toolUseId === block.toolUseId)
    return result?.kind === 'block' && result.block.kind === 'tool_result' ? result.block : null
  }

  private answers(item: BlockFeedMessage): string {
    if (item.block.kind !== 'tool_use' || item.block.toolClass !== 'builtin' || item.block.toolName !== 'AskUserQuestion') return ''
    const tools = new Set<string>()
    let result = ''
    for (const entry of this.messages) {
      if (entry.requestId !== item.requestId) continue
      if (entry.kind === 'block' && entry.block.kind === 'tool_use' && entry.block.toolName === 'AskUserQuestion') tools.add(entry.block.toolUseId)
      if (entry.kind === 'answer') {
        if (result.length > 0) return ''
        result = entry.answers.map((answer: ResolvedAnswer) => answer.question + '\n' + answer.values.join('、')).join('\n\n')
      }
    }
    return tools.size === 1 ? result : ''
  }

  private children(item: BlockFeedMessage): BlockFeedMessage[] {
    const block = item.block
    if (block.kind !== 'tool_use') return []
    const candidates: BlockFeedMessage[] = []
    for (const message of this.messages) {
      if (message.kind === 'block' && message.requestId === item.requestId && message.block.kind !== 'thinking' &&
        message.block.parentToolUseId === block.toolUseId) candidates.push(message)
    }
    const group: ActivityFeedGroup = { kind: 'activity', requestId: item.requestId, blocks: candidates }
    return visibleSteps(group)
  }

  private isAgent(item: BlockFeedMessage): boolean {
    return item.block.kind === 'tool_use' && (item.block.toolName === 'Agent' || item.block.toolName === 'Task')
  }

  @Builder
  private mark(item: BlockFeedMessage): void {
    if (this.status(item) === '执行中') LoadingProgress().width(18).height(18).accessibilityText('执行中')
    else if (this.status(item).length > 0) {
      Text(this.status(item)).fontSize(12).fontColor(Color.White)
        .backgroundColor(this.status(item) === '成功' ? $r('app.color.badge_success') : $r('app.color.badge_warning'))
        .borderRadius(12).padding({ left: 8, right: 8, top: 2, bottom: 2 })
    }
  }

  @Builder
  private detailPage(item: BlockFeedMessage): void {
    Column() {
      Row({ space: 8 }) {
        Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(22).height(22) }
          .width(44).height(44).backgroundColor(Color.Transparent).accessibilityText('返回步骤')
          .onClick(() => { this.selectedStep = -1; this.expandedTool = '' })
        Text(this.title(item)).fontSize(17).fontWeight(FontWeight.Medium).layoutWeight(1).maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
        this.mark(item)
      }.width('100%').padding({ right: 14 })
      Scroll() {
        Column({ space: 10 }) {
          if (this.isAgent(item)) {
            Text('子工具').fontSize(15).fontWeight(FontWeight.Medium)
            if (this.children(item).length === 0) Text('暂未收到子工具').fontSize(13)
            ForEach(this.children(item), (child: BlockFeedMessage) => {
              Column() {
                Button({ type: ButtonType.Normal }) {
                  Row({ space: 8 }) {
                    Text(this.title(child)).fontSize(14).layoutWeight(1).textAlign(TextAlign.Start)
                    this.mark(child)
                    Text(this.expandedTool === child.block.id ? '⌄' : '›')
                  }.width('100%')
                }.width('100%').constraintSize({ minHeight: 44 }).backgroundColor(Color.Transparent)
                  .onClick(() => { this.expandedTool = this.expandedTool === child.block.id ? '' : child.block.id })
                if (this.expandedTool === child.block.id) {
                  ToolDetailContent({ item: child, result: this.result(child), answers: this.answers(child), load: this.load })
                }
              }.width('100%').padding(8).backgroundColor($r('app.color.surface')).borderRadius(10)
            }, (child: BlockFeedMessage) => child.block.id)
          }
          ToolDetailContent({ item, result: this.result(item), answers: this.answers(item), hideEmpty: this.isAgent(item), load: this.load })
        }.width('100%').padding(14).alignItems(HorizontalAlign.Start)
      }.layoutWeight(1).width('100%').align(Alignment.TopStart)
    }.width('100%').height('100%').backgroundColor($r('app.color.surface'))
      .transition(pageSlide())
      .gesture(PanGesture({ direction: PanDirection.Right, distance: 32 })
        .onActionStart((event: GestureEvent) => {
          if ((event.fingerList[0]?.localX ?? 100) - event.offsetX <= 22) { this.selectedStep = -1; this.expandedTool = '' }
        }))
  }

  build() {
    Stack({ alignContent: Alignment.Bottom }) {
      if (this.selectedActivity !== null) {
        if (this.selectedStep >= 0 && this.selectedStep < visibleSteps(this.selectedActivity as ActivityFeedGroup).length) {
          this.detailPage(visibleSteps(this.selectedActivity as ActivityFeedGroup)[this.selectedStep])
        } else {
          Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => { this.selectedActivity = null; this.selectedStep = -1 })
          Column() {
            Blank().width(36).height(5).backgroundColor($r('app.color.border')).borderRadius(3).margin({ top: 10, bottom: 10 })
            Row({ space: 8 }) {
              Text(stepSummary(this.selectedActivity as ActivityFeedGroup)).fontSize(18).fontWeight(FontWeight.Medium).layoutWeight(1)
              if (this.debugEnabled) { Button('调试').height(44).onClick(() => this.onDebugOpen()) }
              Button({ type: ButtonType.Normal }) { Image($r('app.media.drawer_close')).width(18).height(18) }
                .width(44).height(44).backgroundColor($r('app.color.surface_subtle')).accessibilityText('关闭步骤')
                .onClick(() => { this.selectedActivity = null; this.selectedStep = -1 })
            }.width('100%').padding({ left: 20, right: 20, bottom: 14 })
            Scroll() {
              Column() {
                ForEach(visibleSteps(this.selectedActivity as ActivityFeedGroup), (item: BlockFeedMessage, index: number) => {
                  Button({ type: ButtonType.Normal }) {
                    Row({ space: 10 }) {
                      Stack() {
                        if (index < visibleSteps(this.selectedActivity as ActivityFeedGroup).length - 1) {
                          Column().width(1).height(44).backgroundColor($r('app.color.border')).translate({ y: 22 })
                        }
                        Column().width(8).height(8).borderRadius(4).backgroundColor($r('app.color.primary_fill'))
                      }.width(10).height(44)
                      Text(this.title(item)).fontSize(15).layoutWeight(1).maxLines(2).textAlign(TextAlign.Start)
                      this.mark(item)
                      Text('›').fontSize(20).fontColor($r('app.color.text_secondary'))
                    }.width('100%')
                  }.width('100%').constraintSize({ minHeight: 44 }).padding({ left: 16, right: 16 })
                    .backgroundColor(Color.Transparent).onClick(() => { this.selectedStep = index; this.expandedTool = '' })
                }, (item: BlockFeedMessage) => item.block.id)
              }.width('100%')
            }.width('100%').constraintSize({ maxHeight: this.pageHeight * 0.72 }).align(Alignment.TopStart)
          }.padding({ bottom: 16 }).width('100%').backgroundColor($r('app.color.surface')).borderRadius({ topLeft: 24, topRight: 24 })
        }
      }
    }.width('100%').height('100%')
  }
}
