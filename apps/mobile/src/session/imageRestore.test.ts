import { expect, test } from 'vitest'
import { SessionRuntimeManager } from './runtime'
import { createAppStore } from './appStore'
import { createInitialState } from './reducer'
import type { SocketLike } from '../protocol/client'

const envelope = (type: string, payload: unknown, seq = 0) => ({ version: 10, type, sessionId: 's', commandId: '', requestId: '', seq, payload })
test('restores cached image over authenticated session, one at a time', () => {
  const store = createAppStore()
  const state = createInitialState()
  state.sessionId = 's'
  state.messages = [{ kind: 'input', requestId: 'r', source: 'mobile', text: '', status: 'accepted', attachments: [
    { name: 'a.png', mediaType: 'image/png', data: '', ref: 'upload-123:0' },
    { name: 'b.png', mediaType: 'image/png', data: '', ref: 'upload-123:1' },
  ] }]
  store.getState().restoreCachedSession('s', state)
  const sent: string[] = []
  const socket: SocketLike = { send(data) { sent.push(data) }, close() {}, onopen: null, onmessage: null, onclose: null, onerror: null }
  const runtime = new SessionRuntimeManager(store, () => socket, Date.now)
  runtime.ensureConnected({ version: 1, sessionId: 's', wsUrl: 'ws://host/ws', pairToken: 'test', expiresAt: '2099-01-01T00:00:00Z' })
  socket.onopen?.()
  const receive = (value: unknown) => socket.onmessage?.({ data: JSON.stringify(value) })
  receive(envelope('hello_ack', {}))
  receive(envelope('snapshot', { connectionStatus: 'subscribed', snapshot: { sessionId: 's', sessionStatus: 'ready', sessionError: null, seq: 0, turn: { requestId: '', source: '', status: 'idle', message: '', error: null, pendingAction: null }, queueDepth: 0, queuedRequests: [] } }))
  expect(sent.map(s => JSON.parse(s)).filter(x => x.type === 'image_get').map(x => x.payload.ref)).toEqual(['upload-123:0'])
  receive(envelope('image_result', { ref: 'upload-123:0', image: { name: 'a.png', mediaType: 'image/png', data: 'aGVsbG8=' } }))
  expect(sent.map(s => JSON.parse(s)).filter(x => x.type === 'image_get').map(x => x.payload.ref)).toEqual(['upload-123:0', 'upload-123:1'])
  receive(envelope('image_result', { ref: 'upload-123:1' }))
  const message = store.getState().sessions.s!.messages[0]
  expect(message.kind === 'input' && message.attachments?.map(a => a.data)).toEqual(['aGVsbG8=', ''])
  runtime.closeAll()
})
