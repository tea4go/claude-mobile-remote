import { useEffect, useState } from 'react'
import { DotLoading } from 'antd-mobile'
import type { TurnProgress } from '../protocol/types'
const labels: Record<TurnProgress['phase'], string> = {
 waiting: '正在等待 Claude 响应', working: '正在处理请求', thinking: '正在思考', writing: '正在生成回复', tool: '正在执行工具', subtask: '正在执行子任务', compacting: '正在压缩上下文', requesting: '正在请求模型', retrying: 'API 异常，正在重试', waiting_followup: '等待后续响应', tool_result: '正在处理工具结果', subtask_result: '正在处理子任务结果',
}
export default function TurnLoading({ progress, waiting }: { progress?: TurnProgress | null; waiting: boolean }) {
 const [mountedAt] = useState(Date.now)
 const [now, setNow] = useState(Date.now)
 useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer) }, [])
 const total = Math.max(0, Math.floor((now - (progress?.startedAt ?? mountedAt)) / 1000))
 const elapsed = total < 60 ? `${total}s` : [total >= 3600 ? `${Math.floor(total / 3600)}h` : '', Math.floor(total / 60) % 60 ? `${Math.floor(total / 60) % 60}min` : '', total % 60 ? `${total % 60}s` : ''].filter(Boolean).join(' ')
 return <div role="status" className="composer-running" title={progress?.tokensEstimated ? '输出 token 为估算值' : undefined}>
  <DotLoading color="primary" /><span>{waiting ? '等待审批或回答' : progress ? (progress.phase === 'tool' && progress.toolName ? `正在执行 ${progress.toolName}` : labels[progress.phase]) : '正在等待 Claude 响应'}（{elapsed}{progress && (progress.tokensKnown || progress.outputTokens > 0) ? ` · ↓ ${progress.tokensEstimated ? '约 ' : ''}${progress.outputTokens} tokens` : ''}）</span>
 </div>
}
