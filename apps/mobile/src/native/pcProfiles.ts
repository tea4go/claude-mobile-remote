import { Capacitor } from '@capacitor/core'
import { Preferences } from '@capacitor/preferences'
import { validPairingAddress } from './recentPairingAddresses'

export interface PCProfile { name: string; ip: string }
export type PCProfiles = Record<string, PCProfile>
const key = 'cmr.pcProfiles.v1'
export function decodePCProfiles(value: string | null): PCProfiles {
  try {
    const data: unknown = JSON.parse(value ?? '{}')
    if (!data || typeof data !== 'object' || Array.isArray(data)) return {}
    return Object.fromEntries(Object.entries(data).filter(([id, p]) =>
      /^(pc:|address:|unknown:)/.test(id) && p && typeof p.name === 'string' && p.name.trim().length > 0 && p.name.length <= 255 && typeof p.ip === 'string' && validPairingAddress(p.ip) && new URL(`ws://${p.ip}`).port === '').map(([id, p]) => [id, { name: p.name, ip: p.ip }]))
  } catch { return {} }
}
export async function readPCProfiles(): Promise<PCProfiles> {
  return Capacitor.isNativePlatform() ? decodePCProfiles((await Preferences.get({ key })).value) : {}
}
export async function writePCProfiles(profiles: PCProfiles): Promise<void> {
  if (Capacitor.isNativePlatform()) await Preferences.set({ key, value: JSON.stringify(decodePCProfiles(JSON.stringify(profiles))) })
}
export function replacePCIP(wsUrl: string, ip: string): string {
  if (!validPairingAddress(ip) || new URL(`ws://${ip}`).port) throw new Error('请输入有效 IP，不含端口')
  const url = new URL(wsUrl)
  url.hostname = new URL(`ws://${ip}`).hostname
  return url.toString()
}
