import {
  parsePairingInput,
  parsePairingJson,
  type PairingPayload,
  type PairingResult,
} from '../protocol/pairing'
import { defaultSessionName } from '../session/store'

// 正式原生实现须把完整 Pairing 放入系统安全存储；测试可注入内存端口。
export interface SecurePairingPort {
  read(sessionId: string): Promise<unknown | null>
  write(payload: PairingPayload): Promise<void>
  remove(sessionId: string): Promise<void>
}

// 普通私有存储仅保存展示元数据，不能出现 pairToken 或完整 Pairing。
export interface NativePairingMetadata {
  sessionId: string
  name: string
  customName: boolean
  lastConnectedAt: number | null
  pcId?: string
  pcName?: string
}

export interface PairingMetadataPort {
  list(): Promise<NativePairingMetadata[]>
  upsert(metadata: NativePairingMetadata): Promise<void>
  remove(sessionId: string): Promise<void>
}

export interface NativeScanPort {
  // 用户取消扫描时返回 null；正式实现由相机插件提供。
  scan(): Promise<string | null>
}

export interface NativeAppStatePort {
  // 注册前台事件并返回取消监听函数。
  onForeground(listener: () => void): () => void
}

export interface RestoredNativePairings {
  available: Array<{ payload: PairingPayload; metadata: NativePairingMetadata }>
  needsPairing: NativePairingMetadata[]
}

export class NativePairingStorageError extends Error {
  constructor() {
    super('原生配对存储暂不可用')
    this.name = 'NativePairingStorageError'
  }
}

function storageError(): NativePairingStorageError {
  return new NativePairingStorageError()
}

function publicMetadata(record: NativePairingMetadata): NativePairingMetadata {
  return {
    sessionId: record.sessionId,
    name: record.name,
    customName: record.customName,
    lastConnectedAt: record.lastConnectedAt,
    ...(record.pcId === undefined ? {} : { pcId: record.pcId }),
    ...(record.pcName === undefined ? {} : { pcName: record.pcName }),
  }
}

export function parseScannedPairing(input: string, now: number): PairingResult {
  return parsePairingInput(input.trim(), now)
}

export class NativePairingLogic {
  private readonly secure: SecurePairingPort
  private readonly metadata: PairingMetadataPort

  constructor(
    secure: SecurePairingPort,
    metadata: PairingMetadataPort,
  ) {
    this.secure = secure
    this.metadata = metadata
  }

  async restore(now: number): Promise<RestoredNativePairings> {
    let records: NativePairingMetadata[]
    try {
      records = await this.metadata.list()
    } catch {
      throw storageError()
    }

    const result: RestoredNativePairings = { available: [], needsPairing: [] }
    const seen = new Set<string>()
    for (const storedMetadata of records) {
      const record = publicMetadata(storedMetadata)
      if (seen.has(record.sessionId)) continue
      seen.add(record.sessionId)
      let stored: unknown | null
      try {
        stored = await this.secure.read(record.sessionId)
      } catch {
        throw storageError()
      }
      if (stored === null) {
        result.needsPairing.push(record)
        continue
      }

      let parsed: PairingResult | null = null
      try {
        parsed = parsePairingJson(JSON.stringify(stored), now)
      } catch {
        // 损坏的安全存储内容不能进入连接层。
      }
      if (parsed?.ok && parsed.value.sessionId === record.sessionId) {
        result.available.push({ payload: parsed.value, metadata: record })
        continue
      }

      result.needsPairing.push(record)
      if (typeof stored === 'object' && stored !== null && 'expiresAt' in stored &&
        typeof stored.expiresAt === 'string' && Date.parse(stored.expiresAt) <= now) {
        try {
          await this.secure.remove(record.sessionId)
        } catch {
          // 删除失败也不能恢复过期凭证；后续可再次清理。
        }
      }
    }
    return result
  }

  async saveScanned(input: string, now: number): Promise<PairingResult> {
    const parsed = parseScannedPairing(input, now)
    if (!parsed.ok) return parsed

    try {
      const previous = (await this.metadata.list())
        .find((record) => record.sessionId === parsed.value.sessionId)
      await this.metadata.upsert(previous === undefined ? {
        sessionId: parsed.value.sessionId,
        name: defaultSessionName(parsed.value.sessionId),
        customName: false,
        lastConnectedAt: null,
        ...(parsed.value.pcId === undefined ? {} : { pcId: parsed.value.pcId }),
        ...(parsed.value.pcName === undefined ? {} : { pcName: parsed.value.pcName }),
      } : {
        ...publicMetadata(previous),
        pcId: parsed.value.pcId,
        pcName: parsed.value.pcName,
      })
      await this.secure.write(parsed.value)
    } catch {
      throw storageError()
    }
    return parsed
  }

  async scanAndSave(scanner: NativeScanPort, now: number): Promise<PairingResult | null> {
    let input: string | null
    try {
      input = await scanner.scan()
    } catch {
      throw storageError()
    }
    return input === null ? null : this.saveScanned(input, now)
  }

  async rename(sessionId: string, name: string): Promise<void> {
    try {
      const record = (await this.metadata.list()).find((item) => item.sessionId === sessionId)
      if (record === undefined) return
      const trimmed = name.trim()
      await this.metadata.upsert({
        ...publicMetadata(record),
        name: trimmed === '' ? defaultSessionName(sessionId) : trimmed,
        customName: trimmed !== '',
      })
    } catch {
      throw storageError()
    }
  }

  async markConnected(sessionId: string, at: number): Promise<void> {
    try {
      const record = (await this.metadata.list()).find((item) => item.sessionId === sessionId)
      if (record !== undefined) {
        await this.metadata.upsert({ ...publicMetadata(record), lastConnectedAt: at })
      }
    } catch {
      throw storageError()
    }
  }

  async remove(sessionId: string): Promise<void> {
    try {
      await this.secure.remove(sessionId)
      await this.metadata.remove(sessionId)
    } catch {
      throw storageError()
    }
  }
}

export interface ForegroundRefreshPlan {
  refreshSessionIds: string[]
  needsPairingSessionIds: string[]
}

// App 状态事件接线后只需刷新全部有效连接状态；断线重连仍由用户触发。
export function planForegroundRefresh(pairings: PairingPayload[], now: number): ForegroundRefreshPlan {
  const plan: ForegroundRefreshPlan = { refreshSessionIds: [], needsPairingSessionIds: [] }
  const seen = new Set<string>()
  for (const payload of pairings) {
    if (seen.has(payload.sessionId)) continue
    seen.add(payload.sessionId)
    const parsed = parsePairingJson(JSON.stringify(payload), now)
    if (parsed.ok) plan.refreshSessionIds.push(payload.sessionId)
    else plan.needsPairingSessionIds.push(payload.sessionId)
  }
  return plan
}

export function observeNativeForeground(
  appState: NativeAppStatePort,
  pairings: () => PairingPayload[],
  now: () => number,
  onPlan: (plan: ForegroundRefreshPlan) => void,
): () => void {
  return appState.onForeground(() => onPlan(planForegroundRefresh(pairings(), now())))
}
