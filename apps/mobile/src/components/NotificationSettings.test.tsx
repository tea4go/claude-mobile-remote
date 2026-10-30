import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import NotificationSettings from './NotificationSettings'
import type { ExperiencePort } from '../native/experience'

test('只在用户点击时请求授权，拒绝后保持关闭并提供指引', async () => {
  const notificationStatus = vi.fn(async () => ({ enabled: false }))
  const setNotifications = vi.fn(async () => ({ enabled: false }))
  render(<NotificationSettings port={{ notificationStatus, setNotifications } as unknown as ExperiencePort} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '启用会话通知' })).toBeEnabled())
  expect(setNotifications).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: '启用会话通知' }))
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('系统设置'))
  expect(setNotifications).toHaveBeenCalledWith({ enabled: true })
})


test('区分原生注册状态与未配置状态，不再宣称极光暂停', async () => {
  const port = { notificationStatus: vi.fn(async () => ({ enabled: true, remoteConfigured: true, remoteReady: true })), setNotifications: vi.fn() } as unknown as ExperiencePort
  render(<NotificationSettings port={port} />)
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('本机远程通知已就绪'))
  expect(screen.queryByText(/暂时停用/)).not.toBeInTheDocument()
  expect(screen.getByText(/匿名会话标识/)).toBeInTheDocument()
})

test('未配置远程通知时保留在线提醒说明', async () => {
  const port = { notificationStatus: vi.fn(async () => ({ enabled: true, remoteConfigured: false, remoteReady: false })) } as unknown as ExperiencePort
  render(<NotificationSettings port={port} />)
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('尚未配置'))
})

test('旧版本地提醒用户需明确启用后台通知，读取状态不触发SDK注册', async () => {
  const notificationStatus = vi.fn(async () => ({ enabled: true, remoteConfigured: true, remoteEnabled: false, remoteReady: false }))
  const setNotifications = vi.fn(async () => ({ enabled: true }))
  render(<NotificationSettings port={{ notificationStatus, setNotifications } as unknown as ExperiencePort} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '启用后台通知（极光）' })).toBeEnabled())
  expect(setNotifications).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: '启用后台通知（极光）' }))
  await waitFor(() => expect(setNotifications).toHaveBeenCalledWith({ enabled: true }))
})

test('Android设备登记成功也明确提示厂商离线通道未配置', async () => {
  const port = { notificationStatus: vi.fn(async () => ({ enabled: true, remoteConfigured: true, remoteEnabled: true, remoteReady: true, vendorConfigured: false })) } as unknown as ExperiencePort
  render(<NotificationSettings port={port} />)
  await waitFor(() => expect(screen.getByText(/当前手机的离线通知通道尚未配置/)).toBeInTheDocument())
  expect(screen.getByText(/应用退出后可能收不到提醒/)).toBeInTheDocument()
})

test('暂停通知的测试版本不显示启用入口或发起授权', async () => {
  const port = { notificationStatus: vi.fn(async () => ({ enabled: false, available: false })), setNotifications: vi.fn() } as unknown as ExperiencePort
  render(<NotificationSettings port={port} />)
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('暂未开放系统通知'))
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(port.setNotifications).not.toHaveBeenCalled()
})
