import { Button, NavBar } from 'antd-mobile'
import MessageComposer from './MessageComposer'
import MessageFeed from './MessageFeed'
import SessionStatus from './SessionStatus'
import StructuredBlockPlaceholder from './StructuredBlockPlaceholder'
import type { MobileState } from '../session/reducer'
import type { SessionRecord } from '../session/store'

interface SessionDetailProps {
  record: SessionRecord
  state: MobileState
  turnStatus: string
  pairingInvalid: boolean
  onBack: () => void
  onReconnect: () => void
  onSend: (text: string) => void
}

// 会话详情页（B1）：单个 Session 的状态栏、消息流与输入区。
// 连接生命周期由 App 持有，这里只呈现状态并回传操作。
export default function SessionDetail({
  record,
  state,
  turnStatus,
  pairingInvalid,
  onBack,
  onReconnect,
  onSend,
}: SessionDetailProps) {
  const showReconnect =
    (state.phase === 'disconnected' || state.phase === 'protocolError') && !pairingInvalid

  return (
    <div
      style={{ minHeight: '100vh', background: '#f5f5f5', display: 'flex', flexDirection: 'column' }}
    >
      <NavBar back="返回" onBack={onBack}>
        {record.name}
      </NavBar>
      <div style={{ padding: '8px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <SessionStatus
          connection={pairingInvalid ? 'disconnected' : state.connection}
          sessionStatus={state.sessionStatus}
          sessionError={state.sessionError}
          turnStatus={turnStatus}
          queueDepthHint={state.queueDepthHint}
        />
        {/* Thinking / 工具调用结构化事件属于 B2，这里只声明只读占位。 */}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <StructuredBlockPlaceholder kind="thinking" />
          <StructuredBlockPlaceholder kind="tool" />
        </div>
        {pairingInvalid && <div style={{ color: '#e00' }}>配对已失效，请返回列表重新配对</div>}
        {state.phase === 'protocolError' && !pairingInvalid && (
          <div style={{ color: '#e00' }}>状态异常，请重新连接以获取最新状态</div>
        )}
        {state.lastError !== null && (
          <div style={{ color: '#e00' }}>{state.lastError.message}</div>
        )}
        {showReconnect && (
          <Button color="primary" size="small" onClick={onReconnect}>
            重新连接
          </Button>
        )}
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'hidden', padding: '0 16px' }}>
        <MessageFeed messages={state.messages} />
      </div>
      <div style={{ padding: '8px 16px', borderTop: '1px solid #e0e0e0' }}>
        <MessageComposer enabled={state.canSend} onSend={onSend} />
      </div>
    </div>
  )
}
