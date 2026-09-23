import { useRef, useState, type CSSProperties } from 'react'
import { Button, Popover, Space, TextArea } from 'antd-mobile'
import { AddOutline, CheckOutline } from 'antd-mobile-icons'
import { MAX_TEXT_BYTES, type PermissionMode } from '../protocol/types'
import { COMPACT_BUTTON_STYLE, FONT_SIZE_COMPACT, FONT_SIZE_BODY } from './ui'
import './MessageComposer.css'
import cameraIcon from '../assets/icons/svgs/camara.svg'
import documentIcon from '../assets/icons/svgs/document.svg'
import permissionIcon from '../assets/icons/svgs/permission.svg'
import uploadImageIcon from '../assets/icons/svgs/uploadImage.svg'
import voiceIcon from '../assets/icons/svgs/voice.svg'
import sendIcon from '../assets/icons/svgs/send.svg'

interface MessageComposerProps {
  enabled: boolean
  onSend: (text: string) => void
  turnRunning?: boolean
  cancelTarget?: string | null
  cancelEnabled?: boolean
  onCancelTurn?: (requestId: string) => void
  // pending action 来自 PC；移动端只展示同步状态，不复制权限模型。
  permissionPending?: boolean
  permissionMode?: PermissionMode
  onPermissionModeChange?: (mode: PermissionMode) => void
  onCreateSession?: () => void
  createSessionEnabled?: boolean
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
const TOOL_ICON_STYLE: CSSProperties = { width: 20, height: 20, display: 'block' }
const META_STYLE: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: 8,
  minHeight: 28,
  fontSize: FONT_SIZE_COMPACT,
  color: '#64748b',
}

const PERMISSION_OPTIONS = [
  { label: '默认', value: 'default' },
  { label: '自动接受编辑', value: 'acceptEdits' },
  { label: '计划', value: 'plan' },
  { label: '绕过权限', value: 'bypassPermissions' },
] satisfies Array<{ label: string; value: PermissionMode }>

// 当前 wire contract 仍是文本 command。文件和相机入口先接入浏览器选择器，
// 但明确提示“尚未发送”，避免把本地选中文件伪装成 Claude 已收到附件。
export default function MessageComposer({
  enabled,
  onSend,
  turnRunning = false,
  cancelTarget = null,
  cancelEnabled = false,
  onCancelTurn,
  permissionPending = false,
  permissionMode = 'default',
  onPermissionModeChange = () => undefined,
  onCreateSession,
  createSessionEnabled = false,
}: MessageComposerProps) {
  const [text, setText] = useState('')
  const [oversize, setOversize] = useState(false)
  const [attachmentMenuOpen, setAttachmentMenuOpen] = useState(false)
  const [permissionMenuOpen, setPermissionMenuOpen] = useState(false)
  const [selectedFiles, setSelectedFiles] = useState<string[]>([])
  const [nativeNotice, setNativeNotice] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const cameraInputRef = useRef<HTMLInputElement>(null)

  const trimmed = text.trim()
  const oversizeNow = trimmed !== '' && new TextEncoder().encode(trimmed).length > MAX_TEXT_BYTES

  const submit = () => {
    if (trimmed === '' || oversizeNow || !enabled) {
      if (oversizeNow) setOversize(true)
      return
    }
    setOversize(false)
    onSend(trimmed)
    setText('')
  }

  const chooseFiles = (files: FileList | null) => {
    if (files === null || files.length === 0) return
    setSelectedFiles(Array.from(files).map((file) => file.name))
    setNativeNotice('文件已选择；当前版本仍只发送文本，原生附件发送待接入。')
    setAttachmentMenuOpen(false)
  }

  const attachmentActions = [
    {
      key: 'camera',
      icon: <img src={cameraIcon} alt="" style={TOOL_ICON_STYLE} />,
      text: <span data-testid="open-camera">相机</span>,
      onClick: () => cameraInputRef.current?.click(),
    },
    {
      key: 'photo',
      icon: <img src={uploadImageIcon} alt="" style={TOOL_ICON_STYLE} />,
      text: <span data-testid="choose-photo">照片</span>,
      onClick: () => fileInputRef.current?.click(),
    },
    {
      key: 'file',
      icon: <img src={documentIcon} alt="" style={TOOL_ICON_STYLE} />,
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
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {selectedFiles.length > 0 && (
        <div data-testid="composer-attachments" style={{ ...META_STYLE, justifyContent: 'flex-start' }}>
          <span>已选 {selectedFiles.length} 个文件：{selectedFiles.join('、')}</span>
          <Button fill="none" size="mini" style={{ color: '#64748b' }} onClick={() => setSelectedFiles([])}>
            清除
          </Button>
        </div>
      )}
      {nativeNotice !== null && <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#64748b' }}>{nativeNotice}</div>}
      {((oversize && oversizeNow) || oversizeNow) && <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#dc2626' }}>文本超出长度上限</div>}
      <div style={{ position: 'relative' }}>
        {turnRunning && cancelTarget !== null && onCancelTurn !== undefined && (
          <Button
            aria-label="取消当前 Turn"
            data-testid="floating-cancel-turn"
            size="mini"
            fill="outline"
            color="danger"
            style={{
              ...COMPACT_BUTTON_STYLE,
              position: 'absolute',
              right: 8,
              bottom: 'calc(100% + 8px)',
              zIndex: 10,
              minWidth: 0,
              height: 26,
              padding: '2px 10px',
              borderRadius: 13,
              background: 'rgba(255, 255, 255, 0.96)',
              boxShadow: '0 3px 10px rgba(15, 23, 42, 0.12)',
              fontSize: 12,
            }}
            disabled={!cancelEnabled}
            onClick={() => onCancelTurn(cancelTarget)}
          >
            {cancelEnabled ? '停止' : '停止中'}
          </Button>
        )}
        <div style={SHELL_STYLE}>
          <TextArea
            data-testid="composer-input"
            style={INPUT_STYLE}
            value={text}
            onChange={(value) => {
              setText(value)
              if (!oversizeNow) setOversize(false)
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault()
                submit()
              }
            }}
            placeholder="发送文本给 Claude"
            autoSize={{ minRows: 1, maxRows: 4 }}
            disabled={!enabled}
          />
          <div style={{ display: 'flex', alignItems: 'center', width: '100%', minHeight: 32 }}>
            <Space align="center" style={{ '--gap': '2px' }}>
              {onCreateSession !== undefined && (
                <Button
                  aria-label="新建会话"
                  className="composer-icon-button"
                  data-testid="create-session"
                  fill="none"
                  style={ICON_BUTTON_STYLE}
                  disabled={!createSessionEnabled}
                  onClick={onCreateSession}
                >
                  <AddOutline aria-hidden style={{ fontSize: 21 }} />
                </Button>
              )}
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
                  disabled={!enabled}
                >
                  <img src={documentIcon} alt="" style={TOOL_ICON_STYLE} />
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
                  disabled={!enabled || permissionPending}
                  style={ICON_BUTTON_STYLE}
                >
                  <img src={permissionIcon} alt="" style={{ ...TOOL_ICON_STYLE, width: 19, height: 19 }} />
                </Button>
              </Popover.Menu>
            </Space>
            <div style={{ flex: 1 }} />
            <Space align="center" style={{ '--gap': '4px' }}>
              <Button
                aria-label="语音输入（原生能力预留）"
                className="composer-icon-button"
                data-testid="voice-input"
                fill="none"
                style={ICON_BUTTON_STYLE}
                disabled={!enabled}
                onClick={() => setNativeNotice('语音输入需要原生麦克风能力，当前 Web 版本暂未启用。')}
              >
                <img src={voiceIcon} alt="" style={TOOL_ICON_STYLE} />
              </Button>
              <Button
                aria-label="发送"
                className="composer-icon-button"
                fill="none"
                style={ICON_BUTTON_STYLE}
                disabled={!enabled || trimmed === '' || oversizeNow}
                onClick={submit}
              >
                <img src={sendIcon} alt="" style={{ width: 28, height: 28, display: 'block', opacity: !enabled || trimmed === '' || oversizeNow ? 0.28 : 0.9 }} />
              </Button>
            </Space>
          </div>
        </div>
      </div>
      <input ref={fileInputRef} hidden type="file" accept="image/*,.pdf,.txt,.md,.json" multiple onChange={(event) => chooseFiles(event.target.files)} />
      <input ref={cameraInputRef} hidden type="file" accept="image/*" capture="environment" onChange={(event) => chooseFiles(event.target.files)} />
    </div>
  )
}
