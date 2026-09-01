import type { PairingPayload } from '../protocol/types'

export type SessionLocalStatus = 'new' | 'connected' | 'disconnected' | 'needsPairing'

export interface SessionRecord {
  sessionId: string
  wsUrl: string
  pairToken: string
  expiresAt: string
  name: string
  lastConnectedAt: number
  hasConnectedAt: boolean
  status: SessionLocalStatus
}

interface IndexedSessionRecord {
  record: SessionRecord
  index: number
}

const NAME_DIGEST_LENGTH = 8

export function defaultSessionName(sessionId: string): string {
  return `会话 ${sessionId.slice(0, NAME_DIGEST_LENGTH)}`
}

export function createSessionRecord(payload: PairingPayload): SessionRecord {
  const record: SessionRecord = {
    sessionId: payload.sessionId,
    wsUrl: payload.wsUrl,
    pairToken: payload.pairToken,
    expiresAt: payload.expiresAt,
    name: defaultSessionName(payload.sessionId),
    lastConnectedAt: 0,
    hasConnectedAt: false,
    status: 'new',
  }
  return record
}

function cloneRecord(record: SessionRecord): SessionRecord {
  return {
    sessionId: record.sessionId,
    wsUrl: record.wsUrl,
    pairToken: record.pairToken,
    expiresAt: record.expiresAt,
    name: record.name,
    lastConnectedAt: record.lastConnectedAt,
    hasConnectedAt: record.hasConnectedAt,
    status: record.status,
  }
}

export function upsertSessionRecord(records: Array<SessionRecord>, payload: PairingPayload): Array<SessionRecord> {
  const next: Array<SessionRecord> = []
  let found = false
  for (let index = 0; index < records.length; index += 1) {
    const record = records[index]
    if (record.sessionId !== payload.sessionId) {
      next.push(cloneRecord(record))
      continue
    }
    found = true
    next.push({
      sessionId: payload.sessionId,
      wsUrl: payload.wsUrl,
      pairToken: payload.pairToken,
      expiresAt: payload.expiresAt,
      name: record.name,
      lastConnectedAt: record.lastConnectedAt,
      hasConnectedAt: record.hasConnectedAt,
      status: record.status,
    })
  }
  if (!found) {
    next.push(createSessionRecord(payload))
  }
  return next
}

export function removeSessionRecord(records: Array<SessionRecord>, sessionId: string): Array<SessionRecord> {
  const next: Array<SessionRecord> = []
  for (let index = 0; index < records.length; index += 1) {
    if (records[index].sessionId !== sessionId) {
      next.push(cloneRecord(records[index]))
    }
  }
  return next
}

export function renameSessionRecord(records: Array<SessionRecord>, sessionId: string, name: string): Array<SessionRecord> {
  const trimmed = name.trim()
  const next: Array<SessionRecord> = []
  for (let index = 0; index < records.length; index += 1) {
    const record = cloneRecord(records[index])
    if (record.sessionId === sessionId) {
      record.name = trimmed.length > 0 ? trimmed : defaultSessionName(sessionId)
    }
    next.push(record)
  }
  return next
}

export function markSessionRecordStatus(
  records: Array<SessionRecord>,
  sessionId: string,
  status: SessionLocalStatus,
  connectedAt?: number,
): Array<SessionRecord> {
  const next: Array<SessionRecord> = []
  for (let index = 0; index < records.length; index += 1) {
    const record = cloneRecord(records[index])
    if (record.sessionId === sessionId) {
      record.status = status
      if (connectedAt !== undefined) {
        record.lastConnectedAt = connectedAt
        record.hasConnectedAt = true
      }
    }
    next.push(record)
  }
  return next
}

export function listSessionRecords(records: Array<SessionRecord>): Array<SessionRecord> {
  const indexed: Array<IndexedSessionRecord> = []
  for (let index = 0; index < records.length; index += 1) {
    const item: IndexedSessionRecord = {
      record: cloneRecord(records[index]),
      index,
    }
    indexed.push(item)
  }

  indexed.sort((left, right) => {
    if (left.record.hasConnectedAt !== right.record.hasConnectedAt) {
      return left.record.hasConnectedAt ? -1 : 1
    }
    if (left.record.hasConnectedAt && right.record.hasConnectedAt) {
      if (left.record.lastConnectedAt !== right.record.lastConnectedAt) {
        return right.record.lastConnectedAt - left.record.lastConnectedAt
      }
    }
    return left.index - right.index
  })

  const next: Array<SessionRecord> = []
  for (let index = 0; index < indexed.length; index += 1) {
    next.push(indexed[index].record)
  }
  return next
}

export function pairingPayloadFromRecord(record: SessionRecord): PairingPayload {
  return {
    version: 1,
    wsUrl: record.wsUrl,
    sessionId: record.sessionId,
    pairToken: record.pairToken,
    expiresAt: record.expiresAt,
  }
}
