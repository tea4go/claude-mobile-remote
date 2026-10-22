import { useEffect, useState } from 'react'
import { Button } from 'antd-mobile'
import type { ExperiencePort } from '../native/experience'

export default function NotificationSettings({ port }: { port: ExperiencePort }) {
  const [enabled, setEnabled] = useState(false)
  const [busy, setBusy] = useState(true)
  const [notice, setNotice] = useState('')
  useEffect(() => {
    let mounted = true
    void port.notificationStatus().then((status) => { if (mounted) setEnabled(status.enabled) })
      .catch(() => { if (mounted) setNotice('无法读取通知设置') })
      .finally(() => { if (mounted) setBusy(false) })
    return () => { mounted = false }
  }, [port])
  return <div style={{ padding: 16 }}>
    <Button disabled={busy} onClick={() => {
      setBusy(true)
      void port.setNotifications({ enabled: !enabled }).then((status) => {
        setEnabled(status.enabled)
        setNotice(!enabled && !status.enabled ? '通知未获授权，请在系统设置中允许通知' : '')
      }).catch(() => setNotice('无法更新通知设置')).finally(() => setBusy(false))
    }}>{enabled ? '关闭会话通知' : '启用会话通知'}</Button>
    <p>提醒待审批、待回答、任务完成和失败。通知隐藏会话内容；正在查看的会话不提醒。</p>
    <p>极光推送暂时停用。App 在线并收到会话事件时可显示本地提醒；退出 App 后无法接收新通知。</p>
    {notice && <div role="status">{notice}</div>}
  </div>
}
