import { describe, expect, test, vi } from 'vitest'
import type { PairingPayload } from '../protocol/pairing'
import { connectionStateChanged, sessionOpened } from './reducer'
import { createAppStore, selectSession } from './appStore'

const pairing = (sessionId: string): PairingPayload => ({
  version: 1,
  wsUrl: `ws://127.0.0.1/${sessionId}`,
  sessionId,
  pairToken: `token-${sessionId}`,
  expiresAt: '2099-01-01T00:00:00Z',
})

describe('App Store', () => {
  test('按 sessionId 保存配对、状态、未读和导航', () => {
    const store = createAppStore()
    store.getState().upsertPairing(pairing('a'))
    store.getState().dispatchSession('a', sessionOpened(null))
    store.getState().dispatchSession('a', connectionStateChanged('connected'), true)

    expect(store.getState().pairings.records).toHaveLength(1)
    expect(store.getState().sessions.a?.connection).toBe('connected')
    expect(store.getState().unread.a).toBe(true)

    store.getState().openDetail('a')
    expect(store.getState()).toMatchObject({ view: 'detail', currentSessionId: 'a' })
    expect(store.getState().unread.a).toBe(false)
  })

  test('更新 A 不通知只订阅 B 状态的消费者', () => {
    const store = createAppStore()
    store.getState().dispatchSession('a', sessionOpened(null))
    store.getState().dispatchSession('b', sessionOpened(null))
    const listener = vi.fn()
    let previous = selectSession(store.getState(), 'b')
    const unsubscribe = store.subscribe((state) => {
      const next = selectSession(state, 'b')
      if (next !== previous) {
        previous = next
        listener(next)
      }
    })

    store.getState().dispatchSession('a', connectionStateChanged('connected'))
    expect(listener).not.toHaveBeenCalled()
    unsubscribe()
  })

  test('删除 A 只清理 A，不影响 B', () => {
    const store = createAppStore()
    store.getState().upsertPairing(pairing('a'))
    store.getState().upsertPairing(pairing('b'))
    store.getState().dispatchSession('a', sessionOpened(null))
    store.getState().dispatchSession('b', sessionOpened(null))
    store.getState().removePairing('a')

    expect(store.getState().pairings.records.map((record) => record.sessionId)).toEqual(['b'])
    expect(store.getState().sessions.a).toBeUndefined()
    expect(store.getState().sessions.b).toBeDefined()
  })
})
