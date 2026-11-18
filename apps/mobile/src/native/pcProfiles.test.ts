import { expect, test, vi } from 'vitest'
import { decodePCProfiles, readPCProfiles, writePCProfiles, replacePCIP } from './pcProfiles'
const state = vi.hoisted(() => ({ value: null as string | null, native: true }))
vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => state.native } }))
vi.mock('@capacitor/preferences', () => ({ Preferences: { get: async () => ({ value: state.value }), set: async (arg: { value: string }) => { state.value = arg.value } } }))
test('PC 配置重启恢复且只保存名称/IP，过滤未知字段与非法地址', async () => {
  const config = { 'pc:a': { name: '我的电脑', ip: '192.168.1.3', pairToken: 'never-save' }, 'pc:b': { name: '', ip: '192.168.1.4' }, 'pc:c': { name: 'C', ip: '192.168.1.5:123' } }
  await writePCProfiles(config)
  expect(await readPCProfiles()).toEqual({ 'pc:a': { name: '我的电脑', ip: '192.168.1.3' } })
  expect(state.value).not.toContain('never-save')
  expect(decodePCProfiles('{broken')).toEqual({})
  state.native = false
  expect(await readPCProfiles()).toEqual({})
})
test('只替换 IP，保留每会话端口、路径和协议，支持 IPv6', () => {
  expect(replacePCIP('ws://192.168.1.1:8765/s1', '10.0.0.2')).toBe('ws://10.0.0.2:8765/s1')
  expect(replacePCIP('wss://192.168.1.1:8443/s2', '[::1]')).toBe('wss://[::1]:8443/s2')
  expect(() => replacePCIP('ws://localhost/a', '10.0.0.2:99')).toThrow()
  expect(() => replacePCIP('ws://localhost/a', 'bad@host')).toThrow()
})
