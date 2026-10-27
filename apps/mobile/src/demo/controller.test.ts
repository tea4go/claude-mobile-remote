import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DemoController } from './controller'

describe('offline demo', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())
  it('streams serially without transport and accepts another turn', () => {
    const demo = new DemoController(() => undefined)
    demo.send('hello'); demo.send('ignored')
    expect(demo.state.messages).toHaveLength(2)
    vi.runAllTimers()
    expect(demo.state.canSend).toBe(true)
    expect(demo.state.messages[1]).toMatchObject({ status: 'done' })
    demo.send('next'); vi.runAllTimers()
    expect(demo.state.messages).toHaveLength(4)
  })
  it.each(['allow_once', 'deny'] as const)('resolves approval %s', decision => {
    const demo = new DemoController(() => undefined)
    demo.send('演示审批'); vi.runAllTimers()
    const action = demo.state.pendingAction!
    expect(action.kind).toBe('tool_approval')
    expect(demo.resolve({ actionId: 'stale', decision })).toBe(false)
    expect(demo.resolve({ actionId: action.actionId, decision })).toBe(true)
    vi.runAllTimers(); expect(demo.state.canSend).toBe(true)
  })
  it('answers and cancels with no late events after reset or dispose', () => {
    const changed = vi.fn()
    const demo = new DemoController(changed)
    demo.send('演示问答'); vi.runAllTimers()
    expect(demo.state.pendingAction?.kind).toBe('ask_user_question')
    demo.resolve({ actionId: demo.state.pendingAction!.actionId, answers: [] })
    expect(demo.cancel('wrong')).toBe(false)
    expect(demo.cancel('demo-1')).toBe(true)
    const count = changed.mock.calls.length
    vi.runAllTimers(); expect(changed).toHaveBeenCalledTimes(count)
    demo.send('next'); demo.reset(); vi.runAllTimers()
    expect(demo.state.messages).toHaveLength(0)
    demo.send('last'); demo.dispose()
    const disposed = changed.mock.calls.length
    vi.runAllTimers(); expect(changed).toHaveBeenCalledTimes(disposed)
  })
})
