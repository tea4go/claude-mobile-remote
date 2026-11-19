type Point = { x: number; y: number }

export function debugButtonPosition(
  viewport: { left: number; top: number; width: number; height: number },
  buttonWidth: number,
  buttonHeight: number,
  safe: { right: number; bottom: number },
  preferred: Point | null,
  inputTop?: number,
  actionsTop?: number,
): Point {
  const margin = 12
  const minX = viewport.left + margin
  const minY = viewport.top + margin
  const right = viewport.left + viewport.width - safe.right - margin
  const bottom = viewport.top + viewport.height - safe.bottom - margin
  const boundary = inputTop !== undefined && inputTop > viewport.top && inputTop < bottom ? inputTop - 8 : bottom
  const maxX = right - buttonWidth
  const actionsBoundary = preferred === null && actionsTop !== undefined &&
    actionsTop >= minY + buttonHeight + 8 && actionsTop < boundary ? actionsTop - 8 : boundary
  const maxY = actionsBoundary - buttonHeight
  return {
    x: Math.max(minX, Math.min(maxX, preferred?.x ?? maxX)),
    y: Math.max(minY, Math.min(maxY, preferred?.y ?? maxY)),
  }
}
