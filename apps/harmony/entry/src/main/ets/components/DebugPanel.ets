import { pasteboard } from '@kit.BasicServicesKit'
import { fileIo, fileUri } from '@kit.CoreFileKit'
import { systemShare } from '@kit.ShareKit'
import { nativeLifecycle } from '../native/lifecycle'
import { debugLog, logDescription, type LogEntry } from '../debug/log'
@Component
export struct DebugPanel {
  onClose: () => void = () => {}
  @State private entries: Array<LogEntry> = []
  @State private feedback: string = ''
  private unsubscribe: (() => void) | null = null
  aboutToAppear(): void { this.refresh(); this.unsubscribe = debugLog.subscribe(() => this.refresh()) }
  aboutToDisappear(): void { this.unsubscribe?.() }
  private refresh(): void { this.entries = debugLog.list() }
  private copy(): void {
    this.feedback = '正在复制…'
    pasteboard.getSystemPasteboard().setData(pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, debugLog.text())).then(() => { this.feedback = '已复制脱敏日志' }).catch(() => { this.feedback = '复制失败' })
  }
  private async share(): Promise<void> {
    const context = nativeLifecycle.context
    if (context === null) return
    this.feedback = '正在导出…'
    const path = `${context.cacheDir}/cmr-debug.log`
    try {
      const file = await fileIo.open(path, fileIo.OpenMode.CREATE | fileIo.OpenMode.TRUNC | fileIo.OpenMode.WRITE_ONLY)
      try { await fileIo.write(file.fd, debugLog.text()) } finally { await fileIo.close(file) }
      const data = new systemShare.SharedData({ utd: 'general.plain-text', uri: fileUri.getUriFromPath(path) })
      const controller = new systemShare.ShareController(data)
      await controller.show(context, { previewMode: systemShare.SharePreviewMode.DETAIL })
      this.feedback = '已打开分享'
    } catch (_) { this.feedback = '导出失败，可复制日志' }
    finally { try { await fileIo.unlink(path) } catch (_) {} }
  }
  private timeOf(timestamp: number): string {
    const date = new Date(timestamp)
    const clock = [date.getHours(), date.getMinutes(), date.getSeconds()]
      .map((value: number) => value.toString().padStart(2, '0')).join(':')
    return `${clock}.${date.getMilliseconds().toString().padStart(3, '0')}`
  }

  private isError(item: LogEntry): boolean {
    return item.code.endsWith('_error') || item.code.endsWith('_timeout') || item.code === 'pairing_invalid'
  }

  @Builder
  private action(label: string, run: () => void): void {
    Button(label, { type: ButtonType.Normal }).height(34).padding({ left: 9, right: 9 })
      .fontSize(12).fontColor($r('app.color.text')).backgroundColor($r('app.color.surface_subtle'))
      .border({ width: 1, color: $r('app.color.border'), radius: 6 }).onClick(() => run())
  }

  build() {
    Column({ space: 4 }) {
      Row({ space: 8 }) {
        Text(`调试日志（${this.entries.length}/200）`).fontSize(14).fontWeight(FontWeight.Bold).layoutWeight(1)
        this.action('关闭', () => this.onClose())
      }.width('100%')
      Row({ space: 6 }) {
        this.action('清空日志', () => { debugLog.clear(); this.feedback = '日志已清空' })
        this.action('复制日志', () => this.copy())
        this.action('导出日志', () => { void this.share() })
      }.width('100%').padding({ bottom: 4 })
      Divider().color($r('app.color.border'))
      if (this.feedback.length > 0) { Text(this.feedback).fontSize(12).width('100%') }
      Scroll() {
        Column() {
          ForEach(this.entries, (item: LogEntry) => {
            Column({ space: 3 }) {
              Text(`${this.timeOf(item.time)}   ${this.isError(item) ? 'ERROR' : 'INFO'} · ${item.code === 'host' ? 'PC' : 'WS'}   ${item.code}`)
                .fontSize(10).fontColor($r('app.color.text_secondary')).width('100%')
              Text(`${logDescription(item.code)} ${item.detail}`).fontSize(12).width('100%').copyOption(CopyOptions.LocalDevice)
            }.padding({ left: 6, right: 6, top: 7, bottom: 7 }).width('100%')
              .borderWidth({ left: 2, bottom: 1 })
              .borderColor({ left: this.isError(item) ? '#cb3946' : '#b9c9d9', bottom: $r('app.color.border') })
          }, (item: LogEntry, index: number) => `${index}-${item.time}-${item.code}`)
          if (this.entries.length === 0) {
            Text('暂无日志，请关闭面板后复现操作。').fontSize(12).padding(8).width('100%')
          }
        }.width('100%')
      }.layoutWeight(1).width('100%').align(Alignment.TopStart)
    }.padding({ left: 12, right: 12, top: 10, bottom: 10 }).height('88%')
      .width('100%').backgroundColor($r('app.color.surface')).borderRadius(12).clip(true)
      .hitTestBehavior(HitTestMode.Default)
  }
}
