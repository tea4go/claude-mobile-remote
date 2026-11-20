import { describe, expect, test, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
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

  test('首页 PC 卡片只展示，按外部选择筛选会话', () => {
    const noop = () => {}
    const first = { ...item('a', '/work/shared', null), pcId: '0123456789abcdef0123456789abcdef', pcName: 'Mac mini' }
    const second = { ...item('b', '/work/shared', null), pcId: 'abcdef0123456789abcdef0123456789', pcName: 'Studio' }
    render(<ProjectSessionList sessions={[first, second]} selectedPC={`pc:${first.pcId}`} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />)
    expect(screen.getByLabelText('当前电脑 Mac mini').tagName).toBe('DIV')
    expect(screen.queryByText('Studio')).not.toBeInTheDocument()
    expect(screen.queryByText('选择电脑')).not.toBeInTheDocument()
    expect(screen.getAllByTestId('session-list-item')).toHaveLength(1)
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

test('首页显示完整 Windows 路径，同名不同路径仍各自分组', () => {
  const noop = () => {}
  const first = item('win-a', 'C:\\work\\shared\\', null)
  const second = item('win-b', 'D:/remote/shared', null)
  render(<ProjectSessionList compactPCHeader sessions={[first, second]} onOpen={noop} onRemove={noop} onRename={noop} />)
  expect(screen.getAllByText('shared')).toHaveLength(2)
  const firstProject = screen.getByText(first.runtimeState!.projectCwd!).closest('button')
  const secondProject = screen.getByText(second.runtimeState!.projectCwd!).closest('button')
  expect(firstProject).toHaveTextContent('shared')
  expect(secondProject).toHaveTextContent('shared')
  expect(firstProject).not.toBe(secondProject)
  expect(screen.queryByRole('button', { name: '在 shared 新建会话' })).not.toBeInTheDocument()
})

test('iOS 会话左滑区用图标操作，重命名限长、拒绝空白且取消不保存', async () => {
  const session = item('swipe', '/work/alpha', null)
  const onRename = vi.fn()
  const onOpen = vi.fn()
  const onRemove = vi.fn()
  render(<ProjectSessionList compactPCHeader sessions={[session]} onRename={onRename} onOpen={onOpen} onRemove={onRemove} />)
  expect(screen.queryByRole('button', { name: /更多操作/ })).toBeNull()
  fireEvent.click(screen.getByRole('img', { name: '重命名' }))
  const input = await screen.findByRole('textbox', { name: '会话名称' })
  expect(input).toHaveValue(session.name)
  expect(screen.getByRole('dialog', { name: '重命名会话' })).toHaveClass('pairing-code-dialog')
  expect(input).toHaveFocus()
  expect((input as HTMLInputElement).selectionStart).toBe(0)
  expect((input as HTMLInputElement).selectionEnd).toBe(session.name.length)
  fireEvent.change(input, { target: { value: '   ' } })
  expect(screen.getByRole('button', { name: '保存' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: '取消' }))
  expect(onRename).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('img', { name: '重命名' }))
  const reopened = await screen.findByRole('textbox', { name: '会话名称' })
  fireEvent.change(reopened, { target: { value: '名'.repeat(45) } })
  expect(reopened).toHaveValue('名'.repeat(40))
  fireEvent.click(screen.getByRole('button', { name: '保存' }))
  expect(onRename).toHaveBeenCalledExactlyOnceWith(session.sessionId, '名'.repeat(40))
  expect(onOpen).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('img', { name: '删除' }))
  expect(onRemove).not.toHaveBeenCalled()
  fireEvent.click(await screen.findByText('删除', { selector: 'button span' }))
  await waitFor(() => expect(onRemove).toHaveBeenCalledExactlyOnceWith(session.sessionId))
})

test('新版会话卡片区分连接与轮次状态，目录仍可创建和折叠', () => {
  const onCreate = vi.fn()
  const record: SessionListItem = { ...item('a', '/Users/test/work/alpha', '2026-11-14T00:00:00Z'), capabilities: ['create_session'] }
  const connected = { ...record.runtimeState!, connection: 'connected' as const, turnStatus: 'idle' as const }
  const props = { compactPCHeader: true, revisedLayout: true, onCreate, onOpen: vi.fn(), onRemove: vi.fn(), onRename: vi.fn() }
  const view = render(<ProjectSessionList sessions={[{ ...record, runtimeState: connected }]} {...props} />)
  expect(screen.getByText('Token：place***lder', { exact: false })).toBeInTheDocument()
  expect(screen.queryByText(record.pairToken, { exact: false })).not.toBeInTheDocument()
  expect(screen.getByRole('img', { name: '已连接' })).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '已完成' })).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '在 alpha 新建会话' }))
  expect(onCreate).toHaveBeenCalledExactlyOnceWith('a')
  const toggle = screen.getByRole('button', { name: /alpha.*Users/ })
  fireEvent.click(toggle)
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByTestId('session-list-item')).not.toBeInTheDocument()
  fireEvent.click(toggle)
  const collapseAll = screen.getByRole('button', { name: '会话列表' })
  fireEvent.click(collapseAll)
  expect(collapseAll).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByTestId('session-list-item')).not.toBeInTheDocument()
  fireEvent.click(collapseAll)
  expect(screen.getByTestId('session-list-item')).toBeInTheDocument()

  const reply = { kind: 'claude' as const, source: 'terminal' as const, requestId: 'req', text: 'reply', error: null }
  for (const [status, label] of [['running', '进行中'], ['cancelled', '已中断'], ['failed', '已中断']] as const) {
    view.rerender(<ProjectSessionList sessions={[{ ...record, runtimeState: { ...connected, turnStatus: status, messages: [] } }]} {...props} />)
    expect(screen.getByRole('img', { name: label })).toBeInTheDocument()
  }
  const pendingAction = { kind: 'tool_approval' as const, actionId: 'approval', requestId: reply.requestId, toolName: 'Read', preview: '', allowSessionAvailable: false }
  const waiting = { ...connected, pendingAction, turnStatus: 'running' as const }
  view.rerender(<ProjectSessionList sessions={[{ ...record, runtimeState: waiting }]} {...props} />)
  expect(screen.getByRole('img', { name: '等待处理' })).toBeInTheDocument()
  view.rerender(<ProjectSessionList sessions={[{ ...record, runtimeState: { ...waiting, connection: 'disconnected' } }]} {...props} />)
  expect(screen.getByRole('img', { name: '上次同步：等待处理' })).toBeInTheDocument()
  view.rerender(<ProjectSessionList sessions={[{ ...record, pairToken: 'tiny', runtimeState: { ...connected, turnStatus: null, connection: 'disconnected' } }]} {...props} />)
  expect(screen.getByText('Token：***')).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '已断开' })).toBeInTheDocument()
  expect(screen.queryByRole('img', { name: '已完成' })).not.toBeInTheDocument()
})

test('新版目录最近活动顺序按所选 PC 计算，相同目录名不受另一台 PC 的活动影响', () => {
  const mac = 'mac'
  const sessions = [
    { ...item('other', '/work/shared', '2026-11-19T12:00:00Z'), pcId: 'other' },
    { ...item('old', '/work/shared', '2026-11-17T12:00:00Z'), pcId: mac },
    { ...item('new', '/work/new', '2026-11-18T12:00:00Z'), pcId: mac },
  ]
  const view = render(<ProjectSessionList sessions={sessions} revisedLayout compactPCHeader selectedPC="pc:mac" onOpen={vi.fn()} onRename={vi.fn()} onRemove={vi.fn()} />)
  expect([...view.container.querySelectorAll('.project-session-list__project-toggle strong')].map(el => el.textContent)).toEqual(['new', 'shared'])
})

test('新版会话列表标题去掉箭头后仍可统一折叠和展开', () => {
  const noop = () => {}
  render(<ProjectSessionList revisedLayout hideHeadingArrow sessions={[item('a', '/work/alpha', null)]}
    onOpen={noop} onRemove={noop} onRename={noop} />)
  const title = screen.getByRole('button', { name: '会话列表' })
  expect(title.textContent).toBe('会话列表')
  fireEvent.click(title)
  expect(title).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByTestId('session-list-item')).not.toBeInTheDocument()
  fireEvent.click(title)
  expect(screen.getByTestId('session-list-item')).toBeInTheDocument()
})
