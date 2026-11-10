import { PAGE_BACKGROUND, PRIMARY, SECONDARY } from '../ui/styles'
import { ActionMenu, type ActionMenuItem } from '../components/ActionMenu'
import { curves } from '@kit.ArkUI'
import { pageFade } from '../ui/transitions'
import { nativeLifecycle } from '../native/lifecycle'
import { defaultSessionName, type SessionRecord } from '../session/records'
import { groupSessions, type PCGroup, type ProjectGroup } from '../session/projectGroups'
import { ProjectListDataSource, projectListRows, type ProjectListRow } from '../ui/projectListDataSource'

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
  @Prop initialSettings: boolean = false
  onExitSettings: () => void = () => {}
  @Prop scrollOffset: number = 0
  onScrollOffset: (value: number) => void = () => {}
  @Prop @Watch('updateGroups') sessions: Array<SessionRecord> = []
  @Prop @Watch('updateGroups') selectedPC: string = 'all'
  @Prop projectCreating: boolean = false
  onSelectPC: (key: string) => void = () => {}
  onCreateProject: (pcKey: string, cwd: string, sourceId: string) => void = () => {}
  @State private groups: Array<PCGroup> = []
  @State private collapsed: Array<string> = []
  @State private filterOpen: boolean = false
  @State private filterIndex: number = 0
  private readonly listSource: ProjectListDataSource = new ProjectListDataSource()
  private updateGroups(): void { this.groups = groupSessions(this.sessions); this.updateRows() }
  private updateRows(): void { this.listSource.replace(projectListRows(this.groups, this.selectedPC, this.collapsed)) }
  aboutToAppear(): void { this.settings = this.initialSettings; this.updateGroups(); nativeLifecycle.detailBack = this.backHandler }
  private readonly backHandler: () => boolean = () => {
    if (this.filterOpen) { this.filterOpen = false; return true }
    if (this.settings) { this.changePage(); return true }
    return false
  }
  @Prop recentlyViewedId: string = ''
  @Prop selfChecks: Array<string> = []
  @Prop selfCheckError: string = ''
  @Prop notificationsEnabled: boolean = false
  @Prop pushConsent: boolean = false
  @Prop pushStatus: string = ''
  onEnablePush: () => void = () => {}
  onAdd: () => void = () => {}
  onOpen: (sessionId: string) => void = () => {}
  onRemove: (sessionId: string) => void = () => {}
  onRename: (sessionId: string, name: string) => void = () => {}
  onRemovePC: (key: string) => void = () => {}
  onNotifications: () => void = () => {}
  onDemo: () => void = () => {}
  @Prop debugEnabled: boolean = false
  onDebug: (value: boolean) => void = () => {}
  onDebugOpen: () => void = () => {}
  onClearCache: () => void = () => {}
  @State private settings: boolean = false
  @State private computers: boolean = false
  private createProject(pcKey: string, project: ProjectGroup): void {
    const available = project.sessions.filter((record: SessionRecord) => record.status === 'connected' && (record.capabilities ?? []).indexOf('create_session') >= 0)
    this.onCreateProject(pcKey, project.cwd, available.length > 0 ? available[0].sessionId : '')
  }
  private openFilter(): void {
    const index = this.groups.findIndex((pc: PCGroup) => pc.key === this.selectedPC)
    this.filterIndex = index < 0 ? 0 : index + 1
    this.filterOpen = true
  }
  private menuSession: SessionRecord | null = null
  private menuController: CustomDialogController = new CustomDialogController({
    builder: ActionMenu({ title: this.menuSession === null ? '会话操作' : titleOf(this.menuSession),
      items: this.menuItems(), onClose: (): void => { this.menuController.close() } }),
    customStyle: true, alignment: DialogAlignment.Bottom, autoCancel: true,
    openAnimation: { curve: curves.springMotion(0.25, 1) },
    closeAnimation: { curve: curves.springMotion(0.25, 1) },
  })
  private listOffset: number = 0
  @State private renamingId: string = ''
  @State private draftName: string = ''
  private readonly scroller: Scroller = new Scroller()

  private menuItems(): Array<ActionMenuItem> {
    const session = this.menuSession
    if (session === null) return []
    return [
      { title: '进入', action: () => this.onOpen(session.sessionId) },
      { title: '重命名', action: () => { this.renamingId = session.sessionId; this.draftName = titleOf(session) } },
      { title: '移除配对', danger: true, action: () => this.remove(session) },
    ]
  }
  aboutToDisappear(): void { this.menuController.close(); if (nativeLifecycle.detailBack === this.backHandler) nativeLifecycle.detailBack = null }

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
    } else if (this.initialSettings) {
      this.onExitSettings()
    } else {
      this.settings = false
    }
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
        Button('⋯', { type: ButtonType.Normal }).width(44).height(44).fontSize(22).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true)
          .accessibilityText('会话操作').onClick(() => {
            this.menuSession = session
            this.menuController.open()
          })
      }.width('100%').constraintSize({ minHeight: 100 }).padding({ left: 12, right: 4, top: 12, bottom: 12 })
      if (this.renamingId === session.sessionId) {
        Row({ space: 4 }) {
          TextInput({ text: this.draftName, placeholder: '本地名称' }).maxLength(255).layoutWeight(1).onChange((text: string) => { this.draftName = text })
          Button('保存', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true)
            .onClick(() => { this.onRename(session.sessionId, this.draftName); this.renamingId = '' })
          Button('取消', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true)
            .onClick(() => { this.renamingId = '' })
        }.padding(8)
      }
    }.id(`session-item-${session.sessionId}`).width('100%')
      .backgroundColor(this.recentlyViewedId === session.sessionId ? '#f2f7ff' : '#ffffff').border({ width: 1, color: this.recentlyViewedId === session.sessionId ? '#b9d2ff' : '#dfe6f1' }).borderRadius(16)
  }
  build() {
    Stack({ alignContent: Alignment.Bottom }) {
    Column() {
      Row() {
        Text(this.settings ? (this.computers ? 'PC 管理' : '设置') : 'Whale Remote Control').fontSize(22).fontWeight(FontWeight.Bold).layoutWeight(1)
        Button(this.settings ? '返回' : '设置', { type: ButtonType.Normal }).height(44).fontSize(16).fontColor(PRIMARY)
          .backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).constraintSize({ minWidth: 44 }).padding({ left: 10, right: 10 }).onClick(() => this.changePage())
      }.padding({ left: 16, right: 16, top: 8, bottom: 4 }).width('100%')
      if (this.settings) {
        Scroll() {
          Column({ space: 18 }) {
          if (this.computers) {
            ForEach(devices(this.sessions), (record: SessionRecord) => {
              Row({ space: 8 }) {
                Column({ space: 6 }) {
                  Text(pcName(record)).fontSize(16).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
                  Text(this.pcStatus(record)).fontSize(12).fontColor(SECONDARY)
                }.layoutWeight(1).alignItems(HorizontalAlign.Start)
                Button('移除', { type: ButtonType.Normal }).height(44).fontSize(14).fontColor('#d4380d').backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).onClick(() => {
                  AlertDialog.show({ title: '移除此电脑的全部配对？', message: '本机配对与缓存会清除，电脑上的 Claude 会话继续运行。',
                    primaryButton: { value: '移除', action: () => this.onRemovePC(pcKey(record)) }, secondaryButton: { value: '取消', action: () => {} } })
                })
              }.padding(12).width('100%').backgroundColor('#ffffff').borderRadius(12)
            }, (record: SessionRecord) => JSON.stringify([pcKey(record), pcName(record), this.pcStatus(record)]))
            if (this.sessions.length === 0) { Text('暂无已配对电脑').fontSize(14).fontColor(SECONDARY).padding(24) }
          } else {
            Row() {
              Text('PC 管理').fontSize(16).layoutWeight(1)
              Text('›').fontSize(24).fontColor(SECONDARY)
            }.width('100%').height(60).padding({ left: 16, right: 16 }).backgroundColor('#ffffff').borderRadius(16).onClick(() => { this.computers = true })
            Row() {
              Text('体验演示（无需连接电脑）').fontSize(16).layoutWeight(1)
              Text('›').fontSize(24).fontColor(SECONDARY)
            }.width('100%').height(60).padding({ left: 16, right: 16 }).backgroundColor('#ffffff').borderRadius(16).onClick(() => this.onDemo())
            Column({ space: 8 }) {
              Row() {
                Text('调试日志').fontSize(16).layoutWeight(1)
                Toggle({ type: ToggleType.Switch, isOn: this.debugEnabled }).onChange((value: boolean) => this.onDebug(value))
              }.width('100%')
              Text('开启后显示悬浮入口；日志始终在本次运行中记录，最多保留 200 条。').fontSize(12).fontColor(SECONDARY).width('100%')
            }.padding(16).width('100%').backgroundColor('#ffffff').borderRadius(16).alignItems(HorizontalAlign.Start)
            Column({ space: 10 }) {
              Text('本地保存说明').fontSize(16).fontWeight(FontWeight.Medium).width('100%')
              Text('配对凭证保存在系统安全存储；移除配对不会关闭电脑上的会话。').fontSize(13).fontColor(SECONDARY).width('100%')
              Text('保存最近已收到的消息和目录；每会话最多 200 项/2 MiB，总计 20 MiB，保留 30 天。不缓存图片原件，不支持离线发送。').fontSize(13).fontColor(SECONDARY).width('100%')
              Row() {
                Text('清除离线缓存').fontSize(15).layoutWeight(1)
                Button('清除', { type: ButtonType.Normal }).height(44).fontColor(PRIMARY).backgroundColor(Color.Transparent).onClick(() => {
                  AlertDialog.show({ title: '清除本机消息缓存？', message: '配对仍保留；未缓存的历史无法从电脑补回。', primaryButton: { value: '清除', action: () => this.onClearCache() }, secondaryButton: { value: '取消', action: () => {} } })
                })
              }.width('100%')
            }.padding(16).width('100%').backgroundColor('#ffffff').borderRadius(16).alignItems(HorizontalAlign.Start)
            Column({ space: 8 }) {
              Row() {
                Text('在线会话提醒').fontSize(16).layoutWeight(1)
                Toggle({ type: ToggleType.Switch, isOn: this.notificationsEnabled }).selectedColor(PRIMARY).onChange((enabled: boolean) => { if (enabled !== this.notificationsEnabled) this.onNotifications() })
              }.width('100%')
              Text('仅提醒在线收到的事件；后台推送当前未开放。').fontSize(12).fontColor(SECONDARY).width('100%')
            }.padding(16).width('100%').backgroundColor('#ffffff').borderRadius(16).alignItems(HorizontalAlign.Start)

          }
          }.padding({ left: 16, right: 16, top: 12, bottom: 24 }).width('100%').alignItems(HorizontalAlign.Start)
        }.layoutWeight(1).width('100%').align(Alignment.TopStart)
      } else {
        List({ space: 8, scroller: this.scroller }) {
          ListItem() { Text('会话列表').fontSize(28).fontWeight(FontWeight.Bold).width('100%').padding({ top: 12, bottom: 12 }) }
          if (this.groups.length === 0) { ListItem() { Text('扫码连接电脑上的 Claude 会话').fontSize(14).fontColor(SECONDARY).padding(24) } }
          LazyForEach(this.listSource, (row: ProjectListRow) => {
            ListItem() {
              if (row.kind === 'pc' && row.pc !== undefined) {
                Row({ space: 12 }) {
                  Text('电脑').fontSize(14).fontColor(PRIMARY)
                  Column({ space: 4 }) {
                    Text(row.pc.label).fontSize(16).fontWeight(FontWeight.Medium).width('100%')
                    Text(`${row.pc.connected}/${row.pc.sessions.length} 个会话已连接`).fontSize(12).fontColor(SECONDARY).width('100%')
                  }.layoutWeight(1).alignItems(HorizontalAlign.Start)
                  Text('›').fontSize(24).fontColor(SECONDARY)
                }.padding(16).width('100%').backgroundColor('#ffffff').borderRadius(16).onClick(() => this.openFilter())
              } else if (row.kind === 'project' && row.pc !== undefined && row.project !== undefined) {
                Row({ space: 8 }) {
                  Column({ space: 4 }) {
                    Text(`${this.collapsed.indexOf(row.project.key) >= 0 ? '▸' : '▾'}  ${row.project.title}`).fontSize(15).fontWeight(FontWeight.Medium).width('100%')
                    Text(row.project.cwd.length > 0 ? row.project.cwd : '工作目录未知').fontSize(12).fontColor(SECONDARY).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
                  }.layoutWeight(1).alignItems(HorizontalAlign.Start).onClick(() => {
                    const project = row.project as ProjectGroup
                    this.collapsed = this.collapsed.indexOf(project.key) >= 0 ? this.collapsed.filter((key: string) => key !== project.key) : this.collapsed.concat([project.key])
                    this.updateRows()
                  })
                  Button(this.projectCreating ? '…' : '+', { type: ButtonType.Normal }).width(44).height(44).fontSize(24).fontColor(PRIMARY).backgroundColor(Color.Transparent).enabled(!this.projectCreating).accessibilityText(`在 ${row.project.title} 新建会话`).onClick(() => this.createProject((row.pc as PCGroup).key, row.project as ProjectGroup))
                }.width('100%').padding({ top: 4 })
              } else if (row.session !== undefined) {
                this.sessionRow(row.session)
              }
            }
          }, (row: ProjectListRow) => row.key)
        }.layoutWeight(1).width('100%').padding({ left: 16, right: 16 }).cachedCount(3)
          .onAppear(() => { this.listOffset = this.scrollOffset; this.scroller.scrollTo({ xOffset: 0, yOffset: this.scrollOffset }) })
          .onScroll(() => this.onScrollOffset(this.scroller.currentOffset().yOffset))
      }
      if (!this.settings) {
        Button('＋ 新会话', { type: ButtonType.Normal }).height(48).width('100%').fontSize(16).fontColor('#ffffff')
          .backgroundColor(PRIMARY).borderRadius(16).onClick(() => this.onAdd()).margin({ bottom: 12, left: 16, right: 16 })
      }
    }.width('100%').height('100%').backgroundColor(PAGE_BACKGROUND)
    if (this.filterOpen) {
      Column().width('100%').height('100%').backgroundColor('#66000000').onClick(() => { this.filterOpen = false })
      Column({ space: 8 }) {
        Row() {
          Button('取消', { type: ButtonType.Normal }).height(44).fontColor(SECONDARY).backgroundColor(Color.Transparent).onClick(() => { this.filterOpen = false })
          Text('选择电脑').fontSize(18).fontWeight(FontWeight.Medium).layoutWeight(1).textAlign(TextAlign.Center)
          Button('确定', { type: ButtonType.Normal }).height(44).fontColor(PRIMARY).backgroundColor(Color.Transparent).onClick(() => {
            const pc = this.groups[this.filterIndex - 1]
            this.onSelectPC(this.filterIndex === 0 || pc === undefined ? 'all' : pc.key)
            this.filterOpen = false
          })
        }.width('100%')
        TextPicker({ range: ['全部电脑'].concat(this.groups.map((pc: PCGroup) => pc.label)), selected: this.filterIndex })
          .width('100%').height(200).onChange((_value: string | string[], index: number | number[]) => {
            this.filterIndex = typeof index === 'number' ? index : index[0]
          })
      }.padding(16).width('100%').backgroundColor('#ffffff').borderRadius({ topLeft: 24, topRight: 24 })
    }
    }.width('100%').height('100%')
  }
}
