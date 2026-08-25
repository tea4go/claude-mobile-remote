// 单 WebSocket 生命周期与协议编排（设计 §5）。
// 不依赖 React 或 DOM；WebSocket 工厂可注入以便测试。

import { decodeServerMessage } from './decode'
import type { PairingPayload } from './pairing'
import { PROTOCOL_VERSION } from './types'

// 连接内部阶段；onEvent 通过 connectionStateChanged 导出给 reducer。
type Stage = 'connecting' | 'authenticating' | 'subscribing' | 'ready'

export type ClientEvent =
  | { kind: 'connectionStateChanged'; connection: 'connecting' | 'authenticating' | 'subscribing' | 'connected' }
  | { kind: 'snapshot'; snapshot: import('./types').SessionSnapshot }
  | { kind: 'event'; event: import('./types').DomainEvent }
  | {
      kind: 'commandResult'
      commandId: string
      requestId: string
      accepted: boolean
      code: string
      message: string
    }

export type CloseKind =
  // 非正常 WebSocket close（含服务端溢出关闭），统一按已断开 + 结果未知处理。
  | 'disconnected'
  // 服务端 error/unauthorized：配对已失效，禁用重连。
  | 'pairingInvalid'
  // 解码或顺序错误：状态不可信。
  | 'protocolError'

export interface CloseInfo {
  kind: CloseKind
  pendingCommandId: string | null
}

export interface SendResult {
  ok: boolean
  reason?: 'notReady' | 'busy'
}

/* eslint-disable @typescript-eslint/no-explicit-any */
export type SocketLike = {
  send(data: string): void
  close(): void
  // 事件参数类型宽松：兼容 DOM WebSocket 与测试假实现。
  onopen: ((event?: any) => void) | null
  onmessage: ((event: { data: any }) => void) | null
  onclose: ((event?: any) => void) | null
  onerror: ((event?: any) => void) | null
}

export type SocketFactory = (url: string) => SocketLike

function envelope(type: string, sessionId: string, commandId: string, requestId: string, seq: number, payload: unknown): string {
  return JSON.stringify({
    version: PROTOCOL_VERSION,
    type,
    sessionId,
    commandId,
    requestId,
    seq,
    payload,
  })
}

export class ProtocolClient {
  private readonly payload: PairingPayload
  private readonly socketFactory: SocketFactory
  private socket: SocketLike | null = null
  private stage: Stage = 'connecting'
  private pendingCommandId: string | null = null
  private closedByUser = false
  private generation = 0

  onEvent: ((event: ClientEvent) => void) | null = null
  onClose: ((info: CloseInfo) => void) | null = null

  constructor(payload: PairingPayload, socketFactory: SocketFactory) {
    this.payload = payload
    this.socketFactory = socketFactory
  }

  isReady(): boolean {
    return this.stage === 'ready'
  }

  connect(): void {
    if (this.socket !== null) return
    this.closedByUser = false
    const generation = ++this.generation
    const socket = this.socketFactory(this.payload.wsUrl)
    this.socket = socket
    this.stage = 'connecting'
    this.emit({ kind: 'connectionStateChanged', connection: 'connecting' })

    socket.onopen = () => {
      if (generation !== this.generation) return
      this.stage = 'authenticating'
      this.emit({ kind: 'connectionStateChanged', connection: 'authenticating' })
      // hello 携带 sessionId：服务端按 envelope.sessionId 校验 pair token 的会话绑定。
      socket.send(
        envelope('hello', this.payload.sessionId, '', '', 0, {
          pairToken: this.payload.pairToken,
        }),
      )
    }
    socket.onmessage = (message) => {
      if (generation !== this.generation) return
      this.handleMessage(message.data)
    }
    socket.onerror = () => {
      // 错误后浏览器必定还会触发 close；统一在 close 处理。
    }
    socket.onclose = () => {
      if (generation !== this.generation) return
      this.socket = null
      if (this.closedByUser) return
      // 非正常 close 不区分原因（网络断开与服务端溢出关闭在 close 上不可区分）。
      // 断开状态由 onClose 通知，不进入 connectionStateChanged。
      this.onClose?.({ kind: 'disconnected', pendingCommandId: this.pendingCommandId })
      this.pendingCommandId = null
    }
  }

  reconnect(): void {
    this.close()
    this.connect()
  }

  close(): void {
    const socket = this.socket
    this.socket = null
    this.stage = 'connecting'
    this.pendingCommandId = null
    if (socket === null) return
    this.closedByUser = true
    this.generation++
    socket.onopen = null
    socket.onmessage = null
    socket.onclose = null
    socket.onerror = null
    socket.close()
  }

  sendText(text: string, commandId: string): SendResult {
    if (this.stage !== 'ready' || this.socket === null) {
      return { ok: false, reason: 'notReady' }
    }
    if (this.pendingCommandId !== null) {
      return { ok: false, reason: 'busy' }
    }
    this.pendingCommandId = commandId
    this.socket.send(
      envelope('command', this.payload.sessionId, commandId, '', 0, {
        kind: 'send_text',
        text,
      }),
    )
    return { ok: true }
  }

  private emit(event: ClientEvent): void {
    try {
      this.onEvent?.(event)
    } catch {
      // 观察者异常不得影响连接。
    }
  }

  private terminate(kind: CloseKind): void {
    const socket = this.socket
    this.socket = null
    this.closedByUser = true
    this.generation++
    if (socket !== null) {
      socket.onopen = null
      socket.onmessage = null
      socket.onclose = null
      socket.onerror = null
      socket.close()
    }
    this.onClose?.({ kind, pendingCommandId: this.pendingCommandId })
    this.pendingCommandId = null
  }

  private handleMessage(data: string): void {
    let message
    try {
      message = decodeServerMessage(data)
    } catch {
      this.terminate('protocolError')
      return
    }

    if (
      (message.type === 'snapshot' || message.type === 'event' || message.type === 'command_result') &&
      message.sessionId !== this.payload.sessionId
    ) {
      this.terminate('protocolError')
      return
    }

    switch (message.type) {
      case 'hello_ack':
        if (this.stage !== 'authenticating') {
          this.terminate('protocolError')
          return
        }
        this.stage = 'subscribing'
        this.emit({ kind: 'connectionStateChanged', connection: 'subscribing' })
        this.socket?.send(
          envelope('subscribe', this.payload.sessionId, '', '', 0, {}),
        )
        return
      case 'subscribed':
        if (this.stage !== 'subscribing') {
          this.terminate('protocolError')
          return
        }
        // 已订阅，等待 snapshot。
        return
      case 'snapshot':
        if (this.stage !== 'subscribing') {
          this.terminate('protocolError')
          return
        }
        this.stage = 'ready'
        this.emit({ kind: 'connectionStateChanged', connection: 'connected' })
        this.emit({ kind: 'snapshot', snapshot: message.payload.snapshot })
        return
      case 'event':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        this.emit({ kind: 'event', event: message.payload.event })
        return
      case 'command_result':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        if (message.commandId !== this.pendingCommandId) {
          this.terminate('protocolError')
          return
        }
        this.pendingCommandId = null
        this.emit({
          kind: 'commandResult',
          commandId: message.commandId,
          requestId: message.requestId,
          accepted: message.payload.accepted,
          code: message.payload.code,
          message: message.payload.message,
        })
        return
      case 'error':
        if (message.payload.code === 'unauthorized') {
          this.terminate('pairingInvalid')
          return
        }
        // 其他服务端错误：协议会话仍在，但按协议错误终止本连接最保守。
        this.terminate('protocolError')
        return
      case 'unsubscribed':
        if (this.stage !== 'ready') {
          this.terminate('protocolError')
          return
        }
        return
    }
  }
}
