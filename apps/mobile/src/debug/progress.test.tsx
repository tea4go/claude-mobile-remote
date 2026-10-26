import { SessionRuntimeManager } from '../session/runtime'
import { createAppStore } from '../session/appStore'
import type { SocketLike } from '../protocol/client'
import { act, render, screen, cleanup } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { decodeServerMessage } from '../protocol/decode'
import { reducer, createInitialState, snapshotReceived, eventReceived } from '../session/reducer'
import { debugLog, recordHostDiagnostic } from './log'
import TurnLoading from '../components/TurnLoading'
import type { DomainEvent, SessionSnapshot, TurnProgress } from '../protocol/types'
const progress: TurnProgress = { phase: 'writing', startedAt: 1000, updatedAt: 2000, outputTokens: 42, tokensKnown: true, tokensEstimated: true }
const snapshot: SessionSnapshot = { sessionId: 's', sessionStatus: 'ready', sessionError: null, seq: 0, turn: {status:'running',requestId:'r',source:'terminal',message:'',error:null,pendingAction:null,progress},queueDepth:0,queuedRequests:[] }
const event = (kind: DomainEvent['kind'], seq: number, extra: Partial<DomainEvent> = {}): DomainEvent => ({ seq,kind,sessionId:'s',requestId:'r',source:'',text:'',sessionStatus:'',turnStatus:'',error:null,...extra })
const wire = (e: DomainEvent, version=5) => JSON.stringify({version,type:'event',sessionId:'s',commandId:'',requestId:e.requestId,seq:e.seq,payload:{event:e}})
afterEach(()=>{ cleanup();vi.useRealTimers();debugLog.setEnabled(false);debugLog.clear() })
test('v5 decoder preserves safe progress and rejects malformed/legacy progress',()=>{
 const e=event('turn_progress',1,{progress})
 expect(decodeServerMessage(wire(e)).type).toBe('event')
 expect(()=>decodeServerMessage(wire(e,4))).toThrow()
 expect(()=>decodeServerMessage(wire({...e,progress:{...progress,outputTokens:-1}}))).toThrow()
 expect(()=>decodeServerMessage(wire({...e,progress:{...progress,toolName:'private\nvalue'}}))).toThrow()
 expect(()=>decodeServerMessage(wire(event('session_diagnostic',2,{diagnostic:{code:'api_retry',httpStatus:999}})))).toThrow()
})
test('snapshot restores progress; late status cannot change completed or newer turn',()=>{
 let state=reducer(createInitialState(),snapshotReceived(snapshot))
 expect(state.progress?.outputTokens).toBe(42)
 state=reducer(state,eventReceived(event('turn_completed',1,{turnStatus:'completed'})))
 expect(state.progress).toBeNull()
 state=reducer(state,eventReceived(event('turn_progress',2,{progress})))
 expect(state.progress).toBeNull()
 state=reducer(state,eventReceived(event('turn_started',3,{requestId:'new',source:'terminal',text:'input',turnStatus:'running'})))
 state=reducer(state,eventReceived(event('turn_progress',4,{progress})))
 expect(state.progress).toBeNull()
 expect(state.phase).toBe('connected')
})
test('loading uses server start time, shows estimated tokens and permission priority',()=>{
 vi.useFakeTimers();vi.setSystemTime(5000)
 const {rerender,unmount}=render(<TurnLoading progress={progress} waiting={false}/>)
 expect(screen.getByRole('status')).toHaveTextContent('正在生成回复（4s · ↓ 约 42 tokens）')
 act(()=>vi.advanceTimersByTime(1000))
 expect(screen.getByRole('status')).toHaveTextContent('5s')
 rerender(<TurnLoading progress={{...progress,phase:'tool',toolName:'Bash'}} waiting={true}/>)
 expect(screen.getByRole('status')).toHaveTextContent('等待审批或回答')
 unmount();expect(vi.getTimerCount()).toBe(0)
})
test('host diagnostics obey switch, fixed code allowlist and never copy arbitrary fields',()=>{
 debugLog.setEnabled(false)
 recordHostDiagnostic({code:'receive_failed'});expect(debugLog.getSnapshot().entries).toHaveLength(0)
 debugLog.setEnabled(true)
 const raw = {code:'api_retry' as const,httpStatus:429,attempt:2,maxAttempts:5,retryDelayMs:1000,message:'SECRET',sessionId:'PRIVATE',error:'RAW'}
 recordHostDiagnostic(raw)
 recordHostDiagnostic({...raw,code:'RAW' as 'api_retry'})
 const text=debugLog.exportText()
 expect(text).toContain('HTTP 429');expect(text).toContain('重试 2/5');expect(text).not.toMatch(/SECRET|PRIVATE|RAW/)
 expect(debugLog.getSnapshot().entries).toHaveLength(2)
})

test('runtime forwards host diagnostic and lifecycle failures to panel without original errors',()=>{
 debugLog.setEnabled(true)
 const store=createAppStore()
 const socket: SocketLike={send(){},close(){},onopen:null,onmessage:null,onerror:null,onclose:null}
 const runtime=new SessionRuntimeManager(store,()=>socket,Date.now)
 runtime.ensureConnected({version:1,sessionId:'s',wsUrl:'ws://user:password@host:9000/private?token=SECRET',pairToken:'PRIVATE',expiresAt:'2099-01-01T00:00:00Z'})
 socket.onopen?.()
 const send=(value:unknown)=>socket.onmessage?.({data:JSON.stringify(value)})
 send({version:5,type:'hello_ack',sessionId:'',commandId:'',requestId:'',seq:0,payload:{}})
 send({version:5,type:'snapshot',sessionId:'s',commandId:'',requestId:'',seq:0,payload:{connectionStatus:'subscribed',snapshot}})
 socket.onmessage?.({data:wire(event('session_diagnostic',1,{diagnostic:{code:'api_retry',httpStatus:429,attempt:2}}))})
 socket.onmessage?.({data:wire(event('turn_failed',2,{turnStatus:'failed',error:{code:'host_error',message:'RAW SECRET'}}))})
 const logs=debugLog.getSnapshot().entries.filter(e=>e.category==='host')
 expect(logs).toHaveLength(2)
 expect(logs[0]).toMatchObject({code:'api_retry',target:'ws://host:9000',level:'warn'})
 expect(logs[1].code).toBe('turn_failed')
 expect(debugLog.exportText()).not.toMatch(/password|PRIVATE|SECRET|RAW/)
 runtime.closeAll()
})
