import { expect, test } from 'vitest'
import { inputDisplayText } from './inputDisplay'

test('shows escaped multiline prose without altering code or ordinary escapes', () => {
  expect(inputDisplayText('第一行\\n\\n第二行\\n第三行 `x\\ny`')).toBe('第一行\n\n第二行\n第三行 `x\\ny`')
  expect(inputDisplayText('path\\name and \\n example')).toBe('path\\name and \\n example')
  expect(inputDisplayText('real\nline\\nexample')).toBe('real\nline\\nexample')
  expect(inputDisplayText('a\\n\\nb\\n```x\\ny```')).toBe('a\n\nb\n```x\\ny```')
})
