import { describe, expect, test } from 'vitest'
import {
  encodePairingFragment,
  parsePairingFragment,
  parsePairingJson,
  type PairingPayload,
} from './pairing'

const NOW = 1_700_000_000_000

// 固定类别错误文案，不包含原始 JSON 或 token。
const EXPIRED = '配对凭证已过期，请重新配对'
const INVALID = '配对信息无效'

function validPayload(): PairingPayload {
  return {
    version: 1,
    wsUrl: 'ws://127.0.0.1:8765/ws',
    sessionId: 'session-1',
    pairToken: 'test-token-placeholder',
    expiresAt: '2026-08-24T12:00:00Z',
  }
}

describe('parsePairingJson：原始 JSON', () => {
  test('合法 payload 解析成功', () => {
    const result = parsePairingJson(JSON.stringify(validPayload()), NOW)
    expect(result.ok).toBe(true)
    if (result.ok) {
      expect(result.value.sessionId).toBe('session-1')
    }
  })

  test('wss URL 合法', () => {
    const payload = validPayload()
    payload.wsUrl = 'wss://example.com/ws'
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result.ok).toBe(true)
  })

  test('非 JSON 文本 → 配对信息无效', () => {
    const result = parsePairingJson('not json', NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('非对象 JSON（数组）→ 配对信息无效', () => {
    const result = parsePairingJson('[]', NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('version 2 → 配对信息无效', () => {
    const payload = validPayload()
    payload.version = 2
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('version 缺失 → 配对信息无效', () => {
    const payload = validPayload() as unknown as Record<string, unknown>
    delete payload.version
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('http scheme → 配对信息无效', () => {
    const payload = validPayload()
    payload.wsUrl = 'http://127.0.0.1:8765/ws'
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('相对 URL → 配对信息无效', () => {
    const payload = validPayload()
    payload.wsUrl = '/ws'
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('空 sessionId → 配对信息无效', () => {
    const payload = validPayload()
    payload.sessionId = ''
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('空 pairToken → 配对信息无效', () => {
    const payload = validPayload()
    payload.pairToken = ''
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('expiresAt 不可解析 → 配对信息无效', () => {
    const payload = validPayload()
    payload.expiresAt = 'not-a-date'
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('expiresAt 已过期 → 配对凭证已过期', () => {
    const payload = validPayload()
    payload.expiresAt = '2020-01-01T00:00:00Z'
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    expect(result).toEqual({ ok: false, error: EXPIRED })
  })

  test('错误文案不包含 token（不泄露）', () => {
    const payload = validPayload()
    payload.expiresAt = '2020-01-01T00:00:00Z'
    const result = parsePairingJson(JSON.stringify(payload), NOW)
    if (result.ok) throw new Error('expected failure')
    expect(result.error).not.toContain(payload.pairToken)
    expect(result.error).not.toContain(payload.wsUrl)
  })
})

describe('Fragment 编解码', () => {
  test('规范化为无填充 Base64URL 并可往返解析', () => {
    const payload = validPayload()
    const fragment = encodePairingFragment(payload)
    expect(fragment).not.toContain('+')
    expect(fragment).not.toContain('/')
    expect(fragment).not.toContain('=')

    const hash = `#pair=${fragment}`
    const result = parsePairingFragment(hash, NOW)
    expect(result.ok).toBe(true)
    if (result.ok) {
      expect(result.value).toEqual(payload)
    }
  })

  test('标准 Base64（含 + / =）不是 Base64URL，拒绝', () => {
    const payload = validPayload()
    const json = JSON.stringify(payload)
    const base64 = btoa(json)
    if (!/[+/=]/.test(base64)) throw new Error('fixture should contain base64 chars')
    const result = parsePairingFragment(`#pair=${base64}`, NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('空 hash → 配对信息无效', () => {
    const result = parsePairingFragment('', NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('无 pair 键的 hash → 配对信息无效', () => {
    const result = parsePairingFragment('#other=x', NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('Base64URL 内容非合法 JSON → 配对信息无效', () => {
    const garbage = btoa('garbage').replaceAll('+', '-').replaceAll('/', '_')
    const result = parsePairingFragment(`#pair=${garbage}`, NOW)
    expect(result).toEqual({ ok: false, error: INVALID })
  })

  test('Fragment 内过期 payload → 配对凭证已过期', () => {
    const payload = validPayload()
    payload.expiresAt = '2020-01-01T00:00:00Z'
    const fragment = encodePairingFragment(payload)
    const result = parsePairingFragment(`#pair=${fragment}`, NOW)
    expect(result).toEqual({ ok: false, error: EXPIRED })
  })
})
