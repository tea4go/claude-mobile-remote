import { beforeEach, expect, test, vi } from 'vitest'

const state = vi.hoisted(() => ({ value: null as string | null }))
vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => true } }))
vi.mock('@capacitor/preferences', () => ({ Preferences: {
  get: async () => ({ value: state.value }),
  remove: async () => { state.value = null },
  set: async ({ value }: { value: string }) => { state.value = value },
} }))
import { clearRecentPairingAddresses, readRecentPairingAddresses, rememberPairingAddress } from './recentPairingAddresses'

beforeEach(() => { state.value = null })

test('只保存有效地址，去重并按最近使用排序，最多十条', async () => {
  for (let index = 1; index <= 11; index++) await rememberPairingAddress(`10.0.0.${index}`)
  await rememberPairingAddress('10.0.0.3')
  await rememberPairingAddress('ws://host/?token=secret')
  const items = await readRecentPairingAddresses()
  expect(items).toHaveLength(10)
  expect(items[0]).toBe('10.0.0.3')
  expect(items).not.toContain('10.0.0.1')
  expect(state.value).not.toContain('secret')
})

test('清空会排在已有写入之后并持久化', async () => {
  const pending = rememberPairingAddress('10.0.0.1')
  await clearRecentPairingAddresses()
  await pending
  expect(await readRecentPairingAddresses()).toEqual([])
})
