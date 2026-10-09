import { Capacitor } from '@capacitor/core'
import { SecureStorage } from '@aparajita/capacitor-secure-storage'
import { Preferences } from '@capacitor/preferences'
import { App } from '@capacitor/app'
import { CapacitorBarcodeScanner, CapacitorBarcodeScannerTypeHint } from '@capacitor/barcode-scanner'
import type { PairingPayload } from '../protocol/pairing'
import {
  NativePairingLogic,
  type NativeAppStatePort,
  type NativePairingMetadata,
  type NativeScanPort,
  type PairingMetadataPort,
  type SecurePairingPort,
} from './pairingLogic'

const PAIRING_KEY = 'cmr.pairing.v1.'
const METADATA_KEY = 'cmr.metadata.v1.'

function key(prefix: string, sessionId: string): string {
  return `${prefix}${encodeURIComponent(sessionId)}`
}

function metadataValue(value: unknown): NativePairingMetadata | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null
  const record = value as Record<string, unknown>
  if (typeof record.sessionId !== 'string' || record.sessionId === '' ||
    typeof record.name !== 'string' || typeof record.customName !== 'boolean' ||
    (record.pcId !== undefined && (typeof record.pcId !== 'string' || !/^[0-9a-f]{32}$/.test(record.pcId))) ||
    (record.pcName !== undefined && (typeof record.pcName !== 'string' || record.pcName.trim() === '' || [...record.pcName].length > 255 || [...record.pcName].some((character) => {
      const code = character.codePointAt(0) ?? 0
      return code < 32 || code === 127
    }))) ||
    (record.lastConnectedAt !== null &&
      (typeof record.lastConnectedAt !== 'number' || !Number.isFinite(record.lastConnectedAt)))) return null
  return {
    sessionId: record.sessionId,
    name: record.name,
    customName: record.customName,
    lastConnectedAt: record.lastConnectedAt as number | null,
    ...(record.pcId === undefined ? {} : { pcId: record.pcId as string }),
    ...(record.pcName === undefined ? {} : { pcName: record.pcName as string }),
  }
}

export interface NativePairingBindings {
  logic: NativePairingLogic
  scanner: NativeScanPort
  appState: NativeAppStatePort
}

export function createCapacitorPairingPorts(): NativePairingBindings {
  if (!Capacitor.isNativePlatform()) throw new Error('原生配对只在 iOS/Android 可用')

  const secure: SecurePairingPort = {
    read: async (sessionId) => {
      const stored = await SecureStorage.get(key(PAIRING_KEY, sessionId), false, false)
      if (stored === null) return null
      if (typeof stored !== 'string') return stored
      try { return JSON.parse(stored) } catch { return stored }
    },
    write: async (payload: PairingPayload) => {
      await SecureStorage.set(key(PAIRING_KEY, payload.sessionId), JSON.stringify(payload), false, false)
    },
    remove: async (sessionId) => {
      await SecureStorage.remove(key(PAIRING_KEY, sessionId), false)
    },
  }
  const metadata: PairingMetadataPort = {
    list: async () => {
      const { keys } = await Preferences.keys()
      const result: NativePairingMetadata[] = []
      for (const item of keys.filter((name) => name.startsWith(METADATA_KEY)).sort()) {
        const { value } = await Preferences.get({ key: item })
        if (value === null) continue
        let parsed: unknown
        try { parsed = JSON.parse(value) } catch { continue }
        const record = metadataValue(parsed)
        if (record !== null && key(METADATA_KEY, record.sessionId) === item) result.push(record)
      }
      return result
    },
    upsert: async (record) => {
      const safe = metadataValue(record)
      if (safe === null) throw new Error('invalid metadata')
      await Preferences.set({ key: key(METADATA_KEY, safe.sessionId), value: JSON.stringify(safe) })
    },
    remove: async (sessionId) => {
      await Preferences.remove({ key: key(METADATA_KEY, sessionId) })
    },
  }
  const scanner: NativeScanPort = {
    scan: async () => {
      try {
        const result = await CapacitorBarcodeScanner.scanBarcode({
          hint: CapacitorBarcodeScannerTypeHint.QR_CODE,
          scanInstructions: '扫描电脑端的配对二维码',
        })
        return result.ScanResult || null
      } catch (error) {
        // 插件的取消行为因平台而异；用户主动取消时无需触发错误流程。
        if (error instanceof Error && /cancel/i.test(error.message)) return null
        throw error
      }
    },
  }
  const appState: NativeAppStatePort = {
    onForeground: (listener) => {
      let closed = false
      const handle = App.addListener('appStateChange', ({ isActive }) => {
        if (!closed && isActive) listener()
      }).catch(() => null)
      return () => {
        closed = true
        void handle.then((registered) => registered?.remove()).catch(() => undefined)
      }
    },
  }
  return { logic: new NativePairingLogic(secure, metadata), scanner, appState }
}
