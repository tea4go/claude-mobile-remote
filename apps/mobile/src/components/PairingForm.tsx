import SessionIcon from './SessionIcon'
import { useState } from 'react'
import { Button, TextArea } from 'antd-mobile'
import { parsePairingInput } from '../protocol/pairing'
import { safeNetworkTarget } from '../protocol/diagnostics'

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
  const parsed = json.trim() === '' ? null : parsePairingInput(json, 0)
  const displayedTarget = parsed?.ok ? safeNetworkTarget(parsed.value.wsUrl) : json.trim() === '' ? target : null

  return (
    <div className="pairing-form">
      <h2 style={{ margin: 0 }}>连接 Claude 会话</h2>
      <p style={{ margin: 0, color: '#666' }}>粘贴配对 JSON 或完整 #pair Fragment</p>
      <div className="pairing-form__input"><TextArea
        data-testid="pairing-json"
        value={json}
        onChange={(value) => {
          setJson(value)
          setEmptyError(false)
        }}
        placeholder='{"version":1,...}'
        rows={6}
      /></div>
      <div className="pairing-form__target" data-testid="pairing-target" style={{ overflowWrap: 'anywhere' }}>
        <strong>目标 PC 地址（IP / 主机名及端口）</strong>
        <div><SessionIcon name="target" size={16} style={{ color: "#3478f6", marginRight: 8 }} />{displayedTarget ?? '粘贴或扫码后显示目标地址'}</div>
        <small>请与 PC 配对窗口核对；手机所在网络需要能访问此地址。不同路由器共用宽带不代表设备能够互通。</small>
      </div>
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
        className="pairing-form__primary"
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
      {onScan && <Button className="pairing-form__scan" disabled={progress !== null || saving} color="primary" fill="outline" onClick={onScan}><SessionIcon name="scan" size={16} style={{ marginRight: 6 }} />扫描二维码</Button>}
    </div>
  )
}
