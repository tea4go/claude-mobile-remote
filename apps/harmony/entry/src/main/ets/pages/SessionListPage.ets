import { defaultSessionName, type SessionRecord } from '../session/records'

function titleOf(record: SessionRecord): string {
  if (record.customName === true || (record.customName === undefined && record.name !== defaultSessionName(record.sessionId))) return record.name
  return record.sessionTitle ?? record.name
}
function pcKey(record: SessionRecord): string { return record.pcId ?? record.wsUrl }
function pcName(record: SessionRecord): string {
  return record.pcName ?? record.wsUrl.replace(/^wss?:\/\//, '').split('/')[0]
}
function devices(records: Array<SessionRecord>): Array<SessionRecord> {
  const result: Array<SessionRecord> = []
  records.forEach((record: SessionRecord) => { if (!result.some((item: SessionRecord) => pcKey(item) === pcKey(record))) result.push(record) })
  return result
}
function statusLabel(record: SessionRecord): string {
  if (record.status === 'needsPairing') return '需要重新扫码'
  if (record.status !== 'connected') return '未连接 · 待同步'
  return record.runtimeStatus ?? '空闲'
}
function rowKey(record: SessionRecord): string {
  // Never put credentials into render keys; only include the displayed fields.
  return JSON.stringify([record.sessionId, titleOf(record), record.preview, pcName(record), record.projectCwd,
    statusLabel(record), record.unread, record.lastActivityAt])
}
@Component
export struct SessionListPage {
  @Prop scrollOffset: number = 0
  onScrollOffset: (value: number) => void = () => {}
  @Prop sessions: Array<SessionRecord> = []
  @Prop recentlyViewedId: string = ''
  @Prop selfChecks: Array<string> = []
  @Prop selfCheckError: string = ''
  @Prop notificationsEnabled: boolean = false
  onAdd: () => void = () => {}
  onOpen: (sessionId: string) => void = () => {}
  onRemove: (sessionId: string) => void = () => {}
  onRename: (sessionId: string, name: string) => void = () => {}
  onRemovePC: (key: string) => void = () => {}
  onNotifications: () => void = () => {}
  onClearCache: () => void = () => {}
  @State private settings: boolean = false
  @State private renamingId: string = ''
  @State private draftName: string = ''
  private readonly scroller: Scroller = new Scroller()

  private remove(record: SessionRecord): void {
    AlertDialog.show({ title: '移除此配对？', message: '同时清除本机凭证、缓存与提醒，电脑端会话继续运行。',
      primaryButton: { value: '移除', action: () => this.onRemove(record.sessionId) },
      secondaryButton: { value: '取消', action: () => {} } })
  }
  build() {
    Column() {
      Row({ space: 8 }) {
        Text(this.settings ? '设置' : '会话').fontSize(24).fontWeight(FontWeight.Bold).layoutWeight(1)
        Button(this.settings ? '返回' : '设置').height(44).onClick(() => { this.settings = !this.settings })
        if (!this.settings) { Button('扫码配对').height(44).onClick(() => this.onAdd()) }
      }.padding(16).width('100%')
      Scroll(this.scroller) {
        Column({ space: 8 }) {
          if (this.settings) {
            Button(this.notificationsEnabled ? '关闭系统提醒' : '开启系统提醒').height(48).onClick(() => this.onNotifications())
            Text('仅提醒已收到的在线事件；默认隐藏正文。系统挂起或退出后不保证送达。').fontSize(13)
            Button('清除消息缓存').height(48).onClick(() => {
              AlertDialog.show({ title: '清除本机消息缓存？', message: '配对仍保留；未缓存的历史无法从电脑补回。',
                primaryButton: { value: '清除', action: () => this.onClearCache() }, secondaryButton: { value: '取消', action: () => {} } })
            })
            Text('已配对电脑').fontSize(18).fontWeight(FontWeight.Bold)
            ForEach(devices(this.sessions), (record: SessionRecord) => {
              Column({ space: 8 }) {
                Text(pcName(record)).fontSize(16)
                Text(record.wsUrl).fontSize(12)
                Text(`${this.sessions.filter((item: SessionRecord) => pcKey(item) === pcKey(record)).length} 个会话`).fontSize(12)
                Button('移除此电脑').height(44).onClick(() => {
                  AlertDialog.show({ title: '移除此电脑的全部配对？', message: '本机配对与缓存会清除，电脑上的 Claude 会话继续运行。',
                    primaryButton: { value: '移除', action: () => this.onRemovePC(pcKey(record)) }, secondaryButton: { value: '取消', action: () => {} } })
                })
              }.padding(12).width('100%').backgroundColor('#ffffff').borderRadius(12)
            }, (record: SessionRecord) => JSON.stringify([pcKey(record), pcName(record), record.wsUrl,
              this.sessions.filter((item: SessionRecord) => pcKey(item) === pcKey(record)).length]))
          } else if (this.sessions.length === 0) {
            Text('扫码连接电脑上的 Claude 会话').padding(24)
          } else {
            ForEach(this.sessions, (session: SessionRecord) => {
              Column({ space: 6 }) {
                Row({ space: 8 }) {
                  Column({ space: 6 }) {
                    Text(titleOf(session)).fontSize(17).fontWeight(FontWeight.Medium).maxLines(1).width('100%')
                    Text(session.preview ?? '暂无会话内容').fontSize(13).maxLines(2).opacity(0.8).width('100%')
                    Text(`${pcName(session)} · ${session.projectCwd ?? '工作目录未知'}`).fontSize(12).maxLines(1).width('100%')
                    Text(`${statusLabel(session)}${session.unread ? ' · 未读' : ''}${this.recentlyViewedId === session.sessionId ? ' · 刚刚查看' : ''}`).fontSize(12).width('100%')
                    if (session.lastActivityAt !== undefined) { Text(new Date(session.lastActivityAt).toLocaleString()).fontSize(11).opacity(0.6) }
                  }.layoutWeight(1).alignItems(HorizontalAlign.Start).onClick(() => this.onOpen(session.sessionId))
                  Button('⋯').width(44).height(44).accessibilityText('会话操作').onClick(() => {
                    ActionSheet.show({ title: titleOf(session), sheets: [
                      { title: '进入', action: () => this.onOpen(session.sessionId) },
                      { title: '重命名', action: () => { this.renamingId = session.sessionId; this.draftName = titleOf(session) } },
                      { title: '移除配对', action: () => this.remove(session) },
                    ] })
                  })
                }.width('100%').constraintSize({ minHeight: 100 })
                if (this.renamingId === session.sessionId) {
                  Row({ space: 8 }) {
                    TextInput({ text: this.draftName, placeholder: '本地名称' }).maxLength(255).layoutWeight(1).onChange((text: string) => { this.draftName = text })
                    Button('保存').height(44).onClick(() => { this.onRename(session.sessionId, this.draftName); this.renamingId = '' })
                    Button('取消').height(44).onClick(() => { this.renamingId = '' })
                  }
                }
              }.id(`session-item-${session.sessionId}`).padding(12).width('100%').backgroundColor('#ffffff').borderRadius(12)
            }, (session: SessionRecord) => rowKey(session))
          }
        }.padding(16).width('100%').alignItems(HorizontalAlign.Start)
      }.layoutWeight(1).width('100%')
        .onAppear(() => this.scroller.scrollTo({ xOffset: 0, yOffset: this.scrollOffset }))
        .onScroll(() => { if (!this.settings) this.onScrollOffset(this.scroller.currentOffset().yOffset) })
    }.width('100%').height('100%').backgroundColor('#f5f7fa')
  }
}
