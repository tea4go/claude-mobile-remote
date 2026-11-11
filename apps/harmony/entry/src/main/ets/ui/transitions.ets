import { curves } from '@kit.ArkUI'

// Explicit animation settings also animate asynchronous/notification-driven navigation.
// Critically damped opacity only: no slide, scale or layout animation on page changes.
export function pageFade(): TransitionEffect {
  return TransitionEffect.OPACITY.animation({ curve: curves.springMotion(0.25, 1) })
}

// Page navigation slides horizontally while overlays retain their fade.
export function pageSlide(left: boolean = false): TransitionEffect {
  return TransitionEffect.translate({ x: left ? '-15%' : '15%' }).combine(TransitionEffect.OPACITY).animation({ duration: 220, curve: Curve.EaseOut })
}
