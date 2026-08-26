import { Tag } from 'antd-mobile'

interface StructuredBlockPlaceholderProps {
  kind: 'thinking' | 'tool'
}

const TITLES: Record<StructuredBlockPlaceholderProps['kind'], string> = {
  thinking: 'Thinking 过程',
  tool: '工具调用',
}

// 只读占位（B1）：协议里还没有 Thinking/Tool 结构化事件，这里刻意不接受任何
// 内容 props，也不渲染任何可交互控件，避免把占位误读成真实结构化数据。
// 结构化事件与审批/取消在 B2 单独设计。
export default function StructuredBlockPlaceholder({ kind }: StructuredBlockPlaceholderProps) {
  return (
    <div
      data-testid={`structured-placeholder-${kind}`}
      style={{
        border: '1px dashed #d9d9d9',
        borderRadius: 6,
        padding: '6px 10px',
        display: 'flex',
        gap: 8,
        alignItems: 'center',
        flexWrap: 'wrap',
      }}
    >
      <span style={{ fontSize: 12, color: '#666' }}>{TITLES[kind]}</span>
      <Tag color="default">只读占位</Tag>
      <span style={{ fontSize: 12, color: '#999' }}>后续支持，当前不展示明细</span>
    </div>
  )
}
