import { Capacitor } from '@capacitor/core'
import { Preferences } from '@capacitor/preferences'

const key = 'cmr.quickPair.recentAddresses'
let writes: Promise<void> = Promise.resolve()

export function validPairingAddress(value: string): boolean {
  if (!value || value.length > 100 || /[/?#@\s]/.test(value)) return false
  try {
    const url = new URL(`ws://${value}`)
    if (url.hostname.startsWith('[')) return true
    const match = /^(\d{1,3}(?:\.\d{1,3}){3})(?::\d+)?$/.exec(value)
    return !!match && match[1]!.split('.').every((n) => Number(n) <= 255)
  } catch { return false }
}

function decode(value: string | null): string[] {
  try {
    const parsed: unknown = JSON.parse(value ?? '[]')
    if (!Array.isArray(parsed)) return []
    return [...new Set(parsed.filter((item): item is string => typeof item === 'string' && validPairingAddress(item)))].slice(0, 10)
  } catch { return [] }
}

export async function readRecentPairingAddresses(): Promise<string[]> {
  if (!Capacitor.isNativePlatform()) return []
  await writes
  return decode((await Preferences.get({ key })).value)
}

export function rememberPairingAddress(address: string): Promise<void> {
  if (!Capacitor.isNativePlatform() || !validPairingAddress(address)) return Promise.resolve()
  const next = writes.catch(() => {}).then(async () => {
    const existing = decode((await Preferences.get({ key })).value)
    await Preferences.set({ key, value: JSON.stringify([address, ...existing.filter((item) => item !== address)].slice(0, 10)) })
  })
  writes = next.catch(() => {})
  return next
}

export function clearRecentPairingAddresses(): Promise<void> {
  if (!Capacitor.isNativePlatform()) return Promise.resolve()
  const next = writes.catch(() => {}).then(async () => { await Preferences.remove({ key }) })
  writes = next.catch(() => {})
  return next
}
