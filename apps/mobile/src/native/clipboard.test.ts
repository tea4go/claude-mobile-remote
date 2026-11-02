import { beforeEach, expect, test, vi } from 'vitest'

const port = vi.hoisted(() => ({ copyText: vi.fn() }))
vi.mock('./experience', () => ({ nativeExperience: port }))
import { copyText } from './clipboard'

beforeEach(() => { port.copyText.mockReset() })

test('原生应用复用系统剪贴板', async () => {
  port.copyText.mockResolvedValue(undefined)
  await copyText('问题\n答案')
  expect(port.copyText).toHaveBeenCalledWith({ text: '问题\n答案' })
})

test('旧原生壳缺少接口时使用浏览器剪贴板', async () => {
  port.copyText.mockRejectedValue(new Error('unavailable'))
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
  await copyText('内容')
  expect(writeText).toHaveBeenCalledWith('内容')
})

test('系统剪贴板不可用时报告失败', async () => {
  port.copyText.mockRejectedValue(new Error('unavailable'))
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined })
  await expect(copyText('内容')).rejects.toThrow('clipboard unavailable')
})
