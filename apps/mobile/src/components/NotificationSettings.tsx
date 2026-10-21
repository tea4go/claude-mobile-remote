import { useEffect, useState } from 'react'
import { Button } from 'antd-mobile'
import type { ExperiencePort } from '../native/experience'

export default function NotificationSettings({ port }: { port: ExperiencePort }) {
  const [enabled, setEnabled] = useState(false)
  const [busy, setBusy] = useState(true)
  const [notice, setNotice] = useState('')
  const [remoteConfigured, setRemoteConfigured] = useState(false)
  const [remoteReady, setRemoteReady] = useState(false)
  useEffect(() => {
    let mounted = true
    void port.notificationStatus().then((status) => { if (mounted) { setEnabled(status.enabled); setRemoteConfigured(status.remoteConfigured ?? false); setRemoteReady(status.remoteReady ?? false) } })
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
    <p>{remoteConfigured ? (remoteReady ? '极光系统推送已注册。后台或退出后由 APNs 提醒；PC 需保持远程服务运行并配置极光发送凭证。' : '极光 SDK 已配置，设备注册或会话订阅尚未就绪，请稍后重新进入本页查看。') : '极光系统推送尚未配置 AppKey。当前仅提供在线提醒，不能接收退出 App 后的新通知。'}</p>
    {notice && <div role="status">{notice}</div>}
  </div>
}
