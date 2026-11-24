import { useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button, SpinLoading } from 'antd-mobile'
import { LeftOutline } from 'antd-mobile-icons'
import type { ToolDetail } from '../protocol/types'
import MessageMarkdown from './MessageMarkdown'
import CopyTextButton from './CopyTextButton'
import useEdgeBack from './useEdgeBack'
import './ToolDetailPage.css'

export type LoadToolDetail = (requestId: string, toolUseId: string, signal: AbortSignal) => Promise<ToolDetail>

interface DetailContentProps {
  toolName?: string
  thinking?: string
  answers?: string
  fallback: ToolDetail
  requestId: string
  toolUseId?: string
  resultId?: string
  load?: LoadToolDetail
}

interface ToolDetailPageProps extends DetailContentProps {
  title: string
  status?: ReactNode
  subtools?: ReactNode
  onBack: () => void
}

type DetailResult = { attempt: number; resultId: string | undefined } &
  ({ kind: 'failed'; message: string } | { kind: 'ready'; detail: ToolDetail })

function DetailText({ text, markdown = false }: { text: string; markdown?: boolean }) {
  return markdown
    ? <div className="tool-detail-page__markdown"><MessageMarkdown text={text} /></div>
    : <pre data-horizontal-scroll className="tool-detail-page__code" tabIndex={0}><code>{text}</code></pre>
}

export function ToolDetailContent({ toolName, thinking, answers, fallback, requestId, toolUseId, resultId, load, hideEmpty = false }: DetailContentProps & { hideEmpty?: boolean }) {
  const [attempt, setAttempt] = useState(0)
  const [result, setResult] = useState<DetailResult | null>(null)

  useEffect(() => {
    if (!load || !toolUseId) return
    const controller = new AbortController()
    void load(requestId, toolUseId, controller.signal).then(
      detail => { if (!controller.signal.aborted) setResult({ kind: 'ready', detail, attempt, resultId }) },
      error => {
        if (!controller.signal.aborted) {
          setResult({ kind: 'failed', message: error instanceof Error ? error.message : '读取工具详情失败', attempt, resultId })
        }
      },
    )
    return () => controller.abort()
  }, [load, requestId, toolUseId, resultId, attempt])

  const canFetch = load !== undefined && toolUseId !== undefined
  const current = result?.attempt === attempt && result.resultId === resultId ? result : null
  const detail = canFetch && current?.kind === 'ready' ? current.detail : fallback
  const shell = toolName === 'Bash'
  return canFetch && current === null ? (
    <div className="tool-detail-page__feedback" role="status"><SpinLoading color="primary" />正在从电脑加载工具详情</div>
  ) : canFetch && current?.kind === 'failed' ? (
    <div className="tool-detail-page__feedback">
      <p role="alert">{current.message}</p>
      <Button color="primary" onClick={() => setAttempt(value => value + 1)}>重试</Button>
    </div>
  ) : (
    <>
      {thinking !== undefined ? <DetailText text={thinking} markdown /> : <>
        {detail.input && <section>
          <div className="tool-detail-page__section-header"><h2>{shell ? '命令' : '输入'}</h2>
            {shell && !detail.inputTruncated && canFetch && <CopyTextButton text={detail.input} label="复制命令" />}</div>
          <DetailText text={detail.input} />
          {detail.inputTruncated && <p className="tool-detail-page__note">输入过长，请在电脑端查看完整内容</p>}</section>}
        {answers && <section><h2>已提交的答案</h2><DetailText text={answers} /></section>}
        {detail.output && detail.output.trim() !== answers?.trim() && <section>
          <div className="tool-detail-page__section-header"><h2>结果</h2>
            {shell && <CopyTextButton text={detail.output} label="复制输出" />}</div>
          <DetailText text={detail.output} markdown={/^\s*```/m.test(detail.output) && !shell && !['Read', 'Edit', 'Write'].includes(toolName ?? '')} />
          {detail.outputTruncated && <p className="tool-detail-page__note">结果过长，请在电脑端查看完整内容</p>}</section>}
        {!detail.output && !answers && resultId && <p className="tool-detail-page__note">工具已返回，结果没有可显示的文本。</p>}
        {!hideEmpty && !detail.input && !detail.output && !answers && !resultId && <p className="tool-detail-page__note">暂无可显示的工具详情</p>}
      </>}
    </>
  )
}

export default function ToolDetailPage({ title, status, onBack, subtools, ...content }: ToolDetailPageProps) {
  const backButton = useRef<HTMLButtonElement>(null)
  const [leaving, setLeaving] = useState(false)
  const returnToSteps = () => {
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? true) onBack()
    else setLeaving(true)
  }
  const edgeBack = useEdgeBack(returnToSteps)
  useEffect(() => {
    if (!leaving) return
    const timer = window.setTimeout(onBack, 360)
    return () => window.clearTimeout(timer)
  }, [leaving, onBack])

  useEffect(() => { backButton.current?.focus() }, [])
  return createPortal(
    <section className={`tool-detail-page mobile-page--${leaving ? 'exit-back' : 'enter-forward'}`} role="dialog" aria-modal="true" aria-label={title}
      onAnimationEnd={event => { if (leaving && event.target === event.currentTarget && event.animationName === 'mobile-page-exit-back') onBack() }}
      onTouchStart={event => { event.stopPropagation(); edgeBack.onTouchStart(event) }}
      onTouchMove={event => { event.stopPropagation(); edgeBack.onTouchMove(event) }}
      onTouchCancel={event => { event.stopPropagation(); edgeBack.onTouchCancel() }}
      onTouchEnd={event => { event.stopPropagation(); edgeBack.onTouchEnd(event) }}
      onKeyDown={event => { if (event.key === 'Escape') { event.stopPropagation(); returnToSteps() } }}>
      <header className="tool-detail-page__header">
        <button ref={backButton} type="button" className="tool-detail-page__back" aria-label="返回步骤" onClick={returnToSteps}><LeftOutline /></button>
        <h1 title={title}>{title}</h1>
        {status && <span className="tool-detail-page__status">{status}</span>}
      </header>
      <div className={subtools === undefined ? 'tool-detail-page__body tool-detail-page__body--standalone' : 'tool-detail-page__body'}>
        {subtools}
        <ToolDetailContent {...content} hideEmpty={subtools !== undefined} />
      </div>
    </section>,
    document.body,
  )
}
