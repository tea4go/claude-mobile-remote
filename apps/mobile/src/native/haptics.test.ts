import { afterEach, expect, test, vi } from 'vitest'

const fixtures = vi.hoisted(() => ({ port: null as null | { selectionFeedback?: () => Promise<void> } }))
vi.mock('./experience', () => ({ get nativeExperience() { return fixtures.port } }))
import { selectionFeedback } from './haptics'

afterEach(() => { fixtures.port = null })

test('调用一次原生选区触觉反馈', async () => {
  const feedback = vi.fn().mockResolvedValue(undefined)
  fixtures.port = { selectionFeedback: feedback }
  await selectionFeedback()
  expect(feedback).toHaveBeenCalledExactlyOnceWith()
})

test('浏览器、旧壳和原生反馈异常均静默降级', async () => {
  await expect(selectionFeedback()).resolves.toBeUndefined()
  fixtures.port = {}
  await expect(selectionFeedback()).resolves.toBeUndefined()
  fixtures.port = { selectionFeedback: vi.fn().mockRejectedValue(new Error('UNIMPLEMENTED')) }
  await expect(selectionFeedback()).resolves.toBeUndefined()
})
