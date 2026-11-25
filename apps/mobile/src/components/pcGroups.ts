import type { SessionRecord } from '../session/store'
import type { SessionListItem } from './ProjectSessionList'

export type PCProbeResults = Record<string, { ip: string; ok: boolean }>

export function pcIP(pc: PCGroup): string {
  for (const record of pc.sessions) { try { return new URL(record.wsUrl).hostname } catch { /* expired */ } }
  return ''
}

export function pcConnectionStatus(pc: PCGroup<SessionListItem>, probes: PCProbeResults): 'connected' | 'connecting' | 'disconnected' {
  const probe = probes[pc.key]
  if (probe && probe.ip === pcIP(pc)) return probe.ok ? 'connected' : 'disconnected'
  const validSessions = pc.sessions.filter(session => session.status !== 'needsPairing')
  if (validSessions.some(session => session.runtimeState?.connection === 'connected')) return 'connected'
  if (validSessions.some(session => ['connecting', 'authenticating', 'subscribing'].includes(session.runtimeState?.connection ?? ''))) return 'connecting'
  return 'disconnected'
}

export interface PCGroup<T extends SessionRecord = SessionRecord> {
  key: string
  label: string
  legacy: boolean
  sessions: T[]
}

// Older clients parsed UTF-8 Base64 bytes as Latin-1. Repair only reversible names.
export function displayPCName(name: string): string {
  if ([...name].some((character) => character.charCodeAt(0) > 255)) return name
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(Uint8Array.from(name, (character) => character.charCodeAt(0)))
  } catch { return name }
}

export function groupPCs<T extends SessionRecord>(sessions: T[]): PCGroup<T>[] {
  const groups = new Map<string, PCGroup<T>>()
  const nameTimes = new Map<string, number>()
  for (const session of sessions) {
    let key: string
    let label: string
    const legacy = session.pcId === undefined
    if (!legacy) {
      key = `pc:${session.pcId}`
      label = (session.pcName ? displayPCName(session.pcName) : undefined) ?? `PC ${session.pcId!.slice(0, 8).toUpperCase()}`
    } else {
      let address = ''
      try { address = new URL(session.wsUrl).host } catch { /* 无凭证的旧记录没有地址 */ }
      key = address === '' ? `unknown:${session.sessionId}` : `address:${address}`
      label = (session.pcName ? displayPCName(session.pcName) : undefined) ?? (address === '' ? '未知 PC' : address)
    }
    key = session.localPCKey ?? key
    let group = groups.get(key)
    if (group === undefined) {
      group = { key, label, legacy, sessions: [] }
      groups.set(key, group)
    }
    group.sessions.push(session)
    if (session.pcName && (nameTimes.get(key) ?? -Infinity) < (session.lastConnectedAt ?? 0)) {
      group.label = displayPCName(session.pcName)
      nameTimes.set(key, session.lastConnectedAt ?? 0)
    }
  }
  return [...groups.values()]
}
