import type { SessionDiagnostic } from '../protocol/types'
export type LogCode = 'network_lost' | 'network_monitor_failed' | 'target' | 'connect' | 'socket_open' | 'authenticated' | 'subscribed' | 'ready' | 'closed' | 'socket_error' | 'protocol_error' | 'pairing_invalid' | 'connect_timeout' | 'auth_timeout' | 'subscribe_timeout' | 'snapshot_timeout' | 'ping' | 'pong' | 'heartbeat_timeout' | 'legacy' | 'command' | 'command_accepted' | 'command_rejected' | 'command_unknown' | 'reconnect' | 'background' | 'host'
export interface LogEntry { time: number; code: LogCode; detail: string }
export class DebugLog {
  enabled: boolean = true
  private entries: Array<LogEntry> = []
  private listeners: Array<() => void> = []
  subscribe(listener: () => void): () => void { this.listeners.push(listener); return () => { this.listeners = this.listeners.filter((item: () => void) => item !== listener) } }
  private emit(): void { this.listeners.forEach((item: () => void) => item()) }
  setEnabled(value: boolean): void { this.enabled = value; if (!value) this.entries = []; this.emit() }
  clear(): void { this.entries = []; this.emit() }
  list(): Array<LogEntry> { return this.entries.slice().reverse() }
  text(): string { return this.list().map((item: LogEntry) => `${new Date(item.time).toISOString()} ${item.code} ${item.detail}`).join('\n') }
  record(code: LogCode, value: number = 0): void {
    if (!this.enabled) return
    const detail = Number.isFinite(value) && value >= 0 ? `value=${Math.floor(value)}` : ''
    this.entries.push({ time: Date.now(), code, detail }); if (this.entries.length > 200) this.entries.shift(); this.emit()
  }
  connectionTarget(raw: string): void {
    if (!this.enabled) return
    this.entries.push({ time: Date.now(), code: 'target', detail: this.target(raw) })
    if (this.entries.length > 200) this.entries.shift(); this.emit()
  }
  target(raw: string): string {
    const match = /^(wss?):\/\/([a-zA-Z0-9.\-]+|\[[a-fA-F0-9:]+\])(?::([0-9]+))?(?:\/|$)/.exec(raw)
    return match === null ? '目标地址不可显示' : `${match[1]}://${match[2]}:${match[3] ?? (match[1] === 'wss' ? '443' : '80')}`
  }
  host(diagnostic: SessionDiagnostic): void {
    if (!this.enabled) return
    // Only the decoder's closed enum and bounded numeric fields are serialized.
    const detail = `${diagnostic.code} http=${diagnostic.httpStatus ?? 0} attempt=${diagnostic.attempt ?? 0}/${diagnostic.maxAttempts ?? 0} delayMs=${diagnostic.retryDelayMs ?? 0}`
    this.entries.push({ time: Date.now(), code: 'host', detail }); if (this.entries.length > 200) this.entries.shift(); this.emit()
  }
}
export const debugLog: DebugLog = new DebugLog()
export function logDescription(code: LogCode): string {
  switch (code) {
    case 'connect': return '开始连接 WebSocket'
    case 'socket_open': return '传输已建立，开始配对认证'
    case 'authenticated': return '认证成功，等待订阅'
    case 'subscribed': return '订阅成功，等待状态快照'
    case 'ready': return '快照校验通过，连接就绪'
    case 'connect_timeout': return '连接超时（10秒），检查网络/VPN与目标地址'
    case 'auth_timeout': return '认证超时（8秒）'
    case 'subscribe_timeout': return '订阅超时（8秒）'
    case 'snapshot_timeout': return '快照同步超时（10秒）'
    case 'ping': return '发送应用层探活，响应期限5秒'
    case 'pong': return '收到探活响应，value为往返毫秒'
    case 'heartbeat_timeout': return '探活超时，链路不可达；操作不会自动重发'
    case 'legacy': return '降级旧协议，无应用层心跳支持'
    case 'command_unknown': return '操作结果未知，请在电脑核对'
    case 'closed': return 'WebSocket 已关闭，value为关闭码（主动关闭为0）'
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
