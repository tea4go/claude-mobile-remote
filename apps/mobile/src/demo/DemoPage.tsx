import { useEffect, useState } from 'react'
import SessionDetail from '../components/SessionDetail'
import type { SessionRecord } from '../session/store'
import { DemoController, initialDemoState } from './controller'

const record: SessionRecord = { sessionId: 'offline-demo', name: '离线演示', wsUrl: '', pairToken: '', expiresAt: '', lastConnectedAt: null, status: 'new' }
export default function DemoPage({ onExit }: { onExit: () => void }) {
  const [state, setState] = useState(initialDemoState)
  const [controller] = useState(() => new DemoController(setState))
  useEffect(() => () => controller.dispose(), [controller])
  const running = state.messages.find(m => m.kind === 'claude' && m.status === 'streaming')
  const target = running?.kind === 'claude' ? running.requestId : null
  return <div className="offline-demo-page" style={{ height: '100dvh', display: 'flex', flexDirection: 'column' }}>
    <div className="offline-demo-banner">
      <strong>离线演示 · 模拟数据，不连接电脑或 Claude</strong>
      <div>发送任意文字可体验预设流式回复。附件、语音及真实操作请退出后扫码连接电脑。</div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, paddingTop: 6 }}>
        <button onClick={() => controller.reset()}>重新开始</button>
        <button disabled={!state.canSend} onClick={() => controller.send('演示审批')}>演示审批</button>
        <button disabled={!state.canSend} onClick={() => controller.send('演示问答')}>演示问答</button>
        <button onClick={onExit}>退出演示</button>
      </div>
    </div>
    <div style={{ flex: 1, minHeight: 0 }}>
      <SessionDetail textOnly topSafeArea={false} record={record} state={state} experience={null} pairingInvalid={false}
        turnStatus={state.pendingAction ? 'waiting' : target ? 'running' : 'idle'} cancelTarget={target}
        onBack={onExit} onReconnect={() => undefined} onSend={text => controller.send(text)}
        onResolveAction={r => controller.resolve(r)} onCancelTurn={id => controller.cancel(id)} onCancelQueued={() => false}
        onPermissionModeChange={() => undefined} />
    </div>
  </div>
}
