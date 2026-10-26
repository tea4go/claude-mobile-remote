import { useState } from 'react'
import { Button, TextArea } from 'antd-mobile'
import { BLOCK_BUTTON_STYLE } from './ui'

interface PairingFormProps {
  target?: string | null
  initialError: string | null
  onSubmit: (json: string) => void
  onScan?: () => void
  progress?: string | null
  saving?: boolean
  onCancel?: () => void
}

// 配对页：标题、说明、原始 JSON 多行输入、连接按钮与就地错误（设计 §8）。
export default function PairingForm({ target = null, initialError, onSubmit, onScan, progress = null, saving = false, onCancel }: PairingFormProps) {
  const [json, setJson] = useState('')
  const [emptyError, setEmptyError] = useState(false)

  const error = emptyError ? '请输入配对 JSON 或 Fragment' : initialError

  return (
    <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <h2 style={{ margin: 0 }}>连接 Claude 会话</h2>
      <p style={{ margin: 0, color: '#666' }}>粘贴 Service 输出的配对 JSON 或完整 #pair Fragment</p>
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
      {target !== null && <div data-testid="pairing-target" style={{ overflowWrap: 'anywhere' }}>
        <strong>目标 PC 地址（IP / 主机名及端口）</strong>
        <div>{target}</div>
        <small>请与 PC 配对窗口核对；手机所在网络需要能访问此地址。不同路由器共用宽带不代表设备能够互通。</small>
      </div>}
      {error !== null && (
        <div data-testid="pairing-error" style={{ color: '#e00' }}>
          {error}
        </div>
      )}
      {progress !== null && <div role="status">{progress}</div>}
      {progress !== null && <Button disabled={saving} onClick={onCancel}>取消连接</Button>}
      <Button
        disabled={progress !== null || saving}
        color="primary"
        style={BLOCK_BUTTON_STYLE}
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
      {onScan && <Button disabled={progress !== null || saving} color="primary" fill="outline" style={BLOCK_BUTTON_STYLE} onClick={onScan}>扫描二维码</Button>}
    </div>
  )
}
