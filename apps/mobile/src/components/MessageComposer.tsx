import SessionIcon from './SessionIcon'
import TurnLoading from './TurnLoading'
import type { TurnProgress } from '../protocol/types'
import { encodeFile, needsPhotoPreparation, preparePickedFile, validateFiles } from '../native/attachments'
import type { Attachment, AttachmentSendResult } from '../protocol/types'
import { useEffect, useRef, useState, type CSSProperties } from 'react'
import { Button, ImageViewer, Popover, Space, TextArea, type TextAreaRef } from 'antd-mobile'
import { CheckOutline } from 'antd-mobile-icons'
import { MAX_TEXT_BYTES, type PermissionMode } from '../protocol/types'
import { FONT_SIZE_COMPACT, FONT_SIZE_BODY } from './ui'
import './MessageComposer.css'
import { nativeExperience, type ExperiencePort } from '../native/experience'
import { useSpeechDraft } from '../native/useSpeechDraft'

interface MessageComposerProps {
  enabled: boolean
  experience?: ExperiencePort | null
  textOnly?: boolean
  onSendAttachments?: (text: string, attachments: Attachment[]) => Promise<AttachmentSendResult>
  onSend: (text: string) => void
  progress?: TurnProgress | null
  turnWaiting?: boolean
  turnRunning?: boolean
  cancelTarget?: string | null
  cancelEnabled?: boolean
  onCancelTurn?: (requestId: string) => void
  // pending action 来自 PC；移动端只展示同步状态，不复制权限模型。
  permissionPending?: boolean
  permissionMode?: PermissionMode
  onPermissionModeChange?: (mode: PermissionMode) => void
  disabledReason?: string
}

const SHELL_STYLE: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 4,
  background: 'rgba(255, 255, 255, 0.94)',
  border: '1px solid rgba(15, 23, 42, 0.12)',
  borderRadius: 26,
  padding: '10px 12px 8px',
  boxShadow: '0 4px 18px rgba(15, 23, 42, 0.08)',
}

const INPUT_STYLE = {
  '--font-size': FONT_SIZE_BODY,
  '--color': '#1f2937',
  '--placeholder-color': '#b5b8bd',
  padding: '1px 2px 4px',
} as CSSProperties
const ICON_BUTTON_STYLE: CSSProperties = {
  minWidth: 32,
  width: 32,
  height: 32,
  padding: 0,
  border: 0,
  borderRadius: 16,
  background: 'transparent',
  color: '#1f2937',
  fontSize: 22,
  lineHeight: 1,
}
const PERMISSION_OPTIONS = [
  { label: '默认', value: 'default' },
  { label: '自动接受编辑', value: 'acceptEdits' },
  { label: '计划', value: 'plan' },
  { label: '绕过权限', value: 'bypassPermissions' },
] satisfies Array<{ label: string; value: PermissionMode }>

function AttachmentPreview({ file }: { file: File }) {
  const [preview, setPreview] = useState<{ file: File; source: string } | null>(null)
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    if (!file.type.startsWith('image/')) return
    const reader = new FileReader()
    reader.onload = () => { if (typeof reader.result === 'string') setPreview({ file, source: reader.result }) }
    reader.readAsDataURL(file)
    return () => { reader.onload = null; if (reader.readyState === FileReader.LOADING) reader.abort() }
  }, [file])
  if (preview?.file !== file) return <div className="composer-attachment-file">{file.name}</div>
  return <>
    <button type="button" className="composer-attachment-image" aria-label={`预览 ${file.name}`} onClick={() => setVisible(true)}>
      <img src={preview.source} alt="" />
    </button>
    <ImageViewer image={preview.source} visible={visible} onClose={() => setVisible(false)} />
  </>
}

export default function MessageComposer({
  enabled,
  experience = nativeExperience,
  onSend,
  onSendAttachments,
  progress,
  turnWaiting = false,
  turnRunning = false,
  cancelTarget = null,
  cancelEnabled = false,
  onCancelTurn,
  permissionPending = false,
  textOnly = false,
  permissionMode = 'default',
  onPermissionModeChange = () => undefined,
  disabledReason = '当前会话暂不可输入',
}: MessageComposerProps) {
  const [text, setText] = useState('')
  const speech = useSpeechDraft(experience, enabled, (value) => setText((draft) => draft + (draft.trim() ? '\n' : '') + value))
  const [oversize, setOversize] = useState(false)
  const [attachmentMenuOpen, setAttachmentMenuOpen] = useState(false)
  const [permissionMenuOpen, setPermissionMenuOpen] = useState(false)
  const [selectedFiles, setSelectedFiles] = useState<File[]>([])
  const [nativeNotice, setNativeNotice] = useState<string | null>(null)
  const [sendingAttachments, setSendingAttachments] = useState(false)
  const [preparingAttachments, setPreparingAttachments] = useState(false)
  const pickingRef = useRef(false)
  const mountedRef = useRef(true)
  useEffect(() => { mountedRef.current = true; return () => { mountedRef.current = false } }, [])
  const photoInputRef = useRef<HTMLInputElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const cameraInputRef = useRef<HTMLInputElement>(null)
  const textAreaRef = useRef<TextAreaRef>(null)
  const feedbackTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [blockedFeedback, setBlockedFeedback] = useState(false)

  // antd-mobile 的菜单渲染在 Portal 中，按下菜单项同样不能抢走输入焦点。
  // Button 未暴露 pointerdown 属性，因此在文档捕获阶段只筛选指定控件。
  useEffect(() => {
    const keepComposerFocus = (event: PointerEvent) => {
      if (document.activeElement !== textAreaRef.current?.nativeElement) return
      if (event.target instanceof Element && event.target.closest(
        '[data-testid="attachment-toggle"], [data-testid="permission-mode"], .composer-popover-menu',
      )) {
        event.preventDefault()
      }
    }
    document.addEventListener('pointerdown', keepComposerFocus, true)
    return () => document.removeEventListener('pointerdown', keepComposerFocus, true)
  }, [])

  useEffect(() => {
    return () => {
      if (feedbackTimerRef.current !== null) clearTimeout(feedbackTimerRef.current)
    }
  }, [])

  const showBlockedFeedback = () => {
    setBlockedFeedback(true)
    if (feedbackTimerRef.current !== null) clearTimeout(feedbackTimerRef.current)
    feedbackTimerRef.current = setTimeout(() => setBlockedFeedback(false), 2200)
  }

  const trimmed = text.trim()
  const oversizeNow = trimmed !== '' && new TextEncoder().encode(trimmed).length > MAX_TEXT_BYTES

  const submit = () => {
    if ((trimmed === '' && selectedFiles.length === 0) || oversizeNow || !enabled || speech.recording || sendingAttachments || preparingAttachments) {
      if (oversizeNow) setOversize(true)
      return
    }
    setOversize(false)
    speech.clearError()
    if (selectedFiles.length === 0) { onSend(trimmed); setText(''); return }
    if (onSendAttachments === undefined) { setNativeNotice('当前连接未启用附件发送，请更新电脑端'); return }
    setSendingAttachments(true)
    setNativeNotice(null)
    void (async () => {
      try {
        validateFiles(selectedFiles)
        const attachments = await Promise.all(selectedFiles.map(encodeFile))
        if (!mountedRef.current) return
        const result = await onSendAttachments(trimmed, attachments)
        if (!mountedRef.current) return
        if (result === 'accepted') { setText(''); setSelectedFiles([]); setNativeNotice('附件已传到电脑，Claude 将按现有权限读取。') }
        else setNativeNotice(result === 'unknown' ? '发送结果未知，草稿已保留。请先查看会话结果再决定是否重试，避免重复发送。' : '附件未被接收，草稿已保留。请确认电脑端已更新并检查连接。')
      } catch (error) {
        if (mountedRef.current) setNativeNotice(error instanceof Error ? error.message : '附件发送失败，草稿已保留')
      } finally { if (mountedRef.current) setSendingAttachments(false) }
    })()
  }

  const chooseFiles = (files: FileList | null) => {
    if (files === null || files.length === 0 || !enabled || sendingAttachments || pickingRef.current) return
    const picked = Array.from(files)
    if (selectedFiles.length + picked.length > 4) { setNativeNotice('最多添加 4 个附件'); return }
    if (!picked.some(needsPhotoPreparation)) {
      try { const combined = [...selectedFiles, ...picked]; validateFiles(combined); setSelectedFiles(combined); setNativeNotice(null) }
      catch (error) { setNativeNotice(error instanceof Error ? error.message : '附件无效') }
      setAttachmentMenuOpen(false)
      return
    }
    pickingRef.current = true
    setPreparingAttachments(true)
    setAttachmentMenuOpen(false)
    void (async () => {
      try {
        const combined = [...selectedFiles, ...await Promise.all(picked.map(preparePickedFile))]
        validateFiles(combined)
        if (mountedRef.current) { setSelectedFiles(combined); setNativeNotice(null) }
      } catch (error) { if (mountedRef.current) setNativeNotice(error instanceof Error ? error.message : '附件无效') }
      finally { pickingRef.current = false; if (mountedRef.current) setPreparingAttachments(false) }
    })()
  }

  const attachmentActions = [
    {
      key: 'camera',
      icon: <SessionIcon name="camera" size={20} style={{ color: "#3478f6" }} />,
      text: <span data-testid="open-camera">相机</span>,
      onClick: () => cameraInputRef.current?.click(),
    },
    {
      key: 'photo',
      icon: <SessionIcon name="image" size={20} style={{ color: "#3478f6" }} />,
      text: <span data-testid="choose-photo">照片</span>,
      onClick: () => photoInputRef.current?.click(),
    },
    {
      key: 'file',
      icon: <SessionIcon name="file" size={20} style={{ color: "#3478f6" }} />,
      text: <span data-testid="choose-file">文件</span>,
      onClick: () => fileInputRef.current?.click(),
    },
  ]

  const permissionActions = PERMISSION_OPTIONS.map((option) => ({
    key: option.value,
    text: (
      <span
        data-testid={`permission-option-${option.value}`}
        className={option.value === permissionMode ? 'composer-permission-active' : undefined}
      >
        <span>{option.label}</span>
        {option.value === permissionMode && <CheckOutline aria-hidden className="composer-permission-check" />}
      </span>
    ),
    onClick: () => onPermissionModeChange(option.value),
  }))

  return (
    <div data-debug-input-region style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {selectedFiles.length > 0 && (
        <div data-testid="composer-attachments" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span>已选 {selectedFiles.length} 个附件（每个最多 4 MiB，合计 8 MiB）</span>
          <div className="composer-attachment-grid">{selectedFiles.map((file, index) => <div key={`${index}-${file.name}`} className="composer-attachment-item">
            <AttachmentPreview file={file} />
            <button type="button" className="composer-attachment-remove" disabled={sendingAttachments || preparingAttachments} aria-label={`移除 ${file.name}`} onClick={() => setSelectedFiles((items) => items.filter((_, i) => i !== index))}>×</button>
            <span className="composer-attachment-name" title={file.name}>{file.name}</span>
          </div>)}</div>
        </div>
      )}
      {preparingAttachments && <div role="status">正在处理照片或文件…</div>}
      {sendingAttachments && <div role="status">正在发送附件，等待电脑确认…</div>}
      {speech.recording && <div className="composer-speech-status" role="status">{speech.finishing ? '正在识别，请稍候…' : '正在聆听，点击语音按钮结束（最长 60 秒）'}</div>}
      {speech.error !== null && <div role="alert">{speech.error}</div>}
      {nativeNotice !== null && <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#64748b' }}>{nativeNotice}</div>}
      {((oversize && oversizeNow) || oversizeNow) && <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#dc2626' }}>文本超出长度上限</div>}
      {turnRunning && <TurnLoading progress={progress} waiting={turnWaiting} />}
      <div style={{ position: 'relative' }}>
        {blockedFeedback && !enabled && <div className="composer-disabled-feedback" role="status">{disabledReason}</div>}
        <div style={{ ...SHELL_STYLE, ...(blockedFeedback && !enabled ? { borderColor: '#d97706', boxShadow: '0 0 0 3px rgba(217, 119, 6, 0.18)' } : {}) }}>
          <div style={{ position: 'relative' }}>
            <TextArea
              ref={textAreaRef}
              data-testid="composer-input"
              style={INPUT_STYLE}
              value={text}
              onChange={(value) => {
                setText(value)
                speech.clearError()
                if (!oversizeNow) setOversize(false)
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229) {
                  event.preventDefault()
                  submit()
                }
              }}
              placeholder={textOnly ? "输入消息体验模拟回复" : "输入消息或添加附件"}
              autoSize={{ minRows: 1, maxRows: 4 }}
              disabled={!enabled || sendingAttachments || preparingAttachments}
            />
            {!enabled && <button
              type="button"
              className="composer-disabled-hit-area"
              aria-label="输入暂不可用"
              onClick={showBlockedFeedback}
            />}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', width: '100%', minHeight: 32 }}>
            <Space align="center" style={{ '--gap': '2px' }}>
              <Popover.Menu
                className="composer-popover-menu"
                placement="top-start"
                trigger="click"
                visible={attachmentMenuOpen}
                onVisibleChange={setAttachmentMenuOpen}
                actions={attachmentActions}
              >
                <Button
                  aria-label="添加照片或文件"
                  className="composer-icon-button"
                  data-testid="attachment-toggle"
                  fill="none"
                  style={ICON_BUTTON_STYLE}
                  disabled={textOnly || !enabled || sendingAttachments || preparingAttachments}
                >
                  <SessionIcon name="paperclip" size={20} />
                </Button>
              </Popover.Menu>
              <Popover.Menu
                className="composer-popover-menu composer-permission-menu"
                placement="top-start"
                trigger="click"
                visible={permissionMenuOpen}
                onVisibleChange={setPermissionMenuOpen}
                actions={permissionActions}
              >
                <Button
                  aria-label={permissionPending ? '权限请求待确认' : '权限模式'}
                  className="composer-icon-button"
                  data-testid="permission-mode"
                  fill="none"
                  disabled={textOnly || !enabled || permissionPending}
                  style={ICON_BUTTON_STYLE}
                >
                  <SessionIcon name="permission" size={19} />
                </Button>
              </Popover.Menu>
            </Space>
            <div style={{ flex: 1 }} />
            <Space align="center" style={{ '--gap': '12px' }}>
              <Button
                aria-label={experience === null ? '语音输入（原生能力预留）' : speech.finishing ? '正在识别语音' : speech.recording ? '结束语音输入' : '语音输入'}
                aria-pressed={speech.recording}
                aria-busy={speech.finishing}
                className={`composer-icon-button${speech.recording && !speech.finishing ? ' composer-voice-recording' : ''}`}
                data-testid="voice-input"
                fill="none"
                style={ICON_BUTTON_STYLE}
                disabled={textOnly || !enabled || sendingAttachments || preparingAttachments || speech.finishing}
                onClick={() => {
                  if (experience === null) setNativeNotice('语音输入需要原生麦克风能力，当前 Web 版本暂未启用。')
                  else void speech.toggle()
                }}
              >
                <SessionIcon name="microphone" size={20} />
              </Button>
              {turnRunning && cancelTarget !== null && onCancelTurn !== undefined && (
                <Button aria-label="取消当前 Turn" data-testid="floating-cancel-turn" className="composer-icon-button composer-stop-button" fill="none" style={ICON_BUTTON_STYLE} disabled={!cancelEnabled} onClick={() => onCancelTurn(cancelTarget)}>
                  <span className="composer-stop" aria-hidden="true">■</span>
                </Button>
              )}
              {(!turnRunning || trimmed !== '' || selectedFiles.length > 0) && <Button
                aria-label="发送"
                className="composer-icon-button composer-send-button"
                fill="none"
                style={ICON_BUTTON_STYLE}
                disabled={!enabled || sendingAttachments || preparingAttachments || speech.recording || (trimmed === '' && selectedFiles.length === 0) || oversizeNow}
                onClick={submit}
              >
                <SessionIcon name="send" size={20} />
              </Button>}
            </Space>
          </div>
        </div>
      </div>
      <input ref={photoInputRef} data-testid="photo-picker" hidden type="file" accept="image/*" multiple disabled={textOnly || !enabled || sendingAttachments || preparingAttachments} onChange={(event) => { chooseFiles(event.target.files); event.target.value = '' }} />
      <input ref={fileInputRef} data-testid="file-picker" hidden type="file" multiple disabled={textOnly || !enabled || sendingAttachments || preparingAttachments} onChange={(event) => { chooseFiles(event.target.files); event.target.value = '' }} />
      <input ref={cameraInputRef} data-testid="camera-picker" hidden type="file" accept="image/*" capture="environment" disabled={textOnly || !enabled || sendingAttachments || preparingAttachments} onChange={(event) => { chooseFiles(event.target.files); event.target.value = '' }} />
    </div>
  )
}
