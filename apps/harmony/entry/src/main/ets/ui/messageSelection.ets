import { nativeLifecycle } from '../native/lifecycle'

export interface ActiveMessageSelection {
  close: () => void
  refresh: () => void
  contains: (x: number, y: number) => boolean
}

// Only the selected message owns listeners; cached list rows have no timers or
// ability subscriptions. The coordinates at this boundary are window vp.
class MessageSelectionCoordinator {
  private active: ActiveMessageSelection | null = null
  private unsubscribe: (() => void) | null = null
  private frame: number = -1
  open(value: ActiveMessageSelection): void {
    this.close()
    this.active = value
    this.unsubscribe = nativeLifecycle.subscribe(() => {
      if (!nativeLifecycle.foreground) this.close()
    })
  }
  release(value: ActiveMessageSelection): void {
    if (this.active !== value) return
    this.active = null
    this.unsubscribe?.()
    this.unsubscribe = null
    if (this.frame >= 0) clearTimeout(this.frame)
    this.frame = -1
  }
  close(): boolean {
    const selected = this.active
    if (selected === null) return false
    this.release(selected)
    selected.close()
    return true
  }
  outside(x: number, y: number): void {
    if (this.active !== null && !this.active.contains(x, y)) this.close()
  }
  refresh(): void {
    if (this.active === null || this.frame >= 0) return
    this.frame = setTimeout(() => {
      this.frame = -1
      this.active?.refresh()
    }, 16)
  }
}
export const messageSelection = new MessageSelectionCoordinator()
