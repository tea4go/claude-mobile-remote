import type { SessionDiagnostic } from '../protocol/types'
import { safeNetworkTarget } from '../protocol/safeTarget'

// Deliberately no API accepting arbitrary console arguments, errors or payloads.
export const actionText = {
  online: '系统网络状态 online（不代表 PC 可达）', offline: '系统网络状态 offline，现有 WebSocket 断开事件可能延迟',
  foreground: '页面进入前台，检查连接状态', background: '页面进入后台，系统可能暂停 JavaScript 和网络事件',
  enabled: '已开启调试记录', scan: '打开扫码', scan_failed: '扫码失败',
  pairing: '开始配对验证', cancelled: '取消配对', saved: '配对保存成功', save_failed: '配对保存失败',
  send_text: '发送文本', send_attachments: '发送附件', resolve_action: '处理待办',
  approve_action: '处理审批', answer_action: '回答问题', cancel_turn: '停止当前任务',
  cancel_queued: '取消排队输入', set_permission_mode: '设置权限模式',
  create_session: '新建会话', stop_session: '终止会话',
  command: '发送操作', accepted: 'PC 已受理操作', rejected: 'PC 拒绝操作',
  not_ready: '连接未就绪，未发送', busy: '已有操作等待受理，未发送',
  javascript_error: 'JavaScript 异常（原始内容未采集）',
  unhandled_rejection: '未处理的异步异常（原始内容未采集）',
} as const
export interface ConnectionDetails {
  code: string; stage: string; connectionId?: number; attempt?: number; protocolVersion?: number
  phaseElapsedMs?: number; timeoutMs?: number; readyState?: number; closeCode?: number; wasClean?: boolean
}
export interface DebugEntry extends Partial<ConnectionDetails> { id: number; time: number; category: 'action' | 'network' | 'error' | 'host'; level?: 'info' | 'warn' | 'error'; text: string; target?: string; elapsedMs?: number }
let snapshot: { enabled: boolean; entries: DebugEntry[] } = { enabled: true, entries: [] }
let sequence = 0
const listeners = new Set<() => void>()
function publish() { listeners.forEach((listener) => listener()) }
function append(entry: Omit<DebugEntry, 'id' | 'time'>) {
  if (!snapshot.enabled) return
  snapshot = { ...snapshot, entries: [...snapshot.entries.slice(-199), { ...entry, id: ++sequence, time: Date.now() }] }
  publish()
}
export const debugLog = {
  getSnapshot: () => snapshot,
  subscribe: (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener) } },
  setEnabled: (enabled: boolean) => { snapshot = { ...snapshot, enabled }; publish(); if (enabled) recordAction('enabled') },
  clear: () => { snapshot = { ...snapshot, entries: [] }; publish() },
  exportText: () => JSON.stringify({ version: 1, entries: snapshot.entries }, null, 2),
}
export function recordAction(code: keyof typeof actionText) {
  // Runtime allowlist protects against accidental future payload casts.
  if (!Object.hasOwn(actionText, code)) return
  append({ category: code === 'javascript_error' || code === 'unhandled_rejection' ? 'error' : 'action', text: actionText[code] })
}
export function recordConnection(text: string, target: string, elapsedMs: number, details?: ConnectionDetails) {
  const code = details?.code ?? ''
  const level = /timeout|error|invalid/.test(code) ? 'error' : /closed|retry|cancelled|offline|unavailable/.test(code) ? 'warn' : 'info'
  append({ category: 'network', level, text, target: safeNetworkTarget(target), elapsedMs, ...details })
}
export function observeDebugErrors(): () => void {
  const onError = () => recordAction('javascript_error')
  const onRejection = () => recordAction('unhandled_rejection')
  const onOnline = () => recordAction('online')
  const onOffline = () => recordAction('offline')
  const onVisibility = () => recordAction(document.visibilityState === 'visible' ? 'foreground' : 'background')
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('error', onError)
  window.addEventListener('unhandledrejection', onRejection)
  return () => { window.removeEventListener('online', onOnline); window.removeEventListener('offline', onOffline); document.removeEventListener('visibilitychange', onVisibility); window.removeEventListener('error', onError); window.removeEventListener('unhandledrejection', onRejection) }
}

const hostText: Record<SessionDiagnostic['code'], string> = {
 api_retry: 'Claude API 请求失败，SDK 正在重试', query_failed: '宿主发送 Claude 查询失败', receive_failed: 'SDK 接收 Claude 消息失败', response_incomplete: '响应流提前结束，未收到最终结果', assistant_error: 'Claude assistant 返回错误', result_error: 'Claude 最终结果报告失败', turn_failed: '宿主报告当前任务失败', tool_failed: '工具执行返回失败',
}
export function recordHostDiagnostic(d: SessionDiagnostic, context?: { target: string; connectionId: number }) {
 if (!Object.hasOwn(hostText, d.code)) return
 const facts: string[] = []
 if (Number.isInteger(d.httpStatus) && d.httpStatus! >= 100 && d.httpStatus! <= 599) facts.push(`HTTP ${d.httpStatus}`)
 if (Number.isInteger(d.attempt) && d.attempt! >= 0 && d.attempt! <= 1000000) facts.push(`重试 ${d.attempt}${Number.isInteger(d.maxAttempts) && d.maxAttempts! > 0 && d.maxAttempts! <= 1000000 ? '/' + d.maxAttempts : ''}`)
 if (Number.isInteger(d.retryDelayMs) && d.retryDelayMs! >= 0 && d.retryDelayMs! <= 86400000) facts.push(`等待 ${d.retryDelayMs}ms`)
 append({ ...(context ? { target: safeNetworkTarget(context.target), connectionId: context.connectionId } : {}), category: 'host', level: d.code === 'api_retry' ? 'warn' : 'error', code: d.code, text: [hostText[d.code], ...facts].join(' · ') })
}
