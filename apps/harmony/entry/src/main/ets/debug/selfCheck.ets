import {
  ACTION_REQUIRED_EVENT_MESSAGE,
  CONTENT_BLOCK_EVENT_MESSAGE,
  EXPIRED_PAIRING_PAYLOAD,
  FIXTURE_NOW_MS,
  INVALID_TURN_STARTED_EVENT_MESSAGE,
  MISSING_ERROR_EVENT_MESSAGE,
  PRETURN_SNAPSHOT_MESSAGE,
  QUEUED_SNAPSHOT_MESSAGE,
  UNSUPPORTED_MESSAGE,
  VALID_PAIRING_PAYLOAD,
} from './fixtures'
import { decodePairingPayload } from '../protocol/pairing'
import { decodeServerMessage } from '../protocol/decode'
import type { PairingPayload } from '../protocol/types'
import {
  listSessionRecords,
  markSessionRecordStatus,
  renameSessionRecord,
  upsertSessionRecord,
} from '../session/records'

function expectTrue(condition: boolean, label: string): void {
  if (!condition) {
    throw new Error(`self-check failed: ${label}`)
  }
}

function expectThrows(action: () => void, label: string): void {
  let thrown = false
  try {
    action()
  } catch (_error) {
    thrown = true
  }
  if (!thrown) {
    throw new Error(`self-check failed: ${label}`)
  }
}

export function runSelfChecks(): string[] {
  const signals: string[] = []

  const pairing = decodePairingPayload(VALID_PAIRING_PAYLOAD, FIXTURE_NOW_MS)
  expectTrue(pairing.ok, 'valid pairing payload')
  if (!pairing.ok || pairing.payload === null) {
    throw new Error('unreachable')
  }
  expectTrue(pairing.payload.sessionId === 'session-1', 'pairing sessionId')
  signals.push('pairing-ok')

  const expiredPairing = decodePairingPayload(EXPIRED_PAIRING_PAYLOAD, FIXTURE_NOW_MS)
  expectTrue(!expiredPairing.ok, 'expired pairing should fail')
  if (expiredPairing.ok || expiredPairing.error === null) {
    throw new Error('unreachable')
  }
  expectTrue(expiredPairing.error.code === 'expired', 'expired pairing error code')
  signals.push('pairing-expired-ok')

  const preturnSnapshot = decodeServerMessage(PRETURN_SNAPSHOT_MESSAGE)
  expectTrue(preturnSnapshot.type === 'snapshot', 'preturn snapshot type')
  if (preturnSnapshot.type !== 'snapshot') {
    throw new Error('unreachable')
  }
  expectTrue(preturnSnapshot.payload.snapshot.turn.source === '', 'preturn source should stay empty')
  signals.push('preturn-source-ok')

  const contentBlockEvent = decodeServerMessage(CONTENT_BLOCK_EVENT_MESSAGE)
  expectTrue(contentBlockEvent.type === 'event', 'content block event type')
  if (contentBlockEvent.type !== 'event') {
    throw new Error('unreachable')
  }
  expectTrue(contentBlockEvent.payload.event.kind === 'content_block', 'content block kind')
  expectTrue(contentBlockEvent.payload.event.block !== undefined, 'content block payload')

  const actionRequiredEvent = decodeServerMessage(ACTION_REQUIRED_EVENT_MESSAGE)
  expectTrue(actionRequiredEvent.type === 'event', 'action required event type')
  if (actionRequiredEvent.type !== 'event') {
    throw new Error('unreachable')
  }
  expectTrue(actionRequiredEvent.payload.event.kind === 'action_required', 'action required kind')
  expectTrue(actionRequiredEvent.payload.event.pendingAction !== undefined, 'pending action payload')
  expectThrows(() => decodeServerMessage(INVALID_TURN_STARTED_EVENT_MESSAGE), 'invalid turn_started should fail')
  expectThrows(() => decodeServerMessage(MISSING_ERROR_EVENT_MESSAGE), 'missing error field should fail')

  const unsupported = decodeServerMessage(UNSUPPORTED_MESSAGE)
  expectTrue(unsupported.type === 'unsupported', 'unsupported type')
  if (unsupported.type !== 'unsupported') {
    throw new Error('unreachable')
  }
  expectTrue(unsupported.payload.code === 'unknown_block', 'unsupported code')
  signals.push('unsupported-ok')

  const queuedSnapshot = decodeServerMessage(QUEUED_SNAPSHOT_MESSAGE)
  expectTrue(queuedSnapshot.type === 'snapshot', 'queued snapshot type')
  if (queuedSnapshot.type !== 'snapshot') {
    throw new Error('unreachable')
  }
  expectTrue(queuedSnapshot.payload.snapshot.queueDepth === 2, 'queue depth')
  expectTrue(queuedSnapshot.payload.snapshot.queuedRequests.length === 2, 'queued request length')
  expectTrue(queuedSnapshot.payload.snapshot.turn.pendingAction !== null, 'snapshot pending action')
  signals.push('queued-requests-ok')

  const secondPairing: PairingPayload = {
    version: 1,
    wsUrl: 'ws://127.0.0.1:8765/ws',
    sessionId: 'session-2',
    pairToken: 'pair-token-3',
    expiresAt: '2026-09-02T00:00:00Z',
  }
  let records = upsertSessionRecord([], pairing.payload)
  records = renameSessionRecord(records, 'session-1', '桌面会话')
  records = markSessionRecordStatus(records, 'session-1', 'connected', FIXTURE_NOW_MS)
  records = upsertSessionRecord(records, secondPairing)
  const listed = listSessionRecords(records)
  expectTrue(listed.length === 2, 'session record length')
  expectTrue(listed[0].sessionId === 'session-1', 'recent session first')
  expectTrue(listed[0].name === '桌面会话', 'rename should persist')
  expectTrue(listed[0].status === 'connected', 'status should persist')
  expectTrue(listed[1].sessionId === 'session-2', 'new session should remain')
  signals.push('session-records-ok')

  return signals
}
