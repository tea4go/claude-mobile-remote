import { useCallback, useEffect, useMemo, useState } from 'react'
import { NavBar } from 'antd-mobile'
import { useStore } from 'zustand'
import PairingForm from './components/PairingForm'
import SessionDetail from './components/SessionDetail'
import SessionList, { type SessionListItem } from './components/SessionList'
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
import { listSessions } from './session/store'

interface AppProps {
  now?: () => number
  socketFactory?: SocketFactory
}

const defaultSocketFactory: SocketFactory = (url) =>
  new WebSocket(url) as unknown as ReturnType<SocketFactory>
const defaultNow = () => Date.now()
const EMPTY_STATE = createInitialState()

function turnStatusOf(state: MobileState) {
  if (state.pendingAction !== null) return 'waiting' as const
  const claude = [...state.messages].reverse().find((message) => message.kind === 'claude')
  if (claude === undefined || claude.kind !== 'claude') return 'idle' as const
  return claude.status === 'streaming' ? 'running' as const : claude.status
}

function cancelTargetOf(state: MobileState): string | null {
  if (state.pendingAction !== null) return state.pendingAction.requestId
  const streaming = [...state.messages]
    .reverse()
    .find((message) => message.kind === 'claude' && message.status === 'streaming')
  return streaming?.kind === 'claude' ? streaming.requestId : null
}

function App({ now = defaultNow, socketFactory = defaultSocketFactory }: AppProps = {}) {
  const [appStore] = useState(createAppStore)
  const [runtime] = useState(() => new SessionRuntimeManager(appStore, socketFactory, now))

  const pairings = useStore(appStore, (state) => state.pairings)
  const sessionStates = useStore(appStore, (state) => state.sessions)
  const unread = useStore(appStore, (state) => state.unread)
  const pairingInvalidBySession = useStore(appStore, (state) => state.pairingInvalid)
  const view = useStore(appStore, (state) => state.view)
  const currentSessionId = useStore(appStore, (state) => state.currentSessionId)
  const adding = useStore(appStore, (state) => state.adding)
  const pairingError = useStore(appStore, (state) => state.pairingError)

  const sessions = useMemo<SessionListItem[]>(() => listSessions(pairings).map((record) => ({
    ...record,
    runtimeState: sessionStates[record.sessionId],
    unread: unread[record.sessionId] ?? false,
  })), [pairings, sessionStates, unread])

  const openPairing = useCallback((payload: PairingPayload) => {
    appStore.getState().upsertPairing(payload)
    appStore.getState().openDetail(payload.sessionId)
    runtime.ensureConnected(payload)
    window.location.hash = `#pair=${encodePairingFragment(payload)}`
  }, [appStore, runtime])

  const submitJson = useCallback((json: string) => {
    const result = parsePairingInput(json, now())
    if (!result.ok) {
      appStore.getState().setPairingError(result.error)
      return
    }
    openPairing(result.value)
  }, [appStore, now, openPairing])

  useEffect(() => {
    if (window.location.hash === '') return
    const result = parsePairingFragment(window.location.hash, now())
    if (!result.ok) {
      appStore.getState().setPairingError(result.error)
      return
    }
    openPairing(result.value)
  }, [appStore, now, openPairing])

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') runtime.refreshConnectionStates()
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
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
    appStore.getState().showList()
    window.location.hash = ''
  }, [appStore])

  const openSession = useCallback((sessionId: string) => {
    const record = appStore.getState().pairings.records.find((item) => item.sessionId === sessionId)
    if (record === undefined) return
    const payload: PairingPayload = {
      version: PAIRING_PAYLOAD_VERSION,
      wsUrl: record.wsUrl,
      sessionId: record.sessionId,
      pairToken: record.pairToken,
      expiresAt: record.expiresAt,
    }
    appStore.getState().openDetail(sessionId)
    runtime.ensureConnected(payload)
    window.location.hash = `#pair=${encodePairingFragment(payload)}`
  }, [appStore, runtime])

  if (view === 'detail' && currentRecord !== undefined && currentSessionId !== null) {
    return (
      <SessionDetail
        record={currentRecord}
        state={currentState}
        turnStatus={turnStatus}
        pairingInvalid={pairingInvalidBySession[currentSessionId] ?? false}
        cancelTarget={cancelTarget}
        onBack={backToList}
        onReconnect={() => runtime.reconnect(currentSessionId)}
        onSend={(text) => { runtime.sendText(currentSessionId, text) }}
        onPermissionModeChange={(permissionMode: PermissionMode) => {
          runtime.setPermissionMode(currentSessionId, permissionMode)
        }}
        onResolveAction={(resolution: ActionResolution) => runtime.resolveAction(currentSessionId, resolution)}
        onCancelTurn={(requestId) => runtime.cancelTurn(currentSessionId, requestId)}
        onCancelQueued={(requestId) => runtime.cancelQueued(currentSessionId, requestId)}
      />
    )
  }

  if (adding || sessions.length === 0) {
    return (
      <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
        <NavBar
          back={sessions.length === 0 ? null : '返回'}
          onBack={() => {
            appStore.getState().setAdding(false)
            appStore.getState().setPairingError(null)
          }}
        >
          Claude Mobile Remote
        </NavBar>
        <PairingForm initialError={pairingError} onSubmit={submitJson} />
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
      <NavBar back={null}>Claude Mobile Remote</NavBar>
      <SessionList
        sessions={sessions}
        onAdd={() => {
          appStore.getState().setAdding(true)
          appStore.getState().setPairingError(null)
        }}
        onOpen={openSession}
        onRemove={(sessionId) => {
          runtime.closeSession(sessionId)
          appStore.getState().removePairing(sessionId)
        }}
        onRename={(sessionId, name) => appStore.getState().renamePairing(sessionId, name)}
      />
    </div>
  )
}

export default App
