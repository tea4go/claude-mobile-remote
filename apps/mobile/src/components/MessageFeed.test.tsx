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

  // D52：回答提交后必须能在消息流里看到选了什么，包括多选。
  test('答案回显展示问题与所选取值', () => {
    const messages: Message[] = [
      {
        kind: 'answer',
        requestId: 'req-1',
        answers: [
          { questionId: 'q1', header: '框架', question: '选哪个框架？', values: ['React'] },
          { questionId: 'q2', header: '', question: '选哪些端？', values: ['iOS', 'Android'] },
        ],
      },
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByTestId('answer-echo')).toBeInTheDocument()
    expect(screen.getByText('你的回答')).toBeInTheDocument()
    expect(screen.getByText('框架 · 选哪个框架？')).toBeInTheDocument()
    expect(screen.getByText('选哪些端？')).toBeInTheDocument()
    expect(screen.getByText('React')).toBeInTheDocument()
    expect(screen.getByText('iOS')).toBeInTheDocument()
    expect(screen.getByText('Android')).toBeInTheDocument()
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
    // 没有任何可展示内容的占位消息整条跳过，不留下孤立的来源标记。
    expect(container.textContent).not.toContain('Claude')
  })

  test('取消与失败即使没有正文也仍然留痕', () => {
    const cancelled = render(<MessageFeed messages={[claude({ text: '', status: 'cancelled' })]} />)
    expect(cancelled.getByText('已取消')).toBeInTheDocument()
    cancelled.unmount()
    const failed = render(
      <MessageFeed
        messages={[claude({ text: '', status: 'failed', error: { code: 'internal', message: 'boom' } })]}
      />,
    )
    expect(failed.getByText(/boom/)).toBeInTheDocument()
  })
})

describe('MessageFeed：结构化活动分组（B2 §6）', () => {
  test('结构化块只在正文里留一行摘要，不渲染活动内容', () => {
    const { container } = render(
      <MessageFeed
        messages={[block({ id: 'b1', kind: 'thinking', thinking: '先看现有实现' })]}
      />,
    )
    expect(screen.getByText('查看 1 个步骤')).toBeInTheDocument()
    expect(screen.queryByText('先看现有实现')).not.toBeInTheDocument()
    expect(container.textContent).not.toContain('只读占位')
  })

  test('思考与工具合并进同一个分组，思考不再打断连续工具', () => {
    render(
      <MessageFeed
        messages={[
          block({ id: 'b1', kind: 'thinking', thinking: '先看现有实现' }),
          block({ id: 'b2', kind: 'tool_use', toolUseId: 'tu-1', toolClass: 'builtin', toolName: 'Bash', mcpServer: '' }),
          block({ id: 'b3', kind: 'thinking', thinking: '再改代码' }),
          block({ id: 'b4', kind: 'tool_use', toolUseId: 'tu-2', toolClass: 'builtin', toolName: 'Read', mcpServer: '' }),
        ]}
      />,
    )
    expect(screen.getAllByTestId('activity-group')).toHaveLength(1)
    expect(screen.getByText('查看 4 个步骤，运行 2 个工具')).toBeInTheDocument()
  })

  test('摘要行按到达顺序排在两段文本之间', () => {
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
    const text = container.textContent ?? ''
    expect(text.indexOf('开始处理')).toBeLessThan(text.indexOf('查看 1 个步骤'))
    expect(text.indexOf('查看 1 个步骤')).toBeLessThan(text.indexOf('处理完成'))
  })

  test('turn_started 的空占位消息不切开活动摘要与正文', () => {
    // 真实顺序（reducer）：输入 → 空占位 → 内容块 → 带正文的另一条 Claude 消息。
    const { container } = render(
      <MessageFeed
        messages={[
          input({ text: '你好' }),
          claude({ text: '' }),
          block({ id: 'b1', kind: 'thinking', thinking: '先看清要改什么' }),
          block({
            id: 'b2',
            kind: 'tool_use',
            toolUseId: 'tu-1',
            toolClass: 'builtin',
            toolName: 'Bash',
            mcpServer: '',
          }),
          claude({ text: '执行完成' }),
        ]}
      />,
    )
    // 只剩一个「Claude」来源标记（属于带正文的那条）。
    expect(screen.getAllByText('Claude')).toHaveLength(1)
    const text = container.textContent ?? ''
    expect(text.indexOf('查看 2 个步骤')).toBeLessThan(text.indexOf('执行完成'))
  })

  test('工具与同一 toolUseId 的结果合并为一条活动，抽屉内显示工具名与状态', () => {
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
    fireEvent.click(screen.getByTestId('activity-group-summary'))
    expect(screen.getAllByTestId('activity-row')).toHaveLength(1)
    expect(screen.getByText('工具：Grep')).toBeInTheDocument()
    expect(screen.getByText('成功')).toBeInTheDocument()
  })

  test('连续工具合并为一个摘要行', () => {
    render(
      <MessageFeed
        messages={[
          block({ id: 'b6', kind: 'tool_use', toolUseId: 'tu-6', toolClass: 'builtin', toolName: 'Bash', mcpServer: '', preview: 'ls' }),
          block({ id: 'b7', kind: 'tool_result', toolUseId: 'tu-6', isError: false }),
          block({ id: 'b8', kind: 'tool_use', toolUseId: 'tu-8', toolClass: 'builtin', toolName: 'Read', mcpServer: '', preview: 'README.md' }),
        ]}
      />,
    )
    expect(screen.getByText('查看 2 个步骤，运行 2 个工具')).toBeInTheDocument()
    expect(screen.queryByText('工具：Bash')).not.toBeInTheDocument()
  })

  test('孤立工具结果不编造工具名', () => {
    render(
      <MessageFeed
        messages={[block({ id: 'b5', kind: 'tool_result', toolUseId: 'tu-x', isError: true })]}
      />,
    )
    fireEvent.click(screen.getByTestId('activity-group-summary'))
    expect(screen.getByText('工具结果')).toBeInTheDocument()
    expect(screen.getByText('失败')).toBeInTheDocument()
  })

  test('已取消的 Turn 显示已取消标记（区分于取消请求）', () => {
    render(<MessageFeed messages={[claude({ status: 'cancelled', text: '已输出一半' })]} />)
    expect(screen.getByText('已取消')).toBeInTheDocument()
    expect(screen.getByText('已输出一半')).toBeInTheDocument()
  })
})
