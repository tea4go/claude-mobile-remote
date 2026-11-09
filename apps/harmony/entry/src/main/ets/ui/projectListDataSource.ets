import type { PCGroup, ProjectGroup } from '../session/projectGroups'
import type { SessionRecord } from '../session/records'

export interface ProjectListRow {
  kind: 'pc' | 'project' | 'session'
  key: string
  pc?: PCGroup
  project?: ProjectGroup
  session?: SessionRecord
}
export function projectListRows(groups: Array<PCGroup>, selectedPC: string, collapsed: Array<string>): Array<ProjectListRow> {
  const rows: Array<ProjectListRow> = []
  const closed: Set<string> = new Set(collapsed)
  for (const pc of groups) {
    if (selectedPC !== 'all' && selectedPC !== pc.key) continue
    rows.push({ kind: 'pc', key: 'pc:' + pc.key, pc })
    for (const project of pc.projects) {
      rows.push({ kind: 'project', key: 'project:' + project.key, pc, project })
      if (!closed.has(project.key)) {
        for (const session of project.sessions) rows.push({ kind: 'session', key: 'session:' + session.sessionId, session })
      }
    }
  }
  return rows
}
export class ProjectListDataSource implements IDataSource {
  private rows: Array<ProjectListRow> = []
  private listeners: Array<DataChangeListener> = []
  totalCount(): number { return this.rows.length }
  getData(index: number): ProjectListRow { return this.rows[index] }
  registerDataChangeListener(listener: DataChangeListener): void { if (this.listeners.indexOf(listener) < 0) this.listeners.push(listener) }
  unregisterDataChangeListener(listener: DataChangeListener): void { this.listeners = this.listeners.filter((item: DataChangeListener) => item !== listener) }
  replace(rows: Array<ProjectListRow>): void { this.rows = rows; this.listeners.forEach((listener: DataChangeListener) => listener.onDataReloaded()) }
}
