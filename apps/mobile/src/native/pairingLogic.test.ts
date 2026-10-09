import { describe, expect, test, vi } from 'vitest'
import { encodePairingFragment, type PairingPayload } from '../protocol/pairing'
import {
  NativePairingLogic,
  NativePairingStorageError,
  observeNativeForeground,
  parseScannedPairing,
  planForegroundRefresh,
  type NativePairingMetadata,
  type PairingMetadataPort,
  type SecurePairingPort,
} from './pairingLogic'

const now = Date.parse('2026-09-24T10:00:00Z')
const pairing = (sessionId: string, expiresAt = '2026-09-25T10:00:00Z'): PairingPayload => ({
  version: 1,
  wsUrl: `ws://127.0.0.1/${sessionId}`,
  sessionId,
  pairToken: `fixture-token-${sessionId}`,
  expiresAt,
})
const metadata = (sessionId: string): NativePairingMetadata => ({
  sessionId, name: `名称 ${sessionId}`, customName: true, lastConnectedAt: 100,
})

function ports() {
  const events: string[] = []
  const secrets = new Map<string, unknown>()
  const records = new Map<string, NativePairingMetadata>()
  const secure: SecurePairingPort = {
    read: vi.fn(async (id) => secrets.get(id) ?? null),
    write: vi.fn(async (payload) => { events.push(`secure-write:${payload.sessionId}`); secrets.set(payload.sessionId, payload) }),
    remove: vi.fn(async (id) => { events.push(`secure-remove:${id}`); secrets.delete(id) }),
  }
  const ordinary: PairingMetadataPort = {
    list: vi.fn(async () => [...records.values()]),
    upsert: vi.fn(async (record) => { events.push(`metadata-write:${record.sessionId}`); records.set(record.sessionId, record) }),
    remove: vi.fn(async (id) => { events.push(`metadata-remove:${id}`); records.delete(id) }),
  }
  return { logic: new NativePairingLogic(secure, ordinary), secure, ordinary, events, secrets, records }
}

describe('原生配对纯逻辑', () => {
  test('普通元数据保留 PC 编码，失效占位仍可归属设备且不保存凭证', async () => {
    const state = ports()
    const payload = { ...pairing('a'), pcId: '0123456789abcdef0123456789abcdef', pcName: 'zhanglingsiMac-mini.local' }
    await state.logic.saveScanned(JSON.stringify(payload), now)
    expect(state.records.get('a')?.pcId).toBe(payload.pcId)
    expect(state.records.get('a')?.pcName).toBe(payload.pcName)
    expect(JSON.stringify(state.records.get('a'))).not.toContain(payload.pairToken)
    state.secrets.delete('a')
    expect((await state.logic.restore(now)).needsPairing[0]?.pcId).toBe(payload.pcId)
    expect((await state.logic.restore(now)).needsPairing[0]?.pcName).toBe(payload.pcName)
  })
  test('扫码 Fragment 复用现有校验，过期或无效内容不触发存储', async () => {
    const state = ports()
    const valid = pairing('a')
    expect(parseScannedPairing(`  #pair=${encodePairingFragment(valid)}  `, now))
      .toEqual({ ok: true, value: valid })
    expect((await state.logic.saveScanned('#pair=broken', now)).ok).toBe(false)
    expect((await state.logic.saveScanned(JSON.stringify(pairing('a', '2026-09-23T00:00:00Z')), now)).ok).toBe(false)
    expect(state.records.size).toBe(0)
    expect(state.secrets.size).toBe(0)
  })

  test('原生扫描取消不写入；成功扫描沿用同一解析与存储路径', async () => {
    const state = ports()
    expect(await state.logic.scanAndSave({ scan: async () => null }, now)).toBeNull()
    expect(state.events).toEqual([])
    const result = await state.logic.scanAndSave({
      scan: async () => `#pair=${encodePairingFragment(pairing('a'))}`,
    }, now)
    expect(result).toEqual({ ok: true, value: pairing('a') })
    expect(state.secrets.get('a')).toEqual(pairing('a'))
  })

  test('新配对与同 ID 换凭证只把完整负载交给安全端口，保留本地名称', async () => {
    const state = ports()
    const first = pairing('a')
    expect((await state.logic.saveScanned(JSON.stringify(first), now)).ok).toBe(true)
    expect(state.events).toEqual(['metadata-write:a', 'secure-write:a'])
    expect(JSON.stringify([...state.records.values()])).not.toContain(first.pairToken)
    await state.logic.rename('a', '我的会话')
    const rogueMetadata = { ...state.records.get('a')!, pairToken: 'unexpected-token' }
    state.records.set('a', rogueMetadata)
    const replacement = { ...first, pairToken: 'fixture-replacement', wsUrl: 'wss://example.test/ws' }
    await state.logic.saveScanned(`#pair=${encodePairingFragment(replacement)}`, now)
    expect(state.records.get('a')).toMatchObject({ name: '我的会话', customName: true })
    expect(JSON.stringify(state.records.get('a'))).not.toContain('unexpected-token')
    expect(state.secrets.get('a')).toEqual(replacement)
    expect((await state.logic.restore(now)).available).toEqual([{
      payload: replacement, metadata: state.records.get('a'),
    }])
  })

  test('恢复多个记录时隔离缺失、错绑、损坏和过期凭证', async () => {
    const state = ports()
    for (const id of ['a', 'missing', 'wrong', 'broken', 'expired']) state.records.set(id, metadata(id))
    state.secrets.set('a', pairing('a'))
    state.secrets.set('wrong', pairing('another'))
    state.secrets.set('broken', { pairToken: 'invalid' })
    state.secrets.set('expired', pairing('expired', '2026-09-23T00:00:00Z'))

    const restored = await state.logic.restore(now)
    expect(restored.available.map((item) => item.payload.sessionId)).toEqual(['a'])
    expect(restored.needsPairing.map((item) => item.sessionId)).toEqual(['missing', 'wrong', 'broken', 'expired'])
    expect(state.secrets.has('expired')).toBe(false)
    expect(state.records.has('expired')).toBe(true)
  })

  test('安全写入失败时只留下无凭证元数据；删除先清安全记录', async () => {
    const state = ports()
    vi.mocked(state.secure.write).mockRejectedValueOnce(new Error('fixture secret detail'))
    await expect(state.logic.saveScanned(JSON.stringify(pairing('a')), now))
      .rejects.toBeInstanceOf(NativePairingStorageError)
    expect(state.records.has('a')).toBe(true)
    expect(state.secrets.has('a')).toBe(false)
    expect((await state.logic.restore(now)).needsPairing.map((item) => item.sessionId)).toEqual(['a'])

    await state.logic.saveScanned(JSON.stringify(pairing('a')), now)
    state.events.length = 0
    await state.logic.remove('a')
    expect(state.events).toEqual(['secure-remove:a', 'metadata-remove:a'])
    expect(state.records.has('a')).toBe(false)
    expect(state.secrets.has('a')).toBe(false)
  })

  test('安全删除失败时保留元数据并只抛固定错误', async () => {
    const state = ports()
    state.records.set('a', metadata('a'))
    state.secrets.set('a', pairing('a'))
    vi.mocked(state.secure.remove).mockRejectedValueOnce(new Error('fixture secret detail'))
    await expect(state.logic.remove('a')).rejects.toMatchObject({
      name: 'NativePairingStorageError', message: '原生配对存储暂不可用',
    })
    expect(state.records.has('a')).toBe(true)
    expect(state.secrets.has('a')).toBe(true)
  })

  test('前台计划刷新所有有效 Session，过期者要求重新配对', () => {
    expect(planForegroundRefresh([
      pairing('a'), pairing('b'), pairing('a'), pairing('expired', '2026-09-23T00:00:00Z'),
    ], now)).toEqual({
      refreshSessionIds: ['a', 'b'], needsPairingSessionIds: ['expired'],
    })
  })

  test('原生前台事件读取最新配对并在取消监听后停止派发', () => {
    const hooks: { foreground?: () => void } = {}
    let pairings = [pairing('a')]
    const plans: unknown[] = []
    const unsubscribe = observeNativeForeground({
      onForeground: (listener) => {
        hooks.foreground = listener
        return () => { delete hooks.foreground }
      },
    }, () => pairings, () => now, (plan) => plans.push(plan))
    pairings = [pairing('a'), pairing('b')]
    hooks.foreground?.()
    expect(plans).toEqual([{ refreshSessionIds: ['a', 'b'], needsPairingSessionIds: [] }])
    unsubscribe()
    expect(hooks.foreground).toBeUndefined()
  })
})
