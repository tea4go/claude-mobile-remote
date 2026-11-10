import { vibrator } from '@kit.SensorServiceKit'

// One call on successful selection creation. Missing hardware/permission or
// system preferences must never interfere with the selection or clipboard.
export function selectionHaptic(): void {
  try {
    void vibrator.startVibration({ type: 'preset', effectId: vibrator.HapticFeedback.EFFECT_SOFT, count: 1 },
      { usage: 'touch' }).catch(() => {})
  } catch (_) {}
}
