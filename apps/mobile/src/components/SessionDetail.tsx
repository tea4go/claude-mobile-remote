import type { Attachment, AttachmentSendResult } from '../protocol/types'
import { useRef } from 'react'
import type { ExperiencePort } from '../native/experience'
import { Button, NavBar } from 'antd-mobile'
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
  onCreateSession?: () => void
  managementBusy?: boolean
  managementFeedback?: string | null
}

// 会话详情页（B1）：单个 Session 的状态栏、消息流与输入区。
// 连接生命周期由 App 持有，这里只呈现状态并回传操作。
export default function SessionDetail({
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
  onCreateSession,
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
    state.queuedRequests.length > 0 || pairingInvalid || state.phase === 'protocolError' || state.lastError !== null

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
      <div className="mobile-safe-area-header session-detail-header" style={GLASS_SURFACE_STYLE}>
        <NavBar back="" backIcon={<LeftOutline aria-label="返回" />} onBack={onBack} right={<div className="session-detail-header-actions">
          <SessionStatus
            connection={pairingInvalid ? 'disconnected' : state.connection}
            sessionStatus={state.sessionStatus}
            sessionError={state.sessionError}
            turnStatus={turnStatus}
            queueDepthHint={state.queueDepthHint}
            variant="compact"
          />
          {showReconnect && <Button size="mini" color="primary" onClick={onReconnect}>重新连接</Button>}
        </div>}>
          <span className="session-detail-title">{record.name}</span>
        </NavBar>
      </div>
      {state.cachedAt != null && <div role="status" style={{ padding: '8px 16px', fontSize: 12, color: '#666' }}>本地缓存 · {new Date(state.cachedAt).toLocaleString()} · 运行状态待连接 PC 确认</div>}
      <div
        ref={contentScrollRef}
        data-testid="detail-scroll"
        style={{ flex: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden', overscrollBehavior: 'contain' }}
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
        <QueuedRequestList
          requests={state.queuedRequests}
          enabled={commandReady}
          onCancel={onCancelQueued}
        />
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
          experience={experience}
          enabled={state.canSend}
          disabledReason={disabledReason}
          onSend={onSend}
          onSendAttachments={onSendAttachments}
          permissionMode={state.permissionMode}
          onPermissionModeChange={onPermissionModeChange}
          turnRunning={sessionUsable && cancelTarget !== null}
          cancelTarget={cancelTarget}
          cancelEnabled={sessionUsable && commandReady && !cancelRequested}
          onCancelTurn={onCancelTurn}
          permissionPending={state.pendingAction?.kind === 'tool_approval'}
          onCreateSession={record.capabilities?.includes('create_session') ? onCreateSession : undefined}
          createSessionEnabled={commandReady}
        />
      </div>
    </div>
  )
}
