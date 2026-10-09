import { fireEvent, render, screen } from '@testing-library/react'
import { type ComponentProps } from 'react'
import { describe, expect, test, vi } from 'vitest'
import PendingActionCard from './PendingActionCard'
import type { PendingAction, Question } from '../protocol/types'

// 只关心某几个 prop 的用例不必每次都写全五个入参；未给出的走可用默认值。
function Card(
  props: Partial<ComponentProps<typeof PendingActionCard>> & { action: PendingAction },
) {
  return (
    <PendingActionCard
      enabled
      cancelEnabled
      onResolve={vi.fn()}
      onCancel={vi.fn()}
      {...props}
    />
  )
}

function approval(overrides: Partial<Extract<PendingAction, { kind: 'tool_approval' }>> = {}): PendingAction {
  return {
    actionId: 'act-1',
    requestId: 'req-1',
    kind: 'tool_approval',
    toolName: 'Bash',
    preview: '',
    allowSessionAvailable: true,
    ...overrides,
  }
}

function question(overrides: Partial<Question> = {}): Question {
  return {
    questionId: 'q-1',
    header: '主题色',
    question: '希望使用哪种主题色？',
    inputType: 'choice',
    multiSelect: false,
    options: [
      { label: '深色', description: '夜间友好' },
      { label: '浅色', description: '默认' },
    ],
    ...overrides,
  }
}

function ask(questions: Question[]): PendingAction {
  return {
    actionId: 'act-2',
    requestId: 'req-2',
    kind: 'ask_user_question',
    toolName: 'AskUserQuestion',
    allowSessionAvailable: false,
    questions,
  }
}

// Popup 关闭带退场动画（destroyOnClose 在动画结束后才卸载），
// 所以开关断言看状态属性，不依赖动画时序。
function drawerState(): string | null {
  return screen.getByTestId('pending-action').getAttribute('data-drawer')
}

describe('PendingActionCard：底部抽屉与关闭语义（D53）', () => {
  test('事项出现即自动弹出抽屉', () => {
    render(
      <Card
        action={approval()}
        enabled
        cancelEnabled
        onResolve={vi.fn()}
        onCancel={vi.fn()}
      />,
    )
    expect(drawerState()).toBe('open')
    expect(screen.getByTestId('pending-action-card')).toBeInTheDocument()
  })

  test('显式关闭后收起抽屉并取消当前 Turn', () => {
    const onCancel = vi.fn()
    render(
      <Card
        action={approval()}
        enabled
        cancelEnabled
        onResolve={vi.fn()}
        onCancel={onCancel}
      />,
    )
    fireEvent.click(screen.getByLabelText('关闭'))
    expect(onCancel).toHaveBeenCalledWith('req-1')
    expect(drawerState()).toBe('closed')
  })

  test('决议命令未实际发出时保留按钮并允许重试', () => {
    const onResolve = vi.fn(() => false)
    render(<Card action={approval()} onResolve={onResolve} />)

    const button = screen.getByRole('button', { name: '本次允许' })
    fireEvent.click(button)

    expect(onResolve).toHaveBeenCalledWith({ actionId: 'act-1', decision: 'allow_once' })
    expect(button).not.toBeDisabled()
    expect(screen.getByTestId('pending-action-error')).toHaveTextContent('请重试')
  })

  test('点遮罩不关闭，不会误取消整个 Turn', () => {
    const onCancel = vi.fn()
    render(
      <Card
        action={ask([question()])}
        enabled
        cancelEnabled
        onResolve={vi.fn()}
        onCancel={onCancel}
      />,
    )
    const mask = document.querySelector('.mobile-drawer-mask')
    expect(mask).not.toBeNull()
    fireEvent.click(mask!)
    expect(onCancel).not.toHaveBeenCalled()
    expect(drawerState()).toBe('open')
  })

  // 抽屉自身开了 backdropFilter，而遮罩就在它背后，会被一起采样进模糊结果：
  // 用 antd-mobile 默认档（0.55 黑）会把整个抽屉染成灰面，玻璃质感就没了。
  // 这里锁住「遮罩显著淡」和「靠 brightness 提亮暗部」两条，不锁具体色值。
  test('抽屉遮罩显著淡于默认档，玻璃靠 brightness 提亮', () => {
    render(<Card action={approval()} />)
    const mask = document.querySelector('.mobile-drawer-mask')
    expect(mask).not.toBeNull()
    const alpha = /rgba\([^)]*,\s*([\d.]+)\s*\)/.exec(mask!.getAttribute('style') ?? '')?.[1]
    expect(alpha).toBeDefined()
    expect(Number(alpha)).toBeLessThan(0.3)
    const sheet = document.querySelector('.mobile-drawer-sheet')
    expect(sheet).not.toBeNull()
    expect(sheet!.getAttribute('style')).toContain('brightness')
  })

  test('取消命令不可发时关闭按钮禁用', () => {
    const onCancel = vi.fn()
    render(
      <Card
        action={approval()}
        enabled
        cancelEnabled={false}
        onResolve={vi.fn()}
        onCancel={onCancel}
      />,
    )
    const close = screen.getByLabelText('关闭')
    expect(close).toBeDisabled()
    fireEvent.click(close)
    expect(onCancel).not.toHaveBeenCalled()
    expect(drawerState()).toBe('open')
  })

  test('取消未生效、事项仍在时抽屉重新弹出', () => {
    const onCancel = vi.fn()
    const { rerender } = render(
      <Card
        action={approval()}
        enabled
        cancelEnabled
        onResolve={vi.fn()}
        onCancel={onCancel}
      />,
    )
    fireEvent.click(screen.getByLabelText('关闭'))
    expect(drawerState()).toBe('closed')

    // 取消命令在途 → enabled 落假；命令被拒后 enabled 回升而事项仍在。
    rerender(
      <Card
        action={approval()}
        enabled={false}
        cancelEnabled={false}
        onResolve={vi.fn()}
        onCancel={onCancel}
      />,
    )
    rerender(
      <Card
        action={approval()}
        enabled
        cancelEnabled
        onResolve={vi.fn()}
        onCancel={onCancel}
      />,
    )
    expect(drawerState()).toBe('open')
  })
})

describe('PendingActionCard：工具审批（B2 §6）', () => {
  test('显示工具名与三个决议按钮', () => {
    render(<Card action={approval()} enabled onResolve={vi.fn()} />)
    expect(screen.getByTestId('pending-action-card')).toBeInTheDocument()
    expect(screen.getByText(/Bash/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '本次允许' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '本会话允许' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '拒绝' })).toBeInTheDocument()
  })

  test('有 preview 时额外显示单行预览', () => {
    render(
      <Card action={approval({ preview: 'go test ./...' })} enabled onResolve={vi.fn()} />,
    )
    expect(screen.getByTestId('action-preview').textContent).toBe('go test ./...')
  })

  test('无 preview 时只显示工具名且不自行拼接预览', () => {
    render(<Card action={approval()} enabled onResolve={vi.fn()} />)
    expect(screen.queryByTestId('action-preview')).not.toBeInTheDocument()
  })

  test('allowSessionAvailable 为假时不渲染本会话允许', () => {
    render(
      <Card
        action={approval({ allowSessionAvailable: false })}
        enabled
        onResolve={vi.fn()}
      />,
    )
    expect(screen.queryByRole('button', { name: '本会话允许' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '本次允许' })).toBeInTheDocument()
  })

  test('三个按钮分别回传对应决议', () => {
    // 提交后立即本地禁用，因此每个决议各自独立渲染一次。
    for (const decision of ['allow_once', 'allow_session', 'deny'] as const) {
      const label =
        decision === 'allow_once' ? '本次允许' : decision === 'allow_session' ? '本会话允许' : '拒绝'
      const onResolve = vi.fn()
      const { unmount } = render(
        <Card action={approval()} enabled onResolve={onResolve} />,
      )
      fireEvent.click(screen.getByRole('button', { name: label }))
      expect(onResolve).toHaveBeenCalledWith({ actionId: 'act-1', decision })
      unmount()
    }
  })

  test('提交后按钮立即禁用直到 command_result（enabled 为假）', () => {
    const onResolve = vi.fn()
    const { rerender } = render(
      <Card action={approval()} enabled onResolve={onResolve} />,
    )
    fireEvent.click(screen.getByRole('button', { name: '本次允许' }))
    // 本地立即禁用，不等 enabled 变化。
    expect(screen.getByRole('button', { name: '本次允许' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '拒绝' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: '拒绝' }))
    expect(onResolve).toHaveBeenCalledTimes(1)

    // enabled 恢复后（决议被拒绝、action 仍在）按钮重新可用。
    rerender(<Card action={approval()} enabled={false} onResolve={onResolve} />)
    expect(screen.getByRole('button', { name: '本次允许' })).toBeDisabled()
    rerender(<Card action={approval()} enabled onResolve={onResolve} />)
    expect(screen.getByRole('button', { name: '本次允许' })).not.toBeDisabled()
  })

  test('不展示权限规则原文，只展示允许范围按钮', () => {
    render(<Card action={approval()} enabled onResolve={vi.fn()} />)
    // 抽屉内容渲染到 body 上的 portal，断言要看整个文档。
    const text = screen.getByTestId('pending-action-card').textContent ?? ''
    expect(text).not.toContain('PermissionUpdate')
    expect(text).not.toContain('act-1')
  })
})

describe('PendingActionCard：AskUserQuestion（B2 §6）', () => {
  test('header 作小标题、question 作正文两行显示', () => {
    render(<Card action={ask([question()])} enabled onResolve={vi.fn()} />)
    expect(screen.getByTestId('question-header-q-1').textContent).toBe('主题色')
    expect(screen.getByText('希望使用哪种主题色？')).toBeInTheDocument()
  })

  test('单选用 Radio，未回答时不可提交', () => {
    render(<Card action={ask([question()])} enabled onResolve={vi.fn()} />)
    expect(screen.getAllByRole('radio')).toHaveLength(2)
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
  })

  test('单选回答后可提交并回传单个值', () => {
    const onResolve = vi.fn()
    render(<Card action={ask([question()])} enabled onResolve={onResolve} />)
    fireEvent.click(screen.getByText('深色'))
    const submit = screen.getByRole('button', { name: '提交' })
    expect(submit).not.toBeDisabled()
    fireEvent.click(submit)
    expect(onResolve).toHaveBeenCalledWith({
      actionId: 'act-2',
      answers: [{ questionId: 'q-1', values: ['深色'] }],
    })
  })

  test('多选用 Checkbox 并可回传多个值', () => {
    const onResolve = vi.fn()
    render(
      <Card
        action={ask([question({ multiSelect: true })])}
        enabled
        onResolve={onResolve}
      />,
    )
    expect(screen.getAllByRole('checkbox')).toHaveLength(2)
    fireEvent.click(screen.getByText('深色'))
    fireEvent.click(screen.getByText('浅色'))
    fireEvent.click(screen.getByRole('button', { name: '提交' }))
    expect(onResolve).toHaveBeenCalledWith({
      actionId: 'act-2',
      answers: [{ questionId: 'q-1', values: ['深色', '浅色'] }],
    })
  })

  test('多问题必须全部回答后才能提交', () => {
    const onResolve = vi.fn()
    const second = question({
      questionId: 'q-2',
      header: '字号',
      question: '默认字号用多大？',
      options: [
        { label: '标准', description: '' },
        { label: '大号', description: '' },
      ],
    })
    render(<Card action={ask([question(), second])} enabled onResolve={onResolve} />)

    fireEvent.click(screen.getByText('深色'))
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
    fireEvent.click(screen.getByText('大号'))
    fireEvent.click(screen.getByRole('button', { name: '提交' }))
    expect(onResolve).toHaveBeenCalledWith({
      actionId: 'act-2',
      answers: [
        { questionId: 'q-1', values: ['深色'] },
        { questionId: 'q-2', values: ['大号'] },
      ],
    })
  })

  test('选项描述可见但不作为答案值回传', () => {
    const onResolve = vi.fn()
    render(<Card action={ask([question()])} enabled onResolve={onResolve} />)
    expect(screen.getByText('夜间友好')).toBeInTheDocument()
    fireEvent.click(screen.getByText('深色'))
    fireEvent.click(screen.getByRole('button', { name: '提交' }))
    expect(onResolve).toHaveBeenCalledWith({
      actionId: 'act-2',
      answers: [{ questionId: 'q-1', values: ['深色'] }],
    })
  })

  test('choice 问题不渲染自由文本输入框', () => {
    render(<Card action={ask([question()])} enabled onResolve={vi.fn()} />)
    expect(screen.queryByTestId('question-text-q-1')).not.toBeInTheDocument()
  })

  test('inputType 为 text 时渲染输入框，去空白后为空不可提交', () => {
    const onResolve = vi.fn()
    const textQuestion = question({ inputType: 'text', options: [] })
    render(<Card action={ask([textQuestion])} enabled onResolve={onResolve} />)

    // antd-mobile 把 data-testid 挂在包装元素上，输入事件要打到真实 textarea。
    expect(screen.getByTestId('question-text-q-1')).toBeInTheDocument()
    const input = screen.getByRole('textbox')
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
    fireEvent.change(input, { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
    fireEvent.change(input, { target: { value: '  自定义答案  ' } })
    fireEvent.click(screen.getByRole('button', { name: '提交' }))
    expect(onResolve).toHaveBeenCalledWith({
      actionId: 'act-2',
      answers: [{ questionId: 'q-1', values: ['自定义答案'] }],
    })
  })

  test('enabled 为假时不可提交', () => {
    render(<Card action={ask([question()])} enabled={false} onResolve={vi.fn()} />)
    fireEvent.click(screen.getByText('深色'))
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
  })

  test('提交后按钮立即禁用', () => {
    const onResolve = vi.fn()
    render(<Card action={ask([question()])} enabled onResolve={onResolve} />)
    fireEvent.click(screen.getByText('深色'))
    fireEvent.click(screen.getByRole('button', { name: '提交' }))
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '提交' }))
    expect(onResolve).toHaveBeenCalledTimes(1)
  })

  test('问答卡片不出现工具审批的决议按钮', () => {
    render(<Card action={ask([question()])} enabled onResolve={vi.fn()} />)
    expect(screen.queryByRole('button', { name: '本次允许' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '本会话允许' })).not.toBeInTheDocument()
  })

  test('换 action 后重置已选答案', () => {
    const onResolve = vi.fn()
    const { rerender } = render(
      <Card action={ask([question()])} enabled onResolve={onResolve} />,
    )
    fireEvent.click(screen.getByText('深色'))
    expect(screen.getByRole('button', { name: '提交' })).not.toBeDisabled()

    const next: PendingAction = {
      ...(ask([question()]) as Extract<PendingAction, { kind: 'ask_user_question' }>),
      actionId: 'act-3',
    }
    rerender(<Card action={next} enabled onResolve={onResolve} />)
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()
  })
})
