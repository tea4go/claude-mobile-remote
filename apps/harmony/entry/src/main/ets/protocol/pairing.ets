import {
  PAIRING_PAYLOAD_VERSION,
  type PairingPayload,
} from './types'

export type PairingDecodeErrorCode = 'invalid' | 'expired'

export interface PairingDecodeError {
  code: PairingDecodeErrorCode
  message: string
}

export interface PairingDecodeResult {
  ok: boolean
  payload: PairingPayload | null
  error: PairingDecodeError | null
}

interface PairingPayloadCandidate {
  version?: number
  wsUrl?: string
  sessionId?: string
  pairToken?: string
  expiresAt?: string
  capabilities?: string[]
  pcId?: string
  pcName?: string
}

const INVALID_PAIRING_MESSAGE = 'Invalid pairing payload'
const EXPIRED_PAIRING_MESSAGE = 'Pairing token expired'
const PAIRING_KEYS: string[] = ['version', 'wsUrl', 'sessionId', 'pairToken', 'expiresAt', 'capabilities', 'pcId', 'pcName']
const WS_URL_PATTERN = /^wss?:\/\/[^/\s?#]+(?:[/?#][^\s]*)?$/
const BASE64_URL_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'

function createPairingDecodeError(code: PairingDecodeErrorCode, message: string): PairingDecodeError {
  return { code, message }
}

function createPairingDecodeResult(
  ok: boolean,
  payload: PairingPayload | null,
  error: PairingDecodeError | null,
): PairingDecodeResult {
  return { ok, payload, error }
}

function invalidResult(): PairingDecodeResult {
  return createPairingDecodeResult(
    false,
    null,
    createPairingDecodeError('invalid', INVALID_PAIRING_MESSAGE),
  )
}

function expiredResult(): PairingDecodeResult {
  return createPairingDecodeResult(
    false,
    null,
    createPairingDecodeError('expired', EXPIRED_PAIRING_MESSAGE),
  )
}

function hasAllowedKeys(value: Object, allowedKeys: string[]): boolean {
  const keys = Object.keys(value)
  for (let index = 0; index < keys.length; index += 1) {
    if (allowedKeys.indexOf(keys[index]) < 0) {
      return false
    }
  }
  return true
}

function isPlainObject(value: Object | null): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function asPairingCandidate(value: Object | null): PairingPayloadCandidate | null {
  if (!isPlainObject(value)) {
    return null
  }
  return value as PairingPayloadCandidate
}

function readNonEmptyField(field: string | undefined): string | null {
  if (field === undefined || field.length === 0) {
    return null
  }
  return field
}

function decodeBase64UrlText(value: string): string | null {
  if (value.length === 0) {
    return null
  }

  let buffer = 0
  let bits = 0
  let output = ''
  for (let index = 0; index < value.length; index += 1) {
    const digit = BASE64_URL_ALPHABET.indexOf(value[index])
    if (digit < 0) {
      return null
    }
    buffer = (buffer << 6) | digit
    bits += 6
    while (bits >= 8) {
      bits -= 8
      output += String.fromCharCode((buffer >> bits) & 0xff)
    }
  }

  if (bits > 0 && (buffer & ((1 << bits) - 1)) !== 0) {
    return null
  }
  return output
}

function decodeRawPairingText(raw: string): string | null {
  const trimmed = raw.trim()
  if (trimmed.length === 0) {
    return null
  }
  if (!trimmed.startsWith('#pair=')) {
    return trimmed
  }
  return decodeBase64UrlText(trimmed.slice(6))
}

function createPairingPayload(
  wsUrl: string,
  sessionId: string,
  pairToken: string,
  expiresAt: string,
  capabilities?: string[],
  pcId?: string,
  pcName?: string,
): PairingPayload {
  const payload: PairingPayload = {
    version: PAIRING_PAYLOAD_VERSION,
    wsUrl,
    sessionId,
    pairToken,
    expiresAt,
  }
  if (capabilities !== undefined) {
    payload.capabilities = capabilities.slice()
  }
  if (pcId !== undefined) { payload.pcId = pcId }
  if (pcName !== undefined) { payload.pcName = pcName }
  return payload
}

function validatePairingPayload(value: Object | null, nowMs: number): PairingDecodeResult {
  const candidate = asPairingCandidate(value)
  if (candidate === null) {
    return invalidResult()
  }
  if (!hasAllowedKeys(candidate as Object, PAIRING_KEYS)) {
    return invalidResult()
  }

  const version = candidate.version
  const wsUrl = readNonEmptyField(candidate.wsUrl)
  const sessionId = readNonEmptyField(candidate.sessionId)
  const pairToken = readNonEmptyField(candidate.pairToken)
  const expiresAt = readNonEmptyField(candidate.expiresAt)
  if (
    version !== PAIRING_PAYLOAD_VERSION ||
    wsUrl === null ||
    sessionId === null ||
    pairToken === null ||
    expiresAt === null
  ) {
    return invalidResult()
  }

  if (!WS_URL_PATTERN.test(wsUrl)) {
    return invalidResult()
  }

  const expiresAtMs = Date.parse(expiresAt)
  if (Number.isNaN(expiresAtMs)) {
    return invalidResult()
  }
  if (expiresAtMs <= nowMs) {
    return expiredResult()
  }

  const capabilities = candidate.capabilities
  const pcId = candidate.pcId
  const pcName = candidate.pcName
  if (pcId !== undefined && (typeof pcId !== 'string' || !/^[0-9a-f]{32}$/.test(pcId))) {
    return invalidResult()
  }
  if (pcName !== undefined && (typeof pcName !== 'string' || pcName.trim().length === 0 || Array.from(pcName).length > 255 || Array.from(pcName).some((character: string) => {
    const code = character.codePointAt(0) ?? 0
    return code < 32 || code === 127
  }))) {
    return invalidResult()
  }
  if (capabilities !== undefined) {
    if (!Array.isArray(capabilities) || capabilities.length > 2) {
      return invalidResult()
    }
    const seen: string[] = []
    for (let index = 0; index < capabilities.length; index += 1) {
      const capability = capabilities[index]
      if ((capability !== 'create_session' && capability !== 'stop_session') || seen.indexOf(capability) >= 0) {
        return invalidResult()
      }
      seen.push(capability)
    }
  }

  return createPairingDecodeResult(
    true,
    createPairingPayload(wsUrl, sessionId, pairToken, expiresAt, capabilities, pcId, pcName),
    null,
  )
}

export function decodePairingPayload(raw: string, nowMs: number = Date.now()): PairingDecodeResult {
  const pairingText = decodeRawPairingText(raw)
  if (pairingText === null) {
    return invalidResult()
  }

  let parsedValue: Object | null
  try {
    parsedValue = JSON.parse(pairingText) as Object
  } catch (_error) {
    return invalidResult()
  }

  return validatePairingPayload(parsedValue, nowMs)
}
