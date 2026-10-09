import { describe, expect, test } from 'vitest'
import {
  PAIRING_PAYLOAD_VERSION,
  encodePairingFragment,
  parsePairingFragment,
  parsePairingInput,
  parsePairingJson,
  type PairingPayload,
} from './pairing'
import { PROTOCOL_VERSION } from './types'

const NOW = 1_700_000_000_000

// 固定类别错误文案，不包含原始 JSON 或 token。
const EXPIRED = '配对凭证已过期，请重新配对'
const INVALID = '配对信息无效'

function validPayload(): PairingPayload {
  return {
    version: PAIRING_PAYLOAD_VERSION,
    wsUrl: 'ws://127.0.0.1:8765/ws',
    sessionId: 'session-1',
    pairToken: 'test-token-placeholder',
    expiresAt: '2026-08-24T12:00:00Z',
  }
}

// 配对负载版本（internal/pairing/payload.go 的 PayloadVersion）与传输层
// 协议版本（internal/transport/protocol.go 的 Version）是两个独立契约，
// B2 把协议升到 2 时配对负载仍为 1。
describe('配对负载版本与协议版本相互独立', () => {
  test('PAIRING_PAYLOAD_VERSION 为 1，不随协议版本升级', () => {
    expect(PAIRING_PAYLOAD_VERSION).toBe(1)
  })

  test('配对负载不接受传输层协议版本号', () => {
    expect(PROTOCOL_VERSION).not.toBe(PAIRING_PAYLOAD_VERSION)
    const payload = validPayload()
    payload.version = PROTOCOL_VERSION
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({
      ok: false,
      error: INVALID,
    })
  })
})

describe('parsePairingJson：原始 JSON', () => {
  test('可选 PC 编码保留，旧配对兼容，错误编码拒绝', () => {
    const payload = { ...validPayload(), pcId: '0123456789abcdef0123456789abcdef' }
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({ ok: true, value: payload })
    expect(parsePairingJson(JSON.stringify(validPayload()), NOW).ok).toBe(true)
    expect(parsePairingJson(JSON.stringify({ ...payload, pcId: 'machine-1' }), NOW)).toEqual({ ok: false, error: INVALID })
  })
  test('可选 PC 名称保留并拒绝控制字符', () => {
    const payload = { ...validPayload(), pcName: 'zhanglingsiMac-mini.local' }
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({ ok: true, value: payload })
    expect(parsePairingJson(JSON.stringify({ ...payload, pcName: 'bad\nname' }), NOW)).toEqual({ ok: false, error: INVALID })
  })
  test('保留已声明的多 Session 能力且旧配对保持兼容', () => {
    const payload = validPayload()
    payload.capabilities = ['create_session', 'stop_session']
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({ ok: true, value: payload })
    delete payload.capabilities
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({ ok: true, value: payload })
  })

  test('拒绝未知或重复能力', () => {
    const payload = validPayload() as unknown as Record<string, unknown>
    payload.capabilities = ['create_session', 'unknown']
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({ ok: false, error: INVALID })
    payload.capabilities = ['create_session', 'create_session']
    expect(parsePairingJson(JSON.stringify(payload), NOW)).toEqual({ ok: false, error: INVALID })
  })
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

  test('version 2（传输层协议版本号）→ 配对信息无效', () => {
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

describe('parsePairingInput：手工输入', () => {
  test('同时接受原始 JSON 与完整 Fragment', () => {
    const payload = validPayload()
    const jsonResult = parsePairingInput(JSON.stringify(payload), NOW)
    const fragmentResult = parsePairingInput(`#pair=${encodePairingFragment(payload)}`, NOW)

    expect(jsonResult).toEqual({ ok: true, value: payload })
    expect(fragmentResult).toEqual(jsonResult)
  })

  test('非法输入仍返回固定错误', () => {
    expect(parsePairingInput('#pair=garbage', NOW)).toEqual({ ok: false, error: INVALID })
  })
})

describe('Fragment 编解码', () => {
  test('与 Go 端固定 Pairing fixture 互操作', () => {
    const fragment =
      '#pair=eyJ2ZXJzaW9uIjoxLCJ3c1VybCI6IndzOi8vMTkyLjAuMi4xMDo4NzY1L3dzIiwic2Vzc2lvbklkIjoic2Vzc2lvbi0xIiwicGFpclRva2VuIjoidG9rZW4tZml4dHVyZSIsImV4cGlyZXNBdCI6IjIwMjYtMDgtMzFUMTI6MDA6MDBaIn0'
    const result = parsePairingFragment(fragment, NOW)
    expect(result).toEqual({
      ok: true,
      value: {
        version: 1,
        wsUrl: 'ws://192.0.2.10:8765/ws',
        sessionId: 'session-1',
        pairToken: 'token-fixture',
        expiresAt: '2026-08-31T12:00:00Z',
      },
    })
  })

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
