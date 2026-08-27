import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import SessionStatus from './SessionStatus'

describe('SessionStatus', () => {
  test('展示连接、Session 与 Turn 状态', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="ready"
        sessionError={null}
        turnStatus="running"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByText('已连接')).toBeInTheDocument()
    expect(screen.getByText('会话：就绪')).toBeInTheDocument()
    expect(screen.getByText(/Turn：运行中/)).toBeInTheDocument()
    expect(screen.queryByText(/等待任务/)).not.toBeInTheDocument()
  })

  test('连接时有等待任务提示', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="ready"
        sessionError={null}
        turnStatus="idle"
        queueDepthHint={3}
      />,
    )
    expect(screen.getByText('连接时有 3 条等待任务')).toBeInTheDocument()
  })

  test('断开状态', () => {
    render(
      <SessionStatus
        connection="disconnected"
        sessionStatus="ready"
        sessionError={null}
        turnStatus="idle"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByText('已断开')).toBeInTheDocument()
  })
})

describe('SessionStatus：会话错误（B1）', () => {
  test('会话失败时展示错误原因', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="failed"
        sessionError={{ code: 'internal', message: 'claude 进程退出' }}
        turnStatus="idle"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByText('会话：失败')).toBeInTheDocument()
    expect(screen.getByText(/claude 进程退出/)).toBeInTheDocument()
  })

  test('无会话错误时不展示错误行', () => {
    const { container } = render(
      <SessionStatus
        connection="connected"
        sessionStatus="stopped"
        sessionError={null}
        turnStatus="idle"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByText('会话：已停止')).toBeInTheDocument()
    expect(container.textContent).not.toContain('原因')
  })
})

describe('SessionStatus：终态横幅（B2 §6）', () => {
  test('会话正常结束显示终态横幅', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="stopped"
        sessionError={null}
        turnStatus="idle"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByTestId('session-terminal-banner').textContent).toContain('会话已结束')
  })

  test('会话失败显示终态横幅与安全错误文案', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="failed"
        sessionError={{ code: 'internal', message: '会话已异常结束' }}
        turnStatus="failed"
        queueDepthHint={0}
      />,
    )
    const banner = screen.getByTestId('session-terminal-banner')
    expect(banner.textContent).toContain('会话已失败')
    expect(screen.getByText(/会话已异常结束/)).toBeInTheDocument()
  })

  test('运行中不显示终态横幅', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="ready"
        sessionError={null}
        turnStatus="running"
        queueDepthHint={0}
      />,
    )
    expect(screen.queryByTestId('session-terminal-banner')).not.toBeInTheDocument()
  })

  test('等待待处理事项时 Turn 显示等待中', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="ready"
        sessionError={null}
        turnStatus="waiting"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByText(/Turn：等待中/)).toBeInTheDocument()
  })
})
