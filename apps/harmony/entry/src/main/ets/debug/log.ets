import type { SessionDiagnostic } from '../protocol/types'
export type LogCode = 'network_lost' | 'network_monitor_failed' | 'target' | 'connect' | 'socket_open' | 'socket_connect_result' | 'socket_open_error' | 'socket_event_error' | 'hello_send_result' | 'hello_send_error' | 'hello_response' | 'hello_response_error' | 'authenticated' | 'subscribed' | 'ready' | 'closed' | 'socket_error' | 'protocol_error' | 'pairing_invalid' | 'connect_timeout' | 'auth_timeout' | 'subscribe_timeout' | 'snapshot_timeout' | 'ping' | 'ping_send_result' | 'ping_send_error' | 'pong' | 'heartbeat_timeout' | 'legacy' | 'command' | 'command_accepted' | 'command_rejected' | 'command_unknown' | 'reconnect' | 'background' | 'host'
export interface LogEntry { time: number; code: LogCode; detail: string }
export class DebugLog {
  private nextConnectionId: number = 0
  enabled: boolean = false
  private entries: Array<LogEntry> = []
  private listeners: Array<() => void> = []
  subscribe(listener: () => void): () => void { this.listeners.push(listener); return () => { this.listeners = this.listeners.filter((item: () => void) => item !== listener) } }
  private emit(): void { this.listeners.forEach((item: () => void) => item()) }
  setEnabled(value: boolean): void { this.enabled = value; this.emit() }
  clear(): void { this.entries = []; this.emit() }
  list(): Array<LogEntry> { return this.entries.slice().reverse() }
  text(): string { return this.list().map((item: LogEntry) => `${new Date(item.time).toISOString()} ${item.code} ${item.detail}`).join('\n') }
  record(code: LogCode, value: number = 0): void {
    const detail = connectionMetric(code, value)
    this.entries.push({ time: Date.now(), code, detail }); if (this.entries.length > 200) this.entries.shift(); this.emit()
  }
  beginConnection(): number { return ++this.nextConnectionId }
  connection(id: number, raw: string, stage: string, elapsedMs: number, code: LogCode, value: number, detail: string): void {
    const path = /^(?:wss?):\/\/[^/?#]+(\/ws|\/session\/v1)(?:[?#]|$)/.exec(raw)?.[1] ?? ''
    const route = path === '/session/v1' ? '7330 会话入口' : '会话直连'
    const metric = connectionMetric(code, value)
    this.entries.push({ time: Date.now(), code,
      detail: `连接 #${id} ${this.target(raw)}${path} | ${route} | 阶段=${stage} | 耗时=${elapsedMs}ms${metric.length > 0 ? ' | ' + metric : ''}${detail.length > 0 ? ' | ' + detail : ''}` })
    if (this.entries.length > 200) this.entries.shift()
    this.emit()
  }
  target(raw: string): string {
    const match = /^(wss?):\/\/([a-zA-Z0-9.\-]+|\[[a-fA-F0-9:]+\])(?::([0-9]+))?(?:\/|$)/.exec(raw)
    return match === null ? '目标地址不可显示' : `${match[1]}://${match[2]}:${match[3] ?? (match[1] === 'wss' ? '443' : '80')}`
  }
  host(diagnostic: SessionDiagnostic): void {
    // Only the decoder's closed enum and bounded numeric fields are serialized.
    const detail = `${diagnostic.code} http=${diagnostic.httpStatus ?? 0} attempt=${diagnostic.attempt ?? 0}/${diagnostic.maxAttempts ?? 0} delayMs=${diagnostic.retryDelayMs ?? 0}`
    this.entries.push({ time: Date.now(), code: 'host', detail }); if (this.entries.length > 200) this.entries.shift(); this.emit()
  }
}
export const debugLog: DebugLog = new DebugLog()
function connectionMetric(code: LogCode, value: number): string {
  switch (code) {
    case 'socket_connect_result': return value === 1 ? '发起建连=成功' : value === 0 ? '发起建连=失败' : `系统错误码=${value}`
    case 'socket_open_error':
    case 'socket_event_error':
    case 'hello_send_error':
    case 'hello_response_error':
    case 'ping_send_error': return value !== 0 ? `系统错误码=${value}` : ''
    case 'hello_send_result':
    case 'ping_send_result': return `发送回调=${value === 1 ? '成功' : '失败'}`
    case 'hello_response': return `帧类型=${value === 1 ? '文本' : '二进制'}`
    case 'ping':
    case 'heartbeat_timeout':
    case 'connect_timeout':
    case 'auth_timeout':
    case 'subscribe_timeout':
    case 'snapshot_timeout': return `等待上限=${value}ms`
    case 'pong': return `往返耗时=${value}ms`
    case 'closed': return value === 0 ? '' : `关闭码=${value}`
    case 'legacy': return `协议版本=${value}`
    default: return ''
  }
}
export function logDescription(code: LogCode): string {
  switch (code) {
    case 'connect': return '开始连接 WebSocket'
    case 'socket_open': return '传输已建立，开始配对认证'
    case 'socket_connect_result': return '建连请求结果'
    case 'socket_open_error': return '打开回调报错'
    case 'socket_event_error': return '系统 WebSocket 报错'
    case 'hello_send_result': return '认证首帧发送结果'
    case 'hello_send_error': return '认证首帧发送异常'
    case 'hello_response': return '等待认证时收到帧'
    case 'hello_response_error': return '等待认证时接收异常'
    case 'authenticated': return '认证成功，等待订阅'
    case 'subscribed': return '订阅成功，等待状态快照'
    case 'ready': return '快照校验通过，连接就绪'
    case 'connect_timeout': return '连接超时（10秒），检查网络/VPN与目标地址'
    case 'auth_timeout': return '认证超时（8秒）'
    case 'subscribe_timeout': return '订阅超时（8秒）'
    case 'snapshot_timeout': return '快照同步超时（10秒）'
    case 'ping': return '发起应用层心跳'
    case 'ping_send_result': return '心跳发送结果'
    case 'ping_send_error': return '心跳发送异常'
    case 'pong': return '收到心跳响应'
    case 'heartbeat_timeout': return '等待心跳响应超时'
    case 'legacy': return '降级旧协议，无应用层心跳支持'
    case 'command_unknown': return '操作结果未知，请在电脑核对'
    case 'closed': return 'WebSocket 已关闭'
    case 'socket_error': return '连接已中断；未确认操作请在电脑核对'
    case 'protocol_error': return '协议校验失败，连接已关闭'
    case 'pairing_invalid': return '配对认证失效，请重新扫码'
    case 'network_lost': return '系统报告网络丢失，主动断开连接'
    case 'network_monitor_failed': return '系统网络监听不可用，仍通过应用层心跳检测链路'
    case 'command': return '提交会话操作（不记录内容）'
    case 'command_accepted': return '宿主已确认接收操作'
    case 'command_rejected': return '宿主拒绝操作，检查当前会话状态'
    case 'reconnect': return '开始重新连接，不自动重发未确认操作'
    case 'background': return '应用进入后台，释放前台连接'
    case 'host': return '宿主会话诊断'
    case 'target': return '连接目标（不含路径与凭证）'
    default: return code
  }
}
