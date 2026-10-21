import { describe, expect, test } from 'vitest'
import { ReminderPolicy, reminderKind } from './experience'
import type { DomainEvent } from '../protocol/types'

function event(seq: number, kind: DomainEvent['kind'] = 'turn_completed', sessionId = 'one'): DomainEvent {
  return { seq, kind, sessionId, requestId: 'r', source: 'mobile', text: '', sessionStatus: 'ready', turnStatus: 'completed', error: null }
}
describe('系统提醒策略', () => {
  test('snapshot 建立水位，旧事件和重复事件不提醒，不同 Session 独立', () => {
    const policy = new ReminderPolicy()
    policy.snapshot('one', 10)
    expect(policy.event(event(10), null)).toBeNull()
    expect(policy.event(event(11), null)).toBe('completed')
    expect(policy.event(event(11), null)).toBeNull()
    expect(policy.event(event(1, 'turn_failed', 'two'), null)).toBe('failed')
    policy.snapshot('one', 5)
    expect(policy.event(event(11), null)).toBeNull()
  })
  test('当前正在查看的会话不提醒且消耗序号；取消/流式/开始不提醒', () => {
    const policy = new ReminderPolicy()
    expect(policy.event(event(1), 'one')).toBeNull()
    expect(policy.event(event(1), null)).toBeNull()
    expect(reminderKind({ ...event(2), turnStatus: 'cancelled' })).toBeNull()
    expect(reminderKind(event(3, 'text_delta'))).toBeNull()
    expect(reminderKind(event(4, 'turn_started'))).toBeNull()
    expect(reminderKind(event(5, 'session_stopped'))).toBe('stopped')
  })
  test('审批与问答使用固定分类，原始正文/错误不进入提醒参数', () => {
    expect(reminderKind({ ...event(1, 'action_required'), pendingAction: { kind: 'tool_approval' } as DomainEvent['pendingAction'] })).toBe('approval')
    expect(reminderKind({ ...event(2, 'action_required'), pendingAction: { kind: 'ask_user_question' } as DomainEvent['pendingAction'] })).toBe('question')
  })
})
