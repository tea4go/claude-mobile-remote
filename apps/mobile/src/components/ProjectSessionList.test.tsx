import { describe, expect, test, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import ProjectSessionList from './ProjectSessionList'
import { formatActivityTime } from './activityTime'
import { defaultSessionName } from '../session/store'
import { projectGroups } from './projectGroups'
import type { SessionListItem } from './ProjectSessionList'
import { createInitialState } from '../session/reducer'

function item(id: string, cwd: string | null, time: string | null): SessionListItem {
  return {
    sessionId: id, wsUrl: 'ws://pc/ws', pairToken: 'placeholder', expiresAt: '2099-01-01T00:00:00Z',
    name: `会话 ${id}`, lastConnectedAt: null, status: 'connected',
    runtimeState: { ...createInitialState(), projectCwd: cwd, lastActivityAt: time },
  }
}

describe('列表活动时间', () => {
  const local = (year: number, month: number, day: number, hour = 20, minute = 18) => new Date(year, month - 1, day, hour, minute).toISOString()

  test('按本地日期展示今天、昨天、本周、本年和跨年', () => {
    const now = new Date(2026, 10, 6, 23, 59)
    expect(formatActivityTime(local(2026, 11, 6), now)).toBe('20:18')
    expect(formatActivityTime(local(2026, 11, 5), now)).toBe('昨天20:18')
    expect(formatActivityTime(local(2026, 11, 2), now)).toBe('星期一')
    expect(formatActivityTime(local(2026, 11, 1), now)).toBe('11月1日')
    expect(formatActivityTime(local(2025, 10, 20), now)).toBe('2025年10月20日')
    expect(formatActivityTime('invalid', now)).toBe('')
  })

  test('跨年优先显示完整日期，周日和周一各自属于不同周', () => {
    expect(formatActivityTime(local(2025, 12, 31), new Date(2026, 0, 1))).toBe('2025年12月31日')
    expect(formatActivityTime(local(2026, 11, 1), new Date(2026, 10, 3))).toBe('11月1日')
    expect(formatActivityTime(local(2026, 11, 8), new Date(2026, 10, 8))).toBe('20:18')
  })

  test('默认标题不重复时间，保留自定义标题', () => {
    const noop = () => {}
    const createdAt = local(2025, 10, 20)
    const base = item('a', '/work/alpha', createdAt)
    const record = { ...base, name: defaultSessionName('a'), runtimeState: { ...base.runtimeState!, createdAt } }
    const props = { onAdd: noop, onOpen: noop, onRemove: noop, onRename: noop }
    const view = render(<ProjectSessionList sessions={[record]} {...props} />)
    expect(screen.getByTestId('session-list-item')).toHaveTextContent('新会话')
    expect(screen.getByTestId('session-list-item').textContent).not.toContain('新会话 ·')
    view.rerender(<ProjectSessionList sessions={[{ ...record, name: '用户自定义', customName: true }]} {...props} />)
    expect(screen.getByText('用户自定义')).toBeInTheDocument()
  })
})

describe('项目分组', () => {
  test('按活动时间排序组与组内会话，连接时间不参与', () => {
    const groups = projectGroups([
      item('a', '/work/alpha', '2026-09-23T01:00:00Z'),
      item('b', '/work/beta', '2026-09-23T03:00:00Z'),
      item('c', '/work/alpha', '2026-09-23T02:00:00Z'),
    ])
    expect(groups.map((group) => [group.title, group.ids])).toEqual([
      ['beta', ['b']], ['alpha', ['c', 'a']],
    ])
  })

  test('旧服务缺目录进入未知组', () => {
    expect(projectGroups([item('old', null, null)])[0]?.title).toBe('工作目录未知')
  })

  test('列表停留期间固定行序，新会话并入已有目录组', () => {
    const noop = () => {}
    const a = item('a', '/work/alpha', '2026-09-23T01:00:00Z')
    const b = item('b', '/work/alpha', '2026-09-23T02:00:00Z')
    const props = { onAdd: noop, onOpen: noop, onRemove: noop, onRename: noop }
    const view = render(<ProjectSessionList sessions={[a]} {...props} />)
    view.rerender(<ProjectSessionList sessions={[{ ...a, runtimeState: { ...a.runtimeState!, lastActivityAt: '2026-09-23T03:00:00Z' } }, b]} {...props} />)
    expect(screen.getAllByText('/work/alpha')).toHaveLength(1)
    expect(screen.getAllByTestId('session-list-item').map((element) => element.getAttribute('data-session-id'))).toEqual(['a', 'b'])
  })

  test('相同工作目录在不同 PC 下分别显示', () => {
    const noop = () => {}
    const first = { ...item('a', '/work/shared', null), pcId: '0123456789abcdef0123456789abcdef', pcName: 'Mac mini' }
    const second = { ...item('b', '/work/shared', null), pcId: 'abcdef0123456789abcdef0123456789', pcName: 'Studio' }
    render(<ProjectSessionList sessions={[first, second]} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    expect(screen.getByText('Mac mini')).toBeInTheDocument()
    expect(screen.getByText('Studio')).toBeInTheDocument()
    expect(screen.getAllByText('/work/shared')).toHaveLength(2)
  })

  test('项目新建只使用本项目已连接并声明能力的来源，多来源直接创建', () => {
    const noop = () => {}
    const onCreate = vi.fn()
    const connected = (id: string, cwd: string): SessionListItem => ({ ...item(id, cwd, null), capabilities: ['create_session'], runtimeState: { ...createInitialState(), projectCwd: cwd, connection: 'connected' } })
    const a = connected('a', '/work/alpha')
    const b = connected('b', '/work/alpha')
    const other = connected('c', '/work/beta')
    render(<ProjectSessionList sessions={[a, b, other]} onCreate={onCreate} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    fireEvent.click(screen.getByRole('button', { name: '在 alpha 新建会话' }))
    expect(onCreate).toHaveBeenCalledExactlyOnceWith('a')
    expect(screen.queryByRole('group', { name: '选择 alpha 的来源会话' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '在 beta 新建会话' }))
    expect(onCreate).toHaveBeenLastCalledWith('c')
  })

  test('离线时使用目录和电脑身份进入定向配对', () => {
    const noop = () => {}
    const onCreate = vi.fn()
    const onPairProject = vi.fn()
    const pcId = '0123456789abcdef0123456789abcdef'
    render(<ProjectSessionList sessions={[{ ...item('a', '/work/alpha', null), pcId }]} onCreate={onCreate} onPairProject={onPairProject} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    const button = screen.getByRole('button', { name: '在 alpha 新建会话' })
    fireEvent.click(button)
    expect(onCreate).not.toHaveBeenCalled()
    expect(onPairProject).toHaveBeenCalledExactlyOnceWith(pcId, '/work/alpha')
  })

  test('PC 筛选只回传本地选择，保留其他 PC 的 Session', () => {
    const noop = () => {}
    const onSelectPC = vi.fn()
    const first = { ...item('a', '/work/shared', null), pcId: '0123456789abcdef0123456789abcdef', pcName: 'Mac mini' }
    const second = { ...item('b', '/work/shared', null), pcId: 'abcdef0123456789abcdef0123456789', pcName: 'Studio' }
    render(<ProjectSessionList sessions={[first, second]} onSelectPC={onSelectPC} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    fireEvent.click(screen.getByRole('button', { name: '筛选电脑，当前 Mac mini' }))
    expect(screen.getByText('选择电脑')).toBeInTheDocument()
    expect(onSelectPC).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '取消' }))
    expect(onSelectPC).not.toHaveBeenCalled()
    expect(screen.getAllByTestId('session-list-item')).toHaveLength(2)
  })
  test('指定电脑时抽屉标记选中项，列表不显示额外的全部电脑入口', () => {
    const noop = () => {}
    const first = { ...item('a', '/work/alpha', null), pcId: '0123456789abcdef0123456789abcdef', pcName: 'Mac mini' }
    render(<ProjectSessionList sessions={[first]} selectedPC={`pc:${first.pcId}`} onSelectPC={noop} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    expect(screen.queryByText('显示全部电脑')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '筛选电脑，当前 Mac mini' }))
    expect(screen.getByText('选择电脑')).toBeInTheDocument()
    expect(screen.getByText('全部电脑')).toBeInTheDocument()

  })
  test('目录行可折叠和展开，且不触发新建', () => {
    const noop = () => {}
    const create = vi.fn()
    render(<ProjectSessionList sessions={[item('a', '/work/alpha', null)]} onCreate={create} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    const toggle = screen.getByRole('button', { name: /alpha.*work\/alpha/ })
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    fireEvent.click(toggle)
    expect(screen.queryByTestId('session-list-item')).not.toBeInTheDocument()
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(toggle)
    expect(screen.getByTestId('session-list-item')).toBeInTheDocument()
    expect(create).not.toHaveBeenCalled()
  })

})
