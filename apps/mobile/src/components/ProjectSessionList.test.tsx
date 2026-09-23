import { describe, expect, test } from 'vitest'
import { render, screen } from '@testing-library/react'
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
})
