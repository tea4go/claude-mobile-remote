import type { DomainEvent } from '../protocol/types'

export function eventNotification(event: DomainEvent): string {
  if (event.kind === 'action_required') return event.pendingAction?.kind === 'ask_user_question' ? '会话等待你的回答' : '会话等待你的批准'
  if (event.kind === 'turn_completed') return event.turnStatus === 'cancelled' ? '本轮已取消' : '本轮已完成'
  if (event.kind === 'turn_failed' || event.kind === 'session_failed') return '会话发生错误'
  if (event.kind === 'session_stopped') return '会话已停止'
  return ''
}
export class NotificationGate {
  private sequences: Map<string, number> = new Map()
  seed(id: string, seq: number): void { this.sequences.set(id, seq) }
  accept(event: DomainEvent, enabled: boolean, foreground: boolean, currentId: string): string {
    const previous = this.sequences.get(event.sessionId) ?? -1
    if (event.seq <= previous) return ''
    this.sequences.set(event.sessionId, event.seq)
    if (!enabled || (foreground && currentId === event.sessionId)) return ''
    return eventNotification(event)
  }
  remove(id: string): void { this.sequences.delete(id) }
}
