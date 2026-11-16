import { useState, type CSSProperties, type ReactNode } from 'react'
import { Button, Checkbox, Radio, Space, TextArea } from 'antd-mobile'
import BottomDrawer from './BottomDrawer'
import './PendingActionCard.css'
import {
  COMPACT_BUTTON_STYLE,
  DRAWER_HEADER_STYLE,
  ICON_BUTTON_STYLE,
  FONT_SIZE_BODY,
  FONT_SIZE_COMPACT,
  FONT_SIZE_SECONDARY,
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
  // 返回 false 表示命令未真正写入 WebSocket；此时不能把按钮锁死。
  onResolve: (resolution: ActionResolution) => boolean | void
  // 关闭审批取消本轮；关闭提问拒绝本次问题。
  onCancel: (requestId: string) => boolean | void
}

const BODY_STYLE: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 10,
  padding: '10px 12px 20px',
  overflowY: 'auto',
  minHeight: 0,
  flex: '1 1 auto',
  WebkitUserSelect: 'text',
  userSelect: 'text',
  WebkitTouchCallout: 'default',
}

// 待处理事项自动弹出；收起只影响本地展示，可随时重新打开。
function Drawer({
  title,
  visible,
  canClose,
  onClose,
  children,
  footer,
}: {
  title: string
  visible: boolean
  canClose: boolean
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <div data-testid="pending-action" data-drawer={visible ? 'open' : 'closed'}>
      {!visible && <button type="button" className="pending-collapsed" onClick={onClose}><span aria-hidden="true">●</span> 查看待处理事项 <span aria-hidden="true">›</span></button>}
      <BottomDrawer
        visible={visible}
      >
        <div className="pending-header" style={DRAWER_HEADER_STYLE}>
          <span style={{ width: 32, flexShrink: 0 }} />
          <span
            style={{
              flex: 1,
              minWidth: 0,
              textAlign: 'center',
              fontSize: FONT_SIZE_BODY,
              color: '#333',
              wordBreak: 'break-word',
            }}
          >
            {title}
          </span>
          <button
            type="button"
            aria-label="收起"
            style={{ ...ICON_BUTTON_STYLE, width: 52, fontSize: FONT_SIZE_COMPACT, opacity: canClose ? 1 : 0.4 }}
            disabled={!canClose}
            onClick={onClose}
          >
            收起
          </button>
        </div>
        <div data-testid="pending-action-card" style={BODY_STYLE}>
          {/* 关闭语义不直观，直接写明，避免用户把它当成「稍后再说」。 */}
          <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#8c8c8c' }}>收起后可继续查看会话，待处理事项会保留。</div>
          {children}
        </div>
        {footer && <div className="pending-footer">{footer}</div>}
      </BottomDrawer>
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
  // 收起只影响展示，事项和已选择的答案继续保留。
  const [dismissed, setDismissed] = useState(false)
  const [submitError, setSubmitError] = useState(false)
  // 按 React “prop 变化时调整 state”模式在渲染期比较上一次的值，
  // 不用 effect，避开多余渲染。
  const [tracked, setTracked] = useState({ actionId: action.actionId, enabled })
  if (tracked.actionId !== action.actionId) {
    // 换事项就丢弃上一个事项的输入。
    setTracked({ actionId: action.actionId, enabled })
    setSubmitted(false)
    setAnswers({})
    setDismissed(false)
    setSubmitError(false)
  } else if (tracked.enabled !== enabled) {
    setTracked({ actionId: action.actionId, enabled })
    // enabled 落到假说明命令已在途或连接不可用；恢复时解除本地禁用，
    // 使被拒绝但仍在等待的事项可以重新提交。
    if (!enabled) setSubmitted(false)
    else {
      setSubmitError(false)
    }
  }

  const active = enabled && !submitted
  const close = () => setDismissed((current) => !current)
  const cancel = () => {
    if (submitted || !(action.kind === 'tool_approval' ? cancelEnabled : enabled)) return
    const sent = action.kind === 'tool_approval' ? onCancel(action.requestId) : onResolve({ actionId: action.actionId, decision: 'deny' })
    if (sent === false) { setSubmitError(true); return }
    setSubmitted(true)
  }


  if (action.kind === 'tool_approval') {
    const resolve = (decision: ActionDecision) => {
      if (!active) return
      const sent = onResolve({ actionId: action.actionId, decision })
      if (sent === false) {
        setSubmitError(true)
        return
      }
      setSubmitted(true)
    }
    return (
      <Drawer title="权限请求" visible={!dismissed} canClose={true} onClose={close} footer={<Button block disabled={!cancelEnabled || submitted} onClick={cancel}>取消当前任务</Button>}>
        {submitError && <SubmitError />}
        <div style={{ fontSize: FONT_SIZE_SECONDARY }}>Claude 请求使用工具：{action.toolName}</div>
        {/* preview 由服务端按白名单抽取；未下发时只显示工具名，不自行拼接。 */}
        {action.preview !== '' && (
          <div
            data-testid="action-preview"
            style={{
              fontSize: FONT_SIZE_COMPACT,
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
            style={{ ...COMPACT_BUTTON_STYLE, minHeight: 44, paddingInline: 16 }}
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
              style={{ ...COMPACT_BUTTON_STYLE, minHeight: 44, paddingInline: 16 }}
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
            style={{ ...COMPACT_BUTTON_STYLE, minHeight: 44, paddingInline: 16 }}
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
    const sent = onResolve({ actionId: action.actionId, answers: payload })
    if (sent === false) {
      setSubmitError(true)
      return
    }
    setSubmitted(true)
  }

  return (
    <Drawer title="需要你的回答" visible={!dismissed} canClose={true} onClose={close} footer={<div className="pending-footer-actions"><Button block disabled={!active} onClick={cancel}>取消提问</Button><Button color="primary" block disabled={!active || !answered} onClick={submit}>提交</Button></div>}>
      {submitError && <SubmitError />}
      {action.questions.map((question) => (
        <div key={question.questionId} className="pending-question" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <div
            data-testid={`question-header-${question.questionId}`}
            style={{ fontSize: FONT_SIZE_COMPACT, color: '#3478f6' }}
          >
            {question.header}
          </div>
          <div style={{ fontSize: FONT_SIZE_BODY, wordBreak: 'break-word' }}>{question.question}</div>
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
                  <Checkbox className={valuesOf(question).includes(option.label) ? "pending-option pending-option--selected" : "pending-option"} key={option.label} value={option.label} disabled={!active}>
                    <span style={{ fontSize: FONT_SIZE_BODY }}>{option.label}</span>
                    {option.description !== '' && (
                      <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#888' }}>{option.description}</div>
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
                  <Radio className={valuesOf(question).includes(option.label) ? "pending-option pending-option--selected" : "pending-option"} key={option.label} value={option.label} disabled={!active}>
                    <span style={{ fontSize: FONT_SIZE_BODY }}>{option.label}</span>
                    {option.description !== '' && (
                      <div style={{ fontSize: FONT_SIZE_COMPACT, color: '#888' }}>{option.description}</div>
                    )}
                  </Radio>
                ))}
              </Space>
            </Radio.Group>
          )}
        </div>
      ))}

    </Drawer>
  )
}

function SubmitError() {
  return (
    <div data-testid="pending-action-error" style={{ fontSize: FONT_SIZE_COMPACT, color: '#e00' }}>
      操作未成功，请重试；取消提问失败时可在电脑端处理或更新电脑端
    </div>
  )
}
