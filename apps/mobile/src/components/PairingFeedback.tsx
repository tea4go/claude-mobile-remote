import { useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { Button, Input, Toast, unstableSetRender } from 'antd-mobile'
import { createRoot } from 'react-dom/client'

unstableSetRender((node, container) => {
  const root = createRoot(container)
  root.render(node)
  return async () => { root.unmount() }
})

export function PairingLoading({ message, onCancel, cancelDisabled = false }: { message: string; onCancel?: () => void; cancelDisabled?: boolean }) {
  const cancel = useRef(onCancel)
  cancel.current = onCancel
  useEffect(() => {
    const toast = Toast.show({ icon: 'loading', duration: 0, maskClickable: false, maskClassName: 'pairing-loading-toast', maskStyle: { zIndex: 10001 }, content: <div role="status" className="pairing-loading-toast__content"><span>{message}</span><Button size="small" disabled={cancelDisabled} onClick={() => cancel.current?.()}>取消</Button></div> })
    return () => toast.close()
  }, [message, cancelDisabled])
  return null
}

export function PairingToast({ message, onClose }: { message: string; onClose: () => void }) {
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    const toast = Toast.show({ icon: 'fail', position: 'bottom', maskClassName: 'pairing-error-toast', content: message, duration: 3000, maskStyle: { zIndex: 10001 }, afterClose: () => close.current() })
    return () => toast.close()
  }, [message])
  return null
}

export function PairingCodeDialog({ code, onChange, onCancel, onVerify }: { code: string; onChange: (value: string) => void; onCancel: () => void; onVerify: () => void }) {
  return createPortal(<div className="pairing-overlay"><div className="pairing-overlay__dialog pairing-code-dialog" role="dialog" aria-modal="true" aria-label="输入验证码"><h3>输入验证码</h3><p>请输入 PC 上的六位验证码</p><Input aria-label="六位验证码" type="tel" inputMode="numeric" maxLength={6} value={code} onChange={value => onChange(value.replace(/\D/g, ''))} placeholder="请输入六位验证码"/><div className="pairing-code-dialog__actions"><Button onClick={onCancel}>取消</Button><Button color="primary" disabled={code.length !== 6} onClick={onVerify}>验证并连接</Button></div></div></div>, document.body)
}
