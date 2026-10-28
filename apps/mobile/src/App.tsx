import DemoPage from './demo/DemoPage'
import DebugPanel from './components/DebugPanel'
import { debugLog, recordAction } from './debug/log'
import { nativeSessionCache, decodeSessionCache, SessionCacheWriter, type SessionCachePort } from './native/sessionCache'
import { nativeExperience, type ExperiencePort } from './native/experience'
import NotificationSettings from './components/NotificationSettings'
import { ProtocolClient } from './protocol/client'
import { networkDiagnostics, networkCodeText, safeNetworkTarget } from './protocol/diagnostics'
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { List, NavBar, Switch } from 'antd-mobile'
import { LeftOutline } from 'antd-mobile-icons'
import { useStore } from 'zustand'
import PairingForm from './components/PairingForm'
import SessionDetail from './components/SessionDetail'
import ProjectSessionList, { type SessionListItem } from './components/ProjectSessionList'
import PCSettings from './components/PCSettings'
import PageTransition from './components/PageTransition'
import { PAGE_BACKGROUND } from './components/ui'
import type { ActionResolution, SocketFactory } from './protocol/client'
import {
  encodePairingFragment,
  parsePairingFragment,
  parsePairingInput,
  PAIRING_PAYLOAD_VERSION,
  type PairingPayload,
} from './protocol/pairing'
import type { PermissionMode } from './protocol/types'
import { createAppStore } from './session/appStore'
import { createInitialState, type MobileState } from './session/reducer'
import { SessionRuntimeManager } from './session/runtime'
import { observeNativeForeground } from './native/pairingLogic'
import type { NativePairingBindings } from './native/capacitorPorts'

interface AppProps {
  cache?: SessionCachePort | null
  experience?: ExperiencePort | null
  now?: () => number
  socketFactory?: SocketFactory
  native?: NativePairingBindings | null
}

const defaultSocketFactory: SocketFactory = (url) =>
  new WebSocket(url) as unknown as ReturnType<SocketFactory>
const defaultNow = () => Date.now()
const EMPTY_STATE = createInitialState()

function turnStatusOf(state: MobileState) {
  if (state.cachedAt != null) return 'unknown' as const
  if (state.pendingAction !== null) return 'waiting' as const
  const claude = [...state.messages].reverse().find((message) => message.kind === 'claude')
  if (claude === undefined || claude.kind !== 'claude') return 'idle' as const
  return claude.status === 'streaming' ? 'running' as const : claude.status === 'done' ? 'completed' as const : claude.status
}

function cancelTargetOf(state: MobileState): string | null {
  if (state.pendingAction !== null) return state.pendingAction.requestId
  const streaming = [...state.messages]
    .reverse()
    .find((message) => message.kind === 'claude' && message.status === 'streaming')
  return streaming?.kind === 'claude' ? streaming.requestId : null
}

function App({ cache = nativeSessionCache, now = defaultNow, socketFactory = defaultSocketFactory, native = null, experience = nativeExperience }: AppProps = {}) {
  const [demoOpen, setDemoOpen] = useState(false)
  const debugState = useSyncExternalStore(debugLog.subscribe, debugLog.getSnapshot)
  const [appStore] = useState(createAppStore)
  const [runtime] = useState(() => new SessionRuntimeManager(appStore, socketFactory, now))
  const [managementBusy, setManagementBusy] = useState<Record<string, boolean>>({})
  const [managementFeedback, setManagementFeedback] = useState<Record<string, string | null>>({})
  const [recentlyViewedId, setRecentlyViewedId] = useState<string | null>(null)
  const [settingsPage, setSettingsPage] = useState<'settings' | 'pcs' | null>(null)
  const [settingsDirection, setSettingsDirection] = useState<'forward' | 'back'>('forward')
  const [restoring, setRestoring] = useState(native !== null)
  const cacheWriter = useRef<SessionCacheWriter | null>(null)
  const [cacheMessage, setCacheMessage] = useState<string | null>(null)
  const listScrollY = useRef(0)
  const candidate = useRef<ProtocolClient | null>(null)
  const pairingGeneration = useRef(0)
  const [pairingProgress, setPairingProgress] = useState<string | null>(null)
  const [pairingTarget, setPairingTarget] = useState<string | null>(null)
  const [pairingSaving, setPairingSaving] = useState(false)
  const pairingSavingRef = useRef(false)
  const cancelPairing = useCallback(() => {
    if (pairingSavingRef.current) return
    if (candidate.current) recordAction('cancelled')
    pairingGeneration.current++
    candidate.current?.close()
    candidate.current = null
    setPairingProgress(null)
  }, [])
  useEffect(() => () => {
    pairingGeneration.current++
    candidate.current?.close()
    candidate.current = null
  }, [])

  const pairings = useStore(appStore, (state) => state.pairings)
  const sessionStates = useStore(appStore, (state) => state.sessions)
  const unread = useStore(appStore, (state) => state.unread)
  const pairingInvalidBySession = useStore(appStore, (state) => state.pairingInvalid)
  const view = useStore(appStore, (state) => state.view)
  const currentSessionId = useStore(appStore, (state) => state.currentSessionId)
  const adding = useStore(appStore, (state) => state.adding)
  const pairingError = useStore(appStore, (state) => state.pairingError)

  const sessions = useMemo<SessionListItem[]>(() => pairings.records.map((record) => ({
    ...record,
    runtimeState: sessionStates[record.sessionId],
    unread: unread[record.sessionId] ?? false,
  })), [pairings, sessionStates, unread])

  const openPairing = useCallback(async (payload: PairingPayload) => {
    if (pairingSavingRef.current) return
    cancelPairing()
    const generation = ++pairingGeneration.current
    appStore.getState().setPairingError(null)
    appStore.getState().showList()
    appStore.getState().setAdding(true)
    setSettingsPage(null)
    window.location.hash = ''
    recordAction('pairing')
    setPairingTarget(safeNetworkTarget(payload.wsUrl))
    const client = new ProtocolClient(payload, socketFactory)
    candidate.current = client
    const bufferedEvents: import('./protocol/client').ClientEvent[] = []
    const current = () => generation === pairingGeneration.current && candidate.current === client
    client.onClose = () => {
      if (!current()) return
      const diagnostic = networkDiagnostics.getSnapshot().filter((item) => item.sessionId === payload.sessionId).at(-1)
      appStore.getState().setPairingError(diagnostic ? networkCodeText[diagnostic.code] : '连接失败，请重试')
      setPairingProgress(null)
    }
    client.onEvent = (event) => {
      if (!current()) return
      if (event.kind === 'connectionStateChanged') {
        setPairingProgress(event.connection === 'connecting' ? '正在连接 PC…' : event.connection === 'authenticating' ? '正在验证配对…' : '正在同步会话…')
      }
      if (pairingSavingRef.current) {
        if (bufferedEvents.length >= 64) {
          client.close()
          appStore.getState().setPairingError('保存期间收到过多事件，请重试连接')
        } else bufferedEvents.push(event)
        return
      }
      if (event.kind !== 'snapshot') return
      pairingSavingRef.current = true
      setPairingSaving(true)
      setPairingProgress('连接验证成功，正在保存配对…')
      void (async () => {
        try {
          if (native !== null) {
            const saved = await native.logic.saveScanned(JSON.stringify(payload), now(), () => current() && client.isReady() && client.connectionState() === 'connected')
            if (!saved.ok) throw new Error('save failed')
          }
          if (!current() || client.connectionState() !== 'connected') return
          recordAction('saved')
          appStore.getState().upsertPairing(payload)
          runtime.adoptVerified(payload, client, event.snapshot, bufferedEvents)
          candidate.current = null
          appStore.getState().openDetail(payload.sessionId)
          setPairingProgress(null)
          window.location.hash = native === null ? `#pair=${encodePairingFragment(payload)}` : ''
        } catch {
          recordAction('save_failed')
          if (current()) {
            client.close()
            candidate.current = null
            setPairingProgress(null)
            appStore.getState().setPairingError('配对保存失败或连接已断开，请重试')
          }
        } finally {
          pairingSavingRef.current = false
          setPairingSaving(false)
        }
      })()
    }
    client.connect()
  }, [appStore, runtime, native, now, socketFactory, cancelPairing])

  useEffect(() => {
    runtime.setManagementCallbacks((payload) => { void openPairing(payload) }, (sessionId, operation, accepted, code) => {
      setManagementBusy((current) => ({ ...current, [sessionId]: false }))
      const text = accepted
        ? (operation === 'stop_session' ? 'Claude 会话已终止；配对入口仍保留' : null)
        : code === 'result_unknown' ? '操作结果未知，请重连核对；请勿重复新建会话'
        : code === 'session_capacity' ? '活跃会话已达上限' :
          code === 'create_rate_limited' ? '新建太频繁，请稍后重试' : '操作失败，请重试'
      setManagementFeedback((current) => ({ ...current, [sessionId]: text }))
    })
    return () => {
      runtime.setManagementCallbacks(null, null)
    }
  }, [runtime, openPairing])

  const submitJson = useCallback((json: string) => {
    const result = parsePairingInput(json, now())
    if (!result.ok) {
      appStore.getState().setPairingError(result.error)
      return
    }
    void openPairing(result.value)
  }, [appStore, now, openPairing])

  useEffect(() => {
    if (native !== null && restoring) return
    if (window.location.hash === '') return
    const fragment = window.location.hash
    if (native !== null) window.location.hash = ''
    const result = parsePairingFragment(fragment, now())
    if (!result.ok) {
      appStore.getState().setPairingError(result.error)
      return
    }
    void openPairing(result.value)
  }, [appStore, now, openPairing, native, restoring])

  useEffect(() => {
    if (native === null) return
    let active = true
    void native.logic.restore(now()).then(async (restored) => {
      // Restore display files before opening any sockets; live snapshots always win.
      if (cache !== null) await Promise.all([...restored.available.map((item) => item.metadata), ...restored.needsPairing].map(async (record) => {
        try {
          const { data } = await cache.readSessionCache({ sessionId: record.sessionId })
          const cached = decodeSessionCache(data, record.sessionId, now())
          if (active && cached) appStore.getState().restoreCachedSession(record.sessionId, cached)
        } catch { if (active) setCacheMessage('本地消息缓存暂不可用，仍可连接 PC') }
      }))
      if (!active) return
      for (const { payload, metadata } of restored.available) {
        if (appStore.getState().pairings.records.some((item) => item.sessionId === metadata.sessionId)) continue
        appStore.getState().restorePairing(metadata, payload)
        runtime.ensureConnected(payload)
      }
      for (const metadata of restored.needsPairing) {
        if (appStore.getState().pairings.records.some((item) => item.sessionId === metadata.sessionId)) continue
        appStore.getState().restorePairing(metadata)
      }
    }).catch(() => {
      if (active) appStore.getState().setPairingError('原生配对存储暂不可用')
    }).finally(() => {
      if (active) setRestoring(false)
    })
    return () => { active = false }
  }, [appStore, runtime, native, cache, now])

  useEffect(() => {
    if (cache === null || native === null || restoring) return
    const writer = new SessionCacheWriter(appStore, cache, now, () => setCacheMessage('本地消息缓存暂不可用，仍可连接 PC'))
    cacheWriter.current = writer
    const flush = () => { if (document.visibilityState !== 'visible') void writer.flush() }
    document.addEventListener('visibilitychange', flush)
    window.addEventListener('pagehide', flush)
    return () => {
      document.removeEventListener('visibilitychange', flush)
      window.removeEventListener('pagehide', flush)
      writer.dispose()
      if (cacheWriter.current === writer) cacheWriter.current = null
    }
  }, [appStore, cache, native, now, restoring])

  useEffect(() => {
    if (native === null) return
    const dispose = observeNativeForeground(native.appState, () => appStore.getState().pairings.records
      .filter((record) => record.pairToken !== '')
      .map((record) => ({
        version: PAIRING_PAYLOAD_VERSION, wsUrl: record.wsUrl, sessionId: record.sessionId,
        pairToken: record.pairToken, expiresAt: record.expiresAt,
        ...(record.capabilities === undefined ? {} : { capabilities: record.capabilities }),
        ...(record.pcId === undefined ? {} : { pcId: record.pcId }),
        ...(record.pcName === undefined ? {} : { pcName: record.pcName }),
      })), now, (plan) => {
      for (const sessionId of plan.needsPairingSessionIds) {
        runtime.closeSession(sessionId)
        appStore.getState().markPairingStatus(sessionId, 'needsPairing')
        appStore.getState().setPairingInvalid(sessionId, true)
      }
      runtime.refreshConnectionStates()
    })
    return dispose
  }, [appStore, runtime, native, now])

  useEffect(() => {
    if (native === null) return
    const observed = new Map<string, number | null>()
    return appStore.subscribe((state) => {
      for (const record of state.pairings.records) {
        const previous = observed.get(record.sessionId)
        if (record.lastConnectedAt !== null && previous !== undefined && previous !== record.lastConnectedAt) {
          void native.logic.markConnected(record.sessionId, record.lastConnectedAt).catch(() => {
            appStore.getState().setPairingError('原生配对存储暂不可用')
          })
        }
        observed.set(record.sessionId, record.lastConnectedAt)
      }
    })
  }, [appStore, native])

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') runtime.refreshConnectionStates()
    }
    const handleOffline = () => runtime.networkOffline()
    const handleOnline = () => runtime.refreshConnectionStates()
    window.addEventListener('offline', handleOffline)
    window.addEventListener('online', handleOnline)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      window.removeEventListener('offline', handleOffline)
      window.removeEventListener('online', handleOnline)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      runtime.closeAll()
    }
  }, [runtime])

  const currentState = currentSessionId === null
    ? EMPTY_STATE
    : sessionStates[currentSessionId] ?? EMPTY_STATE
  const currentRecord = currentSessionId === null
    ? undefined
    : pairings.records.find((record) => record.sessionId === currentSessionId)
  const turnStatus = useMemo(() => turnStatusOf(currentState), [currentState])
  const cancelTarget = useMemo(() => cancelTargetOf(currentState), [currentState])

  const backToList = useCallback(() => {
    setSettingsPage(null)
    setRecentlyViewedId(appStore.getState().currentSessionId)
    appStore.getState().showList()
    window.location.hash = ''
  }, [appStore])

  const openSession = useCallback((sessionId: string) => {
    const listPage = document.querySelector<HTMLElement>('[data-mobile-page-active="true"][data-mobile-page="list"]')
    if (listPage !== null) listScrollY.current = listPage.scrollTop
    const record = appStore.getState().pairings.records.find((item) => item.sessionId === sessionId)
    if (record === undefined) return
    if ((record.status === 'needsPairing' || record.pairToken === '') && !appStore.getState().sessions[sessionId]?.cachedAt) {
      appStore.getState().setAdding(true)
      appStore.getState().setPairingError('请重新扫码或输入此会话的配对信息')
      return
    }
    const payload: PairingPayload = {
      version: PAIRING_PAYLOAD_VERSION,
      wsUrl: record.wsUrl,
      sessionId: record.sessionId,
      pairToken: record.pairToken,
      expiresAt: record.expiresAt,
      ...(record.capabilities === undefined ? {} : { capabilities: [...record.capabilities] }),
      ...(record.pcId === undefined ? {} : { pcId: record.pcId }),
      ...(record.pcName === undefined ? {} : { pcName: record.pcName }),
    }
    appStore.getState().openDetail(sessionId)
    if (record.status !== 'needsPairing' && record.pairToken !== '') runtime.ensureConnected(payload)
    if (native === null) window.location.hash = `#pair=${encodePairingFragment(payload)}`
  }, [appStore, runtime, native])

  const removePairing = useCallback(async (sessionId: string): Promise<boolean> => {
    if (native !== null) {
      try { await native.logic.remove(sessionId) } catch {
        appStore.getState().setPairingError('原生配对存储暂不可用')
        return false
      }
    }
    runtime.closeSession(sessionId)
    if (experience !== null) await experience.clearNotifications({ sessionId }).catch(() => undefined)
    appStore.getState().removePairing(sessionId)
    return true
  }, [appStore, runtime, native, experience])

  useEffect(() => {
    if (experience?.syncPushSessions === undefined || restoring) return
    const sessions = pairings.records.filter((record) => !pairingInvalidBySession[record.sessionId] && Date.parse(record.expiresAt) > now())
      .map(({ sessionId, pairToken, expiresAt }) => ({ sessionId, pairToken, expiresAt }))
    void experience.syncPushSessions({ sessions, viewedSession: view === 'detail' ? currentSessionId ?? '' : '' }).catch(() => undefined)
  }, [experience, restoring, pairings, pairingInvalidBySession, view, currentSessionId, now])

  useEffect(() => {
    if (experience === null || restoring) return
    runtime.setReminderCallbacks(
      (sessionId, seq, kind) => { void experience.notify({ sessionId, seq, kind }).catch(() => undefined) },
      () => document.visibilityState === 'visible' && appStore.getState().view === 'detail' ? appStore.getState().currentSessionId : null,
    )
    let disposed = false
    const listener = experience.addListener('notificationOpened', ({ sessionId }) => {
      if (!disposed) {
        openSession(sessionId)
        void experience.clearNotifications({ sessionId }).catch(() => undefined)
      }
    }).catch(() => null)
    return () => {
      disposed = true
      runtime.setReminderCallbacks(null)
      void listener.then((handle) => handle?.remove()).catch(() => undefined)
    }
  }, [experience, runtime, appStore, openSession, restoring])

  const pageKey = view === 'detail' && currentRecord !== undefined && currentSessionId !== null
    ? `detail:${currentSessionId}`
    : restoring ? 'restoring' : adding ? 'pairing' : settingsPage ?? (sessions.length === 0 ? 'pairing' : 'list')

  useLayoutEffect(() => {
    if (pageKey !== 'list') return
    const page = document.querySelector<HTMLElement>('[data-mobile-page-active="true"][data-mobile-page="list"]')
    if (page !== null) page.scrollTop = listScrollY.current
  }, [pageKey])

  let pageContent
  if (pageKey.startsWith('detail:') && currentRecord !== undefined && currentSessionId !== null) {
    pageContent = (
      <>
      {pairingError !== null && <div role="alert" style={{ padding: 12, color: '#b00' }}>{pairingError}</div>}
      <SessionDetail
        key={currentSessionId}
        experience={experience}
        record={currentRecord}
        state={currentState}
        turnStatus={turnStatus}
        pairingInvalid={pairingInvalidBySession[currentSessionId] ?? false}
        cancelTarget={cancelTarget}
        onBack={backToList}
        onReconnect={() => runtime.reconnect(currentSessionId)}
        onSendAttachments={(text, attachments) => runtime.sendAttachments(currentSessionId, text, attachments)}
        onSend={(text) => { runtime.sendText(currentSessionId, text) }}
        onPermissionModeChange={(permissionMode: PermissionMode) => {
          runtime.setPermissionMode(currentSessionId, permissionMode)
        }}
        onResolveAction={(resolution: ActionResolution) => runtime.resolveAction(currentSessionId, resolution)}
        onCancelTurn={(requestId) => runtime.cancelTurn(currentSessionId, requestId)}
        onCancelQueued={(requestId) => runtime.cancelQueued(currentSessionId, requestId)}
        managementBusy={managementBusy[currentSessionId] ?? false}
        managementFeedback={managementFeedback[currentSessionId] ?? null}
        onCreateSession={() => {
          if (runtime.createSession(currentSessionId)) {
            setManagementBusy((current) => ({ ...current, [currentSessionId]: true }))
            setManagementFeedback((current) => ({ ...current, [currentSessionId]: null }))
          }
        }}
      />
      </>
    )
  } else if (restoring) {
    pageContent = <div className="mobile-safe-area-content" style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>正在恢复已配对会话…</div>
  } else if ((adding && settingsPage === null) || (sessions.length === 0 && settingsPage === null)) {
    pageContent = (
      <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
        <div className="mobile-safe-area-header">
          <NavBar
            back={sessions.length === 0 ? null : ''}
            backIcon={<LeftOutline aria-label="返回" />}
            right={<button className="mobile-settings-button" onClick={() => { cancelPairing(); appStore.getState().setAdding(false); setSettingsPage('settings') }}>设置</button>}
            onBack={() => {
              if (pairingSavingRef.current) return
              cancelPairing()
              appStore.getState().setAdding(false)
              appStore.getState().setPairingError(null)
            }}
          >
            Claude Mobile Remote
          </NavBar>
        </div>
        <div style={{ padding: 16, textAlign: 'center' }}><button onClick={() => { cancelPairing(); setDemoOpen(true) }}>体验演示（无需连接电脑）</button></div>
        <PairingForm target={pairingTarget} initialError={pairingError} progress={pairingProgress} saving={pairingSaving} onCancel={cancelPairing} onSubmit={submitJson} onScan={native === null ? undefined : () => {
          recordAction('scan')
          const generation = ++pairingGeneration.current
          setPairingProgress('正在扫码…')
          void native.scanner.scan().then((input) => {
            if (generation !== pairingGeneration.current) return
            setPairingProgress(null)
            if (input !== null) submitJson(input.trim())
          }).catch(() => {
            if (generation !== pairingGeneration.current) return
            setPairingProgress(null)
            recordAction('scan_failed')
            appStore.getState().setPairingError('扫码暂不可用，请检查相机权限')
          })
        }} />
      </div>
    )
  } else if (settingsPage === 'settings') {
    pageContent = <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
      <div className="mobile-safe-area-header"><NavBar back="" backIcon={<LeftOutline aria-label="返回" />} onBack={() => setSettingsPage(null)}>设置</NavBar></div>
      <List className="mobile-settings-list">
        <List.Item arrow onClick={() => { setSettingsDirection('forward'); setSettingsPage('pcs') }}>PC 管理</List.Item>
      </List>
      <List><List.Item arrow onClick={() => setDemoOpen(true)}>体验演示（无需连接电脑）</List.Item></List>
      <List header="调试">
        <List.Item description="开启后显示悬浮按钮，记录本次运行中的动作和连接过程。关闭后停止新增记录；退出 App 后清除。" extra={<Switch aria-label="调试日志开关" checked={debugState.enabled} onChange={(enabled) => debugLog.setEnabled(enabled)} />}>调试日志</List.Item>
      </List>
      <List header="本地保存说明">
        <List.Item description="原生 App 重启后恢复已配对会话、设备信息和自定义名称。未过期凭证保存在系统安全存储；移除 PC 或会话会删除对应本机配对。浏览器仅保存在内存。">配对与设备</List.Item>
        <List.Item description="iOS / Android 保存最近已收到的消息、标题与工作目录，重启先展示缓存，再连接 PC 更新状态。每会话最多 200 项/2 MiB，总计 20 MiB，保留 30 天；不缓存图片原件，不支持离线发送。浏览器仅保存在内存。">消息与状态</List.Item>
      </List>
      {cache !== null && <List><List.Item arrow onClick={() => {
        void cacheWriter.current?.clear().then(() => setCacheMessage('离线缓存已清除；当前页面内容仍可继续使用')).catch(() => setCacheMessage('清除缓存失败，请重试'))
      }}>清除离线缓存</List.Item></List>}
      {cacheMessage !== null && <p role="status" style={{ padding: 16 }}>{cacheMessage}</p>}
      {experience !== null && <NotificationSettings port={experience} />}
    </div>
  } else if (settingsPage === 'pcs') {
    pageContent = <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
      <div className="mobile-safe-area-header"><NavBar back="" backIcon={<LeftOutline aria-label="返回" />} onBack={() => { setSettingsDirection('back'); setSettingsPage('settings') }}>PC 管理</NavBar></div>
      {pairingError !== null && <div role="alert" style={{ padding: 12, color: '#b00' }}>{pairingError}</div>}
      <PCSettings sessions={sessions}
        onRemovePC={(sessionIds) => { void (async () => {
          for (const sessionId of sessionIds) {
            if (!await removePairing(sessionId)) break
          }
        })() }}
      />
    </div>
  } else {
    pageContent = (
    <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
      <div className="mobile-safe-area-header">
        <NavBar back={null} right={<button className="mobile-settings-button" onClick={() => { setSettingsDirection('forward'); setSettingsPage('settings') }}>设置</button>}>Claude Mobile Remote</NavBar>
      </div>
      {pairingError !== null && <div role="alert" style={{ padding: 12, color: '#b00' }}>{pairingError}</div>}
      <ProjectSessionList
        sessions={sessions}
        recentlyViewedId={recentlyViewedId}
        onAdd={() => {
          appStore.getState().setAdding(true)
          appStore.getState().setPairingError(null)
        }}
        onOpen={openSession}
        onRemove={(sessionId) => { void removePairing(sessionId) }}
        onRename={(sessionId, name) => {
          if (native !== null) {
            void native.logic.rename(sessionId, name).then(() => appStore.getState().renamePairing(sessionId, name))
              .catch(() => appStore.getState().setPairingError('原生配对存储暂不可用'))
            return
          }
          appStore.getState().renamePairing(sessionId, name)
        }}
      />
    </div>
    )
  }

  if (demoOpen) return <DemoPage onExit={() => setDemoOpen(false)} />

  return <><DebugPanel /><PageTransition
    pageKey={pageKey}
    direction={pageKey === 'list' ? 'back' : pageKey === 'settings' || pageKey === 'pcs' ? settingsDirection : 'forward'}
    onListScroll={(scrollTop) => { listScrollY.current = scrollTop }}
    onEdgeBack={pageKey.startsWith('detail:') ? backToList : pageKey === 'settings' ? () => setSettingsPage(null) : pageKey === 'pcs' ? () => { setSettingsDirection('back'); setSettingsPage('settings') } : undefined}
  >
    {pageContent}
  </PageTransition></>
}

export default App
