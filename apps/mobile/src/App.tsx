import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { NavBar } from 'antd-mobile'
import PairingForm from './components/PairingForm'
import SessionDetail from './components/SessionDetail'
import SessionList from './components/SessionList'
import { PAGE_BACKGROUND } from './components/ui'
import { ProtocolClient, type ActionResolution, type CloseInfo, type SocketFactory } from './protocol/client'
import { encodePairingFragment, parsePairingFragment, parsePairingInput, PAIRING_PAYLOAD_VERSION, type PairingPayload } from './protocol/pairing'
import {
  actionResolutionSent,
  commandAccepted,
  commandRejected,
  commandSent,
  commandUnknown,
  connectionStateChanged,
  connectionFailed,
  createInitialState,
  eventReceived,
  noticeReceived,
  pairLoaded,
  queuedCancelSent,
  reducer,
  reset,
  snapshotReceived,
  turnCancelSent,
} from './session/reducer'
import {
  createInitialSessionStore,
  listSessions,
  markSessionStatus,
  removeSession,
  renameSession,
  upsertSession,
} from './session/store'

// 生成唯一 commandId：优先 Web Crypto，测试环境退化为计数器。
let commandCounter = 0
function generateCommandId(): string {
  const cryptoApi = globalThis.crypto
  if (cryptoApi?.randomUUID !== undefined) {
    return cryptoApi.randomUUID()
  }
  commandCounter += 1
  return `cmd-${commandCounter}`
}

interface AppProps {
  now?: () => number
  socketFactory?: SocketFactory
}

// DOM WebSocket 与 SocketLike 的事件处理器签名存在结构差异，此处统一收敛。
const defaultSocketFactory: SocketFactory = (url) =>
  new WebSocket(url) as unknown as ReturnType<SocketFactory>

function App({
  now = () => Date.now(),
  socketFactory = defaultSocketFactory,
}: AppProps = {}) {
  const [state, dispatch] = useReducer(reducer, undefined, createInitialState)
  const [store, setStore] = useState(createInitialSessionStore)
  const [view, setView] = useState<'list' | 'detail'>('list')
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null)
  // 当前详情页正在连接的配对负载。与 store 记录分开保存：记录会因重命名或
  // 状态更新产生新对象，若连接 effect 依赖记录会导致误重连（B1）。
  const [activePairing, setActivePairing] = useState<PairingPayload | null>(null)
  const [adding, setAdding] = useState(false)
  const [pairingError, setPairingError] = useState<string | null>(null)
  const [pairingInvalid, setPairingInvalid] = useState(false)
  const clientRef = useRef<ProtocolClient | null>(null)
  // now 的默认值是内联函数，每次渲染引用都不同。连接 effect 只能通过 ref
  // 读取它，否则依赖数组每渲染都变化会导致无限重连。
  const nowRef = useRef(now)
  nowRef.current = now

  const sessions = listSessions(store)

  // 配对成功：写入内存记录并进入该 Session 的详情页。
  const openPairing = useCallback(
    (payload: PairingPayload) => {
      setStore((current) => upsertSession(current, payload))
      setCurrentSessionId(payload.sessionId)
      setActivePairing(payload)
      setView('detail')
      setAdding(false)
      setPairingInvalid(false)
      setPairingError(null)
      window.location.hash = `#pair=${encodePairingFragment(payload)}`
      dispatch(pairLoaded())
    },
    [],
  )

  const submitJson = useCallback(
    (json: string) => {
      const result = parsePairingInput(json, now())
      if (!result.ok) {
        setPairingError(result.error)
        return
      }
      openPairing(result.value)
    },
    [now, openPairing],
  )

  // 首次加载：从 Fragment 自动读取并连接当前 Session（A5 行为保留）。
  // 只恢复 Fragment 中的这一个会话，不恢复其他列表记录、旧消息或做 replay。
  useEffect(() => {
    if (window.location.hash === '') return
    const result = parsePairingFragment(window.location.hash, now())
    if (!result.ok) {
      setPairingError(result.error)
      return
    }
    openPairing(result.value)
    // 仅在首次挂载时读取 Fragment。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // activePairing 变化时创建 client 并连接：任意时刻只存在一个活动连接。
  useEffect(() => {
    if (activePairing === null) return
    const sessionId = activePairing.sessionId
    const client = new ProtocolClient(activePairing, socketFactory)
    clientRef.current = client

    client.onEvent = (event) => {
      switch (event.kind) {
        case 'snapshot':
          dispatch(snapshotReceived(event.snapshot))
          setStore((current) =>
            markSessionStatus(current, sessionId, {
              status: 'connected',
              lastConnectedAt: nowRef.current(),
            }),
          )
          break
        case 'event':
          dispatch(eventReceived(event.event))
          break
        case 'commandResult':
          if (event.accepted && event.requestId !== '') {
            dispatch(commandAccepted({ commandId: event.commandId, requestId: event.requestId }))
          } else if (event.accepted) {
            // 接受但缺 requestId：按协议错误处理。
            dispatch(connectionFailed({ kind: 'protocolError' }))
          } else {
            dispatch(
              commandRejected({
                commandId: event.commandId,
                code: event.code,
                message: event.message,
              }),
            )
          }
          break
        case 'connectionStateChanged':
          dispatch(connectionStateChanged(event.connection))
          break
        // 不支持通知只转成可读提示，连接不受影响；但 seq 必须透传以推进
        // seqBase（通知在服务端占用一个 seq，B2 §4.4）。
        case 'unsupported':
          dispatch(noticeReceived(event.code, event.seq))
          break
      }
    }

    client.onClose = (info: CloseInfo) => {
      switch (info.kind) {
        case 'disconnected':
          dispatch(connectionFailed({ kind: 'disconnected' }))
          setStore((current) => markSessionStatus(current, sessionId, { status: 'disconnected' }))
          if (info.pendingCommandId !== null) {
            dispatch(commandUnknown({ commandId: info.pendingCommandId }))
          }
          break
        case 'pairingInvalid':
          dispatch(connectionFailed({ kind: 'disconnected' }))
          setPairingInvalid(true)
          setStore((current) => markSessionStatus(current, sessionId, { status: 'needsPairing' }))
          break
        case 'protocolError':
          dispatch(connectionFailed({ kind: 'protocolError' }))
          setStore((current) => markSessionStatus(current, sessionId, { status: 'disconnected' }))
          if (info.pendingCommandId !== null) {
            dispatch(commandUnknown({ commandId: info.pendingCommandId }))
          }
          break
      }
    }

    client.connect()

    const handleVisibilityChange = () => {
      if (document.visibilityState !== 'visible') return
      dispatch(connectionStateChanged(client.connectionState()))
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      client.onEvent = null
      client.onClose = null
      client.close()
    }
  }, [activePairing, socketFactory])

  const handleSend = useCallback((text: string) => {
    const client = clientRef.current
    if (client === null) return
    const commandId = generateCommandId()
    const result = client.sendText(text, commandId)
    if (result.ok) {
      dispatch(commandSent({ commandId, text }))
    }
  }, [])

  const handleReconnect = useCallback(() => {
    clientRef.current?.reconnect()
    dispatch(pairLoaded())
  }, [])

  // 待处理事项决议（B2 §5）：与自由文本共用单在途门控与 commandId 语义。
  const handleResolveAction = useCallback((resolution: ActionResolution): boolean => {
    const client = clientRef.current
    if (client === null) return false
    const commandId = generateCommandId()
    const result = client.resolveAction(resolution, commandId)
    if (result.ok) {
      dispatch(actionResolutionSent({ commandId, actionId: resolution.actionId }))
    }
    return result.ok
  }, [])

  const handleCancelTurn = useCallback((requestId: string): boolean => {
    const client = clientRef.current
    if (client === null) return false
    const commandId = generateCommandId()
    const result = client.cancelTurn(requestId, commandId)
    if (result.ok) {
      dispatch(turnCancelSent({ commandId, requestId }))
    }
    return result.ok
  }, [])

  const handleCancelQueued = useCallback((requestId: string): boolean => {
    const client = clientRef.current
    if (client === null) return false
    const commandId = generateCommandId()
    const result = client.cancelQueued(requestId, commandId)
    if (result.ok) {
      dispatch(queuedCancelSent({ commandId, requestId }))
    }
    return result.ok
  }, [])

  // 返回列表：关闭当前连接并清理详情状态，保留其他内存记录。
  const backToList = useCallback(() => {
    clientRef.current?.close()
    clientRef.current = null
    setActivePairing(null)
    setCurrentSessionId(null)
    setView('list')
    setAdding(false)
    setPairingInvalid(false)
    setPairingError(null)
    window.location.hash = ''
    dispatch(reset())
  }, [])

  const openSession = useCallback(
    (sessionId: string) => {
      const record = store.records.find((r) => r.sessionId === sessionId)
      if (record === undefined) return
      openPairing({
        // 记录只在校验通过的负载上创建，版本固定为配对负载版本。
        version: PAIRING_PAYLOAD_VERSION,
        wsUrl: record.wsUrl,
        sessionId: record.sessionId,
        pairToken: record.pairToken,
        expiresAt: record.expiresAt,
      })
    },
    [store, openPairing],
  )

  const turnStatus = useMemo(() => {
    // 有待处理事项即等待用户决议，优先于消息流推导（B2 §6）。
    if (state.pendingAction !== null) return 'waiting'
    const claude = [...state.messages].reverse().find((m) => m.kind === 'claude')
    if (claude === undefined) return 'idle'
    return claude.status === 'streaming' ? 'running' : claude.status
  }, [state.messages, state.pendingAction])

  // 可取消目标：待处理事项所属 Turn 优先，否则取最后一个仍在流式的 Turn。
  // 服务端不下发独立的可取消标记，只能由本连接已知状态推导（B2 §5）。
  const cancelTarget = useMemo(() => {
    if (state.pendingAction !== null) return state.pendingAction.requestId
    const streaming = [...state.messages]
      .reverse()
      .find((m) => m.kind === 'claude' && m.status === 'streaming')
    if (streaming === undefined || streaming.kind !== 'claude') return null
    return streaming.requestId
  }, [state.messages, state.pendingAction])

  const currentRecord =
    currentSessionId === null
      ? undefined
      : store.records.find((r) => r.sessionId === currentSessionId)

  // 详情页
  if (view === 'detail' && currentRecord !== undefined) {
    return (
      <SessionDetail
        record={currentRecord}
        state={state}
        turnStatus={turnStatus}
        pairingInvalid={pairingInvalid}
        cancelTarget={cancelTarget}
        onBack={backToList}
        onReconnect={handleReconnect}
        onSend={handleSend}
        onResolveAction={handleResolveAction}
        onCancelTurn={handleCancelTurn}
        onCancelQueued={handleCancelQueued}
      />
    )
  }

  // 配对页：列表为空时直接进入，否则由“新增配对”打开。
  if (adding || sessions.length === 0) {
    return (
      <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
        <NavBar
          back={sessions.length === 0 ? null : '返回'}
          onBack={() => {
            setAdding(false)
            setPairingError(null)
          }}
        >
          Claude Mobile Remote
        </NavBar>
        <PairingForm initialError={pairingError} onSubmit={submitJson} />
      </div>
    )
  }

  // 列表页
  return (
    <div style={{ minHeight: '100vh', background: PAGE_BACKGROUND }}>
      <NavBar back={null}>Claude Mobile Remote</NavBar>
      <SessionList
        sessions={sessions}
        onAdd={() => {
          setAdding(true)
          setPairingError(null)
        }}
        onOpen={openSession}
        onRemove={(sessionId) => setStore((current) => removeSession(current, sessionId))}
        onRename={(sessionId, name) =>
          setStore((current) => renameSession(current, sessionId, name))
        }
      />
    </div>
  )
}

export default App
