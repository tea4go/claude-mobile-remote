import { fireEvent, render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { upsertSession, createInitialSessionStore } from '../session/store'
import PCSettings from './PCSettings'
import { groupPCs } from './pcGroups'
import { createInitialState } from '../session/reducer'

const pcId = '0123456789abcdef0123456789abcdef'
function session(id: string, wsUrl: string, device?: string) {
  return upsertSession(createInitialSessionStore(), {
    version: 1, wsUrl, sessionId: id, pairToken: `test-${id}`, expiresAt: '2099-01-01T00:00:00Z',
    ...(device === undefined ? {} : { pcId: device }),
  }).records[0]!
}

test('同一编码跨地址合并，旧版按地址分组且不混入新编码', () => {
  const records = [
    session('a', 'ws://10.0.0.1:8765/ws', pcId),
    session('b', 'ws://10.0.0.2:8765/ws', pcId),
    session('c', 'ws://10.0.0.1:8765/ws'),
  ]
  const groups = groupPCs(records)
  expect(groups).toHaveLength(2)
  expect(groups[0]?.sessions.map((record) => record.sessionId)).toEqual(['a', 'b'])
  expect(groups[1]?.legacy).toBe(true)
})

test('PC 管理只显示设备，移除 PC 提交该设备的会话 ID', () => {
  const onRemovePC = vi.fn()
  render(<PCSettings sessions={[
    session('a', 'ws://10.0.0.1:8765/ws', pcId),
    session('b', 'ws://10.0.0.2:8765/ws', pcId),
    session('c', 'ws://10.0.0.3:8765/ws', 'abcdef0123456789abcdef0123456789'),
  ]} onRemovePC={onRemovePC} />)
  expect(screen.queryByText('会话 a')).not.toBeInTheDocument()
  expect(screen.queryByText('新增 PC 配对')).not.toBeInTheDocument()
  fireEvent.click(screen.getAllByRole('button', { name: '移除此 PC' })[0]!)
  expect(onRemovePC).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: '确认移除' }))
  expect(onRemovePC).toHaveBeenCalledWith(['a', 'b'])
})

test('机器名优先展示，连接状态由活跃会话归并', () => {
  const connected = session('a', 'ws://10.0.0.1:8765/ws', pcId)
  connected.pcName = 'zhanglingsiMac-mini.local'
  render(<PCSettings sessions={[{ ...connected, runtimeState: { ...createInitialState(), connection: 'connected' } }]} onRemovePC={vi.fn()} />)
  expect(screen.getByText('zhanglingsiMac-mini.local')).toBeInTheDocument()
  expect(screen.getByText(/已连接/)).toBeInTheDocument()
})
