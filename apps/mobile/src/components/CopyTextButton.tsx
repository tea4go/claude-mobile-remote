import { useState } from 'react'
import { copyText } from '../native/clipboard'

export default function CopyTextButton({ text, label }: { text: string; label: string }) {
  const [feedback, setFeedback] = useState('')
  return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
    <button type="button" aria-label={label} onClick={() => {
      void copyText(text).then(() => setFeedback('已复制')).catch(() => setFeedback('复制失败，请长按文字选择复制'))
    }} style={{ border: 0, background: 'transparent', color: '#1677ff', fontSize: 13, padding: '6px 4px' }}>复制</button>
    {feedback && <span role="status" style={{ color: '#888', fontSize: 12 }}>{feedback}</span>}
  </span>
}
