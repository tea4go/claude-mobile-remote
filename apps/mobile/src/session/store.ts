// 页面生命周期内的配对记录（B1）。仅存在于内存：刷新即清空，
// 不写 localStorage / sessionStorage / IndexedDB（硬约束 7、D41）。
// 原生 App 的跨重启安全存储属于 C2，不在此实现。

import type { PairingPayload } from '../protocol/pairing'

// 本地连接状态，仅用于列表展示，不参与协议：
// new 从未连接过；connected 当前或曾经连接成功；
// disconnected 连接已断开；needsPairing 凭证失效需重新配对。
export type SessionLocalStatus = 'new' | 'connected' | 'disconnected' | 'needsPairing'

export interface SessionRecord {
  sessionId: string
  wsUrl: string
  pairToken: string
  expiresAt: string
  name: string
  lastConnectedAt: number | null
  status: SessionLocalStatus
}

export interface SessionStore {
  // 按创建顺序保存；展示顺序由 listSessions 计算，不就地重排。
  records: SessionRecord[]
}

export interface SessionStatusUpdate {
  status: SessionLocalStatus
  lastConnectedAt?: number
}

const NAME_DIGEST_LENGTH = 8

// 默认名称取 sessionId 摘要，避免列表出现无标识的记录。
export function defaultSessionName(sessionId: string): string {
  return `会话 ${sessionId.slice(0, NAME_DIGEST_LENGTH)}`
}

export function createInitialSessionStore(): SessionStore {
  return { records: [] }
}

// 配对成功时写入或刷新记录。同一 sessionId 只保留一条：刷新配对负载，
// 保留用户设置的本地名称与连接历史。
export function upsertSession(store: SessionStore, payload: PairingPayload): SessionStore {
  const index = store.records.findIndex((r) => r.sessionId === payload.sessionId)
  const next: SessionRecord = {
    sessionId: payload.sessionId,
    wsUrl: payload.wsUrl,
    pairToken: payload.pairToken,
    expiresAt: payload.expiresAt,
    name: store.records[index]?.name ?? defaultSessionName(payload.sessionId),
    lastConnectedAt: store.records[index]?.lastConnectedAt ?? null,
    status: store.records[index]?.status ?? 'new',
  }
  if (index < 0) {
    return { records: [...store.records, next] }
  }
  return {
    records: [...store.records.slice(0, index), next, ...store.records.slice(index + 1)],
  }
}

export function removeSession(store: SessionStore, sessionId: string): SessionStore {
  return { records: store.records.filter((r) => r.sessionId !== sessionId) }
}

function replaceRecord(
  store: SessionStore,
  sessionId: string,
  update: (record: SessionRecord) => SessionRecord,
): SessionStore {
  const index = store.records.findIndex((r) => r.sessionId === sessionId)
  if (index < 0) return store
  return {
    records: [
      ...store.records.slice(0, index),
      update(store.records[index]!),
      ...store.records.slice(index + 1),
    ],
  }
}

// 重命名只改本地展示名称，不触碰 sessionId 或配对负载；空名称回退默认摘要。
export function renameSession(store: SessionStore, sessionId: string, name: string): SessionStore {
  const trimmed = name.trim()
  return replaceRecord(store, sessionId, (record) => ({
    ...record,
    name: trimmed === '' ? defaultSessionName(sessionId) : trimmed,
  }))
}

// 未提供 lastConnectedAt 时保留原值：断开或失效不应清空连接历史。
export function markSessionStatus(
  store: SessionStore,
  sessionId: string,
  update: SessionStatusUpdate,
): SessionStore {
  return replaceRecord(store, sessionId, (record) => ({
    ...record,
    status: update.status,
    lastConnectedAt: update.lastConnectedAt ?? record.lastConnectedAt,
  }))
}

// 展示顺序：最近连接优先；从未连接的记录按创建顺序稳定排在后面。
export function listSessions(store: SessionStore): SessionRecord[] {
  return store.records
    .map((record, index) => ({ record, index }))
    .sort((a, b) => {
      const left = a.record.lastConnectedAt
      const right = b.record.lastConnectedAt
      if (left !== right) {
        if (left === null) return 1
        if (right === null) return -1
        return right - left
      }
      return a.index - b.index
    })
    .map((entry) => entry.record)
}
