import { formatActivityTime } from '../ui/activityTime'
import { HomeButton } from '../components/HomeButton'
import { PAGE_BACKGROUND, PRIMARY, SECONDARY } from '../ui/styles'
import { PairingInputDialog } from '../components/PairingInputDialog'
import { APP_BUILD_LABEL } from '../common/BuildInfo'
import { pageSlide } from '../ui/transitions'
import { nativeLifecycle } from '../native/lifecycle'
import { defaultSessionName, type SessionRecord } from '../session/records'
import { groupSessions, maskPairToken, type PCGroup, type ProjectGroup, type PCProfile } from '../session/projectGroups'
import { ProjectListDataSource, projectListRows, type ProjectListRow } from '../ui/projectListDataSource'

function titleOf(record: SessionRecord): string {
  if (record.customName === true || (record.customName === undefined && record.name !== defaultSessionName(record.sessionId))) return record.name
  return record.sessionTitle ?? record.name
}
function pcName(record: SessionRecord): string {
  return record.pcName ?? record.wsUrl.replace(/^wss?:\/\//, '').split('/')[0]
}
function statusLabel(record: SessionRecord): string {
  if (record.status === 'needsPairing') return '需要重新扫码'
  if (record.status !== 'connected') return '未连接'
  return record.runtimeStatus ?? '空闲'
}
function rowKey(record: SessionRecord): string {
  // Never put credentials into render keys; only include the displayed fields.
  return JSON.stringify([record.sessionId, titleOf(record), record.preview, pcName(record), record.projectCwd,
    statusLabel(record), record.runtimeStatus, record.unread, record.lastActivityAt, record.createdAt])
}
@Component
export struct SessionListPage {
  onHome: () => void = () => {}
  onCreateDirectory: (pc: PCGroup, project: ProjectGroup) => void = () => {}
  @Prop projectCreating: boolean = false
  private beganAtEdge: boolean = false
  @Prop initialSettings: boolean = false
  onExitSettings: () => void = () => {}
  @Prop scrollOffset: number = 0
  onScrollOffset: (value: number) => void = () => {}
  onRefresh: () => void = () => {}
  @State private refreshing: boolean = false
  @Prop @Watch('updateRows') pendingRefreshSessions: Array<string> = []
  @Prop @Watch('updateGroups') profiles: Array<PCProfile> = []
  @Prop @Watch('updateGroups') sessions: Array<SessionRecord> = []
  @Prop @Watch('updateGroups') selectedPC: string = 'all'
  onSelectPC: (key: string) => void = () => {}
  @State private groups: Array<PCGroup> = []
  @State private collapsed: Array<string> = []
  @State private filterOpen: boolean = false
  @State private filterIndex: number = 0
  private readonly listSource: ProjectListDataSource = new ProjectListDataSource()
  private updateGroups(): void {
    this.groups = groupSessions(this.sessions)
    for (const computer of this.groups) {
      const profile = this.profiles.find((item: PCProfile) => item.key === computer.key)
      if (profile !== undefined) computer.label = profile.name
    }
    this.updateRows()
  }

  private computerAddress(computer: PCGroup): string {
    return this.profiles.find((item: PCProfile) => item.key === computer.key)?.ip ??
      (/^wss?:\/\/(\[[^\]]+\]|[^/:]+)/.exec(computer.sessions[0].wsUrl)?.[1] ?? '')
  }
  private updateRows(): void { this.listSource.replace(projectListRows(this.groups, this.selectedPC, this.collapsed, this.pendingRefreshSessions)) }
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
  onNotifications: () => void = () => {}
  onDemo: () => void = () => {}
  @Prop debugEnabled: boolean = false
  onDebug: (value: boolean) => void = () => {}
  onDebugOpen: () => void = () => {}
  onDebugActionsTop: (top: number) => void = () => {}
  onClearCache: () => void = () => {}
  @State private settings: boolean = false
  private openFilter(): void {
    const index = this.groups.findIndex((pc: PCGroup) => pc.key === this.selectedPC)
    this.filterIndex = index < 0 ? 0 : index + 1
    this.filterOpen = true
  }
  private listOffset: number = 0
  private renamingId: string = ''
  @State private draftName: string = ''
  @State private renameError: string = ''
  private readonly scroller: Scroller = new Scroller()
  private readonly renameDialog: CustomDialogController = new CustomDialogController({
    builder: PairingInputDialog({ rename: true, value: $draftName, error: $renameError,
      onCancel: (): void => this.renameDialog.close(),
      onVerify: (): void => {
        const name = Array.from(this.draftName).slice(0, 40).join('').trim()
        if (name.length === 0) return
        this.renameDialog.close()
        this.onRename(this.renamingId, name)
      } }),
    customStyle: true, alignment: DialogAlignment.Center,
    closeAnimation: { duration: 0 },
  })

  aboutToDisappear(): void { this.renameDialog.close(); if (nativeLifecycle.detailBack === this.backHandler) nativeLifecycle.detailBack = null }

  @Builder
  private sessionActions(session: SessionRecord): void {
    Row() {
      Button({ type: ButtonType.Normal }) {
        Image($r('app.media.pc_action_more')).width(18).height(18)
      }.width(54).height('100%').padding(0).borderRadius(0).backgroundColor('#ff8a30').accessibilityText('重命名').onClick(() => {
        this.renamingId = session.sessionId
        this.draftName = Array.from(titleOf(session)).slice(0, 40).join('')
        this.renameDialog.open()
      })
      Button({ type: ButtonType.Normal }) {
        Image($r('app.media.pc_action_delete')).width(18).height(18)
      }.width(54).height('100%').padding(0).borderRadius(0).backgroundColor('#ff3141').accessibilityText('删除会话')
        .onClick(() => this.remove(session))
    }
  }

  private remove(record: SessionRecord): void {
    AlertDialog.show({ title: '移除此配对？', message: '同时清除本机凭证、缓存与提醒，电脑端会话继续运行。',
      primaryButton: { value: '移除', action: () => this.onRemove(record.sessionId) },
      secondaryButton: { value: '取消', action: () => {} } })
  }
  private changePage(): void {
    if (!this.settings) {
      this.listOffset = this.scroller.currentOffset().yOffset
      this.settings = true
    } else if (this.initialSettings) {
      this.onExitSettings()
    } else {
      this.settings = false
    }
  }
  private turnIcon(status: string): Resource {
    if (status === '等待处理') return $r('app.media.session_need_check')
    if (status === '进行中') return $r('app.media.session_processing')
    if (status === '已完成') return $r('app.media.session_complete')
    return $r('app.media.session_interrupted')
  }

  private toggleDirectories(): void {
    const keys = this.groups.filter((pc: PCGroup) => this.selectedPC === 'all' || pc.key === this.selectedPC)
      .flatMap((pc: PCGroup) => pc.projects.map((project: ProjectGroup) => project.key))
    this.collapsed = keys.every((key: string) => this.collapsed.includes(key)) ? [] : keys
    this.updateRows()
  }

  @Builder
  private sessionRow(session: SessionRecord): void {
    Stack({ alignContent: Alignment.Start }) {
      Column() {
        Row({ space: 6 }) {
          Circle().width(7).height(7).fill(session.status === 'connected' ? '#31a577' : '#d13d43')
            .accessibilityText(session.status === 'connected' ? '已连接' : statusLabel(session))
          Text(titleOf(session)).fontSize(16).fontWeight(FontWeight.Medium).maxLines(1)
            .textOverflow({ overflow: TextOverflow.Ellipsis }).layoutWeight(1)
          if (session.unread) { Circle().width(7).height(7).fill(PRIMARY).accessibilityText('未读') }
          Text(formatActivityTime(session.lastActivityAt ?? session.createdAt ?? '')).fontSize(12)
            .fontColor('#748198').maxLines(1).flexShrink(0)
        }.width('100%')
        Row({ space: 6 }) {
          Text(session.preview ?? '暂无会话内容').fontSize(13).fontColor(SECONDARY).maxLines(1)
            .textOverflow({ overflow: TextOverflow.Ellipsis }).layoutWeight(1)
          if (session.runtimeStatus && session.runtimeStatus !== '空闲' && session.status !== 'needsPairing') {
            Image(this.turnIcon(session.runtimeStatus)).width(16).height(16)
              .accessibilityText(session.status === 'connected' ? session.runtimeStatus : `上次同步：${session.runtimeStatus}`)
          }
        }.width('100%').padding({ top: 5 })
      }.width('100%').padding({ left: 16, right: 12, top: 12, bottom: 14 }).alignItems(HorizontalAlign.Start)
      if (this.recentlyViewedId === session.sessionId) {
        Column().width(3).height(26).backgroundColor(PRIMARY).borderRadius(3).hitTestBehavior(HitTestMode.None)
      }
    }.id(`session-item-${session.sessionId}`).width('100%')
      .onClick(() => this.onOpen(session.sessionId))
      .backgroundColor(this.recentlyViewedId === session.sessionId ? $r('app.color.surface_selected') : $r('app.color.surface'))
  }
  build() {
    Stack({ alignContent: Alignment.Bottom }) {
    Column() {
      Row() {
        if (this.settings) {
          Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(24).height(24) }.width(44).height(44).padding(0).fontSize(30).fontColor(SECONDARY)
            .backgroundColor($r('app.color.surface')).borderRadius(22).accessibilityText('返回').onClick(() => this.changePage())
          Text('设置').fontSize(18).fontWeight(FontWeight.Medium).layoutWeight(1).textAlign(TextAlign.Center)
          HomeButton({ onOpen: this.onHome })
        } else {
          Button({ type: ButtonType.Normal }) { Image($r('app.media.nav_back')).width(24).height(24) }
            .width(44).height(44).padding(0).backgroundColor($r('app.color.surface')).borderRadius(22)
            .accessibilityText('返回').onClick(() => this.onHome())
          Text('ccBuddy').fontSize(24).fontWeight(FontWeight.Bolder).layoutWeight(1).textAlign(TextAlign.Center)
          HomeButton({ onOpen: this.onHome })
        }
      }.padding({ left: 16, right: 16 }).height(56).width('100%')
      if (this.settings) {
        Scroll() {
          Column({ space: 12 }) {
            Row({ space: 12 }) {
              Image($r('app.media.demo_introduce')).width(24).height(24)
              Text('体验演示（无需连接电脑）').fontSize(16).layoutWeight(1)
              Text('›').fontSize(24).fontColor(SECONDARY)
            }.width('100%').constraintSize({ minHeight: 46 }).padding({ left: 12, right: 12, top: 4, bottom: 4 }).backgroundColor($r('app.color.surface')).borderRadius(16).onClick(() => this.onDemo())
            Column({ space: 6 }) {
              Text('调试').fontSize(12).fontColor(SECONDARY).width('100%')
              Column({ space: 4 }) {
                Row() {
                  Text('调试日志').fontSize(16).layoutWeight(1)
                  Toggle({ type: ToggleType.Switch, isOn: this.debugEnabled }).onChange((value: boolean) => this.onDebug(value))
                }.width('100%')
                Text('开启后显示悬浮入口；日志始终在本次运行中记录，最多保留 200 条。').fontSize(12).fontColor(SECONDARY).width('100%')
              }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%').backgroundColor($r('app.color.surface')).borderRadius(16).alignItems(HorizontalAlign.Start)
            }.width('100%')
            Column({ space: 6 }) {
              Text('本地保存说明').fontSize(12).fontColor(SECONDARY).width('100%')
              Column({ space: 6 }) {
                Text('配对与设备').fontSize(15).fontWeight(FontWeight.Medium).width('100%')
                Text('配对凭证保存在系统安全存储；移除配对不会关闭电脑上的会话。').fontSize(13).fontColor(SECONDARY).width('100%')
                Divider().color($r('app.color.border'))
                Text('消息与状态').fontSize(15).fontWeight(FontWeight.Medium).width('100%')
                Text('保存最近已收到的消息和目录；每会话最多 200 项/2 MiB，总计 20 MiB，保留 30 天。不缓存图片原件，不支持离线发送。').fontSize(13).fontColor(SECONDARY).width('100%')
                Divider().color($r('app.color.border'))
                Row({ space: 8 }) {
                  Image($r('app.media.clear_cache')).width(24).height(24)
                  Text('清除离线缓存').fontSize(15).layoutWeight(1)
                  Button('清除', { type: ButtonType.Normal }).height(44).fontColor(PRIMARY).backgroundColor(Color.Transparent).onClick(() => {
                    AlertDialog.show({ title: '清除本机消息缓存？', message: '配对仍保留；未缓存的历史无法从电脑补回。', primaryButton: { value: '清除', action: () => this.onClearCache() }, secondaryButton: { value: '取消', action: () => {} } })
                  })
                }.width('100%')
              }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%').backgroundColor($r('app.color.surface')).borderRadius(16).alignItems(HorizontalAlign.Start)
            }.width('100%')
            Column({ space: 4 }) {
              Row() {
                Text('在线会话提醒').fontSize(16).layoutWeight(1)
                Toggle({ type: ToggleType.Switch, isOn: this.notificationsEnabled }).selectedColor(PRIMARY).onChange((enabled: boolean) => { if (enabled !== this.notificationsEnabled) this.onNotifications() })
              }.width('100%')
              Text('接收在线会话提醒。').fontSize(12).fontColor(SECONDARY).width('100%')
              Row() {
                Text('后台推送').fontSize(16).layoutWeight(1)
                Toggle({ type: ToggleType.Switch, isOn: this.pushConsent }).selectedColor(PRIMARY).onChange((enabled: boolean) => { if (enabled !== this.pushConsent) this.onEnablePush() })
              }.width('100%')
              Text(this.pushStatus).fontSize(12).fontColor(SECONDARY).width('100%')
            }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%').backgroundColor($r('app.color.surface')).borderRadius(16).alignItems(HorizontalAlign.Start)

            Column({ space: 4 }) {
              Text('关于').fontSize(15).fontWeight(FontWeight.Medium).width('100%')
              Text(APP_BUILD_LABEL).fontSize(12).fontColor(SECONDARY).width('100%')
            }.padding({ left: 12, right: 12, top: 8, bottom: 8 }).width('100%')
              .backgroundColor($r('app.color.surface')).borderRadius(16)
          }.padding({ left: 16, right: 16, top: 12, bottom: 24 }).width('100%').alignItems(HorizontalAlign.Start)
        }.layoutWeight(1).width('100%').align(Alignment.TopStart).transition(pageSlide())
      } else {
        Refresh({ refreshing: $$this.refreshing }) {
        List({ space: 8, scroller: this.scroller }) {
          ListItem() { Row({ space: 10 }) { Text('会话列表').fontSize(28).fontWeight(FontWeight.Bold); Text(`${this.sessions.length} 个会话`).fontSize(12).fontColor(SECONDARY).padding({ left: 10, right: 10, top: 6, bottom: 6 }).backgroundColor($r('app.color.surface_blue')).borderRadius(16) }.width('100%').padding({ top: 6, bottom: 12 }).onClick(() => this.toggleDirectories()) }
          if (this.groups.length === 0) { ListItem() { Text('扫码连接电脑上的 Claude 会话').fontSize(14).fontColor(SECONDARY).padding(24) } }
          LazyForEach(this.listSource, (row: ProjectListRow) => {
            if (row.session === undefined) {
              ListItem() {
                if (row.kind === 'pc' && row.pc !== undefined) {
                  Row({ space: 12 }) {
                    Stack() { Image($r('app.media.pc_host')).width(36).height(36).fillColor(Color.White) }
                      .width(44).height(44).borderRadius(10)
                      .linearGradient({ angle: 145, colors: [['#4854df', 0], ['#126ded', 1]] })
                    Column({ space: 4 }) {
                      Text(row.pc.label).fontSize(16).fontWeight(FontWeight.Medium).width('100%')
                        .maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis })
                      Text(row.pc.sessions[0].pairToken ? 'Token：' + maskPairToken(row.pc.sessions[0].pairToken) : '需重新配对')
                        .fontSize(12).fontColor(SECONDARY).width('100%').maxLines(1)
                    }.layoutWeight(1).alignItems(HorizontalAlign.Start)
                    Column({ space: 4 }) {
                      Text(this.computerAddress(row.pc)).fontSize(12).fontColor(SECONDARY).maxLines(1)
                        .textOverflow({ overflow: TextOverflow.Ellipsis }).constraintSize({ maxWidth: 120 })
                      Row({ space: 4 }) {
                        Circle().width(5).height(5).fill(row.connecting ? '#c88a27' : row.pc.connected > 0 ? $r('app.color.badge_success_text') : SECONDARY)
                        Text(row.connecting ? '连接中' : row.pc.connected > 0 ? '已连接' : '暂未连接').fontSize(11)
                          .fontColor(row.connecting ? $r('app.color.warning') : row.pc.connected > 0 ? $r('app.color.badge_success_text') : SECONDARY)
                      }.padding({ left: 8, right: 8, top: 3, bottom: 3 })
                        .backgroundColor(row.connecting ? $r('app.color.surface_warning') : row.pc.connected > 0 ? $r('app.color.badge_success_surface') : $r('app.color.surface_subtle'))
                        .borderRadius(12)
                    }.alignItems(HorizontalAlign.End)
                  }.padding({ left: 16, right: 16, top: 8, bottom: 8 }).width('100%').backgroundColor($r('app.color.surface')).borderRadius(16)
                } else if (row.kind === 'project' && row.pc !== undefined && row.project !== undefined) {
                  Row({ space: 8 }) {
                    Column({ space: 4 }) {
                      Row({ space: 6 }) { Image(this.collapsed.indexOf((row.project as ProjectGroup).key) >= 0 ? $r('app.media.directory_collapse') : $r('app.media.directory_expand')).width(16).height(16); Text((row.project as ProjectGroup).title).fontSize(15).fontWeight(FontWeight.Medium).layoutWeight(1).maxLines(1) }.width('100%')
                      Text(row.project.cwd.length > 0 ? row.project.cwd : '工作目录未知').fontSize(12)
                        .fontColor(SECONDARY).maxLines(1).ellipsisMode(EllipsisMode.CENTER)
                        .textOverflow({ overflow: TextOverflow.Ellipsis }).width('100%')
                    }.layoutWeight(1).alignItems(HorizontalAlign.Start).onClick(() => {
                      const project = row.project as ProjectGroup
                      this.collapsed = this.collapsed.indexOf(project.key) >= 0 ? this.collapsed.filter((key: string) => key !== project.key) : this.collapsed.concat([project.key])
                      this.updateRows()
                    })
                    if ((row.project as ProjectGroup).cwd.length > 0 &&
                      (row.project as ProjectGroup).sessions.some((record: SessionRecord) => record.pcId === (row.pc as PCGroup).key)) {
                      Button('＋', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(25)
                        .fontColor(PRIMARY).borderRadius(12).clip(true).stateEffect(false)
                        .stateStyles({ normal: { .backgroundColor('#00000000') }, pressed: { .backgroundColor('#e6effe') } })
                        .accessibilityText('在此目录新建会话')
                        .enabled(!this.projectCreating)
                        .onClick(() => this.onCreateDirectory(row.pc as PCGroup, row.project as ProjectGroup))
                    }
                  }.width('100%').padding({ top: 4 })
                }
              }
            } else {
              ListItem() { this.sessionRow(row.session) }
                .swipeAction({ end: this.sessionActions(row.session) })
                .border({ width: 1, color: $r('app.color.border') }).borderRadius(16).clip(true)
            }
          }, (row: ProjectListRow) => row.session === undefined ? row.key :
            JSON.stringify([row.key, row.session.status, row.session.runtimeStatus]))
        }.layoutWeight(1).width('100%').padding({ left: 16, right: 16 }).cachedCount(3).transition(pageSlide(true))
          .onAppear(() => { this.listOffset = this.scrollOffset; this.scroller.scrollTo({ xOffset: 0, yOffset: this.scrollOffset }) })
          .onScroll(() => this.onScrollOffset(this.scroller.currentOffset().yOffset))
        }.layoutWeight(1).width('100%').onRefreshing(() => {
          this.onRefresh()
          setTimeout(() => { this.refreshing = false }, 500)
        })
      }
      if (!this.settings) {
        Row() { Button('＋ 新会话', { type: ButtonType.Normal }).height(52).width('100%').fontSize(16).fontColor(Color.White)
          .backgroundColor($r('app.color.primary_fill')).borderRadius(16).onClick(() => this.onAdd()) }.width('100%').padding({ left: 16, right: 16, top: 8, bottom: 5 })
          .onAreaChange((_previous: Area, current: Area) => this.onDebugActionsTop(Number(current.globalPosition.y)))
          .onDisAppear(() => this.onDebugActionsTop(0))
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
      }.padding(16).width('100%').backgroundColor($r('app.color.surface')).borderRadius({ topLeft: 24, topRight: 24 })
    }
    }.width('100%').height('100%').parallelGesture(PanGesture({ direction: PanDirection.Right, distance: 12 })
      .onActionStart((event: GestureEvent) => {
        this.beganAtEdge = event.fingerList.length > 0 && event.fingerList[0].localX - event.offsetX <= 24
      }).onActionEnd((event: GestureEvent) => {
        if (this.beganAtEdge && event.offsetX >= 65 && !this.backHandler()) this.onHome()
        this.beganAtEdge = false
      }))
  }
}
