import { PRIMARY, SECONDARY } from '../ui/styles'
import { ActionMenu, type ActionMenuItem } from './ActionMenu'
import { curves } from '@kit.ArkUI'
import type { Attachment, PermissionMode } from '../protocol/types'
import { utf8Bytes, type ComposerDraft, emptyDraft } from '../session/attachments'

@Component
export struct MessageComposer {
  @Prop textOnly: boolean = false
  @Prop draft: ComposerDraft = emptyDraft()
  @Prop composerEnabled: boolean = false
  @Prop createSessionAvailable: boolean = false
  @Prop createSessionEnabled: boolean = false
  @Prop running: boolean = false
  @Prop cancelEnabled: boolean = false
  @Prop speechActive: boolean = false
  @Prop selecting: boolean = false
  @Prop permissionMode: PermissionMode = 'default'
  @Prop permissionEnabled: boolean = false
  onPermissionMode: (mode: PermissionMode) => void = () => {}
  onSend: () => void = () => {}
  onDraft: (text: string) => void = () => {}
  onSelect: (kind: string) => void = () => {}
  onRemoveAttachment: (index: number) => void = () => {}
  onSpeech: () => void = () => {}
  onStopSpeech: () => void = () => {}
  onCancelSpeech: () => void = () => {}
  onCancel: () => void = () => {}
  onCreateSession: () => void = () => {}
  @State private preview: string = ''
  private addController: CustomDialogController = new CustomDialogController({
    builder: ActionMenu({ title: '添加', items: this.addItems(),
      onClose: (): void => { this.addController.close() } }),
    customStyle: true, alignment: DialogAlignment.Bottom, autoCancel: true,
    openAnimation: { curve: curves.springMotion(0.25, 1) },
    closeAnimation: { curve: curves.springMotion(0.25, 1) },
  })
  private permissionController: CustomDialogController = new CustomDialogController({
    builder: ActionMenu({ title: '权限模式', items: this.permissionItems(),
      onClose: (): void => { this.permissionController.close() } }),
    customStyle: true, alignment: DialogAlignment.Bottom, autoCancel: true,
    openAnimation: { curve: curves.springMotion(0.25, 1) },
    closeAnimation: { curve: curves.springMotion(0.25, 1) },
  })

  aboutToDisappear(): void {
    this.addController.close()
    this.permissionController.close()
  }
  private addItems(): Array<ActionMenuItem> {
    const items: Array<ActionMenuItem> = []
    if (this.editable()) {
      items.push({ title: '图片', action: () => { if (this.editable()) this.onSelect('photo') } })
      items.push({ title: '拍照', action: () => { if (this.editable()) this.onSelect('camera') } })
      items.push({ title: '文件', action: () => { if (this.editable()) this.onSelect('file') } })
    }
    if (this.createSessionAvailable && this.createSessionEnabled) {
      items.push({ title: '新建会话', action: () => {
        if (this.createSessionAvailable && this.createSessionEnabled) this.onCreateSession()
      } })
    }
    return items
  }
  private selectPermission(mode: PermissionMode): void {
    if (this.permissionEnabled) this.onPermissionMode(mode)
  }
  private permissionItems(): Array<ActionMenuItem> {
    return [
      { title: '默认', action: () => this.selectPermission('default') },
      { title: '自动接受编辑', action: () => this.selectPermission('acceptEdits') },
      { title: '计划', action: () => this.selectPermission('plan') },
      { title: '绕过权限', action: () => this.selectPermission('bypassPermissions') },
    ]
  }

  private editable(): boolean { return !this.draft.sending && !this.speechActive && !this.selecting }
  private permissionLabel(): string {
    if (this.permissionMode === 'acceptEdits') return '自动接受编辑'
    if (this.permissionMode === 'plan') return '计划'
    if (this.permissionMode === 'bypassPermissions') return '绕过权限'
    return '默认'
  }
  build() {
    Column({ space: 8 }) {
      if (this.draft.error.length > 0) { Text(this.draft.error).fontSize(12).fontColor('#a54b00').width('100%') }
      if (this.preview.length > 0) {
        Image(this.preview).height(180).width('100%').objectFit(ImageFit.Contain).onClick(() => { this.preview = '' })
        Text('点按图片收起预览').fontSize(12)
      }
      if (this.draft.attachments.length > 0) {
        Scroll() {
          Row({ space: 8 }) {
            ForEach(this.draft.attachments, (item: Attachment, index: number) => {
              Column({ space: 4 }) {
                if (item.mediaType.startsWith('image/')) {
                  Image(`data:${item.mediaType};base64,${item.data}`).width(64).height(64).objectFit(ImageFit.Cover)
                    .onClick(() => { this.preview = `data:${item.mediaType};base64,${item.data}` })
                }
                Text(item.name).fontSize(12).maxLines(1).width(100)
                Button('移除', { type: ButtonType.Normal }).height(44).fontSize(12).fontColor('#d4380d').backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).enabled(this.editable()).onClick(() => this.onRemoveAttachment(index))
              }
            }, (item: Attachment, index: number) => `${index}-${item.name}-${item.mediaType}`)
          }
        }.scrollable(ScrollDirection.Horizontal).width('100%')
      }
      TextArea({ text: this.draft.text, placeholder: '输入消息，或添加附件' })
        .id('send-input').height(64).fontSize(15).backgroundColor(Color.Transparent).padding({ left: 4, right: 4, top: 6, bottom: 4 }).enabled(this.editable()).onChange((text: string) => this.onDraft(text))
      if (utf8Bytes(this.draft.text) > 1024 * 1024) { Text('文本超出 1 MiB 上限').fontColor('#b3261e').fontSize(12) }
      Row({ space: 0 }) {
        if (this.speechActive) {
          Button('完成录音', { type: ButtonType.Normal }).height(44).fontSize(13).fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).onClick(() => this.onStopSpeech())
          Button('取消', { type: ButtonType.Normal }).height(44).fontSize(13).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).onClick(() => this.onCancelSpeech())
        } else if (!this.textOnly) {
          Button('+', { type: ButtonType.Normal }).width(44).height(44).fontSize(26).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true)
            .accessibilityText('添加附件或新建会话').enabled(this.editable() || (this.createSessionAvailable && this.createSessionEnabled)).onClick(() => {
              this.addController.open()
            })
          Button('语音', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(13).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true)
            .enabled(this.editable()).onClick(() => this.onSpeech())
        }
        if (!this.speechActive && !this.textOnly) {
          Button(this.permissionLabel(), { type: ButtonType.Normal }).height(44).constraintSize({ maxWidth: 110 }).padding({ left: 6, right: 6 }).fontSize(12).fontColor(SECONDARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).enabled(this.permissionEnabled).onClick(() => {
            this.permissionController.open()
          })
        }
        Blank()
        if (this.running) {
          LoadingProgress().width(16).height(16)
          Button('■', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(18).fontColor('#252525').backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).accessibilityText('停止当前任务').enabled(this.cancelEnabled).onClick(() => this.onCancel())
        } else {
          Button(this.draft.sending ? '…' : '↑', { type: ButtonType.Normal }).width(44).height(44).padding(0).fontSize(27).fontColor(PRIMARY).backgroundColor(Color.Transparent).borderRadius(8).hoverEffect(HoverEffect.Highlight).clip(true).accessibilityText(this.draft.sending ? '等待受理' : '发送消息')
            .enabled(this.composerEnabled && this.editable() && utf8Bytes(this.draft.text) <= 1024 * 1024 && (this.draft.text.trim().length > 0 || this.draft.attachments.length > 0))
            .onClick(() => this.onSend())
        }
      }.width('100%')
    }.width('100%').padding({ left: 8, right: 8, top: 6, bottom: 2 }).alignItems(HorizontalAlign.Start).backgroundColor('#ffffff').borderRadius(26).border({ width: 1, color: '#e1e3e7' })
  }
}
