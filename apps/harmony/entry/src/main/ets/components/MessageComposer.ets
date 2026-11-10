import { PRIMARY, SECONDARY } from '../ui/styles'
import type { Attachment, PermissionMode } from '../protocol/types'
import { utf8Bytes, type ComposerDraft, emptyDraft } from '../session/attachments'
import { messageSelection } from '../ui/messageSelection'

@Component
export struct MessageComposer {
  @Prop textOnly: boolean = false
  @Prop @Watch('draftChanged') draft: ComposerDraft = emptyDraft()
  @Prop composerEnabled: boolean = false
  @Prop createSessionAvailable: boolean = false
  @Prop createSessionEnabled: boolean = false
  @Prop running: boolean = false
  @Prop cancelEnabled: boolean = false
  @Prop speechActive: boolean = false
  @Prop speechFinishing: boolean = false
  @Prop selecting: boolean = false
  @Prop permissionMode: PermissionMode = 'default'
  @Prop permissionEnabled: boolean = false
  @Prop queueCount: number = 0
  onQueue: () => void = () => {}
  onPreviewImage: (value: string) => void = () => {}
  onPermissionMode: (mode: PermissionMode) => void = () => {}
  onSend: () => void = () => {}
  onDraft: (text: string) => void = () => {}
  onSelect: (kind: string) => void = () => {}
  onRemoveAttachment: (index: number) => void = () => {}
  onClearAttachments: () => void = () => {}
  onSpeech: () => void = () => {}
  onStopSpeech: () => void = () => {}
  onCancelSpeech: () => void = () => {}
  onCancel: () => void = () => {}
  onCreateSession: () => void = () => {}
  @State private textBytes: number = 0
  private countedText: string = ''
  @State private addMenuOpen: boolean = false
  @State private permissionMenuOpen: boolean = false

  aboutToAppear(): void { this.draftChanged() }
  private draftChanged(): void {
    if (this.countedText !== this.draft.text) {
      this.countedText = this.draft.text
      this.textBytes = utf8Bytes(this.draft.text)
    }
  }
  private chooseAttachment(kind: string): void {
    this.addMenuOpen = false
    if (this.editable() && this.composerEnabled) this.onSelect(kind)
  }
  private selectPermission(mode: PermissionMode): void {
    this.permissionMenuOpen = false
    if (this.permissionEnabled) this.onPermissionMode(mode)
  }
  @Builder
  private attachmentMenu() {
    Menu() {
      MenuItem({ content: '相机' }).onClick(() => this.chooseAttachment('camera'))
      MenuItem({ content: '照片' }).onClick(() => this.chooseAttachment('photo'))
      MenuItem({ content: '文件' }).onClick(() => this.chooseAttachment('file'))
    }
  }
  @Builder
  private permissionMenu() {
    Menu() {
      MenuItem({ content: this.permissionMode === 'default' ? '默认  ✓' : '默认' }).onClick(() => this.selectPermission('default'))
      MenuItem({ content: this.permissionMode === 'acceptEdits' ? '自动接受编辑  ✓' : '自动接受编辑' }).onClick(() => this.selectPermission('acceptEdits'))
      MenuItem({ content: this.permissionMode === 'plan' ? '计划  ✓' : '计划' }).onClick(() => this.selectPermission('plan'))
      MenuItem({ content: this.permissionMode === 'bypassPermissions' ? '绕过权限  ✓' : '绕过权限' }).onClick(() => this.selectPermission('bypassPermissions'))
    }
  }

  private editable(): boolean { return !this.draft.sending && !this.speechActive && !this.selecting }
  private permissionLabel(): string {
    if (this.permissionMode === 'acceptEdits') return '自动接受编辑'
    if (this.permissionMode === 'plan') return '计划'
    if (this.permissionMode === 'bypassPermissions') return '绕过权限'
    return '默认'
  }
  build() {
    Column({ space: 6 }) {
      if (this.draft.error.length > 0) { Text(this.draft.error).fontSize(12).fontColor('#a54b00').width('100%') }
      if (this.draft.attachments.length > 0) {
        Text(`已选 ${this.draft.attachments.length} 个附件（每个最多 4 MiB，合计 8 MiB）`).fontSize(12).fontColor(SECONDARY).width('100%')
        Scroll() {
          Row({ space: 12 }) {
            ForEach(this.draft.attachments, (item: Attachment, index: number) => {
              Column({ space: 3 }) {
                Stack({ alignContent: Alignment.TopEnd }) {
                  if (item.mediaType.startsWith('image/')) {
                    Image(`data:${item.mediaType};base64,${item.data}`).width(76).height(76).objectFit(ImageFit.Cover).borderRadius(8)
                      .onClick(() => this.onPreviewImage(`data:${item.mediaType};base64,${item.data}`))
                  } else {
                    Text(item.name).fontSize(11).maxLines(3).textOverflow({ overflow: TextOverflow.Ellipsis })
                      .width(76).height(76).padding(6).backgroundColor('#eef1f5').borderRadius(8)
                  }
                  Button('×', { type: ButtonType.Normal }).width(25).height(25).padding(0).fontSize(20).fontColor(Color.White)
                    .backgroundColor('#60656d').border({ width: 2, color: Color.White }).borderRadius(13)
                    .accessibilityText(`移除 ${item.name}`).enabled(this.editable()).onClick(() => this.onRemoveAttachment(index))
                }.width(76).height(76)
                Text(item.name).fontSize(11).fontColor(SECONDARY).maxLines(1).textOverflow({ overflow: TextOverflow.Ellipsis }).width(76)
              }
            }, (item: Attachment, index: number) => `${index}-${item.name}-${item.mediaType}`)
          }.padding({ top: 4, right: 4 })
        }.scrollable(ScrollDirection.Horizontal).width('100%')
      }
      if (this.speechActive) {
        Text(this.speechFinishing ? '正在识别，请稍候…' : '正在聆听，点击语音按钮结束').fontSize(12).fontColor(SECONDARY)
      }
      if (this.textBytes > 1024 * 1024) { Text('文本超出 1 MiB 上限').fontColor('#b3261e').fontSize(12) }
      Column({ space: 4 }) {
        TextArea({ text: this.draft.text, placeholder: this.textOnly ? '输入消息体验模拟回复' : '输入消息或添加附件' })
          .id('send-input').height('auto').constraintSize({ minHeight: 36, maxHeight: 100 }).fontSize(15).backgroundColor(Color.Transparent)
          .padding({ left: 2, right: 2, top: 2, bottom: 4 }).enabled(this.editable() && this.composerEnabled)
          .onChange((text: string) => this.onDraft(text))
          .onFocus(() => { messageSelection.close() })
        Row({ space: 2 }) {
          if (!this.textOnly) {
            Button({ type: ButtonType.Normal }) {
              Image($r('app.media.composer_paperclip')).width(20).height(20)
            }.width(40).height(40).padding(0).backgroundColor(Color.Transparent).accessibilityText('添加附件')
              .enabled(this.editable() && this.composerEnabled).onClick(() => { this.addMenuOpen = true })
              .bindMenu(this.addMenuOpen, this.attachmentMenu, { placement: Placement.TopLeft, onDisappear: () => { this.addMenuOpen = false } })
            Button({ type: ButtonType.Normal }) {
              Image($r('app.media.composer_permission')).width(20).height(20)
            }.width(40).height(40).padding(0).backgroundColor(Color.Transparent).accessibilityText(`权限模式：${this.permissionLabel()}`)
              .enabled(this.permissionEnabled).onClick(() => { this.permissionMenuOpen = true })
              .bindMenu(this.permissionMenuOpen, this.permissionMenu, { placement: Placement.TopLeft, onDisappear: () => { this.permissionMenuOpen = false } })
            if (this.queueCount > 0) {
              Button(`☰ ${this.queueCount}`, { type: ButtonType.Normal }).height(40).padding({ left: 4, right: 4 }).fontSize(13)
                .fontColor(PRIMARY).backgroundColor(Color.Transparent).accessibilityText(`查看排队队列（${this.queueCount}）`).onClick(() => this.onQueue())
            }
            if (this.draft.attachments.length > 0) {
              Button({ type: ButtonType.Normal }) {
                Image($r('app.media.composer_clear')).width(19).height(19)
              }.width(40).height(40).padding(0)
                .backgroundColor(Color.Transparent).accessibilityText('清除所有待发送的图片和附件').enabled(this.editable()).onClick(() => this.onClearAttachments())
            }
          }
          Blank()
          if (!this.textOnly) {
            Button({ type: ButtonType.Normal }) {
              Image($r('app.media.composer_microphone')).width(20).height(20)
            }.width(40).height(40).padding(0).borderRadius(20).backgroundColor(this.speechActive ? '#e7efff' : Color.Transparent)
              .accessibilityText(this.speechFinishing ? '正在识别语音' : this.speechActive ? '结束语音输入' : '语音输入')
              .enabled(this.composerEnabled && !this.draft.sending && !this.selecting && !this.speechFinishing)
              .onClick(() => { if (this.speechActive) this.onStopSpeech(); else this.onSpeech() })
          }
          if (this.running) {
            Button('■', { type: ButtonType.Normal }).width(32).height(32).padding(0).fontSize(14).fontColor(Color.White)
              .backgroundColor('#192437').borderRadius(16).accessibilityText('停止当前任务').enabled(this.cancelEnabled).onClick(() => this.onCancel())
          }
          if (!this.running || this.draft.text.trim().length > 0 || this.draft.attachments.length > 0) {
            Button({ type: ButtonType.Normal }) {
              Image($r('app.media.composer_send')).width(20).height(20)
            }.width(32).height(32).padding(0).backgroundColor(PRIMARY).borderRadius(16)
              .accessibilityText(this.draft.sending ? '等待受理' : '发送消息')
              .enabled(this.composerEnabled && this.editable() && this.textBytes <= 1024 * 1024 && (this.draft.text.trim().length > 0 || this.draft.attachments.length > 0))
              .onClick(() => this.onSend())
          }
        }.width('100%')
      }.width('100%').padding({ left: 12, right: 12, top: 10, bottom: 8 }).alignItems(HorizontalAlign.Start)
        .backgroundColor('#ffffff').borderRadius(26).border({ width: 1, color: '#e1e3e7' })
    }.width('100%').alignItems(HorizontalAlign.Start)
  }
}
