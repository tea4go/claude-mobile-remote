import { notificationManager } from '@kit.NotificationKit'
import { common, wantAgent } from '@kit.AbilityKit'
import type { DomainEvent } from '../protocol/types'
import { NotificationGate } from '../session/notifications'

export class LocalNotifications {
  private readonly gate: NotificationGate = new NotificationGate()
  private readonly ids: Map<string, number> = new Map()
  private nextId: number = 1
  private queue: Promise<void> = Promise.resolve()
  enabled: boolean = false
  foreground: boolean = true
  currentId: string = ''
  private readonly context: common.UIAbilityContext
  constructor(context: common.UIAbilityContext) { this.context = context }
  async enable(): Promise<void> {
    await notificationManager.requestEnableNotification(this.context)
    this.enabled = await notificationManager.isNotificationEnabled()
    if (!this.enabled) throw new Error('通知权限未获授权')
  }
  seed(id: string, seq: number): void { this.gate.seed(id, seq) }
  private serial(operation: () => Promise<void>): Promise<void> {
    const result = this.queue.then(operation)
    this.queue = result.catch(() => {})
    return result
  }
  async event(event: DomainEvent): Promise<void> {
    const text = this.gate.accept(event, this.enabled, this.foreground, this.currentId)
    if (text.length === 0) return
    let id = this.ids.get(event.sessionId)
    if (id === undefined) { id = this.nextId++; this.ids.set(event.sessionId, id) }
    const notificationId = id
    return this.serial(async () => {
      const id = notificationId
      if (!this.enabled || this.ids.get(event.sessionId) !== id) return
      const agent = await wantAgent.getWantAgent({
        wants: [{ bundleName: this.context.abilityInfo.bundleName, abilityName: this.context.abilityInfo.name,
          parameters: { cmrSessionId: event.sessionId } }],
        operationType: wantAgent.OperationType.START_ABILITY,
        requestCode: id,
        wantAgentFlags: [wantAgent.WantAgentFlags.UPDATE_PRESENT_FLAG],
      })
      if (!this.enabled || this.ids.get(event.sessionId) !== id || (this.foreground && this.currentId === event.sessionId)) return
      await notificationManager.publish({ id, label: event.sessionId,
        slotType: notificationManager.SlotType.SOCIAL_COMMUNICATION,
        content: { notificationContentType: notificationManager.ContentType.NOTIFICATION_CONTENT_BASIC_TEXT,
          normal: { title: 'Claude Mobile Remote', text, additionalText: '打开应用查看当前状态' } },
        wantAgent: agent, isRemoveAllowed: true, tapDismissed: true,
      })
      if (!this.enabled || this.ids.get(event.sessionId) !== id || (this.foreground && this.currentId === event.sessionId)) await notificationManager.cancel(id, event.sessionId)
    })
  }
  async remove(id: string): Promise<void> {
    this.gate.remove(id)
    await this.dismiss(id)
  }
  async dismiss(id: string): Promise<void> {
    const notificationId = this.ids.get(id)
    // Invalidate before awaiting the platform: even a failed cancel must block late publication.
    this.ids.delete(id)
    if (notificationId !== undefined) await notificationManager.cancel(notificationId, id)
  }
  async clear(): Promise<void> {
    this.ids.clear()
    await this.serial(async () => { await notificationManager.cancelAll() })
  }
}
