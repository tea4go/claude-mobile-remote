import { useEffect, useRef, type ReactNode } from 'react'
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

export function PairingInputDialog({ title, description, children, submitText, submitDisabled, onCancel, onSubmit, preserveInputFocus }: {
  title: string
  description: string
  children: ReactNode
  submitText: string
  submitDisabled?: boolean
  onCancel: () => void
  onSubmit: () => void
  preserveInputFocus?: boolean
}) {
  return createPortal(<div className="pairing-overlay"><div className="pairing-overlay__dialog pairing-code-dialog" role="dialog" aria-modal="true" aria-label={title}>
    <h3>{title}</h3><p>{description}</p>
    {children}
    <div className="pairing-code-dialog__actions" onMouseDown={preserveInputFocus ? event => event.preventDefault() : undefined}>
      <Button onClick={onCancel}>取消</Button>
      <Button color="primary" disabled={submitDisabled} onClick={onSubmit}>{submitText}</Button>
    </div>
  </div></div>, document.body)
}

export function PairingCodeDialog({ code, error, onChange, onCancel, onVerify }: { error?: string; code: string; onChange: (value: string) => void; onCancel: () => void; onVerify: () => void }) {
  return <PairingInputDialog title="输入验证码" description="请输入 PC 上的六位验证码" submitText="验证并连接" submitDisabled={code.length !== 6} onCancel={onCancel} onSubmit={onVerify}>
    <Input aria-label="六位验证码" type="tel" inputMode="numeric" maxLength={6} value={code} onChange={value => onChange(value.replace(/\D/g, ''))} placeholder="请输入六位验证码"/>
    {error && <div className="pairing-notice pairing-notice--error" role="alert">{error}</div>}
  </PairingInputDialog>
}
