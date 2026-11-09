import type { Attachment, AttachmentSendResult } from '../protocol/types'
import { useEffect, useRef, useState } from 'react'
import type { ExperiencePort } from '../native/experience'
import { Button, NavBar, Popup, Toast } from 'antd-mobile'
import { LeftOutline } from 'antd-mobile-icons'
import MessageComposer from './MessageComposer'
import MessageFeed from './MessageFeed'
import PendingActionCard from './PendingActionCard'
import QueuedRequestList from './QueuedRequestList'
import SessionStatus from './SessionStatus'
import {
  FONT_SIZE_COMPACT,
  GLASS_SURFACE_STYLE,
  SESSION_BACKGROUND,
} from './ui'
import type { ActionResolution } from '../protocol/client'
import type { PermissionMode } from '../protocol/types'
import type { MobileState } from '../session/reducer'
import type { SessionRecord } from '../session/store'

interface SessionDetailProps {
  topSafeArea?: boolean
  textOnly?: boolean
  experience?: ExperiencePort | null
  record: SessionRecord
  state: MobileState
  turnStatus: string
  pairingInvalid: boolean
  // 当前可取消 Turn 的 request ID；无可取消 Turn 时为 null。
  cancelTarget: string | null
  onBack: () => void
  onReconnect: () => void
  onSendAttachments?: (text: string, attachments: Attachment[]) => Promise<AttachmentSendResult>
  onSend: (text: string) => void
  onPermissionModeChange?: (mode: PermissionMode) => void
  onResolveAction: (resolution: ActionResolution) => boolean | void
  onCancelTurn: (requestId: string) => boolean | void
  onCancelQueued: (requestId: string) => boolean | void
  onSteerQueued?: (requestId: string) => boolean | void
  managementBusy?: boolean
  managementFeedback?: string | null
}

// 会话详情页（B1）：单个 Session 的状态栏、消息流与输入区。
// 连接生命周期由 App 持有，这里只呈现状态并回传操作。
export default function SessionDetail({
  topSafeArea = true,
  textOnly = false,
  experience,
  record,
  state,
  turnStatus,
  pairingInvalid,
  cancelTarget,
  onBack,
  onReconnect,
  onSend,
  onSendAttachments,
  onPermissionModeChange = () => undefined,
  onResolveAction,
  onCancelTurn,
  onCancelQueued,
  onSteerQueued,
  managementBusy = false,
  managementFeedback = null,
}: SessionDetailProps) {
  const showReconnect =
    (state.phase === 'disconnected' || state.phase === 'protocolError') && !pairingInvalid
  // 所有命令共用单在途门控（B2 §5）：取消类命令不依赖 canSend，
  // 因为等待待处理事项时自由文本被禁用但仍应可取消。
  const sessionUsable = state.sessionStatus !== 'stopped' && state.sessionStatus !== 'failed'
  const commandReady =
    !pairingInvalid && state.phase === 'connected' && state.pendingCommand === null && !managementBusy
  const cancelRequested = state.pendingCommand?.kind === 'cancel_turn'
  const contentScrollRef = useRef<HTMLDivElement>(null)
  const [queueOpen, setQueueOpen] = useState(false)
  const showQueue = state.canSteerQueue && state.queuedRequests.length > 0
  const queueOperation = useRef<{ kind: 'cancel' | 'steer'; previousError: MobileState['lastError'] } | null>(null)
  useEffect(() => {
    if (!showQueue || state.pendingAction !== null) setQueueOpen(false)
  }, [showQueue, state.pendingAction])
  useEffect(() => {
    setQueueOpen(false)
    queueOperation.current = null
  }, [record.sessionId])
  useEffect(() => {
    const operation = queueOperation.current
    if (!operation) return
    if (state.lastError !== null && state.lastError !== operation.previousError) {
      Toast.show({ content: operation.kind === 'steer' && state.lastError.code === 'turn_not_cancellable' ? '当前回复无法引导，请等待执行或刷新队列' : state.lastError.message })
      queueOperation.current = null
    } else if (state.phase !== 'connected') {
      Toast.show({ content: '连接已断开，队列操作结果待重连后确认' })
      queueOperation.current = null
    } else if (state.pendingCommand === null) {
      queueOperation.current = null
    }
  }, [state])
  const operateQueue = (requestId: string, operation: typeof onSteerQueued, kind: 'cancel' | 'steer') => {
    if (!sessionUsable || !commandReady || !operation) {
      Toast.show({ content: '当前无法操作队列，请检查连接后重试' })
      return
    }
    queueOperation.current = { kind, previousError: state.lastError }
    if (operation(requestId) === false) {
      queueOperation.current = null
      Toast.show({ content: '队列已变化或操作未受理，请检查后重试' })
    }
  }
  const disabledReason = pairingInvalid
    ? '配对已失效，请返回列表重新配对'
    : state.sessionStatus === 'stopped'
      ? '会话已结束，无法输入'
      : state.sessionStatus === 'failed'
        ? '会话已失败，无法输入'
        : state.phase !== 'connected'
          ? '连接不可用，暂时无法输入'
          : state.pendingAction !== null
            ? '请先处理待确认事项'
            : state.pendingCommand !== null
              ? '正在处理上一项操作，请稍后'
              : '当前会话暂不可输入'
  const showDetailNotices = state.sessionStatus === 'stopped' || state.sessionStatus === 'failed' ||
    state.sessionError !== null || managementFeedback !== null || state.lastNotice !== null ||
    pairingInvalid || state.phase === 'protocolError' || state.lastError !== null

  return (
    <div
      style={{
        height: '100%',
        background: SESSION_BACKGROUND,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      {/* 导航、状态与输入栏固定，中间的详情和消息共用滚动区。 */}
      <div className={`${topSafeArea ? 'mobile-safe-area-header' : ''} session-detail-header`} style={GLASS_SURFACE_STYLE}>
        <NavBar back="" backIcon={<LeftOutline aria-label="返回" fontSize={24} />} onBack={onBack} right={<div className="session-detail-header-actions">
          {showReconnect && <Button size="mini" color="primary" onClick={onReconnect}>重新连接</Button>}
        </div>}>
          <div className="session-detail-heading">
            <span className="session-detail-title">{record.name}</span>
            <SessionStatus
              connection={pairingInvalid ? 'disconnected' : state.connection}
              sessionStatus={state.sessionStatus}
              sessionError={state.sessionError}
              turnStatus={turnStatus}
              queueDepthHint={state.queueDepthHint}
              variant="compact"
            />
          </div>
        </NavBar>
      </div>
      {state.cachedAt != null && <div role="status" style={{ padding: '8px 16px', fontSize: 12, color: '#666' }}>本地缓存 · {new Date(state.cachedAt).toLocaleString()} · 运行状态待连接 PC 确认</div>}
      <div
        ref={contentScrollRef}
        data-testid="detail-scroll"
        style={{ flex: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden', overscrollBehavior: 'contain', position: 'relative', isolation: 'isolate' }}
      >
      {showDetailNotices && <div style={{ flexShrink: 0, padding: '8px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <SessionStatus
          connection={pairingInvalid ? 'disconnected' : state.connection}
          sessionStatus={state.sessionStatus}
          sessionError={state.sessionError}
          turnStatus={turnStatus}
          queueDepthHint={state.queueDepthHint}
          variant="details"
        />
        {managementFeedback !== null && <div role="status">{managementFeedback}</div>}
        {/* 不支持通知（B2 §4.4）：只提示并引导回电脑端，不断开连接。 */}
        {state.lastNotice !== null && (
          <div
            data-testid="session-notice"
            style={{ fontSize: FONT_SIZE_COMPACT, color: '#8a5a00' }}
          >
            {state.lastNotice.message}
          </div>
        )}
        {pairingInvalid && <div style={{ color: '#e00' }}>配对已失效，请返回列表重新配对</div>}
        {state.phase === 'protocolError' && !pairingInvalid && (
          <div style={{ color: '#e00' }}>状态异常，请重新连接以获取最新状态</div>
        )}
        {state.lastError !== null && (
          <div style={{ color: '#e00' }}>{state.lastError.message}</div>
        )}
      </div>}
      <div style={{ padding: '0 16px' }}>
        <MessageFeed messages={state.messages} scrollContainerRef={contentScrollRef} />
      </div>
      </div>
      <div
        style={{
          ...GLASS_SURFACE_STYLE,
          position: 'relative',
          zIndex: 1,
          background: '#f9fafc',
          padding: '8px 16px calc(8px + env(safe-area-inset-bottom, 0px))',
          borderTop: '1px solid rgba(15, 23, 42, 0.06)',
          display: 'flex',
          flexDirection: 'column',
          gap: 8,
          flexShrink: 0,
        }}
      >
        {/* 待处理事项以底部抽屉强呈现（D53）：事项出现即弹出，显式关闭即取消当前 Turn。 */}
        {state.pendingAction !== null && (
          <PendingActionCard
            action={state.pendingAction}
            enabled={state.canResolveAction}
            cancelEnabled={commandReady}
            onResolve={onResolveAction}
            onCancel={onCancelTurn}
          />
        )}
        <MessageComposer
          textOnly={textOnly}
          experience={experience}
          enabled={state.canSend}
          disabledReason={disabledReason}
          onSend={onSend}
          onSendAttachments={onSendAttachments}
          permissionMode={state.permissionMode}
          onPermissionModeChange={onPermissionModeChange}
          queueCount={showQueue ? state.queuedRequests.length : 0}
          queueOpen={queueOpen}
          onOpenQueue={() => setQueueOpen(true)}
          progress={state.progress}
          turnWaiting={state.pendingAction !== null}
          turnRunning={sessionUsable && state.connection === 'connected' && cancelTarget !== null}
          cancelTarget={cancelTarget}
          cancelEnabled={sessionUsable && commandReady && !cancelRequested}
          onCancelTurn={onCancelTurn}
          permissionPending={state.pendingAction?.kind === 'tool_approval'}
        />
      </div>
      <Popup visible={queueOpen && showQueue} onMaskClick={() => setQueueOpen(false)} onClose={() => setQueueOpen(false)} showCloseButton bodyStyle={{ borderTopLeftRadius: 20, borderTopRightRadius: 20, maxHeight: '70dvh', display: 'flex', flexDirection: 'column', padding: '20px 16px calc(16px + env(safe-area-inset-bottom, 0px))' }}>
        <div style={{ fontSize: 17, fontWeight: 600, paddingRight: 32, marginBottom: 16 }}>排队队列（{state.queuedRequests.length}）</div>
        {state.phase !== 'connected' && <div role="status" style={{ marginBottom: 12, color: '#888' }}>连接已断开，显示上次队列；重新连接后更新</div>}
        <div style={{ overflowY: 'auto', minHeight: 0 }}>
          <QueuedRequestList requests={state.queuedRequests} enabled={sessionUsable && commandReady} onCancel={(id) => operateQueue(id, onCancelQueued, 'cancel')} onSteer={state.canSteerQueue && cancelTarget !== null && state.pendingAction === null && onSteerQueued ? (id) => operateQueue(id, onSteerQueued, 'steer') : undefined} />
        </div>
      </Popup>
    </div>
  )
}
