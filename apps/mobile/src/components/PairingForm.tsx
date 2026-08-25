import { useState } from 'react'
import { Button, TextArea } from 'antd-mobile'

interface PairingFormProps {
  initialError: string | null
  onSubmit: (json: string) => void
}

// 配对页：标题、说明、原始 JSON 多行输入、连接按钮与就地错误（设计 §8）。
export default function PairingForm({ initialError, onSubmit }: PairingFormProps) {
  const [json, setJson] = useState('')
  const [emptyError, setEmptyError] = useState(false)

  const error = emptyError ? '请输入配对 JSON' : initialError

  return (
    <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <h2 style={{ margin: 0 }}>连接 Claude 会话</h2>
      <p style={{ margin: 0, color: '#666' }}>粘贴 Service 输出的配对 JSON</p>
      <TextArea
        data-testid="pairing-json"
        value={json}
        onChange={(value) => {
          setJson(value)
          setEmptyError(false)
        }}
        placeholder='{"version":1,...}'
        rows={6}
      />
      {error !== null && (
        <div data-testid="pairing-error" style={{ color: '#e00' }}>
          {error}
        </div>
      )}
      <Button
        color="primary"
        onClick={() => {
          if (json.trim() === '') {
            setEmptyError(true)
            return
          }
          onSubmit(json)
        }}
      >
        连接
      </Button>
    </div>
  )
}
