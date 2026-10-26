import { recordConnection, type ConnectionDetails } from '../debug/log'
import { safeNetworkTarget } from './safeTarget'
export { safeNetworkTarget } from './safeTarget'
// Only allowlisted values reach diagnostics. Never accept raw frames/errors/QR text.
export type NetworkStage = 'connecting' | 'authenticating' | 'subscribing' | 'connected' | 'closed'
export type NetworkCode = 'heartbeat_sent' | 'heartbeat_received' | 'heartbeat_timeout' | 'heartbeat_unavailable' | 'network_offline' | 'started' | 'socket_open' | 'authenticated' | 'ready' | 'connect_timeout' | 'auth_timeout' | 'snapshot_timeout' | 'attempt_timeout' | 'command_timeout' | 'socket_error' | 'socket_closed' | 'pairing_invalid' | 'protocol_error' | 'cancelled' | 'manual_retry' | 'protocol_retry' | 'constructor_error'
export interface NetworkDiagnostic {
  time: number
  sessionId: string
  target: string
  stage: NetworkStage
  code: NetworkCode
  elapsedMs: number
}
let records: NetworkDiagnostic[] = []
const listeners = new Set<() => void>()
export const networkDiagnostics = {
  getSnapshot: () => records,
  subscribe: (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener) } },
  clear: () => { records = []; listeners.forEach((listener) => listener()) },
}
export function recordNetworkDiagnostic(sessionId: string, url: string, stage: NetworkStage, code: NetworkCode, startedAt: number, details: Omit<ConnectionDetails, 'code' | 'stage'> = {}): void {
  const time = Date.now()
  records = [...records.slice(-99), { time, sessionId, target: safeNetworkTarget(url), stage, code, elapsedMs: Math.max(0, time - startedAt) }]
  const closeDescription = details.closeCode === undefined ? '' : `；${closeCodeText(details.closeCode)}`
  recordConnection(networkDebugText[code] + closeDescription, url, Math.max(0, time - startedAt), { ...details, code, stage })
  listeners.forEach((listener) => listener())
}
export const networkCodeText: Record<NetworkCode, string> = {
  heartbeat_sent: '正在验证 PC 连接', heartbeat_received: 'PC 心跳响应正常', heartbeat_timeout: 'PC 心跳响应超时，连接已断开', heartbeat_unavailable: '旧版 PC 不支持应用层心跳检测', network_offline: '系统报告离线，连接已断开',
  manual_retry: '正在手动重连', protocol_retry: '正在协商兼容协议并重连', constructor_error: '无法创建 WebSocket 连接',
  started: '正在连接 PC', socket_open: '正在验证配对', authenticated: '正在同步会话', ready: '连接验证成功',
  connect_timeout: '连接 PC 超时：请检查地址、本地网络权限、VPN、主副网互通及 PC 防火墙',
  auth_timeout: 'PC 未及时完成认证，请检查服务状态', snapshot_timeout: 'PC 未及时返回会话状态',
  attempt_timeout: '配对验证超时，请检查网络后重试', command_timeout: '等待 PC 受理超时，结果未知，请核对后再操作',
  socket_error: '连接失败：请检查本地网络权限、PC 地址、主副网路由和防火墙；系统未提供具体原因',
  socket_closed: '与 PC 的连接已断开', pairing_invalid: '配对凭证无效或过期，请重新扫码',
  protocol_error: 'PC 返回的协议或会话状态无效', cancelled: '已取消连接',
}

const networkDebugText: Record<NetworkCode, string> = {
  heartbeat_sent: '发送应用层 ping，等待 PC pong（5000 ms），不提交会话操作',
  heartbeat_received: '收到匹配 Session 的 pong，双向链路探测成功；10 秒后再次探测',
  heartbeat_timeout: '应用层 pong 等待超时（5000 ms），主动断开；待受理操作结果未知，需手动重连',
  heartbeat_unavailable: '已降级到旧版 PC 协议，无应用层心跳；静默断网仍可能依赖系统事件或操作超时发现',
  network_offline: '系统报告网络离线，主动断开当前连接；保留配对，等待手动重连',
  started: '创建 WebSocket，等待传输层连接建立（建连期限 8000 ms，总期限 15000 ms）',
  socket_open: 'WebSocket 已打开，发送 hello 并等待鉴权响应（4000 ms）',
  authenticated: 'hello_ack 鉴权成功，发送 subscribe 并等待首份 snapshot（4000 ms）',
  ready: '已收到有效 snapshot，连接就绪，可以发送操作',
  connect_timeout: '建连期限已到，未收到 open；关闭本次尝试。检查目标可达性、路由、权限与防火墙',
  auth_timeout: '鉴权期限已到，未收到有效 hello_ack；连接终止',
  snapshot_timeout: '订阅期限已到，未收到有效 snapshot；连接终止',
  attempt_timeout: '本次尝试总期限已到（含协议降级），连接终止',
  command_timeout: 'PC 操作受理响应超时，连接终止；执行结果未知，禁止直接重复提交',
  socket_error: 'WebSocket error 事件，连接终止；WebView 未提供底层原因，不能据此判断 DNS/TCP/TLS/权限故障',
  constructor_error: 'WebSocket 构造失败，尚未建立连接；异常原文未采集',
  socket_closed: '收到 WebSocket close 事件；不会自动重试，需手动重连或重新配对',
  pairing_invalid: '服务端鉴权拒绝，配对无效或过期；需重新扫码',
  protocol_error: '协议解析、版本或会话状态校验失败，连接终止',
  cancelled: '客户端主动关闭连接，停止此连接的事件处理',
  manual_retry: '用户触发重连，关闭旧连接并开始新的连接尝试',
  protocol_retry: '服务端要求兼容协议，降级版本后重新连接；沿用原尝试总期限',
}
function closeCodeText(code: number): string {
  const meanings: Record<number, string> = { 1000: '正常关闭', 1001: '端点离开', 1002: '协议错误', 1003: '不支持的数据', 1005: '未提供关闭码', 1006: '异常中断，未完成关闭握手', 1007: '数据格式无效', 1008: '策略拒绝', 1009: '消息过大', 1011: '服务端内部错误', 1012: '服务重启', 1013: '服务暂不可用', 1015: 'TLS 握手失败' }
  return meanings[code] ?? '服务端或应用自定义关闭码'
}
