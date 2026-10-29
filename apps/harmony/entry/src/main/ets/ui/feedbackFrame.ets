import { FrameCallback } from '@kit.ArkUI'

// Let the changed button/loading state render before starting a native page or
// constructing the full demo. This is a frame boundary, not an arbitrary delay.
export class FeedbackFrame extends FrameCallback {
  private action: (() => void) | null
  constructor(action: () => void) { super(); this.action = action }
  onFrame(_time: number): void {}
  onIdle(_remaining: number): void {
    const action = this.action
    this.action = null
    action?.()
  }
}
