import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import MessageFeed from './MessageFeed'
import type { Message } from '../session/reducer'

function claude(overrides: Partial<Extract<Message, { kind: 'claude' }>> = {}): Message {
  return {
    kind: 'claude',
    requestId: 'req-1',
    source: 'terminal',
    text: '',
    status: 'streaming',
    error: null,
    ...overrides,
  }
}

function block(b: Extract<Message, { kind: 'block' }>['block']): Message {
  return { kind: 'block', requestId: 'req-1', block: b }
}

function input(overrides: Partial<Extract<Message, { kind: 'input' }>> = {}): Message {
  return {
    kind: 'input',
    source: 'mobile',
    requestId: 'req-1',
    text: '你好',
    status: 'accepted',
    ...overrides,
  }
}

describe('MessageFeed', () => {
  test('流式文本变化时在底部自动滚动', () => {
    const scrollIntoView = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
    const first = render(<MessageFeed messages={[claude({ text: '第一段' })]} />)
    scrollIntoView.mockClear()

    first.rerender(<MessageFeed messages={[claude({ text: '第一段第二段' })]} />)

    expect(scrollIntoView).toHaveBeenCalled()
  })

  test('用户向上阅读时新消息不强制滚到底部', () => {
    const scrollIntoView = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
    const { getByTestId, rerender } = render(<MessageFeed messages={[claude({ text: '第一段' })]} />)
    const feed = getByTestId('message-feed')
    Object.defineProperties(feed, {
      scrollTop: { configurable: true, value: 100, writable: true },
      scrollHeight: { configurable: true, value: 1000 },
      clientHeight: { configurable: true, value: 400 },
    })
    fireEvent.scroll(feed)
    scrollIntoView.mockClear()

    rerender(<MessageFeed messages={[claude({ text: '第一段第二段' })]} />)

    expect(scrollIntoView).not.toHaveBeenCalled()
  })

  test('本页面手机输入展示正文与已接收', () => {
    const messages: Message[] = [
      input({ source: 'mobile', requestId: 'req-1', text: '你好', status: 'accepted' }),
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText('你好')).toBeInTheDocument()
    expect(screen.getByText(/已接收/)).toBeInTheDocument()
  })

  test('结果未知的手机输入展示提示', () => {
    const messages: Message[] = [
      input({ source: 'mobile', requestId: null, text: '你好', status: 'unknown' }),
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText(/结果未知/)).toBeInTheDocument()
  })

  test('PC 输入展示正文与电脑端标识（A5.1）', () => {
    const messages: Message[] = [
      input({ source: 'terminal', requestId: 'req-pc', text: '来自电脑的问题', status: null }),
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText('来自电脑的问题')).toBeInTheDocument()
    expect(screen.getByText(/电脑端/)).toBeInTheDocument()
  })

  test('其他手机连接的输入只显示手机端标识，无确认状态（A5.1）', () => {
    const messages: Message[] = [
      input({ source: 'mobile', requestId: 'req-other', text: '另一台手机', status: null }),
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText('另一台手机')).toBeInTheDocument()
    expect(screen.getByText(/手机端/)).toBeInTheDocument()
    expect(screen.queryByText(/已接收/)).not.toBeInTheDocument()
    expect(screen.queryByText(/结果未知/)).not.toBeInTheDocument()
  })

  test('Claude 流式文本按纯文本渲染并保留换行', () => {
    const messages: Message[] = [claude({ text: '第一行\n第二行' })]
    const { container } = render(<MessageFeed messages={messages} />)
    expect(container.textContent).toContain('第一行\n第二行')
  })

  test('Turn 失败展示错误信息', () => {
    const messages: Message[] = [
      claude({ status: 'failed', error: { code: 'internal', message: 'boom' } }),
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText(/boom/)).toBeInTheDocument()
  })
})

describe('MessageFeed：Markdown 渲染边界（B1）', () => {
  test('Claude 文本按 Markdown 渲染列表与代码块', () => {
    const messages: Message[] = [claude({ text: '- 步骤一\n- 步骤二\n\n```go\nfmt.Println(1)\n```' })]
    const { container } = render(<MessageFeed messages={messages} />)
    expect(container.querySelectorAll('li')).toHaveLength(2)
    expect(container.querySelector('pre code')?.textContent).toContain('fmt.Println(1)')
  })

  test('Claude 文本中的恶意 HTML 不进入 DOM', () => {
    const messages: Message[] = [claude({ text: '<img src=x onerror="x()"><script>x()</script>' })]
    const { container } = render(<MessageFeed messages={messages} />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('script')).toBeNull()
  })

  test('输入消息保持纯文本，不解析 Markdown', () => {
    const messages: Message[] = [input({ text: '**不要加粗** 和 `不要代码`' })]
    const { container } = render(<MessageFeed messages={messages} />)
    expect(container.querySelector('strong')).toBeNull()
    expect(container.querySelector('code')).toBeNull()
    expect(container.textContent).toContain('**不要加粗** 和 `不要代码`')
  })

  test('空 Claude 文本不渲染消息气泡', () => {
    const { container } = render(<MessageFeed messages={[claude({ text: '' })]} />)
    expect(container.querySelector('p')).toBeNull()
  })
})

describe('MessageFeed：结构化内容块（B2 §6）', () => {
  test('结构化块渲染为默认折叠的摘要行，不再出现只读占位', () => {
    const { container } = render(
      <MessageFeed
        messages={[block({ id: 'b1', kind: 'thinking', thinking: '先看现有实现' })]}
      />,
    )
    expect(screen.getByText('思考过程')).toBeInTheDocument()
    expect(screen.queryByText('先看现有实现')).not.toBeInTheDocument()
    expect(container.textContent).not.toContain('只读占位')
  })

  test('工具调用与文本按到达顺序混排', () => {
    const { container } = render(
      <MessageFeed
        messages={[
          claude({ text: '开始处理' }),
          block({
            id: 'b2',
            kind: 'tool_use',
            toolUseId: 'tu-1',
            toolClass: 'builtin',
            toolName: 'Bash',
            mcpServer: '',
          }),
          claude({ requestId: 'req-2', text: '处理完成' }),
        ]}
      />,
    )
    fireEvent.click(screen.getByText('工具调用（1）'))
    const text = container.textContent ?? ''
    expect(text.indexOf('开始处理')).toBeLessThan(text.indexOf('Bash'))
    expect(text.indexOf('Bash')).toBeLessThan(text.indexOf('处理完成'))
  })

  test('工具结果关联同一 toolUseId 的工具名', () => {
    render(
      <MessageFeed
        messages={[
          block({
            id: 'b3',
            kind: 'tool_use',
            toolUseId: 'tu-9',
            toolClass: 'builtin',
            toolName: 'Grep',
            mcpServer: '',
          }),
          block({ id: 'b4', kind: 'tool_result', toolUseId: 'tu-9', isError: false }),
        ]}
      />,
    )
    expect(screen.getByText('工具调用（1）')).toBeInTheDocument()
    fireEvent.click(screen.getByText('工具调用（1）'))
    expect(screen.getByText(/工具：Grep/)).toBeInTheDocument()
    expect(screen.getByText('成功')).toBeInTheDocument()
  })

  test('连续工具合并为一个默认折叠的分组', () => {
    render(
      <MessageFeed
        messages={[
          block({ id: 'b6', kind: 'tool_use', toolUseId: 'tu-6', toolClass: 'builtin', toolName: 'Bash', mcpServer: '', preview: 'ls' }),
          block({ id: 'b7', kind: 'tool_result', toolUseId: 'tu-6', isError: false }),
          block({ id: 'b8', kind: 'tool_use', toolUseId: 'tu-8', toolClass: 'builtin', toolName: 'Read', mcpServer: '', preview: 'README.md' }),
        ]}
      />,
    )
    expect(screen.getByText('工具调用（2）')).toBeInTheDocument()
    expect(screen.queryByText(/工具：Bash/)).not.toBeInTheDocument()
  })

  test('孤立工具结果不编造工具名', () => {
    render(
      <MessageFeed
        messages={[block({ id: 'b5', kind: 'tool_result', toolUseId: 'tu-x', isError: true })]}
      />,
    )
    expect(screen.getByText('工具结果')).toBeInTheDocument()
    expect(screen.getByText('失败')).toBeInTheDocument()
  })

  test('已取消的 Turn 显示已取消标记（区分于取消请求）', () => {
    render(<MessageFeed messages={[claude({ status: 'cancelled', text: '已输出一半' })]} />)
    expect(screen.getByText('已取消')).toBeInTheDocument()
    expect(screen.getByText('已输出一半')).toBeInTheDocument()
  })
})
