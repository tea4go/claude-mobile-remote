import type { SessionRecord } from './records'

export interface PairingRecordStorage { saveRecords(records: Array<SessionRecord>): Promise<void> }
export type PairingCommitResult = 'saved' | 'cancelled' | 'failed' | 'rollback_failed'

// Persist first; if connectivity is lost or a write fails, restore the previous records.
// The caller owns a single in-flight operation and must not edit records during it.
export async function commitPairingRecords(storage: PairingRecordStorage, candidate: Array<SessionRecord>,
  previous: Array<SessionRecord>, stillValid: () => boolean): Promise<PairingCommitResult> {
  try {
    await storage.saveRecords(candidate)
  } catch (_) {
    try { await storage.saveRecords(previous) } catch (_) { return 'rollback_failed' }
    return 'failed'
  }
  if (!stillValid()) {
    try { await storage.saveRecords(previous) } catch (_) { return 'rollback_failed' }
    return 'cancelled'
  }
  return 'saved'
}
