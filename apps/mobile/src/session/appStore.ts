import { createStore, type StoreApi } from 'zustand/vanilla'
import type { PairingPayload } from '../protocol/pairing'
import {
  createInitialState,
  reducer,
  type Action,
  type MobileState,
} from './reducer'
import {
  createInitialSessionStore,
  markSessionStatus,
  removeSession,
  renameSession,
  restoreSession,
  upsertSession,
  type SessionLocalStatus,
  type SessionRecord,
  type SessionStore,
} from './store'

export type AppView = 'list' | 'detail'

export interface AppStoreState {
  pairings: SessionStore
  sessions: Record<string, MobileState>
  unread: Record<string, boolean>
  pairingInvalid: Record<string, boolean>
  view: AppView
  currentSessionId: string | null
  adding: boolean
  pairingError: string | null
  upsertPairing(payload: PairingPayload): void
  restorePairing(record: Pick<SessionRecord, 'sessionId' | 'name' | 'customName' | 'lastConnectedAt' | 'pcId' | 'pcName'>, payload?: PairingPayload): void
  renamePairing(sessionId: string, name: string): void
  markPairingStatus(sessionId: string, status: SessionLocalStatus, lastConnectedAt?: number): void
  removePairing(sessionId: string): void
  dispatchSession(sessionId: string, action: Action, visibleEvent?: boolean): void
  openDetail(sessionId: string): void
  showList(): void
  setAdding(adding: boolean): void
  setPairingError(error: string | null): void
  setPairingInvalid(sessionId: string, invalid: boolean): void
}

export type AppStore = StoreApi<AppStoreState>

export function createAppStore(): AppStore {
  return createStore<AppStoreState>((set) => ({
    pairings: createInitialSessionStore(),
    sessions: {},
    unread: {},
    pairingInvalid: {},
    view: 'list',
    currentSessionId: null,
    adding: false,
    pairingError: null,
    upsertPairing: (payload) => set((state) => ({
      pairings: upsertSession(state.pairings, payload),
      pairingInvalid: { ...state.pairingInvalid, [payload.sessionId]: false },
    })),
    restorePairing: (record, payload) => set((state) => ({
      pairings: restoreSession(state.pairings, record, payload),
      pairingInvalid: { ...state.pairingInvalid, [record.sessionId]: payload === undefined },
    })),
    renamePairing: (sessionId, name) => set((state) => ({
      pairings: renameSession(state.pairings, sessionId, name),
    })),
    markPairingStatus: (sessionId, status, lastConnectedAt) => set((state) => ({
      pairings: markSessionStatus(state.pairings, sessionId, { status, lastConnectedAt }),
    })),
    removePairing: (sessionId) => set((state) => {
      const sessions = { ...state.sessions }
      const unread = { ...state.unread }
      const pairingInvalid = { ...state.pairingInvalid }
      delete sessions[sessionId]
      delete unread[sessionId]
      delete pairingInvalid[sessionId]
      return {
        pairings: removeSession(state.pairings, sessionId),
        sessions,
        unread,
        pairingInvalid,
        currentSessionId: state.currentSessionId === sessionId ? null : state.currentSessionId,
        view: state.currentSessionId === sessionId ? 'list' : state.view,
      }
    }),
    dispatchSession: (sessionId, action, visibleEvent = false) => set((state) => {
      const current = state.sessions[sessionId] ?? createInitialState()
      const next = reducer(current, action)
      const isCurrentDetail = state.view === 'detail' && state.currentSessionId === sessionId
      return {
        sessions: { ...state.sessions, [sessionId]: next },
        unread: visibleEvent && !isCurrentDetail
          ? { ...state.unread, [sessionId]: true }
          : state.unread,
      }
    }),
    openDetail: (sessionId) => set((state) => ({
      view: 'detail',
      currentSessionId: sessionId,
      adding: false,
      pairingError: null,
      unread: { ...state.unread, [sessionId]: false },
    })),
    showList: () => set({
      view: 'list',
      currentSessionId: null,
      adding: false,
      pairingError: null,
    }),
    setAdding: (adding) => set({ adding }),
    setPairingError: (pairingError) => set({ pairingError }),
    setPairingInvalid: (sessionId, invalid) => set((state) => ({
      pairingInvalid: { ...state.pairingInvalid, [sessionId]: invalid },
    })),
  }))
}

export function selectSession(state: AppStoreState, sessionId: string): MobileState | undefined {
  return state.sessions[sessionId]
}

export function selectPairing(state: AppStoreState, sessionId: string): SessionRecord | undefined {
  return state.pairings.records.find((record) => record.sessionId === sessionId)
}
