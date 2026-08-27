import { Button, NavBar } from 'antd-mobile'
import MessageComposer from './MessageComposer'
import MessageFeed from './MessageFeed'
import PendingActionCard from './PendingActionCard'
import QueuedRequestList from './QueuedRequestList'
import SessionStatus from './SessionStatus'
import type { ActionResolution } from '../protocol/client'
import type { MobileState } from '../session/reducer'
import type { SessionRecord } from '../session/store'

interface SessionDetailProps {
  record: SessionRecord
  state: MobileState
  turnStatus: string
  pairingInvalid: boolean
  // 当前可取消 Turn 的 request ID；无可取消 Turn 时为 null。
  cancelTarget: string | null
  onBack: () => void
  onReconnect: () => void
  onSend: (text: string) => void
  onResolveAction: (resolution: ActionResolution) => void
  onCancelTurn: (requestId: string) => void
  onCancelQueued: (requestId: string) => void
}

// 会话详情页（B1）：单个 Session 的状态栏、消息流与输入区。
// 连接生命周期由 App 持有，这里只呈现状态并回传操作。
export default function SessionDetail({
  record,
  state,
  turnStatus,
  pairingInvalid,
  cancelTarget,
  onBack,
  onReconnect,
  onSend,
  onResolveAction,
  onCancelTurn,
  onCancelQueued,
}: SessionDetailProps) {
  const showReconnect =
    (state.phase === 'disconnected' || state.phase === 'protocolError') && !pairingInvalid
  // 所有命令共用单在途门控（B2 §5）：取消类命令不依赖 canSend，
  // 因为等待待处理事项时自由文本被禁用但仍应可取消。
  const commandReady =
    !pairingInvalid && state.phase === 'connected' && state.pendingCommand === null
  const cancelRequested = state.pendingCommand?.kind === 'cancel_turn'

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
        {/* 不支持通知（B2 §4.4）：只提示并引导回电脑端，不断开连接。 */}
        {state.lastNotice !== null && (
          <div
            data-testid="session-notice"
            style={{ fontSize: 12, color: '#8a5a00' }}
          >
            {state.lastNotice.message}
          </div>
        )}
        <QueuedRequestList
          requests={state.queuedRequests}
          enabled={commandReady}
          onCancel={onCancelQueued}
        />
        {cancelTarget !== null && (
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <Button
              size="mini"
              fill="outline"
              color="danger"
              disabled={!commandReady}
              onClick={() => onCancelTurn(cancelTarget)}
            >
              取消当前 Turn
            </Button>
            {/* 发出取消请求不等于已取消（B2 §7）。 */}
            {cancelRequested && <span style={{ fontSize: 12, color: '#888' }}>已请求取消</span>}
          </div>
        )}
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
      <div style={{ padding: '8px 16px', borderTop: '1px solid #e0e0e0', display: 'flex', flexDirection: 'column', gap: 8 }}>
        {/* 待处理事项卡片固定在输入区上方（B2 §6），不随消息流滚走。 */}
        {state.pendingAction !== null && (
          <PendingActionCard
            action={state.pendingAction}
            enabled={state.canResolveAction}
            onResolve={onResolveAction}
          />
        )}
        <MessageComposer enabled={state.canSend} onSend={onSend} />
      </div>
    </div>
  )
}
