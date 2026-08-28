import { Button, Tag } from 'antd-mobile'
import { COMPACT_BUTTON_STYLE } from './ui'
import type { QueuedRequest } from '../protocol/types'

interface QueuedRequestListProps {
  requests: QueuedRequest[]
  // 连接就绪且无在途命令时才允许发取消命令。
  enabled: boolean
  onCancel: (requestId: string) => void
}

// 排队输入列表（B2 §6）：按 snapshot 的 queuedRequests 顺序展示服务端截断后的
// 预览，逐项取消只针对对应 request ID。不展示 request ID 本身。
export default function QueuedRequestList({
  requests,
  enabled,
  onCancel,
}: QueuedRequestListProps) {
  if (requests.length === 0) return null

  return (
    <div
      data-testid="queued-request-list"
      style={{ display: 'flex', flexDirection: 'column', gap: 6 }}
    >
      <div style={{ fontSize: 12, color: '#888' }}>等待处理的输入</div>
      {requests.map((request) => (
        <div
          key={request.requestId}
          data-testid="queued-request-item"
          style={{ display: 'flex', gap: 8, alignItems: 'center' }}
        >
          <Tag color="default" style={{ fontSize: 11, padding: '0 5px' }}>
            {request.source === 'terminal' ? '电脑端' : '手机端'}
          </Tag>
          <span style={{ flex: 1, fontSize: 13, wordBreak: 'break-word' }}>{request.preview}</span>
          <Button
            size="mini"
            fill="outline"
            style={COMPACT_BUTTON_STYLE}
            disabled={!enabled}
            onClick={() => onCancel(request.requestId)}
          >
            取消
          </Button>
        </div>
      ))}
    </div>
  )
}
