import { Collapse, Tag } from 'antd-mobile'
import type { ContentBlock } from '../protocol/types'

interface StructuredBlockProps {
  block: ContentBlock
  toolName?: string
  toolResult?: Extract<ContentBlock, { kind: 'tool_result' }>
}

export interface ToolActivity {
  tool: Extract<ContentBlock, { kind: 'tool_use' }>
  result?: Extract<ContentBlock, { kind: 'tool_result' }>
}

const ROW_STYLE = {
  display: 'flex',
  gap: 8,
  alignItems: 'center',
  flexWrap: 'wrap' as const,
  background: '#fff',
  border: '1px solid #f0f0f0',
  borderRadius: 6,
  padding: '6px 10px',
}

const LABEL_STYLE = { fontSize: 13, color: '#555', wordBreak: 'break-word' as const }

function toolTitle(block: Extract<ContentBlock, { kind: 'tool_use' }>): string {
  const prefix = block.toolClass === 'mcp' ? `MCP：${block.mcpServer} / ${block.toolName}` : `工具：${block.toolName}`
  return block.preview === undefined || block.preview === '' ? prefix : `${prefix} ｜ ${block.preview}`
}

// B2：Thinking 默认折叠；工具调用和结果在 MessageFeed 中先按 toolUseId 合并。
export default function StructuredBlock({ block, toolName = '', toolResult }: StructuredBlockProps) {
  if (block.kind === 'thinking') {
    return (
      <div data-testid="structured-block-thinking">
        <Collapse>
          <Collapse.Panel key="thinking" title="思考过程" destroyOnClose>
            <div style={{ fontSize: 13, color: '#666', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {block.thinking}
            </div>
          </Collapse.Panel>
        </Collapse>
      </div>
    )
  }

  if (block.kind === 'tool_use') {
    const completed = toolResult !== undefined
    return (
      <div data-testid="structured-block-tool-use" style={ROW_STYLE}>
        <span style={LABEL_STYLE}>{toolTitle(block)}</span>
        <Tag color={completed ? (toolResult.isError ? 'danger' : 'success') : 'default'}>
          {completed ? (toolResult.isError ? '失败' : '成功') : '执行中'}
        </Tag>
      </div>
    )
  }

  return (
    <div data-testid="structured-block-tool-result" style={ROW_STYLE}>
      <span style={LABEL_STYLE}>{toolName === '' ? '工具结果' : `工具结果：${toolName}`}</span>
      <Tag color={block.isError ? 'danger' : 'success'}>{block.isError ? '失败' : '成功'}</Tag>
    </div>
  )
}

export function ToolActivityGroup({ activities }: { activities: ToolActivity[] }) {
  return (
    <div data-testid="structured-block-tool-group">
      <Collapse>
        <Collapse.Panel key="tools" title={`工具调用（${activities.length}）`} destroyOnClose>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {activities.map((activity) => (
              <StructuredBlock key={activity.tool.id} block={activity.tool} toolResult={activity.result} />
            ))}
          </div>
        </Collapse.Panel>
      </Collapse>
    </div>
  )
}
