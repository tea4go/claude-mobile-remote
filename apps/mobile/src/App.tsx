import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { Button, NavBar } from 'antd-mobile'
import MessageComposer from './components/MessageComposer'
import MessageFeed from './components/MessageFeed'
import PairingForm from './components/PairingForm'
import SessionStatus from './components/SessionStatus'
import { ProtocolClient, type CloseInfo, type SocketFactory } from './protocol/client'
import { encodePairingFragment, parsePairingFragment, parsePairingJson, type PairingPayload } from './protocol/pairing'
import {
  commandAccepted,
  commandRejected,
  commandSent,
  commandUnknown,
  connectionStateChanged,
  connectionFailed,
  createInitialState,
  eventReceived,
  pairLoaded,
  reducer,
  reset,
  snapshotReceived,
} from './session/reducer'

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
  const [pairing, setPairing] = useState<PairingPayload | null>(null)
  const [pairingError, setPairingError] = useState<string | null>(null)
  const [pairingInvalid, setPairingInvalid] = useState(false)
  const clientRef = useRef<ProtocolClient | null>(null)

  // Fragment 或粘贴 JSON → 校验通过后进入连接。
  const startPairing = useCallback(
    (payload: PairingPayload) => {
      setPairingInvalid(false)
      setPairing(payload)
      setPairingError(null)
      window.location.hash = `#pair=${encodePairingFragment(payload)}`
      dispatch(pairLoaded())
    },
    [],
  )

  const submitJson = useCallback(
    (json: string) => {
      const result = parsePairingJson(json, now())
      if (!result.ok) {
        setPairingError(result.error)
        return
      }
      startPairing(result.value)
    },
    [now, startPairing],
  )

  // 首次加载：从 Fragment 自动读取并连接。
  useEffect(() => {
    if (window.location.hash === '') return
    const result = parsePairingFragment(window.location.hash, now())
    if (!result.ok) {
      setPairingError(result.error)
      return
    }
    startPairing(result.value)
    // 仅在首次挂载时读取 Fragment。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // pairing 变化时创建 client 并连接。
  useEffect(() => {
    if (pairing === null) return
    const client = new ProtocolClient(pairing, socketFactory)
    clientRef.current = client

    client.onEvent = (event) => {
      switch (event.kind) {
        case 'snapshot':
          dispatch(snapshotReceived(event.snapshot))
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
      }
    }

    client.onClose = (info: CloseInfo) => {
      switch (info.kind) {
        case 'disconnected':
          dispatch(connectionFailed({ kind: 'disconnected' }))
          if (info.pendingCommandId !== null) {
            dispatch(commandUnknown({ commandId: info.pendingCommandId }))
          }
          break
        case 'pairingInvalid':
          dispatch(connectionFailed({ kind: 'disconnected' }))
          setPairingInvalid(true)
          break
        case 'protocolError':
          dispatch(connectionFailed({ kind: 'protocolError' }))
          if (info.pendingCommandId !== null) {
            dispatch(commandUnknown({ commandId: info.pendingCommandId }))
          }
          break
      }
    }

    client.connect()
    return () => {
      client.onEvent = null
      client.onClose = null
      client.close()
    }
  }, [pairing, socketFactory])

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

  const handleReset = useCallback(() => {
    clientRef.current?.close()
    clientRef.current = null
    setPairing(null)
    setPairingInvalid(false)
    setPairingError(null)
    window.location.hash = ''
    dispatch(reset())
  }, [])

  const turnStatus = useMemo(() => {
    const claude = [...state.messages].reverse().find((m) => m.kind === 'claude')
    if (claude === undefined) return 'idle'
    return claude.status === 'streaming' ? 'running' : claude.status
  }, [state.messages])

  // 配对页
  if (pairing === null) {
    return (
      <div style={{ minHeight: '100vh', background: '#f5f5f5' }}>
        <NavBar back={null}>Claude Mobile Remote</NavBar>
        <PairingForm initialError={pairingError} onSubmit={submitJson} />
      </div>
    )
  }

  // 会话页
  const showReconnect =
    (state.phase === 'disconnected' || state.phase === 'protocolError') && !pairingInvalid

  return (
    <div style={{ minHeight: '100vh', background: '#f5f5f5', display: 'flex', flexDirection: 'column' }}>
      <NavBar back={null}>Claude Mobile Remote</NavBar>
      <div style={{ padding: '8px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <SessionStatus
          connection={pairingInvalid ? 'disconnected' : state.connection}
          sessionStatus={state.sessionStatus}
          turnStatus={turnStatus}
          queueDepthHint={state.queueDepthHint}
        />
        {pairingInvalid && (
          <div style={{ color: '#e00' }}>配对已失效，请更换配对</div>
        )}
        {state.phase === 'protocolError' && !pairingInvalid && (
          <div style={{ color: '#e00' }}>状态异常，请重新连接以获取最新状态</div>
        )}
        {state.lastError !== null && (
          <div style={{ color: '#e00' }}>{state.lastError.message}</div>
        )}
        {showReconnect && (
          <Button color="primary" size="small" onClick={handleReconnect}>
            重新连接
          </Button>
        )}
        <Button size="small" onClick={handleReset}>
          更换配对
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'hidden', padding: '0 16px' }}>
        <MessageFeed messages={state.messages} />
      </div>
      <div style={{ padding: '8px 16px', borderTop: '1px solid #e0e0e0' }}>
        <MessageComposer enabled={state.canSend} onSend={handleSend} />
      </div>
    </div>
  )
}

export default App
