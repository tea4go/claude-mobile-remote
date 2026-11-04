import { describe, expect, test, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import ProjectSessionList from './ProjectSessionList'
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

  test('项目新建只使用本项目已连接并声明能力的来源，多来源要求选择', () => {
    const noop = () => {}
    const onCreate = vi.fn()
    const connected = (id: string, cwd: string): SessionListItem => ({ ...item(id, cwd, null), capabilities: ['create_session'], runtimeState: { ...createInitialState(), projectCwd: cwd, connection: 'connected' } })
    const a = connected('a', '/work/alpha')
    const b = connected('b', '/work/alpha')
    const other = connected('c', '/work/beta')
    render(<ProjectSessionList sessions={[a, b, other]} onCreate={onCreate} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    fireEvent.click(screen.getByRole('button', { name: '在 alpha 新建会话' }))
    expect(onCreate).not.toHaveBeenCalled()
    expect(screen.getByRole('group', { name: '选择 alpha 的来源会话' })).toHaveTextContent('会话 a')
    fireEvent.click(screen.getByRole('button', { name: '会话 b' }))
    expect(onCreate).toHaveBeenCalledExactlyOnceWith('b')
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
    expect(screen.getByRole('button', { name: '全部电脑' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'Studio' }))
    expect(onSelectPC).toHaveBeenCalledExactlyOnceWith('pc:abcdef0123456789abcdef0123456789')
    expect(screen.getAllByTestId('session-list-item')).toHaveLength(2)
  })
  test('指定电脑时抽屉标记选中项，列表不显示额外的全部电脑入口', () => {
    const noop = () => {}
    const first = { ...item('a', '/work/alpha', null), pcId: '0123456789abcdef0123456789abcdef', pcName: 'Mac mini' }
    render(<ProjectSessionList sessions={[first]} selectedPC={`pc:${first.pcId}`} onSelectPC={noop} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    expect(screen.queryByText('显示全部电脑')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '筛选电脑，当前 Mac mini' }))
    expect(screen.getByRole('button', { name: 'Mac mini' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: '全部电脑' })).toHaveAttribute('aria-pressed', 'false')
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
