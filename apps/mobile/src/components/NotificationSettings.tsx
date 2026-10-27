import { useEffect, useState } from 'react'
import { Button } from 'antd-mobile'
import type { ExperiencePort } from '../native/experience'

type Status = Awaited<ReturnType<ExperiencePort['notificationStatus']>>
export default function NotificationSettings({ port }: { port: ExperiencePort }) {
  const [status, setStatus] = useState<Status>({ enabled: false })
  const [busy, setBusy] = useState(true)
  const [notice, setNotice] = useState('')
  useEffect(() => {
    let mounted = true
    const refresh = (initial = false) => {
      void port.notificationStatus().then((value) => { if (mounted) setStatus(value) })
        .catch(() => { if (mounted) setNotice('无法读取通知设置') })
        .finally(() => { if (mounted && initial) setBusy(false) })
    }
    refresh(true)
    const timer = window.setInterval(() => refresh(), 5000)
    return () => { mounted = false; window.clearInterval(timer) }
  }, [port])
  const { enabled, remoteConfigured, remoteEnabled, remoteReady, pushError } = status
  return <div style={{ padding: 16 }}>
    <Button disabled={busy} onClick={() => {
      setBusy(true)
      void port.setNotifications({ enabled: !enabled }).then(async (value) => {
        setStatus((previous) => ({ ...previous, enabled: value.enabled }))
        setNotice(!enabled && !value.enabled ? '通知未获授权，请在系统设置中允许通知' : '')
        setStatus(await port.notificationStatus())
      }).catch(() => setNotice('无法更新通知设置')).finally(() => setBusy(false))
    }}>{enabled ? '关闭会话通知' : '启用会话通知'}</Button>
    <p>提醒待审批、待回答、任务完成和失败。通知隐藏会话内容；前台正在查看的会话不提醒。</p>
    {remoteConfigured === undefined
      ? <p>App 在线并收到会话事件时可显示本地提醒。</p>
      : <>
        <p>远程通知使用极光推送；启用后会向 edo 登记匿名会话标识与本机推送 ID，用于 App 在后台或退出后的系统提醒。电脑端需启用通知服务，点击后需能连接电脑。</p>
        {enabled && remoteConfigured && remoteEnabled === false && <Button disabled={busy} onClick={() => {
          setBusy(true)
          void port.setNotifications({ enabled: true }).then(() => port.notificationStatus()).then(setStatus)
            .catch(() => setNotice('无法启用后台通知')).finally(() => setBusy(false))
        }}>启用后台通知（极光）</Button>}
        <p role="status">{!remoteConfigured ? '远程通知尚未配置，当前仅提供在线提醒。'
          : !enabled ? '远程通知已关闭。'
          : remoteEnabled === false ? '后台通知尚未启用，当前仅提供在线提醒。'
          : pushError ? '远程通知注册暂未成功，请检查系统权限与网络。'
          : remoteReady ? '本机远程通知已就绪。' : '正在注册远程通知…'}</p>
      </>}
    {notice && <div role="alert">{notice}</div>}
  </div>
}
