import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import SelectableMessage from './SelectableMessage'

const clipboard = vi.hoisted(() => ({ copyText: vi.fn() }))
const haptics = vi.hoisted(() => ({ selectionFeedback: vi.fn().mockResolvedValue(undefined) }))
vi.mock('../native/clipboard', () => clipboard)
vi.mock('../native/haptics', () => haptics)

const box = (left = 0, top = 100, width = 360, height = 200) => ({ left, top, right: left + width, bottom: top + height, x: left, y: top, width, height, toJSON() {} })
const frames = new Map<number, FrameRequestCallback>()
let frameId = 0
const finger = (x = 100, y = 150) => ({ identifier: 1, clientX: x, clientY: y })
function flushFrame() {
  const pending = [...frames.values()]
  frames.clear()
  act(() => { pending.forEach((callback) => callback(16)) })
}
function hold(node: Element) {
  fireEvent.touchStart(node, { touches: [finger()] })
  act(() => vi.advanceTimersByTime(500))
}
function pointer(node: Element | Document, type: string, x: number, y: number, id = 1) {
  const event = new Event(type, { bubbles: true, cancelable: true })
  Object.defineProperties(event, { pointerId: { value: id }, button: { value: 0 }, clientX: { value: x }, clientY: { value: y } })
  fireEvent(node, event)
}

beforeEach(() => {
  vi.useFakeTimers()
  clipboard.copyText.mockResolvedValue(undefined)
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(() => box())
  // A simple real-Range geometry adapter. It makes pointer/keyboard selection
  // observable without using the browser's Selection as application state.
  const createRange = document.createRange.bind(document)
  vi.spyOn(document, 'createRange').mockImplementation(() => {
    const range = createRange()
    Object.defineProperty(range, 'getClientRects', { value: () => range.startContainer instanceof Text && range.startContainer === range.endContainer && range.endOffset > range.startOffset
      ? [box(20 + range.startOffset * 10, 120, (range.endOffset - range.startOffset) * 10, 20)] : [] })
    return range
  })
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => { frames.set(++frameId, callback); return frameId })
  vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => { frames.delete(id) })
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  frames.clear()
  window.getSelection()?.removeAllRanges()
  clipboard.copyText.mockReset()
  haptics.selectionFeedback.mockClear()
})

test('长按过程中只建立应用选区和一次轻震，松手与系统菜单事件不重新全选', async () => {
  render(<SelectableMessage>消息正文</SelectableMessage>)
  const text = screen.getByText('消息正文')
  const addRange = vi.spyOn(window.getSelection()!, 'addRange')
  hold(text)
  expect(screen.getByRole('button', { name: '复制' })).toBeVisible()
  expect(screen.getByRole('button', { name: '调整选区起点' })).toBeInTheDocument()
  expect(document.querySelector('.message-selection-highlight')).toBeInTheDocument()
  expect(addRange).not.toHaveBeenCalled()
  expect(window.getSelection()?.toString()).toBe('')
  expect(haptics.selectionFeedback).toHaveBeenCalledTimes(1)
  const end = new Event('touchend', { bubbles: true, cancelable: true })
  fireEvent(text, end)
  expect(end.defaultPrevented).toBe(true)
  const menu = new MouseEvent('contextmenu', { bubbles: true, cancelable: true })
  fireEvent(text, menu)
  expect(menu.defaultPrevented).toBe(true)
  expect(haptics.selectionFeedback).toHaveBeenCalledTimes(1)
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制' })) })
  expect(clipboard.copyText).toHaveBeenCalledExactlyOnceWith('消息正文')
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
})

test('普通滚动不阻止默认行为，不触发全选或轻震', () => {
  render(<SelectableMessage>消息正文</SelectableMessage>)
  const text = screen.getByText('消息正文')
  fireEvent.touchStart(text, { touches: [finger()] })
  const move = new Event('touchmove', { bubbles: true, cancelable: true })
  Object.defineProperty(move, 'touches', { value: [finger(100, 190)] })
  fireEvent(text, move)
  expect(move.defaultPrevented).toBe(false)
  act(() => vi.advanceTimersByTime(1000))
  fireEvent.contextMenu(text)
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
  expect(haptics.selectionFeedback).not.toHaveBeenCalled()
})

test.each(['cancel', 'multitouch'])('%s 取消待触发长按，后续触摸菜单不兜底全选', (trigger) => {
  render(<SelectableMessage>消息正文</SelectableMessage>)
  const text = screen.getByText('消息正文')
  fireEvent.touchStart(text, { touches: [finger()] })
  if (trigger === 'cancel') fireEvent.touchCancel(text)
  else fireEvent.touchStart(text, { touches: [finger(), { ...finger(), identifier: 2 }] })
  act(() => vi.advanceTimersByTime(1000))
  fireEvent.contextMenu(text)
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
  expect(haptics.selectionFeedback).not.toHaveBeenCalled()
})

test('触摸取消已完成的长按不清空选区，也不在延迟菜单事件时重复触发', async () => {
  render(<SelectableMessage>消息正文</SelectableMessage>)
  const text = screen.getByText('消息正文')
  hold(text)
  fireEvent.touchCancel(text)
  fireEvent.contextMenu(text)
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制' })) })
  expect(clipboard.copyText).toHaveBeenCalledExactlyOnceWith('消息正文')
  expect(haptics.selectionFeedback).toHaveBeenCalledTimes(1)
})

test('拖动起止手柄后复制局部文本，不依赖系统选区，也不再次震动', async () => {
  render(<SelectableMessage>消息正文内容</SelectableMessage>)
  const text = screen.getByText('消息正文内容')
  hold(text)
  fireEvent.touchEnd(text)
  const begin = screen.getByRole('button', { name: '调整选区起点' })
  pointer(begin, 'pointerdown', 20, 110)
  // Grab offset preserves the caret's line midpoint: moving 20px selects at index 2.
  pointer(document, 'pointermove', 40, 110)
  pointer(document, 'pointerup', 40, 110)
  const end = screen.getByRole('button', { name: '调整选区终点' })
  pointer(end, 'pointerdown', 80, 150)
  pointer(document, 'pointermove', 60, 150)
  pointer(document, 'pointerup', 60, 150)
  fireEvent(document, new Event('selectionchange'))
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制' })) })
  expect(clipboard.copyText).toHaveBeenCalledExactlyOnceWith('正文')
  expect(haptics.selectionFeedback).toHaveBeenCalledTimes(1)
  expect(window.getSelection()?.toString()).toBe('')
})

test('键盘逐字调整手柄，emoji 和组合字符不被截断', async () => {
  render(<SelectableMessage>甲👨‍👩‍👧‍👦é乙</SelectableMessage>)
  fireEvent.contextMenu(screen.getByText('甲👨‍👩‍👧‍👦é乙'), { button: 2, clientX: 100, clientY: 150 })
  const begin = screen.getByRole('button', { name: '调整选区起点' })
  fireEvent.keyDown(begin, { key: 'ArrowRight' })
  fireEvent.keyDown(screen.getByRole('button', { name: '调整选区终点' }), { key: 'ArrowLeft' })
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制' })) })
  expect(clipboard.copyText).toHaveBeenCalledWith('👨‍👩‍👧‍👦é')
  expect(haptics.selectionFeedback).not.toHaveBeenCalled()
})

test.each(['focus', 'blur', 'outside', 'escape', 'pagehide'])('%s 后关闭选区、高亮和菜单', (trigger) => {
  render(<><SelectableMessage>消息正文</SelectableMessage><input aria-label="输入" /></>)
  hold(screen.getByText('消息正文'))
  if (trigger === 'focus') fireEvent.focusIn(screen.getByRole('textbox'))
  if (trigger === 'blur') fireEvent(window, new Event('blur'))
  if (trigger === 'outside') fireEvent.pointerDown(document.body)
  if (trigger === 'escape') fireEvent.keyDown(document, { key: 'Escape' })
  if (trigger === 'pagehide') fireEvent(window, new Event('pagehide'))
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
  expect(document.querySelector('.message-selection-overlay')).not.toBeInTheDocument()
})

test('流式更新或 Markdown 结构变更清理旧选区', async () => {
  const { rerender } = render(<SelectableMessage>回复内容</SelectableMessage>)
  hold(screen.getByText('回复内容'))
  await act(async () => { rerender(<SelectableMessage><p>回复内容继续生成</p></SelectableMessage>) })
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
  expect(document.querySelector('.message-selection-overlay')).not.toBeInTheDocument()
  expect(clipboard.copyText).not.toHaveBeenCalled()
})

test('详情页进入退出动画时清理顶层选区，返回时不恢复旧菜单', async () => {
  const { rerender } = render(<div data-mobile-page-active="true"><SelectableMessage>消息正文</SelectableMessage></div>)
  hold(screen.getByText('消息正文'))
  await act(async () => { rerender(<div data-mobile-page-active="false"><SelectableMessage>消息正文</SelectableMessage></div>) })
  expect(document.querySelector('.message-selection-overlay')).not.toBeInTheDocument()
  await act(async () => { rerender(<div data-mobile-page-active="true"><SelectableMessage>消息正文</SelectableMessage></div>) })
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
})

test('手柄在滚动边缘持续滚动，取消拖动后停止且保留当前选区', () => {
  render(<div data-testid="detail-scroll" style={{ overflowY: 'auto' }}><SelectableMessage>消息正文</SelectableMessage></div>)
  const scroll = screen.getByTestId('detail-scroll')
  vi.spyOn(scroll, 'getBoundingClientRect').mockReturnValue(box(0, 100, 360, 200))
  Object.defineProperties(scroll, { clientHeight: { configurable: true, value: 200 }, clientWidth: { configurable: true, value: 360 }, scrollHeight: { configurable: true, value: 1000 } })
  const text = screen.getByText('消息正文')
  hold(text)
  fireEvent.touchEnd(text)
  pointer(screen.getByRole('button', { name: '调整选区终点' }), 'pointerdown', 60, 150)
  pointer(document, 'pointermove', 60, 400)
  flushFrame()
  expect(scroll.scrollTop).toBeGreaterThan(0)
  pointer(document, 'pointercancel', 60, 400)
  const stoppedAt = scroll.scrollTop
  flushFrame()
  expect(scroll.scrollTop).toBe(stoppedAt)
  expect(screen.getByRole('button', { name: '复制' })).toBeVisible()
  expect(haptics.selectionFeedback).toHaveBeenCalledTimes(1)
})

test('复制失败保留选区可重试，连续点击仅写入一次', async () => {
  clipboard.copyText.mockRejectedValueOnce(new Error('unavailable'))
  render(<SelectableMessage>消息正文</SelectableMessage>)
  hold(screen.getByText('消息正文'))
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制' })) })
  expect(screen.getByRole('status')).toHaveTextContent('复制失败，请重试')
  expect(screen.getByRole('button', { name: '复制' })).toBeVisible()
  await act(async () => {
    const button = screen.getByRole('button', { name: '复制' })
    fireEvent.click(button)
    fireEvent.click(button)
  })
  expect(clipboard.copyText).toHaveBeenCalledTimes(2)
  expect(screen.getByRole('status')).toHaveTextContent('已复制')
})

test('同一时间仅一条消息有选区，旧复制结果不关闭新选区', async () => {
  let finish: (() => void) | undefined
  clipboard.copyText.mockReturnValueOnce(new Promise<void>((resolve) => { finish = resolve }))
  render(<><SelectableMessage>第一条</SelectableMessage><SelectableMessage>第二条</SelectableMessage></>)
  fireEvent.contextMenu(screen.getByText('第一条'), { button: 2, clientX: 100, clientY: 150 })
  fireEvent.click(screen.getByRole('button', { name: '复制' }))
  fireEvent.contextMenu(screen.getByText('第二条'), { button: 2, clientX: 100, clientY: 150 })
  await act(async () => { finish!() })
  expect(screen.getAllByRole('button', { name: '复制' })).toHaveLength(1)
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制' })) })
  expect(clipboard.copyText).toHaveBeenLastCalledWith('第二条')
})

test('长消息菜单跟随触摸点及可视区域，选中后可继续滚动且不重复震动', () => {
  render(<div data-testid="detail-scroll" style={{ overflowY: 'auto' }}><SelectableMessage>长消息内容</SelectableMessage></div>)
  const scroll = screen.getByTestId('detail-scroll')
  const text = screen.getByText('长消息内容')
  Object.defineProperty(scroll, 'getBoundingClientRect', { configurable: true, value: () => box(0, 100, 400, 500) })
  Object.defineProperties(scroll, { clientHeight: { configurable: true, value: 500 }, clientWidth: { configurable: true, value: 400 } })
  let top = -500
  Object.defineProperty(text, 'getBoundingClientRect', { configurable: true, value: () => box(0, top, 400, 1200) })
  fireEvent.touchStart(text, { touches: [finger(200, 540)] })
  act(() => vi.advanceTimersByTime(500))
  fireEvent.touchEnd(text)
  const menu = screen.getByRole('button', { name: '复制' }).parentElement!
  expect(menu.style.top).toBe('488px')
  fireEvent.touchStart(text, { touches: [finger(200, 540)] })
  fireEvent.touchMove(text, { touches: [finger(200, 400)] })
  fireEvent.touchEnd(text)
  top = -900
  fireEvent.scroll(scroll)
  flushFrame()
  expect(menu.style.top).toBe('104px')
  expect(haptics.selectionFeedback).toHaveBeenCalledTimes(1)
  expect(screen.getByRole('button', { name: '复制' })).toBeVisible()
})

test('空消息及链接操作不启动选区或轻震', () => {
  const { container } = render(<><SelectableMessage>{''}</SelectableMessage><SelectableMessage><a href="https://example.com">链接</a></SelectableMessage></>)
  const empty = container.querySelector('.selectable-message-content')!
  hold(empty)
  hold(screen.getByText('链接'))
  expect(screen.queryByRole('button', { name: '复制' })).not.toBeInTheDocument()
  expect(haptics.selectionFeedback).not.toHaveBeenCalled()
})
