import type { SessionRecord } from './records'

export interface ProjectGroup { key: string; cwd: string; title: string; sessions: Array<SessionRecord> }
export interface PCGroup { key: string; label: string; connected: number; sessions: Array<SessionRecord>; projects: Array<ProjectGroup> }
export function sessionPCKey(record: SessionRecord): string { return record.pcId ?? record.wsUrl }
export function sessionPCName(record: SessionRecord): string { return record.pcName ?? record.wsUrl.replace(/^wss?:\/\//, '').split('/')[0] }
export function groupSessions(records: Array<SessionRecord>): Array<PCGroup> {
  const pcs: Array<PCGroup> = []
  const pcMap: Map<string, PCGroup> = new Map()
  const projectMaps: Map<string, Map<string, ProjectGroup>> = new Map()
  for (const record of records) {
    const key = sessionPCKey(record)
    let pc = pcMap.get(key)
    if (pc === undefined) {
      pc = { key, label: sessionPCName(record), connected: 0, sessions: [], projects: [] }
      pcMap.set(key, pc); projectMaps.set(key, new Map()); pcs.push(pc)
    }
    pc.sessions.push(record)
    if (record.status === 'connected') pc.connected += 1
    const cwd = record.projectCwd ?? ''
    const projects = projectMaps.get(key) as Map<string, ProjectGroup>
    let project = projects.get(cwd)
    if (project === undefined) {
      const parts = cwd.replace(/\\/g, '/').split('/').filter((part: string) => part.length > 0)
      project = { key: JSON.stringify([key, cwd]), cwd, title: parts.length > 0 ? parts[parts.length - 1] : '工作目录未知', sessions: [] }
      projects.set(cwd, project); pc.projects.push(project)
    }
    project.sessions.push(record)
  }
  return pcs
}

// Bound the work to the displayed prefix, even for very long streamed messages.
export function sessionPreview(text: string): string {
  let result = ''
  let whitespace = false
  for (let index = 0; index < text.length && index < 4096 && result.length < 100; index += 1) {
    const char = text.charAt(index)
    if (/\s/.test(char)) { if (result.length > 0) whitespace = true; continue }
    if (whitespace && result.length < 99) result += ' '
    whitespace = false
    if (result.length < 100) result += char
  }
  return result
}

export interface PCProfile { key: string; name: string; ip: string }

export function replaceComputerHost(wsUrl: string, ip: string): string {
  return wsUrl.replace(/^(wss?:\/\/)(\[[^\]]+\]|[^/:?#]+)/, (_match: string, scheme: string) => scheme + ip)
}

export function validComputerIP(ip: string): boolean {
  if (ip.startsWith('[') !== ip.endsWith(']')) return false
  if (/^\[?[0-9a-f:]+\]?$/i.test(ip) && ip.includes(':')) {
    const value = ip.replace(/^\[|\]$/g, '')
    const parts = value.split('::')
    if (parts.length > 2) return false
    let count = 0
    for (const side of parts) {
      if (side.length === 0) continue
      for (const segment of side.split(':')) {
        if (!/^[0-9a-f]{1,4}$/i.test(segment)) return false
        count++
      }
    }
    return parts.length === 2 ? count < 8 : count === 8
  }
  return /^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$/.test(ip) && ip.split('.').every((part: string) => Number(part) < 256)
}

export function maskPairToken(token: string): string {
  return token.length > 9 ? token.slice(0, 5) + '***' + token.slice(-4) : '***'
}
