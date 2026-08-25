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

  test('手机输入展示正文与已接收', () => {
    const messages: Message[] = [
      { kind: 'mobile', requestId: 'req-1', text: '你好', status: 'accepted' },
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText('你好')).toBeInTheDocument()
    expect(screen.getByText('已接收')).toBeInTheDocument()
  })

  test('结果未知的手机输入展示提示', () => {
    const messages: Message[] = [
      { kind: 'mobile', requestId: null, text: '你好', status: 'unknown' },
    ]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText(/结果未知/)).toBeInTheDocument()
  })

  test('电脑端发起的 Turn 只显示来源标记，不显示用户正文', () => {
    const messages: Message[] = [claude({ source: 'terminal' })]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText('电脑端发起')).toBeInTheDocument()
  })

  test('手机端发起的 Turn 显示来源标记', () => {
    const messages: Message[] = [claude({ source: 'mobile' })]
    render(<MessageFeed messages={messages} />)
    expect(screen.getByText('手机端发起')).toBeInTheDocument()
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
