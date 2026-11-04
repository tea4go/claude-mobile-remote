import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { Capacitor } from '@capacitor/core'

const back = vi.hoisted(() => ({ handler: null as (() => void) | null }))
vi.mock('@capacitor/app', () => ({ App: { addListener: vi.fn(async (_event: string, handler: () => void) => {
  back.handler = handler
  return { remove: async () => { if (back.handler === handler) back.handler = null } }
}) } }))
import App from './App'

test('Android back follows app navigation and never navigates behind a dialog', async () => {
  vi.spyOn(Capacitor, 'getPlatform').mockReturnValue('android')
  const view = render(<App cache={null} experience={null} />)
  await waitFor(() => expect(back.handler).not.toBeNull())
  fireEvent.click(screen.getByText('• 体验演示（无需连接电脑）'))
  await screen.findByText('离线演示 · 模拟数据，不连接电脑或 Claude')
  const dialog = document.createElement('div')
  dialog.setAttribute('role', 'dialog')
  document.body.append(dialog)
  act(() => back.handler?.())
  expect(screen.getByText('离线演示 · 模拟数据，不连接电脑或 Claude')).toBeInTheDocument()
  dialog.remove()
  act(() => back.handler?.())
  expect(await screen.findByText('连接 Claude 会话')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: /设置/ }))
  await screen.findByText('PC 管理')
  act(() => back.handler?.())
  expect(await screen.findByText('连接 Claude 会话')).toBeInTheDocument()
  view.unmount()
  vi.restoreAllMocks()
})
