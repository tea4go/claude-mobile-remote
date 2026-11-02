import { fireEvent, render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

const clipboard = vi.hoisted(() => ({ copyText: vi.fn() }))
vi.mock('../native/clipboard', () => clipboard)
import CopyTextButton from './CopyTextButton'

test('复制失败时提示用户长按选择', async () => {
  clipboard.copyText.mockRejectedValue(new Error('unavailable'))
  render(<CopyTextButton label="复制问题" text="内容" />)
  fireEvent.click(screen.getByRole('button', { name: '复制问题' }))
  expect(await screen.findByRole('status')).toHaveTextContent('复制失败，请长按文字选择复制')
})
