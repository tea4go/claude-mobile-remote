import { useEffect, useState } from 'react'
import { ActionSheet } from 'antd-mobile'
import type { QueuedRequest } from '../protocol/types'
import './QueuedRequestList.css'

interface QueuedRequestListProps {
  requests: QueuedRequest[]
  enabled: boolean
  onCancel: (requestId: string) => void
  onSteer?: (requestId: string) => void
}

export default function QueuedRequestList({ requests, enabled, onCancel, onSteer }: QueuedRequestListProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const selected = requests.find((request) => request.requestId === selectedId)
  useEffect(() => {
    if (!enabled || (selectedId !== null && !selected)) setSelectedId(null)
  }, [enabled, selectedId, selected])

  if (requests.length === 0) return null

  return <div data-testid="queued-request-list" className="queued-request-list">
    <div className="queued-request-list__hint">按顺序等待处理，点击查看操作</div>
    {requests.map((request, index) => <button
      key={request.requestId}
      type="button"
      data-testid="queued-request-item"
      className="queued-request-list__item"
      onClick={() => setSelectedId(request.requestId)}
      disabled={!enabled}
    >
      <span className="queued-request-list__meta">第 {index + 1} 项 · {request.source === 'terminal' ? '电脑端' : '手机端'}</span>
      <span className="queued-request-list__preview">{request.preview || '（无文本预览）'}</span>
    </button>)}
    <ActionSheet
      visible={!!selected && enabled}
      extra="队列操作"
      cancelText="关闭"
      closeOnAction
      closeOnMaskClick
      onClose={() => setSelectedId(null)}
      actions={[
        ...(onSteer ? [{ key: 'steer', text: '引导', onClick: () => { if (selected) onSteer(selected.requestId) } }] : []),
        { key: 'cancel', text: '取消排队', danger: true, onClick: () => { if (selected) onCancel(selected.requestId) } },
      ]}
    />
  </div>
}
