import { Capacitor, registerPlugin, type PluginListenerHandle } from '@capacitor/core'
import type { DomainEvent } from '../protocol/types'

export interface ExperiencePort {
  syncPushSessions?(options: { sessions: { sessionId: string; pairToken: string; expiresAt: string }[]; viewedSession: string }): Promise<void>
  recognize(): Promise<{ text: string }>
  stopSpeech(): Promise<void>
  cancelSpeech(): Promise<void>
  notificationStatus(): Promise<{ enabled: boolean; remoteConfigured?: boolean; remoteReady?: boolean; pushError?: string }>
  setNotifications(options: { enabled: boolean }): Promise<{ enabled: boolean }>
  notify(options: { sessionId: string; seq: number; kind: ReminderKind }): Promise<void>
  clearNotifications(options: { sessionId: string }): Promise<void>
  addListener(event: 'notificationOpened', callback: (value: { sessionId: string }) => void): Promise<PluginListenerHandle>
}
export type ReminderKind = 'approval' | 'question' | 'completed' | 'failed' | 'stopped'
const plugin = registerPlugin<ExperiencePort>('SessionExperience')
export const iosExperience = Capacitor.getPlatform() === 'ios' ? plugin : null

export function reminderKind(event: DomainEvent): ReminderKind | null {
  if (event.kind === 'action_required') return event.pendingAction?.kind === 'tool_approval' ? 'approval' : 'question'
  if (event.kind === 'turn_completed') return event.turnStatus === 'cancelled' ? null : 'completed'
  if (event.kind === 'turn_failed' || event.kind === 'session_failed') return 'failed'
  if (event.kind === 'session_stopped') return 'stopped'
  return null
}

// 每次连接的 Snapshot 建立水位；重连与重复 seq 均不补发提醒。
export class ReminderPolicy {
  private watermarks = new Map<string, number>()
  snapshot(sessionId: string, seq: number): void {
    this.watermarks.set(sessionId, Math.max(seq, this.watermarks.get(sessionId) ?? 0))
  }
  event(event: DomainEvent, viewedSession: string | null): ReminderKind | null {
    const previous = this.watermarks.get(event.sessionId) ?? 0
    if (event.seq <= previous) return null
    this.watermarks.set(event.sessionId, event.seq)
    return event.sessionId === viewedSession ? null : reminderKind(event)
  }
  remove(sessionId: string): void { this.watermarks.delete(sessionId) }
}
