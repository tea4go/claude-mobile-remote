import { useState } from 'react'
import { Button, TextArea } from 'antd-mobile'
import { MAX_TEXT_BYTES } from '../protocol/types'

interface MessageComposerProps {
  enabled: boolean
  onSend: (text: string) => void
}

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
        <div style={{ fontSize: 12, color: '#e00' }}>文本超出长度上限</div>
      ) : null}
      <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
        <TextArea
          data-testid="composer-input"
          value={text}
          onChange={(value) => {
            setText(value)
            if (!oversizeNow) setOversize(false)
          }}
          placeholder="发送文本给 Claude"
          rows={2}
          disabled={!enabled}
        />
        <Button
          color="primary"
          disabled={!enabled || trimmed === '' || oversizeNow}
          onClick={submit}
        >
          发送
        </Button>
      </div>
    </div>
  )
}
