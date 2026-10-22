import type { Attachment, PermissionMode } from '../protocol/types'
import { utf8Bytes, type ComposerDraft, emptyDraft } from '../session/attachments'

@Component
export struct MessageComposer {
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
                Button('移除').height(44).enabled(this.editable()).onClick(() => this.onRemoveAttachment(index))
              }
            }, (item: Attachment, index: number) => `${index}-${JSON.stringify(item)}`)
          }
        }.scrollable(ScrollDirection.Horizontal).width('100%')
      }
      TextArea({ text: this.draft.text, placeholder: '输入消息，或添加附件' })
        .id('send-input').height(76).enabled(this.editable()).onChange((text: string) => this.onDraft(text))
      if (utf8Bytes(this.draft.text) > 1024 * 1024) { Text('文本超出 1 MiB 上限').fontColor('#b3261e').fontSize(12) }
      Row({ space: 6 }) {
        if (!this.speechActive) {
          Button('图片').height(44).enabled(this.editable()).onClick(() => this.onSelect('photo'))
          Button('拍照').height(44).enabled(this.editable()).onClick(() => this.onSelect('camera'))
          Button('文件').height(44).enabled(this.editable()).onClick(() => this.onSelect('file'))
        }
        if (this.speechActive) {
          Button('完成录音').height(44).onClick(() => this.onStopSpeech())
          Button('取消').height(44).onClick(() => this.onCancelSpeech())
        } else {
          Button('语音').height(44).enabled(this.editable()).onClick(() => this.onSpeech())
        }
      }.width('100%')
      Row({ space: 8 }) {
        Button(this.permissionLabel()).height(44).enabled(this.permissionEnabled).onClick(() => {
          ActionSheet.show({ title: '权限模式', sheets: [
            { title: '默认', action: () => this.onPermissionMode('default') },
            { title: '自动接受编辑', action: () => this.onPermissionMode('acceptEdits') },
            { title: '计划', action: () => this.onPermissionMode('plan') },
            { title: '绕过权限', action: () => this.onPermissionMode('bypassPermissions') },
          ] })
        })
        if (this.createSessionAvailable) {
          Button('+').width(44).height(44).accessibilityText('新建会话')
            .enabled(this.createSessionEnabled).onClick(() => this.onCreateSession())
        }
        Blank()
        if (this.running) {
          LoadingProgress().width(24).height(24)
          Button('停止').height(44).enabled(this.cancelEnabled).onClick(() => this.onCancel())
        } else {
          Button(this.draft.sending ? '等待受理…' : '发送').height(44)
            .enabled(this.composerEnabled && this.editable() && utf8Bytes(this.draft.text) <= 1024 * 1024 && (this.draft.text.trim().length > 0 || this.draft.attachments.length > 0))
            .onClick(() => this.onSend())
        }
      }.width('100%')
    }.width('100%').alignItems(HorizontalAlign.Start)
  }
}
