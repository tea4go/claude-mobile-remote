import { useState, type CSSProperties } from 'react'
import { Button, TextArea } from 'antd-mobile'
import { MAX_TEXT_BYTES } from '../protocol/types'
import { COMPACT_BUTTON_STYLE, FONT_SIZE_COMPACT, FONT_SIZE_BODY } from './ui'

interface MessageComposerProps {
  enabled: boolean
  onSend: (text: string) => void
}

// 胶囊形输入区（参考 docs/specs/images/session_detail.png）：输入框与发送按钮
// 包在同一个圆角容器里，默认单行高，随内容增高到四行为止。
const SHELL_STYLE: CSSProperties = {
  display: 'flex',
  alignItems: 'flex-end',
  gap: 8,
  background: 'rgba(255, 255, 255, 0.9)',
  border: '1px solid rgba(0, 0, 0, 0.08)',
  borderRadius: 20,
  padding: '5px 6px 5px 14px',
}

// TextArea 的字号由 CSS 变量控制，行内样式需要断言成 CSSProperties。
const INPUT_STYLE = { '--font-size': FONT_SIZE_BODY } as CSSProperties

const SEND_BUTTON_STYLE: CSSProperties = { ...COMPACT_BUTTON_STYLE, padding: '4px 14px', borderRadius: 14 }

// 底部固定输入（§7）：空白或超限文本不发送；1 MiB 按 UTF-8 字节计算。
export default function MessageComposer({ enabled, onSend }: MessageComposerProps) {
  const [text, setText] = useState('')
  const [oversize, setOversize] = useState(false)

  const trimmed = text.trim()
  const oversizeNow =
    trimmed !== '' && new TextEncoder().encode(trimmed).length > MAX_TEXT_BYTES

  const submit = () => {
    if (trimmed === '') return
    if (oversizeNow) {
      setOversize(true)
      return
    }
    setOversize(false)
    onSend(trimmed)
    setText('')
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {(oversize && oversizeNow) || oversizeNow ? (
        <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#e00' }}>文本超出长度上限</div>
      ) : null}
      <div style={SHELL_STYLE}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <TextArea
            data-testid="composer-input"
            style={INPUT_STYLE}
            value={text}
            onChange={(value) => {
              setText(value)
              if (!oversizeNow) setOversize(false)
            }}
            placeholder="发送文本给 Claude"
            autoSize={{ minRows: 1, maxRows: 4 }}
            disabled={!enabled}
          />
        </div>
        <Button
          color="primary"
          size="small"
          style={SEND_BUTTON_STYLE}
          disabled={!enabled || trimmed === '' || oversizeNow}
          onClick={submit}
        >
          发送
        </Button>
      </div>
    </div>
  )
}
