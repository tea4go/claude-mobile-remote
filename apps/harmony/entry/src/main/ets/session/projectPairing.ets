import type { PairingPayload } from '../protocol/types'

export function projectMatches(targetPC: string, targetCwd: string, pairing: PairingPayload, projectCwd?: string): boolean {
  return targetPC.length > 0 && targetCwd.length > 0 && pairing.pcId === targetPC && projectCwd === targetCwd
}
export function createdPairingAddress(source: string, target: string): string | null {
  const pattern = /^(wss?):\/\/(\[[0-9a-fA-F:]+\]|[a-zA-Z0-9.\-]+)(?::([0-9]+))?([/?][^#]*)?$/
  const origin = pattern.exec(source)
  const child = pattern.exec(target)
  if (origin === null || child === null || origin[1] !== child[1]) return null
  const suffix = child[4] ?? '/'
  return `${child[1]}://${origin[2]}${child[3] ? ':' + child[3] : ''}${suffix.startsWith('?') ? '/' : ''}${suffix}`
}
