import type { SessionRecord, SessionLocalStatus } from '../session/records'
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
  if (!record.hasConnectedAt) {
    return '从未连接'
  }
  return `最近连接 ${new Date(record.lastConnectedAt).toLocaleTimeString()}`
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
          ForEach(this.sessions, (session: SessionRecord) => {
            Column({ space: 8 }) {
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
                Text(session.name)
                  .fontSize(16)
                  .fontWeight(FontWeight.Medium)
              }

              Text(`ID ${session.sessionId.slice(0, 8)} · ${addressDigest(session.wsUrl)}`)
                .fontSize(12)
                .opacity(0.7)

              Text(`${statusLabel(session.status)} · ${lastConnectedLabel(session)}`)
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
                    this.draftName = session.name
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
