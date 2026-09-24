import { beforeEach, expect, test, vi } from 'vitest'
import { createCapacitorPairingPorts } from './capacitorPorts'

const fake = vi.hoisted(() => ({
  native: true,
  secure: new Map<string, unknown>(),
  preferences: new Map<string, string>(),
  onState: null as ((state: { isActive: boolean }) => void) | null,
  scanResult: '#pair=invalid',
}))

vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => fake.native } }))
vi.mock('@aparajita/capacitor-secure-storage', () => ({
  SecureStorage: {
    get: async (key: string) => fake.secure.get(key) ?? null,
    set: async (key: string, value: unknown) => { fake.secure.set(key, value) },
    remove: async (key: string) => fake.secure.delete(key),
  },
}))
vi.mock('@capacitor/preferences', () => ({
  Preferences: {
    keys: async () => ({ keys: [...fake.preferences.keys()] }),
    get: async ({ key }: { key: string }) => ({ value: fake.preferences.get(key) ?? null }),
    set: async ({ key, value }: { key: string; value: string }) => { fake.preferences.set(key, value) },
    remove: async ({ key }: { key: string }) => { fake.preferences.delete(key) },
  },
}))
vi.mock('@capacitor/barcode-scanner', () => ({
  CapacitorBarcodeScanner: { scanBarcode: async () => ({ ScanResult: fake.scanResult }) },
  CapacitorBarcodeScannerTypeHint: { QR_CODE: 0 },
}))
vi.mock('@capacitor/app', () => ({
  App: { addListener: async (_event: string, listener: (state: { isActive: boolean }) => void) => {
    fake.onState = listener
    return { remove: async () => { fake.onState = null } }
  } },
}))

beforeEach(() => {
  fake.native = true
  fake.secure.clear()
  fake.preferences.clear()
  fake.onState = null
})

test('适配器在浏览器拒绝创建，避免 Web 插件的 localStorage 回退', () => {
  fake.native = false
  expect(() => createCapacitorPairingPorts()).toThrow()
  expect(fake.preferences.size).toBe(0)
})

test('完整凭证只写安全存储，Preferences 仅有白名单元数据并可恢复', async () => {
  const { logic } = createCapacitorPairingPorts()
  const payload = {
    version: 1, wsUrl: 'ws://127.0.0.1:8765/ws', sessionId: 'native-1',
    pairToken: 'fixture-token', expiresAt: '2099-01-01T00:00:00Z',
  }
  expect((await logic.saveScanned(JSON.stringify(payload), Date.now())).ok).toBe(true)
  expect([...fake.secure.values()]).toContain(JSON.stringify(payload))
  expect([...fake.preferences.values()].join('')).not.toContain(payload.pairToken)
  expect((await logic.restore(Date.now())).available[0]?.payload).toEqual(payload)
  await logic.rename(payload.sessionId, '本地名称')
  await logic.markConnected(payload.sessionId, 42)
  expect((await logic.restore(Date.now())).available[0]?.metadata).toMatchObject({ name: '本地名称', lastConnectedAt: 42 })
  await logic.remove(payload.sessionId)
  expect(fake.secure.size).toBe(0)
  expect(fake.preferences.size).toBe(0)
})

test('扫码与前台事件通过插件端口转发，注销后停止回调', async () => {
  const { scanner, appState } = createCapacitorPairingPorts()
  fake.scanResult = '#pair=fixture'
  expect(await scanner.scan()).toBe('#pair=fixture')
  let count = 0
  const dispose = appState.onForeground(() => { count += 1 })
  await vi.waitFor(() => expect(fake.onState).not.toBeNull())
  fake.onState?.({ isActive: false })
  fake.onState?.({ isActive: true })
  expect(count).toBe(1)
  dispose()
  await vi.waitFor(() => expect(fake.onState).toBeNull())
})
