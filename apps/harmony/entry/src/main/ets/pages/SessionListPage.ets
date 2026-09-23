import { defaultSessionName, type SessionRecord, type SessionLocalStatus } from '../session/records'
import { SelfCheckPanel } from '../components/SelfCheckPanel'

function statusLabel(status: SessionLocalStatus): string {
  if (status === 'connected') {
    return '已连接'
  }
  if (status === 'disconnected') {
    return '已断开'
  }
  if (status === 'needsPairing') {
    return '需要重新配对'
  }
  return '未连接'
}

function lastConnectedLabel(record: SessionRecord): string {
  const value = record.lastActivityAt ?? record.createdAt
  return value === undefined ? '活动时间未知' : new Date(value).toLocaleString()
}

function cwdOf(record: SessionRecord): string { return record.projectCwd ?? '' }

function groupRecords(records: Array<SessionRecord>): Array<SessionRecord> {
  const ordered: Array<SessionRecord> = []
  const seen: Array<string> = []
  for (let index = 0; index < records.length; index += 1) {
    const key = cwdOf(records[index])
    if (seen.indexOf(key) < 0) seen.push(key)
  }
  for (let group = 0; group < seen.length; group += 1) {
    for (let index = 0; index < records.length; index += 1) {
      if (cwdOf(records[index]) === seen[group]) ordered.push(records[index])
    }
  }
  return ordered
}

function directoryTitle(cwd: string): string {
  if (cwd.length === 0) return '工作目录未知'
  const parts = cwd.replace(/[\\/]+$/, '').split(/[\\/]/)
  return parts[parts.length - 1] || cwd
}

function titleOf(record: SessionRecord): string {
  if (record.customName === true || (record.customName === undefined && record.name !== defaultSessionName(record.sessionId))) return record.name
  if (record.sessionTitle !== undefined) return record.sessionTitle
  if (record.createdAt !== undefined) return `新会话 · ${new Date(record.createdAt).toLocaleString()}`
  return record.name
}

function sourceOf(record: SessionRecord): string {
  if (record.capabilities !== undefined && record.capabilities.indexOf('create_session') >= 0) return 'CLI'
  return 'Remote'
}

function addressDigest(wsUrl: string): string {
  const marker = '://'
  const markerIndex = wsUrl.indexOf(marker)
  if (markerIndex < 0) {
    return '地址无效'
  }
  const hostStart = markerIndex + marker.length
  let slashIndex = wsUrl.indexOf('/', hostStart)
  if (slashIndex < 0) {
    slashIndex = wsUrl.length
  }
  return wsUrl.slice(hostStart, slashIndex)
}

@Component
export struct SessionListPage {
  @Prop sessions: Array<SessionRecord> = []
  @Prop recentlyViewedId: string = ''
  @Prop selfChecks: Array<string> = []
  @Prop selfCheckError: string = ''
  onAdd: () => void = () => {}
  onOpen: (sessionId: string) => void = () => {}
  onRemove: (sessionId: string) => void = () => {}
  onRename: (sessionId: string, name: string) => void = () => {}

  @State private renamingId: string = ''
  @State private draftName: string = ''

  build() {
    Scroll() {
      Column({ space: 16 }) {
        Text('Claude Mobile Remote')
          .fontSize(24)
          .fontWeight(FontWeight.Bold)
          .width('100%')
          .textAlign(TextAlign.Start)

        SelfCheckPanel({
          results: this.selfChecks,
          error: this.selfCheckError,
        })

        Text('会话列表')
          .fontSize(18)
          .fontWeight(FontWeight.Medium)
          .width('100%')

        if (this.sessions.length === 0) {
          Text('还没有已配对的会话，请先新增一个配对。')
            .fontSize(14)
            .opacity(0.7)
            .width('100%')
        } else {
          ForEach(groupRecords(this.sessions), (session: SessionRecord, index: number) => {
            Column({ space: 8 }) {
              if (index === 0 || cwdOf(groupRecords(this.sessions)[index - 1]) !== cwdOf(session)) {
                Text(directoryTitle(cwdOf(session))).fontSize(18).fontWeight(FontWeight.Bold)
                Text(session.projectCwd ?? '旧版服务未提供工作目录').fontSize(12).opacity(0.7)
              }
              if (this.renamingId === session.sessionId) {
                Row({ space: 8 }) {
                  TextInput({
                    text: this.draftName,
                    placeholder: '输入本地名称',
                  })
                    .id(`rename-input-${session.sessionId}`)
                    .layoutWeight(1)
                    .onChange((value: string) => {
                      this.draftName = value
                    })

                  Button('保存')
                    .onClick(() => {
                      this.onRename(session.sessionId, this.draftName)
                      this.renamingId = ''
                      this.draftName = ''
                    })

                  Button('取消')
                    .onClick(() => {
                      this.renamingId = ''
                      this.draftName = ''
                    })
                }
              } else {
                Text(titleOf(session))
                  .fontSize(16)
                  .fontWeight(FontWeight.Medium)
              }

              Text(session.preview ?? '暂无会话内容').fontSize(13).opacity(0.8)

              Text(`${sourceOf(session)} · ID ${session.sessionId.slice(0, 8)} · ${addressDigest(session.wsUrl)}`)
                .fontSize(12)
                .opacity(0.7)

              Text(`${statusLabel(session.status)} · ${session.runtimeStatus ?? '空闲'} · ${lastConnectedLabel(session)}${session.unread ? ' · 未读' : ''}${this.recentlyViewedId === session.sessionId ? ' · 刚刚查看' : ''}`)
                .fontSize(12)
                .opacity(0.7)

              Row({ space: 8 }) {
                Button('进入')
                  .onClick(() => {
                    this.onOpen(session.sessionId)
                  })

                Button('重命名')
                  .onClick(() => {
                    this.renamingId = session.sessionId
                    this.draftName = titleOf(session)
                  })

                Button('删除')
                  .onClick(() => {
                    this.onRemove(session.sessionId)
                  })
              }
            }
            .id(`session-item-${session.sessionId}`)
            .padding(16)
            .width('100%')
            .alignItems(HorizontalAlign.Start)
            .backgroundColor('#ffffff')
            .borderRadius(12)
          }, (session: SessionRecord) => session.sessionId)
        }

        Button('新增配对')
          .id('add-session-button')
          .onClick(() => {
            this.onAdd()
          })
      }
      .padding(16)
      .width('100%')
      .alignItems(HorizontalAlign.Start)
    }
    .width('100%')
    .height('100%')
  }
}
