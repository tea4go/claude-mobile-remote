import { nativeLifecycle } from '../native/lifecycle'
import { MarkdownText } from './MarkdownText'
import { FeedDataSource, feedRenderKey, projectFeed, type FeedRow, type ActivityFeedGroup } from '../ui/feedDataSource'
import { toolNamesByTurn, toolNameForResult, toolStatusesByTurn } from '../session/stepIndex'
import { messageSelection } from '../ui/messageSelection'
import { inputDisplayText } from '../session/inputDisplay'
import type {
  Attachment,
  ContentBlock,
  ResolvedAnswer,
} from '../protocol/types'
import type { FeedMessage, BlockFeedMessage } from '../session/model'

function blockTitle(block: ContentBlock, requestId: string, tools: Map<string, string>): string {
  if (block.kind === 'thinking') return '思考过程'
  if (block.kind === 'tool_use') return block.toolClass === 'mcp' ? `MCP：${block.mcpServer} / ${block.toolName}` : `工具：${block.toolName}`
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

function blockBody(block: ContentBlock): string {
  if (block.kind === 'thinking') {
    return block.thinking.length > 0 ? block.thinking : 'thinking'
  }
  if (block.kind === 'tool_use') {
    if (block.preview !== undefined && block.preview.length > 0) {
      return `${block.toolName} · ${block.preview}`
    }
    return block.toolName
  }
  return (block.output ?? (block.isError ? '工具执行返回错误' : '工具执行已完成')) + (block.truncated ? '\n[结果已截断]' : '')
}

function inputStatusLabel(status: 'accepted' | 'unknown' | ''): string {
  if (status === 'accepted') {
    return ' · 已接收'
  }
  if (status === 'unknown') {
    return ' · 结果未知（连接已断开）'
  }
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
  @Prop debugEnabled: boolean = false
  onDebugOpen: () => void = () => {}
  private readonly source: FeedDataSource = new FeedDataSource()
  @Link selectedActivity: ActivityFeedGroup | null
  @Link selectedStep: number
  private toolNames: Map<string, string> = new Map()
  private toolStatuses: Map<string, string> = new Map()
  private previousMessages: Array<FeedMessage> = []

  aboutToAppear(): void {
    this.messagesChanged(); nativeLifecycle.messageBack = this.backHandler
    setTimeout(() => {
      if (this.source.totalCount() > 0) this.scroller.scrollToIndex(this.source.totalCount() - 1, false, ScrollAlign.END)
    }, 500)
  }
  aboutToDisappear(): void { messageSelection.close(); if (nativeLifecycle.messageBack === this.backHandler) nativeLifecycle.messageBack = null }
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

  build() {
    Stack({ alignContent: Alignment.Bottom }) {
    List({ space: 10, scroller: this.scroller }) {
      if (this.messages.length === 0) {
        ListItem() { Text('还没有消息。').fontSize(14).opacity(0.7) }
      } else {
        LazyForEach(this.source, (message: FeedRow, index: number) => {
          ListItem() {
          if (message.kind === 'input') {
            Row() {
              Blank()
              Column({ space: 6 }) {
                Column({ space: 6 }) {
                  if (message.attachments !== undefined) {
                    ForEach(message.attachments, (attachment: Attachment) => {
                      if (attachment.mediaType.startsWith('image/')) {
                        Image(`data:${attachment.mediaType};base64,${attachment.data}`).height(140).width('100%').objectFit(ImageFit.Contain)
                          .onClick(() => this.onPreviewImage(`data:${attachment.mediaType};base64,${attachment.data}`))
                      }
                      Text(attachment.name).fontSize(12)
                    }, (attachment: Attachment, itemIndex: number) => `${itemIndex}-${attachment.name}`)
                  }
                  MarkdownText({ text: inputDisplayText(message.text), plain: true, scrollController: this.scroller })
                }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%').backgroundColor('#3478f6').borderRadius(16)
                Text(`${message.source === 'terminal' ? '电脑端' : '手机端'}${inputStatusLabel(message.status)}`)
                  .fontSize(12).fontColor('#8e8e93').width('100%').textAlign(TextAlign.End)
              }.width('80%').alignItems(HorizontalAlign.End)
            }.width('100%')
          } else if (message.kind === 'answer') {
            Column({ space: 8 }) {
              Row({ space: 6 }) { Image($r('app.media.claude_mark')).width(18).height(18); Text('Claude').fontSize(12).fontColor('#748198') }.width('100%')

              ForEach(message.answers, (answer: ResolvedAnswer) => {
                Column({ space: 4 }) {
                  Text(answerTitle(answer)).copyOption(CopyOptions.LocalDevice)
                    .fontSize(13)
                    .fontWeight(FontWeight.Medium)
                    .width('100%')

                  ForEach(answer.values, (value: string) => {
                    Row({ space: 8 }) {
                      Text('✓').fontSize(13).fontColor('#ffffff').width(20).height(20).textAlign(TextAlign.Center).backgroundColor('#32946c').borderRadius(5)
                      Text(value).copyOption(CopyOptions.LocalDevice).fontSize(14).fontColor('#394760').layoutWeight(1)
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
            .backgroundColor('#ffffff')
            .borderRadius(12)
          } else if (message.kind === 'activity') {
            Button({ type: ButtonType.Normal }) {
              Row() {
                Text(`☷  ${stepSummary(message)}`).fontSize(14).fontColor('#394760').layoutWeight(1).textAlign(TextAlign.Start)
                Text('›').fontSize(20).fontColor('#748198')
              }.width('100%')
            }.width('100%').height(48).padding({ left: 14, right: 14 }).backgroundColor('#ffffff').borderRadius(14)
              .onClick(() => { this.selectedActivity = message; this.selectedStep = -1 })
          } else if (message.kind === 'block') {
            Text('步骤').fontSize(13)
          } else if (message.text.trim().length > 0 || message.status === 'streaming' || message.status === 'failed' || message.status === 'cancelled') {
            Column({ space: 8 }) {
              Row({ space: 6 }) { Image($r('app.media.claude_mark')).width(18).height(18); Text('Claude').fontSize(12).fontColor('#748198') }.width('100%')

              if (message.text.trim().length > 0) { Column() { MarkdownText({ text: message.text, scrollController: this.scroller }) }.padding(14).width('100%').backgroundColor('#ffffff').borderRadius(16) }

              if (message.status === 'streaming') { LoadingProgress().width(20).height(20) }
              if (message.status === 'cancelled') {
                Text('已取消')
                  .fontSize(12)
                  .padding({ left: 8, right: 8, top: 4, bottom: 4 })
                  .backgroundColor('#fff4d6')
                  .borderRadius(999)
              }

              if (message.status === 'failed' && message.error !== null) {
                Text(`Turn 失败：${message.error.message}`)
                  .fontSize(12)
                  .fontColor('#c0392b')
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
    .onScroll(() => { messageSelection.refresh(); this.onAtBottom(this.scroller.isAtEnd()) })
    .onReachEnd(() => this.onAtBottom(true))
    }.width('100%').height('100%')
  }
}

@Component
export struct ActivityOverlay {
  @Link selectedActivity: ActivityFeedGroup | null
  @Link selectedStep: number
  @Prop messages: Array<FeedMessage> = []
  @Prop debugEnabled: boolean = false
  @Prop pageHeight: number = 720
  onDebugOpen: () => void = () => {}
  build() {
    Stack({ alignContent: Alignment.Bottom }) {
    if (this.selectedActivity !== null) {
      Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => { this.selectedActivity = null; this.selectedStep = -1 })
      Column() {
        Blank().width(36).height(5).backgroundColor('#d5deea').borderRadius(3).margin({ top: 10, bottom: 10 })
        Row({ space: 8 }) {
          if (this.selectedStep >= 0) {
            Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(22).height(22) }
              .width(36).height(44).padding(0).backgroundColor(Color.Transparent).onClick(() => { this.selectedStep = -1 })
          }
          Text(this.selectedStep >= 0 ? blockTitle(visibleSteps(this.selectedActivity as ActivityFeedGroup)[this.selectedStep].block, (this.selectedActivity as ActivityFeedGroup).requestId, toolNamesByTurn(this.messages)) : stepSummary(this.selectedActivity as ActivityFeedGroup))
            .fontSize(18).fontWeight(FontWeight.Medium).layoutWeight(1).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis })
          if (this.debugEnabled) { Button('调试', { type: ButtonType.Normal }).height(44).fontColor('#3478f6').backgroundColor(Color.Transparent).onClick(() => this.onDebugOpen()) }
          Button({ type: ButtonType.Normal }) { Image($r('app.media.drawer_close')).width(18).height(18) }
            .width(44).height(44).padding(0).backgroundColor('#f8fafc').border({ width: 1, color: '#e9eef5' }).borderRadius(22).onClick(() => { this.selectedActivity = null; this.selectedStep = -1 })
        }.width('100%').padding({ left: 20, right: 20, bottom: 14 })
        Blank().height(1).width('100%').backgroundColor('#edf0f4')
        Scroll() {
          Column() {
            ForEach(visibleSteps(this.selectedActivity as ActivityFeedGroup), (item: BlockFeedMessage, itemIndex: number) => {
              if (this.selectedStep < 0) {
                Row({ space: 10 }) {
                  Column() {
                    Blank().width(8).height(8).borderRadius(4).backgroundColor(item.block.kind === 'thinking' ? '#3478f6' : (toolStatusesByTurn(this.messages).get(JSON.stringify([item.requestId, item.block.toolUseId])) === '已完成' ? '#00b578' : '#b88020'))
                    if (itemIndex < visibleSteps(this.selectedActivity as ActivityFeedGroup).length - 1) { Blank().width(1).layoutWeight(1).backgroundColor('#dce5f0') }
                  }.width(10).height('100%').padding({ top: 6 })
                  Column({ space: 4 }) {
                    Row({ space: 8 }) {
                      Text(blockTitle(item.block, item.requestId, toolNamesByTurn(this.messages))).fontSize(15).layoutWeight(1).maxLines(1)
                      if (item.block.kind === 'tool_use') {
                        Text(toolStatusesByTurn(this.messages).get(JSON.stringify([item.requestId, item.block.toolUseId])) === '已完成' ? '成功' : (toolStatusesByTurn(this.messages).get(JSON.stringify([item.requestId, item.block.toolUseId])) ?? '执行中'))
                          .fontSize(12).fontColor('#ffffff').backgroundColor(toolStatusesByTurn(this.messages).get(JSON.stringify([item.requestId, item.block.toolUseId])) === '已完成' ? '#00b578' : '#b88020').borderRadius(12).padding({ left: 8, right: 8, top: 2, bottom: 2 })
                      }
                      Text('›').fontSize(22).fontColor('#bfbfbf')
                    }.width('100%')
                    if (item.block.kind === 'tool_use' && (item.block.preview ?? '').length > 0) {
                      Text(item.block.preview ?? '').fontSize(13).fontColor('#8c8c8c').maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
                    }
                  }.layoutWeight(1).alignItems(HorizontalAlign.Start)
                }.width('100%').height(itemIndex === visibleSteps(this.selectedActivity as ActivityFeedGroup).length - 1 ? 74 : 100).padding({ left: 20, right: 20, top: 18 }).alignItems(VerticalAlign.Top).onClick(() => { this.selectedStep = itemIndex })
              } else if (this.selectedStep === itemIndex) {
                Column({ space: 18 }) {
                  Text(item.block.kind === 'thinking' ? '内容' : item.block.kind === 'tool_use' ? '输入' : '结果').fontSize(13).fontColor('#8c8c8c').width('100%')
                  Text(blockBody(item.block)).fontSize(15).fontColor('#333333').copyOption(CopyOptions.LocalDevice).width('100%')
                  if (item.block.kind === 'tool_use') {
                    ForEach((this.selectedActivity as ActivityFeedGroup).blocks, (result: BlockFeedMessage) => {
                      if (result.block.kind === 'tool_result' && item.block.kind === 'tool_use' && result.block.toolUseId === item.block.toolUseId) {
                        Text('结果').fontSize(13).fontColor('#8c8c8c').width('100%')
                        Text(blockBody(result.block)).fontSize(15).fontColor('#333333').copyOption(CopyOptions.LocalDevice).width('100%')
                      }
                    }, (result: BlockFeedMessage) => result.block.id)
                  }
                }.width('100%').padding({ left: 24, right: 24, top: 18, bottom: 24 }).alignItems(HorizontalAlign.Start)
              }
            }, (item: BlockFeedMessage, itemIndex: number) => `${itemIndex}-${item.block.id}`)
          }.width('100%').alignItems(HorizontalAlign.Start)
        }.width('100%').constraintSize({ maxHeight: this.pageHeight * 0.72 }).align(Alignment.TopStart)
      }.padding({ bottom: 16 }).width('100%').backgroundColor('#ffffff').borderRadius({ topLeft: 24, topRight: 24 })
    }
    }.width('100%').height('100%')
  }
}
