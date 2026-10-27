import { pasteboard } from '@kit.BasicServicesKit'
import { fileIo, fileUri } from '@kit.CoreFileKit'
import { systemShare } from '@kit.ShareKit'
import { nativeLifecycle } from '../native/lifecycle'
import { debugLog, logDescription, type LogEntry } from '../debug/log'
@Component
export struct DebugPanel {
  onClose: () => void = () => {}
  @State private entries: Array<LogEntry> = []
  @State private logEnabled: boolean = true
  @State private feedback: string = ''
  private unsubscribe: (() => void) | null = null
  aboutToAppear(): void { this.refresh(); this.unsubscribe = debugLog.subscribe(() => this.refresh()) }
  aboutToDisappear(): void { this.unsubscribe?.() }
  private refresh(): void { this.entries = debugLog.list(); this.logEnabled = debugLog.enabled }
  private copy(): void {
    pasteboard.getSystemPasteboard().setData(pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, debugLog.text())).then(() => { this.feedback = '已复制脱敏日志' }).catch(() => { this.feedback = '复制失败' })
  }
  private async share(): Promise<void> {
    const context = nativeLifecycle.context
    if (context === null) return
    const path = `${context.cacheDir}/cmr-debug.log`
    try {
      const file = fileIo.openSync(path, fileIo.OpenMode.CREATE | fileIo.OpenMode.TRUNC | fileIo.OpenMode.WRITE_ONLY)
      try { fileIo.writeSync(file.fd, debugLog.text()) } finally { fileIo.closeSync(file) }
      const data = new systemShare.SharedData({ utd: 'general.plain-text', uri: fileUri.getUriFromPath(path) })
      const controller = new systemShare.ShareController(data)
      await controller.show(context, { previewMode: systemShare.SharePreviewMode.DETAIL })
      this.feedback = '已打开分享'
    } catch (_) { this.feedback = '导出失败，可复制日志' }
    finally { try { fileIo.unlinkSync(path) } catch (_) {} }
  }
  build() {
    Column({ space: 4 }) {
      Row({ space: 4 }) {
        Text('调试日志').fontSize(16).layoutWeight(1)
        Toggle({ type: ToggleType.Switch, isOn: this.logEnabled }).onChange((value: boolean) => debugLog.setEnabled(value))
        Button('关闭').fontSize(12).height(36).onClick(() => this.onClose())
      }
      Row({ space: 4 }) {
        Button('清空').fontSize(12).height(32).onClick(() => debugLog.clear())
        Button('复制').fontSize(12).height(32).onClick(() => this.copy())
        Button('导出').fontSize(12).height(32).onClick(() => { void this.share() })
        Text(`${this.entries.length}/200 · 仅内存`).fontSize(11)
      }
      if (this.feedback.length > 0) { Text(this.feedback).fontSize(12) }
      Scroll() {
        Column({ space: 2 }) {
          ForEach(this.entries, (item: LogEntry) => {
            Column({ space: 2 }) {
              Text(`${new Date(item.time).toLocaleTimeString()} · ${item.code}`).fontSize(11).fontColor('#666666').width('100%')
              Text(`${logDescription(item.code)} ${item.detail}`).fontSize(12).width('100%')
            }.padding(6).backgroundColor('#f3f4f6').borderRadius(4).width('100%')
          }, (item: LogEntry, index: number) => `${index}-${item.time}`)
          if (this.entries.length === 0) { Text('暂无日志；不采集凭证、会话正文或原始异常').fontSize(12).padding(12) }
        }.width('100%')
      }.layoutWeight(1).width('100%')
    }.padding(10).height('75%').width('100%').backgroundColor('#ffffff').borderRadius(16)
  }
}
