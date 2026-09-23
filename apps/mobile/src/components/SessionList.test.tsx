import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import type { PairingPayload } from '../protocol/pairing'
import {
  createInitialSessionStore,
  defaultSessionName,
  listSessions,
  markSessionStatus,
  renameSession,
  upsertSession,
  type SessionStore,
} from '../session/store'
import SessionList from './ProjectSessionList'

function payload(sessionId: string, wsUrl = 'ws://127.0.0.1:8082/ws'): PairingPayload {
  return {
    version: 1,
    wsUrl,
    sessionId,
    pairToken: 'pair-token-must-not-render',
    expiresAt: '2099-01-01T00:00:00Z',
  }
}

function storeWith(...ids: string[]): SessionStore {
  return ids.reduce((store, id) => upsertSession(store, payload(id)), createInitialSessionStore())
}

function noop() {}

describe('SessionList：空态', () => {
  test('无记录时显示空态说明与新增入口', () => {
    render(
      <SessionList
        sessions={[]}
        onAdd={noop}
        onOpen={noop}
        onRemove={noop}
        onRename={noop}
      />,
    )

    expect(screen.getByText(/还没有已配对的会话/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新增配对' })).toBeInTheDocument()
  })
})

describe('SessionList：记录展示', () => {
  test('列表只保留标题和预览，不常驻展示诊断信息', () => {
    const sessions = listSessions(storeWith('session-abcdefgh-1'))
    render(
      <SessionList sessions={sessions} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />,
    )

    expect(screen.getByText(defaultSessionName('session-abcdefgh-1'))).toBeInTheDocument()
    expect(screen.getByText('暂无会话内容')).toBeInTheDocument()
    expect(screen.queryByText(/127\.0\.0\.1:8082/)).not.toBeInTheDocument()
    expect(screen.queryByText(/活动时间未知/)).not.toBeInTheDocument()
  })

  test('展示自定义名称，不显示无须处理的已连接标签', () => {
    let store = storeWith('s-1')
    store = renameSession(store, 's-1', '重构分支')
    store = markSessionStatus(store, 's-1', { status: 'connected', lastConnectedAt: 1000 })

    render(
      <SessionList
        sessions={listSessions(store)}
        onAdd={noop}
        onOpen={noop}
        onRemove={noop}
        onRename={noop}
      />,
    )

    expect(screen.getByText('重构分支')).toBeInTheDocument()
    expect(screen.queryByText(/已连接/)).not.toBeInTheDocument()
    expect(screen.queryByText(/从未连接/)).not.toBeInTheDocument()
  })

  test('凭证已失效的记录提示需要重新配对', () => {
    const store = markSessionStatus(storeWith('s-1'), 's-1', { status: 'needsPairing' })
    render(
      <SessionList
        sessions={listSessions(store)}
        onAdd={noop}
        onOpen={noop}
        onRemove={noop}
        onRename={noop}
      />,
    )

    expect(screen.getByText(/需要重新配对/)).toBeInTheDocument()
  })

  test('同一 sessionId 重新配对后列表仍只有一条', () => {
    let store = storeWith('s-1')
    store = upsertSession(store, payload('s-1', 'ws://127.0.0.1:9090/ws'))

    render(
      <SessionList
        sessions={listSessions(store)}
        onAdd={noop}
        onOpen={noop}
        onRemove={noop}
        onRename={noop}
      />,
    )

    expect(screen.getAllByTestId('session-list-item')).toHaveLength(1)
    expect(screen.queryByText(/127\.0\.0\.1:9090/)).not.toBeInTheDocument()
  })

  test('不渲染 Pair Token', () => {
    const sessions = listSessions(storeWith('s-1'))
    const { container } = render(
      <SessionList sessions={sessions} onAdd={noop} onOpen={noop} onRemove={noop} onRename={noop} />,
    )

    expect(container.innerHTML).not.toContain('pair-token-must-not-render')
  })
})

describe('SessionList：操作', () => {
  test('点击记录进入详情', () => {
    const onOpen = vi.fn()
    render(
      <SessionList
        sessions={listSessions(storeWith('s-1', 's-2'))}
        onAdd={noop}
        onOpen={onOpen}
        onRemove={noop}
        onRename={noop}
      />,
    )

    fireEvent.click(screen.getAllByTestId('session-list-item')[1]!)

    expect(onOpen).toHaveBeenCalledExactlyOnceWith('s-2')
  })

  test('删除只回调目标记录', () => {
    const onRemove = vi.fn()
    const onOpen = vi.fn()
    render(
      <SessionList
        sessions={listSessions(storeWith('s-1', 's-2'))}
        onAdd={noop}
        onOpen={onOpen}
        onRemove={onRemove}
        onRename={noop}
      />,
    )

    fireEvent.click(screen.getAllByRole('button', { name: /更多操作/ })[0]!)
    fireEvent.click(screen.getByText('删除'))

    expect(onRemove).toHaveBeenCalledExactlyOnceWith('s-1')
    expect(onOpen).not.toHaveBeenCalled()
  })

  test('重命名提交新的本地名称', () => {
    const onRename = vi.fn()
    render(
      <SessionList
        sessions={listSessions(storeWith('s-1'))}
        onAdd={noop}
        onOpen={noop}
        onRemove={noop}
        onRename={onRename}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: /更多操作/ }))
    fireEvent.click(screen.getByText('重命名'))
    // antd-mobile Input 把 data-testid 挂在外层容器，改由 role 定位真实 input。
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '主分支' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    expect(onRename).toHaveBeenCalledExactlyOnceWith('s-1', '主分支')
  })

  test('取消重命名不回调', () => {
    const onRename = vi.fn()
    render(
      <SessionList
        sessions={listSessions(storeWith('s-1'))}
        onAdd={noop}
        onOpen={noop}
        onRemove={noop}
        onRename={onRename}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: /更多操作/ }))
    fireEvent.click(screen.getByText('重命名'))
    fireEvent.click(screen.getByRole('button', { name: '取消' }))

    expect(onRename).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  test('新增配对入口回调', () => {
    const onAdd = vi.fn()
    render(
      <SessionList
        sessions={listSessions(storeWith('s-1'))}
        onAdd={onAdd}
        onOpen={noop}
        onRemove={noop}
        onRename={noop}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '新增配对' }))

    expect(onAdd).toHaveBeenCalledOnce()
  })
})
