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
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('系统设置'))
  expect(setNotifications).toHaveBeenCalledWith({ enabled: true })
})
