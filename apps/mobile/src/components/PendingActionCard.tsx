import { useState, type CSSProperties, type ReactNode } from 'react'
import { Button, Checkbox, Popup, Radio, Space, TextArea } from 'antd-mobile'
import {
  COMPACT_BUTTON_STYLE,
  DRAWER_HEADER_STYLE,
  DRAWER_MASK_STYLE,
  DRAWER_SHEET_STYLE,
  ICON_BUTTON_STYLE,
} from './ui'
import type { ActionResolution } from '../protocol/client'
import type { ActionDecision, PendingAction, Question } from '../protocol/types'

interface PendingActionCardProps {
  action: PendingAction
  // 父组件的 canResolveAction：连接就绪、有待处理事项且无在途命令。
  enabled: boolean
  // 取消命令是否可发（父组件的单在途命令门控）；为假时关闭按钮禁用，
  // 避免出现「抽屉已收起但取消没发出去」的悬空状态。
  cancelEnabled: boolean
  onResolve: (resolution: ActionResolution) => void
  // 显式关闭 = 放弃这一轮，向 Claude 发取消信号（D53）。
  onCancel: (requestId: string) => void
}

const BODY_STYLE: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 10,
  padding: '10px 12px 20px',
  overflowY: 'auto',
}

// 待处理事项抽屉（B2 §6 + D53）：工具审批与 AskUserQuestion 共用同一形态，
// 事项出现即自动弹出，只能由标题行的关闭按钮收起，点遮罩与下滑不关闭——
// 关闭等价于取消当前 Turn，不能被误触触发。
// 内容必须自包含，不依赖历史上下文，因为重连后的 snapshot 不重放结构化内容；
// 只展示服务端下发的 preview 与问答内容，不展示 actionId、工具原始 input
// 或权限规则原文。
function Drawer({
  title,
  visible,
  canClose,
  onClose,
  children,
}: {
  title: string
  visible: boolean
  canClose: boolean
  onClose: () => void
  children: ReactNode
}) {
  return (
    <div data-testid="pending-action" data-drawer={visible ? 'open' : 'closed'}>
      <Popup
        visible={visible}
        position="bottom"
        destroyOnClose
        closeOnMaskClick={false}
        bodyStyle={DRAWER_SHEET_STYLE}
        maskStyle={DRAWER_MASK_STYLE}
      >
        <div style={DRAWER_HEADER_STYLE}>
          <span style={{ width: 32, flexShrink: 0 }} />
          <span
            style={{
              flex: 1,
              minWidth: 0,
              textAlign: 'center',
              fontSize: 15,
              color: '#333',
              wordBreak: 'break-word',
            }}
          >
            {title}
          </span>
          <button
            type="button"
            aria-label="关闭"
            style={{ ...ICON_BUTTON_STYLE, opacity: canClose ? 1 : 0.4 }}
            disabled={!canClose}
            onClick={onClose}
          >
            ✕
          </button>
        </div>
        <div data-testid="pending-action-card" style={BODY_STYLE}>
          {/* 关闭语义不直观，直接写明，避免用户把它当成「稍后再说」。 */}
          <div style={{ fontSize: 12, color: '#8c8c8c' }}>关闭此面板将取消当前 Turn</div>
          {children}
        </div>
      </Popup>
    </div>
  )
}

export default function PendingActionCard({
  action,
  enabled,
  cancelEnabled,
  onResolve,
  onCancel,
}: PendingActionCardProps) {
  // 提交后立即本地禁用，等 command_result 让父组件重新给出 enabled。
  const [submitted, setSubmitted] = useState(false)
  const [answers, setAnswers] = useState<Record<string, string[]>>({})
  // 显式关闭后本地收起。取消成功时父组件会清掉事项并卸载本组件；
  // 若取消被拒绝而事项仍在，下面按 enabled 回升重新弹出，不会留下无法处理的事项。
  const [dismissed, setDismissed] = useState(false)
  // 按 React “prop 变化时调整 state”模式在渲染期比较上一次的值，
  // 不用 effect，避开多余渲染。
  const [tracked, setTracked] = useState({ actionId: action.actionId, enabled })
  if (tracked.actionId !== action.actionId) {
    // 换事项就丢弃上一个事项的输入。
    setTracked({ actionId: action.actionId, enabled })
    setSubmitted(false)
    setAnswers({})
    setDismissed(false)
  } else if (tracked.enabled !== enabled) {
    setTracked({ actionId: action.actionId, enabled })
    // enabled 落到假说明命令已在途或连接不可用；恢复时解除本地禁用，
    // 使被拒绝但仍在等待的事项可以重新提交，也把已收起的抽屉重新弹出。
    if (!enabled) setSubmitted(false)
    else setDismissed(false)
  }

  const active = enabled && !submitted
  const close = () => {
    if (!cancelEnabled) return
    setDismissed(true)
    onCancel(action.requestId)
  }

  if (action.kind === 'tool_approval') {
    const resolve = (decision: ActionDecision) => {
      if (!active) return
      setSubmitted(true)
      onResolve({ actionId: action.actionId, decision })
    }
    return (
      <Drawer title="权限请求" visible={!dismissed} canClose={cancelEnabled} onClose={close}>
        <div style={{ fontSize: 13 }}>Claude 请求使用工具：{action.toolName}</div>
        {/* preview 由服务端按白名单抽取；未下发时只显示工具名，不自行拼接。 */}
        {action.preview !== '' && (
          <div
            data-testid="action-preview"
            style={{
              fontSize: 12,
              color: '#555',
              background: 'rgba(255, 255, 255, 0.7)',
              border: '1px solid rgba(255, 255, 255, 0.8)',
              borderRadius: 6,
              padding: '6px 8px',
              wordBreak: 'break-word',
            }}
          >
            {action.preview}
          </div>
        )}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Button
            color="primary"
            size="small"
            style={COMPACT_BUTTON_STYLE}
            disabled={!active}
            onClick={() => resolve('allow_once')}
          >
            本次允许
          </Button>
          {action.allowSessionAvailable && (
            <Button
              color="primary"
              fill="outline"
              size="small"
              style={COMPACT_BUTTON_STYLE}
              disabled={!active}
              onClick={() => resolve('allow_session')}
            >
              本会话允许
            </Button>
          )}
          <Button
            color="danger"
            fill="outline"
            size="small"
            style={COMPACT_BUTTON_STYLE}
            disabled={!active}
            onClick={() => resolve('deny')}
          >
            拒绝
          </Button>
        </div>
      </Drawer>
    )
  }

  // 自由文本答案去空白后必须非空；choice 答案只接受 options 中存在的 label。
  const valuesOf = (question: Question): string[] => {
    const raw = answers[question.questionId] ?? []
    if (question.inputType === 'text') {
      const text = (raw[0] ?? '').trim()
      return text === '' ? [] : [text]
    }
    return raw.filter((value) => question.options.some((option) => option.label === value))
  }

  const setChoice = (question: Question, values: string[]) => {
    setAnswers((current) => ({ ...current, [question.questionId]: values }))
  }

  const answered = action.questions.every((question) => valuesOf(question).length > 0)

  const submit = () => {
    if (!active || !answered) return
    const payload = action.questions.map((question) => ({
      questionId: question.questionId,
      values: valuesOf(question),
    }))
    setSubmitted(true)
    onResolve({ actionId: action.actionId, answers: payload })
  }

  return (
    <Drawer title="需要你的回答" visible={!dismissed} canClose={cancelEnabled} onClose={close}>
      {action.questions.map((question) => (
        <div key={question.questionId} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <div
            data-testid={`question-header-${question.questionId}`}
            style={{ fontSize: 12, color: '#888' }}
          >
            {question.header}
          </div>
          <div style={{ fontSize: 14, wordBreak: 'break-word' }}>{question.question}</div>
          {question.inputType === 'text' ? (
            <TextArea
              data-testid={`question-text-${question.questionId}`}
              rows={2}
              placeholder="输入你的回答"
              disabled={!active}
              value={answers[question.questionId]?.[0] ?? ''}
              onChange={(value) => setChoice(question, [value])}
            />
          ) : question.multiSelect ? (
            <Checkbox.Group
              value={valuesOf(question)}
              onChange={(values) => setChoice(question, values.map(String))}
            >
              <Space direction="vertical">
                {question.options.map((option) => (
                  <Checkbox key={option.label} value={option.label} disabled={!active}>
                    <span style={{ fontSize: 14 }}>{option.label}</span>
                    {option.description !== '' && (
                      <div style={{ fontSize: 12, color: '#888' }}>{option.description}</div>
                    )}
                  </Checkbox>
                ))}
              </Space>
            </Checkbox.Group>
          ) : (
            <Radio.Group
              value={valuesOf(question)[0] ?? ''}
              onChange={(value) => setChoice(question, [String(value)])}
            >
              <Space direction="vertical">
                {question.options.map((option) => (
                  <Radio key={option.label} value={option.label} disabled={!active}>
                    <span style={{ fontSize: 14 }}>{option.label}</span>
                    {option.description !== '' && (
                      <div style={{ fontSize: 12, color: '#888' }}>{option.description}</div>
                    )}
                  </Radio>
                ))}
              </Space>
            </Radio.Group>
          )}
        </div>
      ))}
      <Button
        color="primary"
        size="small"
        style={COMPACT_BUTTON_STYLE}
        disabled={!active || !answered}
        onClick={submit}
      >
        提交
      </Button>
    </Drawer>
  )
}
