import type { common, Want } from '@kit.AbilityKit'

class NativeLifecycle {
  context: common.UIAbilityContext | null = null
  onBack: (() => boolean) | null = null
  detailBack: (() => boolean) | null = null
  foreground: boolean = true
  pendingSessionId: string = ''
  private listeners: Array<() => void> = []
  attach(context: common.UIAbilityContext, want: Want): void { this.context = context; this.open(want) }
  open(want: Want): void {
    const id = want.parameters?.cmrSessionId
    if (typeof id === 'string' && id.length > 0 && id.length <= 256) this.pendingSessionId = id
    this.emit()
  }
  active(value: boolean): void { this.foreground = value; this.emit() }
  subscribe(listener: () => void): () => void {
    this.listeners.push(listener)
    return () => { this.listeners = this.listeners.filter((item) => item !== listener) }
  }
  private emit(): void { this.listeners.forEach((listener) => listener()) }
}
export const nativeLifecycle = new NativeLifecycle()
