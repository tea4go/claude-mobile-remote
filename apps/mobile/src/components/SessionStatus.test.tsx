import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import SessionStatus from './SessionStatus'

describe('SessionStatus', () => {
  test('展示连接、Session 与 Turn 状态', () => {
    render(
      <SessionStatus
        connection="connected"
        sessionStatus="ready"
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
        turnStatus="idle"
        queueDepthHint={0}
      />,
    )
    expect(screen.getByText('已断开')).toBeInTheDocument()
  })
})
