import { copyVisibleText } from '../native/clipboard'
import { nativeLifecycle } from '../native/lifecycle'
import { MarkdownText } from './MarkdownText'
import { FeedDataSource, feedRenderKey, projectFeed, type FeedRow, type ActivityFeedGroup } from '../ui/feedDataSource'
import { toolNamesByTurn, toolNameForResult, toolStatusesByTurn } from '../session/stepIndex'
import { markdownVisibleText } from '../session/markdown'
import { messageSelection } from '../ui/messageSelection'
import type {
  Attachment,
  ContentBlock,
  ResolvedAnswer,
} from '../protocol/types'
import type { FeedMessage, BlockFeedMessage } from '../session/model'

function blockTitle(block: ContentBlock, requestId: string, tools: Map<string, string>): string {
  if (block.kind === 'thinking') return '思考'
  if (block.kind === 'tool_use') return '工具调用'
  const toolName = toolNameForResult(tools, requestId, block.toolUseId)
  return toolName.length > 0 ? `工具结果 · ${toolName}` : '工具结果'
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
  @State private selectedActivity: ActivityFeedGroup | null = null
  @State private selectedStep: number = -1
  private toolNames: Map<string, string> = new Map()
  private toolStatuses: Map<string, string> = new Map()
  private previousMessages: Array<FeedMessage> = []

  aboutToAppear(): void { this.messagesChanged(); nativeLifecycle.messageBack = this.backHandler }
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
                  MarkdownText({ text: message.text, plain: true, scrollController: this.scroller })
                }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%').backgroundColor('#3478f6').borderRadius(16)
                Button('复制', { type: ButtonType.Normal }).height(36).fontSize(12).fontColor('#3478f6').backgroundColor(Color.Transparent).onClick(() => { void copyVisibleText(message.text).catch(() => {}) })
                Text(`${message.source === 'terminal' ? '电脑端' : '手机端'}${inputStatusLabel(message.status)}`)
                  .fontSize(12).fontColor('#8e8e93').width('100%').textAlign(TextAlign.End)
              }.width('80%').alignItems(HorizontalAlign.End)
            }.width('100%')
          } else if (message.kind === 'answer') {
            Column({ space: 8 }) {
              Text('你的回答')
                .fontSize(12)
                .opacity(0.7)
                .width('100%')

              Button('复制回答', { type: ButtonType.Normal }).height(36).fontSize(12).fontColor('#3478f6').backgroundColor(Color.Transparent).onClick(() => { void copyVisibleText(message.answers.map((answer: ResolvedAnswer) => `${answerTitle(answer)}\n${answer.values.join('、')}`).join('\n\n')).catch(() => {}) })
              ForEach(message.answers, (answer: ResolvedAnswer) => {
                Column({ space: 4 }) {
                  Text(answerTitle(answer))
                    .fontSize(13)
                    .fontWeight(FontWeight.Medium)
                    .width('100%')

                  ForEach(answer.values, (value: string) => {
                    Text(value)
                      .fontSize(12)
                      .padding({ left: 8, right: 8, top: 4, bottom: 4 })
                      .backgroundColor('#eef5ff')
                      .borderRadius(999)
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
                Text(`执行步骤 · ${message.blocks.length} 项`).fontSize(13).fontColor('#3478f6').layoutWeight(1).textAlign(TextAlign.Start)
                Text('›').fontSize(20).fontColor('#748198')
              }.width('100%')
            }.width('100%').height(48).padding({ left: 14, right: 14 }).backgroundColor('#ffffff').borderRadius(14)
              .onClick(() => { this.selectedActivity = message; this.selectedStep = -1 })
          } else if (message.kind === 'block') {
            Text('步骤').fontSize(13)
          } else {
            Column({ space: 8 }) {
              Text('Claude')
                .fontSize(12)
                .opacity(0.7)
                .width('100%')

              Column() { MarkdownText({ text: message.text, scrollController: this.scroller }) }.padding(14).width('100%').backgroundColor('#ffffff').borderRadius(16)
              Button('复制', { type: ButtonType.Normal }).height(36).fontSize(12).fontColor('#3478f6').backgroundColor(Color.Transparent).onClick(() => { void copyVisibleText(markdownVisibleText(message.text)).catch(() => {}) })

              if (message.status === 'streaming') { LoadingProgress().width(20).height(20) }
              if (message.status === 'done') { Text('已完成').fontSize(12).opacity(0.6) }
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
          }
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
    if (this.selectedActivity !== null) {
      Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => { this.selectedActivity = null; this.selectedStep = -1 })
      Column({ space: 8 }) {
        Row() {
          if (this.selectedStep >= 0) { Button('‹', { type: ButtonType.Normal }).width(44).height(44).fontSize(26).backgroundColor(Color.Transparent).onClick(() => { this.selectedStep = -1 }) }
          Text(this.selectedStep >= 0 ? '步骤详情' : '执行步骤').fontSize(18).fontWeight(FontWeight.Medium).layoutWeight(1)
          if (this.debugEnabled) { Button('调试', { type: ButtonType.Normal }).height(44).fontColor('#3478f6').backgroundColor(Color.Transparent).onClick(() => this.onDebugOpen()) }
          Button('关闭', { type: ButtonType.Normal }).height(44).fontColor('#3478f6').backgroundColor(Color.Transparent).onClick(() => { this.selectedActivity = null; this.selectedStep = -1 })
        }.width('100%')
        Scroll() {
          Column({ space: 10 }) {
            ForEach((this.selectedActivity as ActivityFeedGroup).blocks, (item: BlockFeedMessage, itemIndex: number) => {
              if (this.selectedStep < 0 || this.selectedStep === itemIndex) {
                Column({ space: 6 }) {
                  Text(blockTitle(item.block, item.requestId, this.toolNames)).fontSize(14).fontWeight(FontWeight.Medium).width('100%')
                  if (item.block.kind === 'tool_use') { Text(this.toolStatuses.get(JSON.stringify([item.requestId, item.block.toolUseId])) ?? '进行中').fontSize(12).fontColor('#748198').width('100%') }
                  Text(blockBody(item.block)).fontSize(13).fontColor('#748198').maxLines(this.selectedStep < 0 ? 2 : 10000).textOverflow({ overflow: TextOverflow.Ellipsis }).copyOption(CopyOptions.LocalDevice).width('100%')
                }.padding(12).width('100%').backgroundColor('#f4f6fa').borderRadius(12).onClick(() => { if (this.selectedStep < 0) this.selectedStep = itemIndex })
              }
            }, (item: BlockFeedMessage, itemIndex: number) => `${itemIndex}-${item.block.id}`)
          }.width('100%').alignItems(HorizontalAlign.Start)
        }.width('100%').constraintSize({ maxHeight: 440 }).align(Alignment.TopStart)
      }.padding(16).width('100%').backgroundColor('#ffffff').borderRadius({ topLeft: 24, topRight: 24 })
    }
    }.width('100%').height('100%')
  }
}
