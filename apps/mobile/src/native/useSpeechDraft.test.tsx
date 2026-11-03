import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import MessageComposer from '../components/MessageComposer'
import type { ExperiencePort } from './experience'

function port() {
  let resolve!: (value: { text: string; cancelled?: boolean }) => void
  let reject!: (error: Error) => void
  const result = new Promise<{ text: string; cancelled?: boolean }>((yes, no) => { resolve = yes; reject = no })
  const fake = {
    recognize: vi.fn(() => result),
    stopSpeech: vi.fn(async () => { resolve({ text: '识别文字' }) }),
    cancelSpeech: vi.fn(async () => { resolve({ text: '' }) }),
  }
  return { fake: fake as unknown as ExperiencePort, resolve, reject }
}
describe('语音草稿', () => {
  test('停止后等待最终识别，禁止重复停止及发送，结果只追加一次', async () => {
    const { fake, resolve } = port()
    fake.stopSpeech = vi.fn(async () => undefined)
    const onSend = vi.fn()
    render(<MessageComposer enabled experience={fake} onSend={onSend} />)
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    expect(screen.getByTestId('voice-input')).toHaveClass('composer-voice-recording')
    fireEvent.click(screen.getByRole('button', { name: '结束语音输入' }))
    const finishing = screen.getByRole('button', { name: '正在识别语音' })
    expect(finishing).toBeDisabled()
    expect(finishing).not.toHaveClass('composer-voice-recording')
    expect(screen.getByRole('status')).toHaveTextContent('正在识别')
    fireEvent.click(finishing)
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(fake.stopSpeech).toHaveBeenCalledOnce()
    expect(onSend).not.toHaveBeenCalled()
    await act(async () => resolve({ text: '最终识别结果' }))
    expect(screen.getByRole('textbox')).toHaveValue('最终识别结果')
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '语音输入' })).toHaveAttribute('aria-pressed', 'false')
    expect(onSend).not.toHaveBeenCalled()
  })

  test('后台主动取消不提示空识别，不写入取消时的文字', async () => {
    const { fake, resolve } = port()
    render(<MessageComposer enabled experience={fake} onSend={vi.fn()} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '已有草稿' } })
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    await act(async () => resolve({ text: '取消内容', cancelled: true }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('textbox')).toHaveValue('已有草稿')
  })

  test('真正空识别提示重试，手动编辑后清除旧提示', async () => {
    const { fake, resolve } = port()
    render(<MessageComposer enabled experience={fake} onSend={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    await act(async () => resolve({ text: '  ' }))
    expect(screen.getByRole('alert')).toHaveTextContent('未识别到文字，请重试')
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '手动文字' } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  test('保留原草稿的空识别提示在手动发送后清除', async () => {
    const { fake, resolve } = port()
    const onSend = vi.fn()
    render(<MessageComposer enabled experience={fake} onSend={onSend} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '原草稿' } })
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    await act(async () => resolve({ text: '' }))
    expect(screen.getByRole('alert')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(onSend).toHaveBeenCalledWith('原草稿')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  test('结束识别后禁用，迟到最终结果不会写入新页面草稿', async () => {
    const { fake, resolve } = port()
    fake.stopSpeech = vi.fn(async () => undefined)
    fake.cancelSpeech = vi.fn(async () => undefined)
    const { rerender } = render(<MessageComposer enabled experience={fake} onSend={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    fireEvent.click(screen.getByRole('button', { name: '结束语音输入' }))
    rerender(<MessageComposer enabled={false} experience={fake} onSend={vi.fn()} />)
    await act(async () => resolve({ text: '迟到最终结果' }))
    expect(fake.cancelSpeech).toHaveBeenCalledOnce()
    expect(screen.getByRole('textbox')).toHaveValue('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  test('保留原草稿，点击停止追加结果，不自动发送', async () => {
    const { fake } = port()
    const onSend = vi.fn()
    render(<MessageComposer enabled experience={fake} onSend={onSend} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '原草稿' } })
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' })
    expect(onSend).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '结束语音输入' }))
    await waitFor(() => expect(screen.getByRole('textbox')).toHaveValue('原草稿\n识别文字'))
    expect(onSend).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '发送' }))
    expect(onSend).toHaveBeenCalledWith('原草稿\n识别文字')
  })
  test('禁用时取消，迟到结果不能写入草稿', async () => {
    const { fake, resolve } = port()
    fake.cancelSpeech = vi.fn(async () => undefined)
    const { rerender } = render(<MessageComposer enabled experience={fake} onSend={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    rerender(<MessageComposer enabled={false} experience={fake} onSend={vi.fn()} />)
    await act(async () => resolve({ text: '迟到文字' }))
    expect(fake.cancelSpeech).toHaveBeenCalledOnce()
    expect(screen.getByRole('textbox')).toHaveValue('')
  })
  test('切页卸载取消录音，拒绝权限不泄露原始错误', async () => {
    const first = port()
    const { unmount } = render(<MessageComposer enabled experience={first.fake} onSend={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    unmount()
    expect(first.fake.cancelSpeech).toHaveBeenCalledOnce()
    const second = port()
    render(<MessageComposer enabled experience={second.fake} onSend={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: '语音输入' }))
    await act(async () => second.reject(new Error('private diagnostic')))
    expect(screen.getByRole('alert')).toHaveTextContent('语音不可用')
    expect(screen.queryByText('private diagnostic')).not.toBeInTheDocument()
  })
})
