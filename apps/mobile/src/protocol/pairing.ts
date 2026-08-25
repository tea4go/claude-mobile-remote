// 配对负载解析与规范化（D28/D30）。
// 错误只返回固定类别与安全文案，不拼接原始 JSON、网络消息或 token。

import { PROTOCOL_VERSION } from './types'

export interface PairingPayload {
  version: number
  wsUrl: string
  sessionId: string
  pairToken: string
  expiresAt: string
}

export type PairingResult =
  | { ok: true; value: PairingPayload }
  | { ok: false; error: string }

const ERROR_INVALID = '配对信息无效'
const ERROR_EXPIRED = '配对凭证已过期，请重新配对'

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function validatePayload(value: unknown, now: number): PairingResult {
  if (!isObject(value)) return { ok: false, error: ERROR_INVALID }

  const { version, wsUrl, sessionId, pairToken, expiresAt } = value
  if (version !== PROTOCOL_VERSION) return { ok: false, error: ERROR_INVALID }
  if (
    typeof wsUrl !== 'string' ||
    typeof sessionId !== 'string' ||
    typeof pairToken !== 'string' ||
    typeof expiresAt !== 'string'
  ) {
    return { ok: false, error: ERROR_INVALID }
  }
  if (sessionId === '' || pairToken === '') {
    return { ok: false, error: ERROR_INVALID }
  }

  let url: URL
  try {
    url = new URL(wsUrl)
  } catch {
    return { ok: false, error: ERROR_INVALID }
  }
  if (url.protocol !== 'ws:' && url.protocol !== 'wss:') {
    return { ok: false, error: ERROR_INVALID }
  }

  const expires = Date.parse(expiresAt)
  if (Number.isNaN(expires)) return { ok: false, error: ERROR_INVALID }
  if (expires <= now) return { ok: false, error: ERROR_EXPIRED }

  return {
    ok: true,
    value: { version, wsUrl, sessionId, pairToken, expiresAt },
  }
}

// 解析 A4 输出的原始配对 JSON。now 可注入以便测试。
export function parsePairingJson(text: string, now: number): PairingResult {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return { ok: false, error: ERROR_INVALID }
  }
  return validatePayload(parsed, now)
}

function base64UrlEncode(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
}

function base64UrlDecode(text: string): string {
  const normalized = text.replaceAll('-', '+').replaceAll('_', '/')
  const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
  return atob(padded)
}

// 把已校验 payload 规范化为无填充 Base64URL Fragment 内容（不含 `#pair=` 前缀）。
export function encodePairingFragment(payload: PairingPayload): string {
  const json = JSON.stringify(payload)
  return base64UrlEncode(new TextEncoder().encode(json))
}

// 从 `#pair=<Base64URL(JSON)>` 形式的 hash 解析配对负载。
export function parsePairingFragment(hash: string, now: number): PairingResult {
  const match = /^#pair=([A-Za-z0-9_-]+)$/.exec(hash)
  if (!match) return { ok: false, error: ERROR_INVALID }
  try {
    const json = base64UrlDecode(match[1]!)
    return parsePairingJson(json, now)
  } catch {
    return { ok: false, error: ERROR_INVALID }
  }
}
