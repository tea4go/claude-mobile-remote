import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import PageTransition from './PageTransition'

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

test('前进与返回保留短暂退场页，连续切换不叠加旧页', () => {
  vi.stubGlobal('matchMedia', () => ({ matches: false }))
  vi.useFakeTimers()
  const { container, rerender } = render(<PageTransition pageKey="list" direction="back">列表</PageTransition>)

  rerender(<PageTransition pageKey="detail:a" direction="forward">详情 A</PageTransition>)
  expect(container.querySelector('.mobile-page--enter-forward')).toHaveTextContent('详情 A')
  expect(container.querySelector('.mobile-page--exit-forward')).toHaveAttribute('inert')
  expect(container.querySelectorAll('.mobile-page')).toHaveLength(2)

  rerender(<PageTransition pageKey="list" direction="back">列表</PageTransition>)
  expect(container.querySelector('.mobile-page--exit-back')).toHaveTextContent('详情 A')
  expect(container.querySelectorAll('.mobile-page')).toHaveLength(2)
  act(() => vi.advanceTimersByTime(360))
  expect(container.querySelectorAll('.mobile-page')).toHaveLength(1)
  expect(screen.getByText('列表')).toBeInTheDocument()
})

test('系统减少动态效果时直接切换页面', () => {
  vi.stubGlobal('matchMedia', () => ({ matches: true }))
  const { container, rerender } = render(<PageTransition pageKey="list" direction="back">列表</PageTransition>)
  rerender(<PageTransition pageKey="pairing" direction="forward">配对</PageTransition>)
  expect(container.querySelectorAll('.mobile-page')).toHaveLength(1)
  expect(screen.getByText('配对')).toBeInTheDocument()
})

test('详情仅接受从左边缘开始的明确右滑返回', () => {
  const onEdgeBack = vi.fn()
  const { container } = render(<PageTransition pageKey="detail:a" direction="forward" onEdgeBack={onEdgeBack}>详情</PageTransition>)
  const stack = container.querySelector('.mobile-page-stack')!
  const swipe = (x: number, y: number, endX: number, endY: number) => {
    fireEvent.touchStart(stack, { touches: [{ clientX: x, clientY: y }] })
    fireEvent.touchMove(stack, { touches: [{ clientX: endX, clientY: endY }] })
    fireEvent.touchEnd(stack, { changedTouches: [{ clientX: endX, clientY: endY }] })
  }
  swipe(15, 100, 110, 105)
  expect(onEdgeBack).toHaveBeenCalledOnce()
  swipe(60, 100, 200, 100)
  swipe(15, 100, 110, 180)
  swipe(15, 100, 60, 100)
  expect(onEdgeBack).toHaveBeenCalledOnce()
})
