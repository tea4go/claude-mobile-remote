import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import ActivityGroup, { type Activity } from './ActivityGroup'
import type { ContentBlock } from '../protocol/types'

type ToolUse = Extract<ContentBlock, { kind: 'tool_use' }>
type ToolResult = Extract<ContentBlock, { kind: 'tool_result' }>

function thinking(text: string, id = 'act-thinking'): Activity {
  return { kind: 'thinking', id, text }
}

function builtinTool(toolName: string, preview?: string): ToolUse {
  return {
    id: 'blk-tool',
    kind: 'tool_use',
    toolUseId: 'tu-1',
    toolClass: 'builtin',
    toolName,
    mcpServer: '',
    ...(preview === undefined ? {} : { preview }),
  }
}

function mcpTool(server: string, toolName: string): ToolUse {
  return {
    id: 'blk-mcp',
    kind: 'tool_use',
    toolUseId: 'tu-2',
    toolClass: 'mcp',
    toolName,
    mcpServer: server,
  }
}

function toolResult(isError: boolean): ToolResult {
  return { id: 'blk-result', kind: 'tool_result', toolUseId: 'tu-1', isError }
}

function tool(use: ToolUse, result?: ToolResult): Activity {
  return { kind: 'tool', id: use.id, tool: use, ...(result === undefined ? {} : { result }) }
}

function openList() {
  fireEvent.click(screen.getByTestId('activity-group-summary'))
}

// Popup 关闭是带退场动画的（destroyOnClose 在动画结束后才卸载），
// 所以关闭断言看状态机当前层级，不依赖动画时序。
function drawerLevel(): string | null {
  return screen.getByTestId('activity-group').getAttribute('data-drawer')
}

describe('ActivityGroup：摘要行（B2 会话详情）', () => {
  test('默认只显示一行摘要，不渲染任何活动内容', () => {
    render(<ActivityGroup activities={[thinking('先读配置'), tool(builtinTool('Bash', 'ls'))]} />)
    expect(screen.getByText('查看 2 个步骤，运行 1 个工具')).toBeInTheDocument()
    expect(screen.queryByTestId('activity-list')).not.toBeInTheDocument()
    expect(screen.queryByText('先读配置')).not.toBeInTheDocument()
    expect(screen.queryByText(/工具：Bash/)).not.toBeInTheDocument()
  })

  test('全部是思考步骤时不出现工具计数', () => {
    render(<ActivityGroup activities={[thinking('第一步'), thinking('第二步', 'act-2')]} />)
    expect(screen.getByText('查看 2 个步骤')).toBeInTheDocument()
  })

  test('点击摘要行从抽屉展示活动列表', () => {
    render(<ActivityGroup activities={[thinking('先读配置'), tool(builtinTool('Bash', 'ls cmd'))]} />)
    openList()
    expect(screen.getByTestId('activity-list')).toBeInTheDocument()
    expect(screen.getAllByTestId('activity-row')).toHaveLength(2)
    expect(screen.getByText('思考过程')).toBeInTheDocument()
    expect(screen.getByText('工具：Bash')).toBeInTheDocument()
  })

  test('抽屉可关闭，关闭后回到未展开层级', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash'))]} />)
    openList()
    expect(drawerLevel()).toBe('list')
    fireEvent.click(screen.getByLabelText('关闭'))
    expect(drawerLevel()).toBe('closed')
  })
})

describe('ActivityGroup：列表行状态（B2 §6）', () => {
  test('工具未返回结果时显示执行中', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash'))]} />)
    openList()
    expect(screen.getByText('执行中')).toBeInTheDocument()
  })

  test('结果到达后显示成功', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash'), toolResult(false))]} />)
    openList()
    expect(screen.getByText('成功')).toBeInTheDocument()
    expect(screen.queryByText('失败')).not.toBeInTheDocument()
  })

  test('错误结果显示失败', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash'), toolResult(true))]} />)
    openList()
    expect(screen.getByText('失败')).toBeInTheDocument()
  })

  test('MCP 工具显示 Server 与工具名', () => {
    render(<ActivityGroup activities={[tool(mcpTool('files', 'list_dir'))]} />)
    openList()
    expect(screen.getByText('MCP：files / list_dir')).toBeInTheDocument()
  })

  test('Task 只作为普通工具行，不出现任务卡片或耗时', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Task'))]} />)
    openList()
    const list = screen.getByTestId('activity-list')
    expect(screen.getByText('工具：Task')).toBeInTheDocument()
    expect(list.textContent).not.toContain('耗时')
    expect(list.textContent).not.toContain('子任务')
    expect(list.querySelector('progress')).toBeNull()
  })

  test('孤立工具结果不编造工具名', () => {
    render(
      <ActivityGroup
        activities={[{ kind: 'result', id: 'orphan', toolName: '', result: toolResult(true) }]}
      />,
    )
    openList()
    expect(screen.getByText('工具结果')).toBeInTheDocument()
    expect(screen.getByText('失败')).toBeInTheDocument()
  })

  test('列表行只显示 preview 首行摘要，长文本截断', () => {
    const preview = `${'a'.repeat(60)}\n第二行不应出现`
    render(<ActivityGroup activities={[tool(builtinTool('Bash', preview))]} />)
    openList()
    const list = screen.getByTestId('activity-list')
    expect(list.textContent).toContain('…')
    expect(list.textContent).not.toContain('第二行不应出现')
  })
})

describe('ActivityGroup：详情抽屉（B2 §6）', () => {
  test('点击列表行进入详情，状态并入标题行，正文只放输入', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash', 'ls cmd && echo README'))]} />)
    openList()
    fireEvent.click(screen.getByTestId('activity-row'))

    expect(screen.getByTestId('activity-detail')).toBeInTheDocument()
    expect(screen.getByText('工具：Bash')).toBeInTheDocument()
    expect(screen.getByText('输入')).toBeInTheDocument()
    expect(screen.getByText('ls cmd && echo README')).toBeInTheDocument()
    // 状态标签跟在抽屉标题后面，详情正文不再单独占一块「状态」区域。
    expect(screen.getByText('执行中')).toBeInTheDocument()
    expect(screen.getByTestId('activity-detail').textContent).not.toContain('状态')
    // 进入详情后列表抽屉退出，两个抽屉不会同时存在。
    expect(screen.queryByTestId('activity-list')).not.toBeInTheDocument()
  })

  test('详情不使用 pre/code 承载正文，也不出现输出区', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash', 'ls cmd'))]} />)
    openList()
    fireEvent.click(screen.getByTestId('activity-row'))

    const detail = screen.getByTestId('activity-detail')
    expect(detail.querySelector('pre')).toBeNull()
    expect(detail.querySelector('code')).toBeNull()
    expect(detail.textContent).not.toContain('输出')
    expect(detail.textContent).not.toContain('参数')
  })

  test('工具没有 preview 时如实说明未提供输入预览', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Read'))]} />)
    openList()
    fireEvent.click(screen.getByTestId('activity-row'))
    expect(screen.getByText('本工具未提供输入预览')).toBeInTheDocument()
  })

  test('思考详情展示完整正文，且不解析 Markdown 或 HTML', () => {
    render(<ActivityGroup activities={[thinking('**不要加粗** <script>x()</script>')]} />)
    openList()
    fireEvent.click(screen.getByTestId('activity-row'))

    const detail = screen.getByTestId('activity-detail')
    expect(detail.textContent).toContain('**不要加粗** <script>x()</script>')
    expect(detail.querySelector('strong')).toBeNull()
    expect(detail.querySelector('script')).toBeNull()
    // 思考没有执行状态，标题行不出现状态标签。
    expect(detail.textContent).not.toContain('状态')
    expect(screen.queryByText('执行中')).toBeNull()
  })

  test('详情可返回列表抽屉', () => {
    render(<ActivityGroup activities={[thinking('先读配置'), tool(builtinTool('Bash'))]} />)
    openList()
    fireEvent.click(screen.getAllByTestId('activity-row')[1]!)
    fireEvent.click(screen.getByLabelText('返回'))

    expect(screen.getByTestId('activity-list')).toBeInTheDocument()
    expect(screen.queryByTestId('activity-detail')).not.toBeInTheDocument()
  })

  test('详情可直接关闭整个抽屉', () => {
    render(<ActivityGroup activities={[tool(builtinTool('Bash'))]} />)
    openList()
    fireEvent.click(screen.getByTestId('activity-row'))
    expect(drawerLevel()).toBe('detail')
    fireEvent.click(screen.getByLabelText('关闭'))

    expect(drawerLevel()).toBe('closed')
  })
})
