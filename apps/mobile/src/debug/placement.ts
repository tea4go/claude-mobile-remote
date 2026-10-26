// Fixed coordinates use the layout viewport; aim at the lower middle of the
// visible viewport, then move above any visible input region and keyboard.
export function debugButtonBottom(layoutHeight: number, viewportBottom: number, inputTops: number[], viewportTop = 0): number {
  const visibleInputs = inputTops.filter((top) => top > viewportTop && top < viewportBottom)
  const preferredBottom = viewportTop + (viewportBottom - viewportTop) * 0.68 + 18
  const boundary = Math.min(preferredBottom, viewportBottom - 8, ...visibleInputs.map((top) => top - 8))
  return Math.max(12, layoutHeight - boundary)
}
