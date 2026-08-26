import { describe, expect, it, vi } from 'vitest'
import type { PairingPayload } from '../protocol/pairing'
import {
  createInitialSessionStore,
  defaultSessionName,
  listSessions,
  markSessionStatus,
  removeSession,
  renameSession,
  upsertSession,
} from './store'

function payload(sessionId: string, overrides: Partial<PairingPayload> = {}): PairingPayload {
  return {
    version: 1,
    wsUrl: 'ws://127.0.0.1:8082/ws',
    sessionId,
    pairToken: `pair-token-${sessionId}`,
    expiresAt: '2026-08-27T00:00:00Z',
    ...overrides,
  }
}

describe('createInitialSessionStore', () => {
  it('从空列表开始', () => {
    expect(listSessions(createInitialSessionStore())).toEqual([])
  })
})

describe('upsertSession', () => {
  it('按配对负载新增一条记录并使用默认名称', () => {
    const store = upsertSession(createInitialSessionStore(), payload('session-aaaabbbb-1'))
    const records = listSessions(store)

    expect(records).toHaveLength(1)
    expect(records[0]).toMatchObject({
      sessionId: 'session-aaaabbbb-1',
      wsUrl: 'ws://127.0.0.1:8082/ws',
      pairToken: 'pair-token-session-aaaabbbb-1',
      expiresAt: '2026-08-27T00:00:00Z',
      name: defaultSessionName('session-aaaabbbb-1'),
      status: 'new',
      lastConnectedAt: null,
    })
  })

  it('同一 sessionId 更新为一条记录并刷新配对负载', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(
      store,
      payload('s-1', {
        wsUrl: 'ws://127.0.0.1:9090/ws',
        pairToken: 'pair-token-rotated',
        expiresAt: '2026-08-28T00:00:00Z',
      }),
    )
    const records = listSessions(store)

    expect(records).toHaveLength(1)
    expect(records[0]).toMatchObject({
      sessionId: 's-1',
      wsUrl: 'ws://127.0.0.1:9090/ws',
      pairToken: 'pair-token-rotated',
      expiresAt: '2026-08-28T00:00:00Z',
    })
  })

  it('同一 sessionId 重新配对时保留本地名称、状态与最近连接时间', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = renameSession(store, 's-1', '重构分支')
    store = markSessionStatus(store, 's-1', { status: 'connected', lastConnectedAt: 1000 })
    store = upsertSession(store, payload('s-1', { pairToken: 'pair-token-rotated' }))

    expect(listSessions(store)[0]).toMatchObject({
      name: '重构分支',
      status: 'connected',
      lastConnectedAt: 1000,
      pairToken: 'pair-token-rotated',
    })
  })

  it('不同 sessionId 保持两条记录', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(store, payload('s-2'))

    expect(listSessions(store).map((r) => r.sessionId)).toEqual(['s-1', 's-2'])
  })

  it('返回新对象而不修改原 store', () => {
    const store = createInitialSessionStore()
    const next = upsertSession(store, payload('s-1'))

    expect(next).not.toBe(store)
    expect(listSessions(store)).toEqual([])
  })
})

describe('removeSession', () => {
  it('只删除目标记录', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(store, payload('s-2'))
    store = removeSession(store, 's-1')

    expect(listSessions(store).map((r) => r.sessionId)).toEqual(['s-2'])
  })

  it('删除不存在的记录时列表不变', () => {
    const store = upsertSession(createInitialSessionStore(), payload('s-1'))

    expect(listSessions(removeSession(store, 's-missing')).map((r) => r.sessionId)).toEqual(['s-1'])
  })
})

describe('renameSession', () => {
  it('只改变目标记录的本地名称', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(store, payload('s-2'))
    store = renameSession(store, 's-1', '主分支会话')
    const [first, second] = listSessions(store)

    expect(first).toMatchObject({
      sessionId: 's-1',
      name: '主分支会话',
      pairToken: 'pair-token-s-1',
    })
    expect(second?.name).toBe(defaultSessionName('s-2'))
  })

  it('空名称或纯空白回退到默认摘要', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = renameSession(store, 's-1', '临时名称')

    expect(listSessions(renameSession(store, 's-1', ''))[0]?.name).toBe(defaultSessionName('s-1'))
    expect(listSessions(renameSession(store, 's-1', '   '))[0]?.name).toBe(defaultSessionName('s-1'))
  })

  it('去除名称首尾空白', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = renameSession(store, 's-1', '  带空白  ')

    expect(listSessions(store)[0]?.name).toBe('带空白')
  })
})

describe('markSessionStatus', () => {
  it('更新状态与最近连接时间且不影响其他记录', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(store, payload('s-2'))
    store = markSessionStatus(store, 's-1', { status: 'connected', lastConnectedAt: 1000 })
    const records = listSessions(store)

    expect(records.find((r) => r.sessionId === 's-1')).toMatchObject({
      status: 'connected',
      lastConnectedAt: 1000,
    })
    expect(records.find((r) => r.sessionId === 's-2')).toMatchObject({
      status: 'new',
      lastConnectedAt: null,
    })
  })

  it('只更新状态时保留原最近连接时间', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = markSessionStatus(store, 's-1', { status: 'connected', lastConnectedAt: 1000 })
    store = markSessionStatus(store, 's-1', { status: 'disconnected' })

    expect(listSessions(store)[0]).toMatchObject({
      status: 'disconnected',
      lastConnectedAt: 1000,
    })
  })

  it('支持标记需要重新配对', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = markSessionStatus(store, 's-1', { status: 'needsPairing' })

    expect(listSessions(store)[0]?.status).toBe('needsPairing')
  })
})

describe('listSessions 排序', () => {
  it('最近连接优先', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(store, payload('s-2'))
    store = upsertSession(store, payload('s-3'))
    store = markSessionStatus(store, 's-1', { status: 'connected', lastConnectedAt: 1000 })
    store = markSessionStatus(store, 's-3', { status: 'connected', lastConnectedAt: 2000 })

    expect(listSessions(store).map((r) => r.sessionId)).toEqual(['s-3', 's-1', 's-2'])
  })

  it('未连接记录按创建顺序稳定排列', () => {
    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = upsertSession(store, payload('s-2'))
    store = upsertSession(store, payload('s-3'))

    expect(listSessions(store).map((r) => r.sessionId)).toEqual(['s-1', 's-2', 's-3'])
    expect(listSessions(store).map((r) => r.sessionId)).toEqual(['s-1', 's-2', 's-3'])
  })
})

describe('存储边界', () => {
  it('所有操作都不写浏览器持久化存储', () => {
    // localStorage 与 sessionStorage 共用 Storage.prototype，一个 spy 即可覆盖两者。
    const setItem = vi.spyOn(Storage.prototype, 'setItem')

    let store = upsertSession(createInitialSessionStore(), payload('s-1'))
    store = renameSession(store, 's-1', '本地名称')
    store = markSessionStatus(store, 's-1', { status: 'connected', lastConnectedAt: 1000 })
    store = removeSession(store, 's-1')
    listSessions(store)

    expect(setItem).not.toHaveBeenCalled()
    setItem.mockRestore()
  })
})
