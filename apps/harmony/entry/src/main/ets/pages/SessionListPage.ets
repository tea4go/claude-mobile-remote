import { PAGE_BACKGROUND, PRIMARY, SECONDARY } from '../ui/styles'
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
  if (record.status !== 'connected') return '未连接'
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
  onDemo: () => void = () => {}
  @Prop debugEnabled: boolean = true
  onDebug: (value: boolean) => void = () => {}
  onDebugOpen: () => void = () => {}
  onClearCache: () => void = () => {}
  @State private settings: boolean = false
  @State private computers: boolean = false
  private listOffset: number = 0
  @State private renamingId: string = ''
  @State private draftName: string = ''
  private readonly scroller: Scroller = new Scroller()

  private remove(record: SessionRecord): void {
    AlertDialog.show({ title: '移除此配对？', message: '同时清除本机凭证、缓存与提醒，电脑端会话继续运行。',
      primaryButton: { value: '移除', action: () => this.onRemove(record.sessionId) },
      secondaryButton: { value: '取消', action: () => {} } })
  }
  private changePage(): void {
    if (!this.settings) {
      this.listOffset = this.scroller.currentOffset().yOffset
      this.settings = true
    } else if (this.computers) {
      this.computers = false
    } else {
      this.settings = false
    }
    this.scroller.scrollTo({ xOffset: 0, yOffset: this.settings ? 0 : this.listOffset })
  }
  private timeOf(record: SessionRecord): string {
    if (record.lastActivityAt === undefined) return ''
    const date = new Date(record.lastActivityAt)
    if (Number.isNaN(date.getTime())) return ''
    return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
  }
  private pcStatus(record: SessionRecord): string {
    const group = this.sessions.filter((item: SessionRecord) => pcKey(item) === pcKey(record))
    return `${group.filter((item: SessionRecord) => item.status === 'connected').length}/${group.length} 已连接`
  }
  @Builder
  private sessionRow(session: SessionRecord) {
    Column() {
      Row({ space: 4 }) {
        Column({ space: 4 }) {
          Row({ space: 5 }) {
            Text(titleOf(session)).fontSize(16).fontWeight(FontWeight.Medium).maxLines(1)
              .textOverflow({ overflow: TextOverflow.Ellipsis }).layoutWeight(1)
            if (session.unread) { Circle().width(7).height(7).fill(PRIMARY).accessibilityText('未读') }
            if (this.recentlyViewedId === session.sessionId) { Text('刚刚查看').fontSize(11).fontColor(PRIMARY) }
          }.width('100%')
          Text(session.preview ?? '暂无会话内容').fontSize(13).fontColor(SECONDARY).maxLines(1)
            .textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
          Text(session.projectCwd ?? '工作目录未知').fontSize(12).fontColor('#8e8e93').maxLines(1)
            .textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
        }.layoutWeight(1).alignItems(HorizontalAlign.Start).onClick(() => this.onOpen(session.sessionId))
        Column({ space: 4 }) {
          Text(statusLabel(session)).fontSize(11).fontColor(session.status === 'needsPairing' || session.runtimeStatus === '等待处理' ? '#b7791f' : SECONDARY)
            .maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%').textAlign(TextAlign.End)
          Text(this.timeOf(session)).fontSize(11).fontColor(SECONDARY)
        }.width(72).alignItems(HorizontalAlign.End).onClick(() => this.onOpen(session.sessionId))
        Button('⋯', { type: ButtonType.Normal }).width(44).height(44).fontSize(22).fontColor(SECONDARY).backgroundColor(Color.Transparent)
          .accessibilityText('会话操作').onClick(() => {
            ActionSheet.show({ message: '', title: titleOf(session), sheets: [
              { title: '进入', action: () => this.onOpen(session.sessionId) },
              { title: '重命名', action: () => { this.renamingId = session.sessionId; this.draftName = titleOf(session) } },
              { title: '移除配对', action: () => this.remove(session) },
            ] })
          })
      }.width('100%').constraintSize({ minHeight: 100 }).padding({ left: 12, right: 4, top: 12, bottom: 12 })
      if (this.renamingId === session.sessionId) {
        Row({ space: 4 }) {
          TextInput({ text: this.draftName, placeholder: '本地名称' }).maxLength(255).layoutWeight(1).onChange((text: string) => { this.draftName = text })
          Button('保存', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor(PRIMARY).backgroundColor(Color.Transparent)
            .onClick(() => { this.onRename(session.sessionId, this.draftName); this.renamingId = '' })
          Button('取消', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor(SECONDARY).backgroundColor(Color.Transparent)
            .onClick(() => { this.renamingId = '' })
        }.padding(8)
      }
    }.id(`session-item-${session.sessionId}`).width('100%')
      .backgroundColor(this.recentlyViewedId === session.sessionId ? '#f2f7ff' : '#ffffff')
  }
  build() {
    Column() {
      Row() {
        Text(this.settings ? (this.computers ? '已配对电脑' : '设置') : '会话列表').fontSize(22).fontWeight(FontWeight.Bold).layoutWeight(1)
        Button(this.settings ? '返回' : '设置', { type: ButtonType.Normal }).height(44).fontSize(16).fontColor(PRIMARY)
          .backgroundColor(Color.Transparent).padding({ left: 10, right: 0 }).onClick(() => this.changePage())
      }.padding({ left: 16, right: 16, top: 8, bottom: 4 }).width('100%')
      Scroll(this.scroller) {
        Column({ space: 18 }) {
          if (this.settings && this.computers) {
            ForEach(devices(this.sessions), (record: SessionRecord) => {
              Row({ space: 8 }) {
                Column({ space: 6 }) {
                  Text(pcName(record)).fontSize(16).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
                  Text(this.pcStatus(record)).fontSize(12).fontColor(SECONDARY)
                }.layoutWeight(1).alignItems(HorizontalAlign.Start)
                Button('移除', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor('#d4380d').backgroundColor(Color.Transparent).onClick(() => {
                  AlertDialog.show({ title: '移除此电脑的全部配对？', message: '本机配对与缓存会清除，电脑上的 Claude 会话继续运行。',
                    primaryButton: { value: '移除', action: () => this.onRemovePC(pcKey(record)) }, secondaryButton: { value: '取消', action: () => {} } })
                })
              }.padding(12).width('100%').backgroundColor('#ffffff').borderRadius(12)
            }, (record: SessionRecord) => JSON.stringify([pcKey(record), pcName(record), this.pcStatus(record)]))
            if (this.sessions.length === 0) { Text('暂无已配对电脑').fontSize(14).fontColor(SECONDARY).padding(24) }
          } else if (this.settings) {
            Column() {
              Button('体验演示（无需连接电脑）').height(44).onClick(() => this.onDemo())
              Row() {
                Text('调试日志').layoutWeight(1)
                Toggle({ type: ToggleType.Switch, isOn: this.debugEnabled }).onChange((value: boolean) => this.onDebug(value))
                Button('查看').height(36).fontSize(12).onClick(() => this.onDebugOpen())
              }.padding(12).width('100%')
              Row() {
                Text('系统提醒').fontSize(16).layoutWeight(1)
                Toggle({ type: ToggleType.Switch, isOn: this.notificationsEnabled }).selectedColor(PRIMARY)
                  .onChange((enabled: boolean) => { if (enabled !== this.notificationsEnabled) this.onNotifications() })
              }.height(54).padding({ left: 16, right: 16 }).width('100%')
              Divider().color('#eeeeef')
              Button({ type: ButtonType.Normal }) {
                Text('清除消息缓存').fontSize(16).fontColor('#252525').width('100%').textAlign(TextAlign.Start)
              }.width('100%').height(54).padding({ left: 16, right: 16 }).backgroundColor(Color.Transparent).onClick(() => {
                  AlertDialog.show({ title: '清除本机消息缓存？', message: '配对仍保留；未缓存的历史无法从电脑补回。',
                    primaryButton: { value: '清除', action: () => this.onClearCache() }, secondaryButton: { value: '取消', action: () => {} } })
                })
              Divider().color('#eeeeef')
              Row() {
                Text('已配对电脑').fontSize(16).layoutWeight(1)
                Text('›').fontSize(22).fontColor('#a0a0a5')
              }.width('100%').height(54).padding({ left: 16, right: 16 }).onClick(() => { this.computers = true; this.scroller.scrollTo({ xOffset: 0, yOffset: 0 }) })
            }.width('100%').backgroundColor('#ffffff').borderRadius(12).clip(true)
            Text('仅提醒已收到的在线事件；默认隐藏正文。系统挂起或退出后不保证送达。').fontSize(13).fontColor(SECONDARY).width('100%')
          } else {
            if (this.sessions.length === 0) { Text('扫码连接电脑上的 Claude 会话').fontSize(14).fontColor(SECONDARY).padding(24) }
            ForEach(devices(this.sessions), (device: SessionRecord) => {
              Column({ space: 14 }) {
                Text(pcName(device)).fontSize(15).fontWeight(FontWeight.Medium).fontColor(SECONDARY).padding({ left: 4 }).width('100%')
                Column() {
                  ForEach(this.sessions.filter((item: SessionRecord) => pcKey(item) === pcKey(device)), (session: SessionRecord, index: number) => {
                    if (index > 0) { Divider().color('#eeeeef').margin({ left: 12 }) }
                    this.sessionRow(session)
                  }, (session: SessionRecord) => JSON.stringify([rowKey(session), this.recentlyViewedId === session.sessionId]))
                }.width('100%').borderRadius(12).clip(true).backgroundColor('#ffffff')
              }.width('100%').alignItems(HorizontalAlign.Start)
            }, (record: SessionRecord) => JSON.stringify([pcKey(record), pcName(record)]))
            Button('新增配对', { type: ButtonType.Normal }).height(46).width('100%').fontSize(16).fontColor(PRIMARY)
              .backgroundColor('#ffffff').borderRadius(12).onClick(() => this.onAdd())
          }
        }.padding({ left: 16, right: 16, top: 12, bottom: 24 }).width('100%').alignItems(HorizontalAlign.Start)
      }.layoutWeight(1).width('100%').align(Alignment.TopStart)
        .onAppear(() => { this.listOffset = this.scrollOffset; this.scroller.scrollTo({ xOffset: 0, yOffset: this.scrollOffset }) })
        .onScroll(() => { if (!this.settings) this.onScrollOffset(this.scroller.currentOffset().yOffset) })
    }.width('100%').height('100%').backgroundColor(PAGE_BACKGROUND)
  }
}
