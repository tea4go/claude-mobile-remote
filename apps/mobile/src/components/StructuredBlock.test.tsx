import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import StructuredBlock from './StructuredBlock'
import type { ContentBlock } from '../protocol/types'

function thinking(text: string): ContentBlock {
  return { id: 'blk-1', kind: 'thinking', thinking: text }
}

function builtinTool(toolName: string): ContentBlock {
  return {
    id: 'blk-2',
    kind: 'tool_use',
    toolUseId: 'tu-1',
    toolClass: 'builtin',
    toolName,
    mcpServer: '',
  }
}

function builtinToolWithPreview(toolName: string, preview: string): ContentBlock {
  return {
    id: 'blk-preview',
    kind: 'tool_use',
    toolUseId: 'tu-preview',
    toolClass: 'builtin',
    toolName,
    mcpServer: '',
    preview,
  }
}

function mcpTool(server: string, toolName: string): ContentBlock {
  return {
    id: 'blk-3',
    kind: 'tool_use',
    toolUseId: 'tu-2',
    toolClass: 'mcp',
    toolName,
    mcpServer: server,
  }
}

function toolResult(isError: boolean): ContentBlock {
  return { id: 'blk-4', kind: 'tool_result', toolUseId: 'tu-1', isError }
}

describe('StructuredBlock：Thinking（B2 §6）', () => {
  test('默认折叠：只显示摘要行，不渲染思考正文', () => {
    render(<StructuredBlock block={thinking('先读配置再改代码')} />)
    expect(screen.getByText('思考过程')).toBeInTheDocument()
    expect(screen.queryByText('先读配置再改代码')).not.toBeInTheDocument()
  })

  test('展开后显示思考正文', () => {
    render(<StructuredBlock block={thinking('先读配置再改代码')} />)
    fireEvent.click(screen.getByText('思考过程'))
    expect(screen.getByText('先读配置再改代码')).toBeInTheDocument()
  })

  test('思考正文按纯文本渲染，不解析 Markdown 或 HTML', () => {
    const { container } = render(
      <StructuredBlock block={thinking('**不要加粗** <script>x()</script>')} />,
    )
    fireEvent.click(screen.getByText('思考过程'))
    expect(container.querySelector('strong')).toBeNull()
    expect(container.querySelector('script')).toBeNull()
    expect(container.textContent).toContain('**不要加粗** <script>x()</script>')
  })
})

describe('StructuredBlock：工具调用（B2 §6）', () => {
  test('内置工具只显示工具名与执行状态', () => {
    const { container } = render(<StructuredBlock block={builtinTool('Bash')} />)
    expect(screen.getByText(/Bash/)).toBeInTheDocument()
    expect(screen.getByText('执行中')).toBeInTheDocument()
    // 协议不下发 input：不得出现命令正文、参数或文件详情的渲染载体。
    expect(container.querySelector('pre')).toBeNull()
    expect(container.querySelector('code')).toBeNull()
    expect(container.textContent).not.toContain('参数')
    expect(container.textContent).not.toContain('MCP')
  })

  test('工具行不提供任何交互控件', () => {
    render(<StructuredBlock block={builtinTool('Read')} />)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  test('工具行显示服务端抽取的安全 preview', () => {
    render(<StructuredBlock block={builtinToolWithPreview('Bash', 'ls cmd && echo README')} />)
    expect(screen.getByText(/工具：Bash ｜ ls cmd && echo README/)).toBeInTheDocument()
  })

  test('MCP 工具显示 Server 与工具名', () => {
    render(<StructuredBlock block={mcpTool('files', 'list_dir')} />)
    expect(screen.getByText(/files/)).toBeInTheDocument()
    expect(screen.getByText(/list_dir/)).toBeInTheDocument()
    expect(screen.getByText(/MCP/)).toBeInTheDocument()
  })

  test('Task 只作为普通工具行，不出现任务卡片、状态、耗时或聚合数量', () => {
    const { container } = render(<StructuredBlock block={builtinTool('Task')} />)
    expect(screen.getByText(/Task/)).toBeInTheDocument()
    expect(container.textContent).not.toContain('任务')
    expect(container.textContent).not.toContain('耗时')
    expect(container.textContent).not.toContain('子任务')
    expect(container.querySelector('progress')).toBeNull()
  })
})

describe('StructuredBlock：工具结果（B2 §6）', () => {
  test('成功结果只显示成功状态', () => {
    const { container } = render(<StructuredBlock block={toolResult(false)} toolName="Bash" />)
    expect(screen.getByText(/Bash/)).toBeInTheDocument()
    expect(screen.getByText('成功')).toBeInTheDocument()
    expect(container.textContent).not.toContain('失败')
  })

  test('失败结果只显示失败状态，不展开完整结果', () => {
    const { container } = render(<StructuredBlock block={toolResult(true)} toolName="Bash" />)
    expect(screen.getByText('失败')).toBeInTheDocument()
    expect(screen.queryAllByRole('button')).toHaveLength(0)
    expect(container.querySelector('pre')).toBeNull()
  })

  test('AskUserQuestion 的结果不显示为错误', () => {
    const { container } = render(
      <StructuredBlock block={toolResult(false)} toolName="AskUserQuestion" />,
    )
    expect(screen.getByText('成功')).toBeInTheDocument()
    expect(container.textContent).not.toContain('失败')
  })

  test('无法关联工具名时只显示工具结果与状态，不编造名称', () => {
    render(<StructuredBlock block={toolResult(false)} />)
    expect(screen.getByText('工具结果')).toBeInTheDocument()
    expect(screen.getByText('成功')).toBeInTheDocument()
  })
})
