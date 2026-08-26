import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import StructuredBlockPlaceholder from './StructuredBlockPlaceholder'

describe('StructuredBlockPlaceholder：只读占位', () => {
  test('Thinking 占位说明后续支持', () => {
    render(<StructuredBlockPlaceholder kind="thinking" />)
    expect(screen.getByText(/Thinking/)).toBeInTheDocument()
    expect(screen.getByText(/后续支持/)).toBeInTheDocument()
  })

  test('Tool 占位说明后续支持', () => {
    render(<StructuredBlockPlaceholder kind="tool" />)
    expect(screen.getByText(/工具调用/)).toBeInTheDocument()
    expect(screen.getByText(/后续支持/)).toBeInTheDocument()
  })

  test('占位标明只读', () => {
    render(<StructuredBlockPlaceholder kind="tool" />)
    expect(screen.getByText(/只读/)).toBeInTheDocument()
  })
})

describe('StructuredBlockPlaceholder：不伪造内容与操作', () => {
  test('不渲染任何按钮或输入控件', () => {
    const { container } = render(
      <div>
        <StructuredBlockPlaceholder kind="thinking" />
        <StructuredBlockPlaceholder kind="tool" />
      </div>,
    )
    expect(container.querySelectorAll('button')).toHaveLength(0)
    expect(container.querySelectorAll('input')).toHaveLength(0)
    expect(container.querySelectorAll('textarea')).toHaveLength(0)
  })

  test('不出现审批、拒绝或取消字样', () => {
    const { container } = render(
      <div>
        <StructuredBlockPlaceholder kind="thinking" />
        <StructuredBlockPlaceholder kind="tool" />
      </div>,
    )
    const text = container.textContent ?? ''
    expect(text).not.toContain('批准')
    expect(text).not.toContain('允许')
    expect(text).not.toContain('拒绝')
    expect(text).not.toContain('取消')
  })

  test('不伪造工具名称、输入或结果字段', () => {
    const { container } = render(<StructuredBlockPlaceholder kind="tool" />)
    const text = container.textContent ?? ''
    expect(text).not.toContain('参数')
    expect(text).not.toContain('结果')
    expect(text).not.toContain('输出')
  })
})
