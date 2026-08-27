import { useState } from 'react'
import { Button, Checkbox, Radio, Space, TextArea } from 'antd-mobile'
import type { ActionResolution } from '../protocol/client'
import type { ActionDecision, PendingAction, Question } from '../protocol/types'

interface PendingActionCardProps {
  action: PendingAction
  // 父组件的 canResolveAction：连接就绪、有待处理事项且无在途命令。
  enabled: boolean
  onResolve: (resolution: ActionResolution) => void
}

const CARD_STYLE = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: 8,
  background: '#fff',
  border: '1px solid #e0e0e0',
  borderRadius: 8,
  padding: 12,
}

// 底部交互卡片（B2 §6，参考 docs/specs/images/session_approve.png）：工具审批与
// AskUserQuestion 共用同一形态。卡片必须自包含，不依赖历史上下文，因为重连后的
// snapshot 不重放结构化内容。只展示服务端下发的 preview 与问答内容，不展示
// actionId、工具原始 input 或权限规则原文。
export default function PendingActionCard({ action, enabled, onResolve }: PendingActionCardProps) {
  // 提交后立即本地禁用，等 command_result 让父组件重新给出 enabled。
  const [submitted, setSubmitted] = useState(false)
  const [answers, setAnswers] = useState<Record<string, string[]>>({})
  // 按 React “prop 变化时调整 state”模式在渲染期比较上一次的值，
  // 不用 effect，避开多余渲染。
  const [tracked, setTracked] = useState({ actionId: action.actionId, enabled })
  if (tracked.actionId !== action.actionId) {
    // 换事项就丢弃上一个事项的输入。
    setTracked({ actionId: action.actionId, enabled })
    setSubmitted(false)
    setAnswers({})
  } else if (tracked.enabled !== enabled) {
    setTracked({ actionId: action.actionId, enabled })
    // enabled 落到假说明命令已在途或连接不可用；恢复时解除本地禁用，
    // 使被拒绝但仍在等待的事项可以重新提交。
    if (!enabled) setSubmitted(false)
  }

  const active = enabled && !submitted

  if (action.kind === 'tool_approval') {
    const resolve = (decision: ActionDecision) => {
      if (!active) return
      setSubmitted(true)
      onResolve({ actionId: action.actionId, decision })
    }
    return (
      <div data-testid="pending-action-card" style={CARD_STYLE}>
        <div style={{ fontWeight: 600 }}>权限请求</div>
        <div style={{ fontSize: 13 }}>Claude 请求使用工具：{action.toolName}</div>
        {/* preview 由服务端按白名单抽取；未下发时只显示工具名，不自行拼接。 */}
        {action.preview !== '' && (
          <div
            data-testid="action-preview"
            style={{
              fontSize: 12,
              color: '#555',
              background: '#f7f7f7',
              borderRadius: 6,
              padding: '6px 8px',
              wordBreak: 'break-word',
            }}
          >
            {action.preview}
          </div>
        )}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Button color="primary" size="small" disabled={!active} onClick={() => resolve('allow_once')}>
            本次允许
          </Button>
          {action.allowSessionAvailable && (
            <Button
              color="primary"
              fill="outline"
              size="small"
              disabled={!active}
              onClick={() => resolve('allow_session')}
            >
              本会话允许
            </Button>
          )}
          <Button color="danger" fill="outline" size="small" disabled={!active} onClick={() => resolve('deny')}>
            拒绝
          </Button>
        </div>
      </div>
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
    <div data-testid="pending-action-card" style={CARD_STYLE}>
      <div style={{ fontWeight: 600 }}>需要你的回答</div>
      {action.questions.map((question) => (
        <div
          key={question.questionId}
          style={{ display: 'flex', flexDirection: 'column', gap: 6 }}
        >
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
      <Button color="primary" size="small" disabled={!active || !answered} onClick={submit}>
        提交
      </Button>
    </div>
  )
}
