import type { SessionListItem } from './ProjectSessionList'

export interface ProjectGroup { cwd: string | null; title: string; ids: string[] }

function activity(session: SessionListItem): number {
  const value = session.runtimeState?.lastActivityAt ?? session.runtimeState?.createdAt
  const parsed = value ? Date.parse(value) : NaN
  return Number.isFinite(parsed) ? parsed : 0
}

// Group by the received cwd string, without inferring machine identity.
export function projectGroups(sessions: SessionListItem[]): ProjectGroup[] {
  const ordered = sessions.map((session, index) => ({ session, index }))
    .sort((a, b) => activity(b.session) - activity(a.session) || a.index - b.index)
  const groups = new Map<string, ProjectGroup>()
  for (const { session } of ordered) {
    const cwd = session.runtimeState?.projectCwd ?? null
    const key = cwd ?? ''
    let group = groups.get(key)
    if (group === undefined) {
      const title = cwd ? cwd.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || cwd : '工作目录未知'
      group = { cwd, title, ids: [] }
      groups.set(key, group)
    }
    group.ids.push(session.sessionId)
  }
  return [...groups.values()]
}
