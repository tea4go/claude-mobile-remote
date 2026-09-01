import type {
  ContentBlock,
  ResolvedAnswer,
} from '../protocol/types'
import type { FeedMessage } from '../session/model'

interface MessageSegment {
  kind: 'text' | 'code'
  text: string
}

function summarizeToolResult(messages: Array<FeedMessage>, index: number, toolUseId: string): string {
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    const candidate = messages[cursor]
    if (candidate.kind !== 'block') {
      continue
    }
    const block = candidate.block
    if (block.kind === 'tool_use' && block.toolUseId === toolUseId) {
      return block.toolName
    }
  }
  return ''
}

function blockTitle(block: ContentBlock, messages: Array<FeedMessage>, index: number): string {
  if (block.kind === 'thinking') {
    return '思考'
  }
  if (block.kind === 'tool_use') {
    return '工具调用'
  }
  const toolName = summarizeToolResult(messages, index, block.toolUseId)
  if (toolName.length > 0) {
    return `工具结果 · ${toolName}`
  }
  return '工具结果'
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
  return block.isError ? '工具执行返回错误' : '工具执行已完成'
}

function splitClaudeText(text: string): Array<MessageSegment> {
  const segments: Array<MessageSegment> = []
  const fence = '```'
  let cursor = 0
  let inCode = false

  while (cursor < text.length) {
    const fenceIndex = text.indexOf(fence, cursor)
    if (fenceIndex < 0) {
      const tail = text.slice(cursor)
      if (tail.length > 0) {
        segments.push({ kind: inCode ? 'code' : 'text', text: tail })
      }
      break
    }

    const part = text.slice(cursor, fenceIndex)
    if (part.length > 0) {
      segments.push({ kind: inCode ? 'code' : 'text', text: part })
    }
    cursor = fenceIndex + fence.length
    inCode = !inCode
  }

  if (segments.length === 0 && text.length > 0) {
    segments.push({ kind: 'text', text })
  }
  return segments
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
  @Prop messages: Array<FeedMessage> = []

  build() {
    Column({ space: 10 }) {
      Text('消息流')
        .fontSize(18)
        .fontWeight(FontWeight.Medium)

      if (this.messages.length === 0) {
        Text('还没有消息。')
          .fontSize(14)
          .opacity(0.7)
      } else {
        ForEach(this.messages, (message: FeedMessage, index: number) => {
          if (message.kind === 'input') {
            Column({ space: 6 }) {
              Text(message.text)
                .fontSize(14)
                .width('100%')
                .textAlign(TextAlign.Start)

              Text(`${message.source === 'terminal' ? '电脑端' : '手机端'}${inputStatusLabel(message.status)}`)
                .fontSize(12)
                .opacity(0.7)
                .width('100%')
                .textAlign(TextAlign.End)
            }
            .padding(12)
            .width('100%')
            .alignItems(HorizontalAlign.End)
            .backgroundColor('#e6f4ff')
            .borderRadius(12)
          } else if (message.kind === 'answer') {
            Column({ space: 8 }) {
              Text('你的回答')
                .fontSize(12)
                .opacity(0.7)
                .width('100%')

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
          } else if (message.kind === 'block') {
            Column({ space: 6 }) {
              Text(blockTitle(message.block, this.messages, index))
                .fontSize(12)
                .opacity(0.7)
                .width('100%')

              Text(blockBody(message.block))
                .fontSize(14)
                .width('100%')
                .textAlign(TextAlign.Start)
            }
            .padding(12)
            .width('100%')
            .alignItems(HorizontalAlign.Start)
            .backgroundColor('#ffffff')
            .borderRadius(12)
          } else {
            Column({ space: 8 }) {
              Text('Claude')
                .fontSize(12)
                .opacity(0.7)
                .width('100%')

              ForEach(splitClaudeText(message.text), (segment: MessageSegment) => {
                if (segment.kind === 'code') {
                  Text(segment.text)
                    .fontSize(13)
                    .fontFamily('monospace')
                    .width('100%')
                    .padding(10)
                    .backgroundColor('#f4f6f8')
                    .borderRadius(8)
                } else {
                  Text(segment.text)
                    .fontSize(14)
                    .width('100%')
                    .textAlign(TextAlign.Start)
                }
              }, (segment: MessageSegment, segmentIndex: number) => `${message.requestId}-${index}-${segmentIndex}`)

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
            .padding(12)
            .width('100%')
            .alignItems(HorizontalAlign.Start)
            .backgroundColor('#ffffff')
            .borderRadius(12)
          }
        }, (message: FeedMessage, index: number) => `${message.kind}-${message.requestId}-${index}`)
      }
    }
    .id('message-list')
    .width('100%')
    .alignItems(HorizontalAlign.Start)
  }
}
