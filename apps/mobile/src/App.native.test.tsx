import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import type { ExperiencePort } from './native/experience'
import App from './App'
import { NativePairingLogic, type NativePairingMetadata } from './native/pairingLogic'
import type { NativePairingBindings } from './native/capacitorPorts'
import { encodePairingFragment, type PairingPayload } from './protocol/pairing'

class FakeSocket {
  static instances: FakeSocket[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  readonly url: string
  constructor(url: string) {
    this.url = url
    FakeSocket.instances.push(this)
  }
  send(_data: string) {}
  close() { this.closed = true }
}

function pairing(sessionId: string): PairingPayload {
  return {
    version: 1, wsUrl: `ws://127.0.0.1:8765/${sessionId}`, sessionId,
    pairToken: `fixture-${sessionId}`, expiresAt: '2099-01-01T00:00:00Z',
  }
}

function bindings(initial: Array<{ metadata: NativePairingMetadata; payload?: PairingPayload }> = []): {
  native: NativePairingBindings
  secure: Map<string, unknown>
  metadata: Map<string, NativePairingMetadata>
  foreground: () => void
} {
  const secure = new Map<string, unknown>()
  const metadata = new Map<string, NativePairingMetadata>()
  for (const entry of initial) {
    metadata.set(entry.metadata.sessionId, entry.metadata)
    if (entry.payload) secure.set(entry.metadata.sessionId, entry.payload)
  }
  let listener: (() => void) | null = null
  return {
    secure, metadata,
    native: {
      logic: new NativePairingLogic({
        read: async (id) => secure.get(id) ?? null,
        write: async (payload) => { secure.set(payload.sessionId, payload) },
        remove: async (id) => { secure.delete(id) },
      }, {
        list: async () => [...metadata.values()],
        upsert: async (record) => { metadata.set(record.sessionId, record) },
        remove: async (id) => { metadata.delete(id) },
      }),
      scanner: { scan: async () => `#pair=${encodePairingFragment(pairing('scanned'))}` },
      appState: { onForeground: (next) => {
        listener = next
        return () => { listener = null }
      } },
    },
    foreground: () => { listener?.() },
  }
}

afterEach(() => {
  window.location.hash = ''
  FakeSocket.instances = []
})

test('原生启动恢复有效配对并保留失效占位；占位不会连接', async () => {
  const native = bindings([
    { metadata: { sessionId: 'ready', name: '工作', customName: true, lastConnectedAt: 123 }, payload: pairing('ready') },
    { metadata: { sessionId: 'missing', name: '旧会话', customName: true, lastConnectedAt: null } },
  ])
  render(<App native={native.native} socketFactory={(url) => new FakeSocket(url)} />)
  await waitFor(() => expect(screen.getAllByTestId('session-list-item')).toHaveLength(2))
  expect(FakeSocket.instances).toHaveLength(1)
  expect(FakeSocket.instances[0]?.url).toBe(pairing('ready').wsUrl)
  expect(screen.getByText('需要重新配对')).toBeInTheDocument()
  fireEvent.click(screen.getAllByTestId('session-list-item')[1]!)
  expect(screen.getByText('扫描二维码')).toBeInTheDocument()
  expect(FakeSocket.instances).toHaveLength(1)
})

test('设置页按 PC 编码移除本地配对，其他 PC 保留且安全记录同步删除', async () => {
  const firstPC = '0123456789abcdef0123456789abcdef'
  const secondPC = 'abcdef0123456789abcdef0123456789'
  const native = bindings(['a', 'b', 'c'].map((id) => ({
    metadata: { sessionId: id, name: `会话 ${id}`, customName: true, lastConnectedAt: null },
    payload: { ...pairing(id), pcId: id === 'c' ? secondPC : firstPC },
  })))
  render(<App native={native.native} socketFactory={(url) => new FakeSocket(url)} />)
  await waitFor(() => expect(screen.getAllByTestId('session-list-item')).toHaveLength(3))
  fireEvent.click(screen.getByRole('button', { name: '设置' }))
  fireEvent.click(screen.getByText('PC 管理'))
  expect(screen.getAllByRole('button', { name: '移除此 PC' })).toHaveLength(2)
  fireEvent.click(screen.getAllByRole('button', { name: '移除此 PC' })[0]!)
  fireEvent.click(screen.getByRole('button', { name: '确认移除' }))
  await waitFor(() => expect(native.metadata.has('a') || native.metadata.has('b')).toBe(false))
  expect(native.secure.has('a') || native.secure.has('b')).toBe(false)
  expect(native.secure.has('c')).toBe(true)
  expect(screen.getAllByRole('button', { name: '移除此 PC' })).toHaveLength(1)
})

test('原生扫码安全保存后建立连接，URL 不留配对凭证；前台过期后不重连', async () => {
  let currentTime = Date.parse('2026-09-24T00:00:00Z')
  const native = bindings()
  render(<App native={native.native} now={() => currentTime} socketFactory={(url) => new FakeSocket(url)} />)
  await waitFor(() => expect(screen.getByText('扫描二维码')).toBeInTheDocument())
  fireEvent.click(screen.getByText('扫描二维码'))
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1))
  expect(native.secure.has('scanned')).toBe(true)
  expect(native.metadata.get('scanned')).toMatchObject({ sessionId: 'scanned', name: '会话 scanned' })
  expect(window.location.hash).toBe('')

  currentTime = Date.parse('2100-01-01T00:00:00Z')
  act(() => native.foreground())
  fireEvent.click(screen.getByLabelText('返回'))
  expect(screen.getByText('需要重新配对')).toBeInTheDocument()
  expect(FakeSocket.instances[0]?.closed).toBe(true)
})


test('通知点击在原生恢复完成后打开对应 Session，卸载移除监听', async () => {
  const native = bindings([
    { metadata: { sessionId: 'ready', name: '通知目标', customName: true, lastConnectedAt: null }, payload: pairing('ready') },
  ])
  let opened: ((value: { sessionId: string }) => void) | undefined
  const remove = vi.fn(async () => undefined)
  const clearNotifications = vi.fn(async () => undefined)
  const experience = {
    addListener: vi.fn(async (_event: 'notificationOpened', callback: (value: { sessionId: string }) => void) => { opened = callback; return { remove } }),
    clearNotifications,
    notify: vi.fn(async () => undefined),
  } as unknown as ExperiencePort
  const { unmount } = render(<App native={native.native} experience={experience} socketFactory={(url) => new FakeSocket(url)} />)
  await waitFor(() => expect(opened).toBeDefined())
  act(() => opened?.({ sessionId: 'missing' }))
  expect(FakeSocket.instances).toHaveLength(1)
  act(() => opened?.({ sessionId: 'ready' }))
  expect(screen.getByTestId('detail-scroll')).toBeInTheDocument()
  expect(clearNotifications).toHaveBeenCalledWith({ sessionId: 'ready' })
  expect(FakeSocket.instances).toHaveLength(1)
  unmount()
  await waitFor(() => expect(remove).toHaveBeenCalledOnce())
})
