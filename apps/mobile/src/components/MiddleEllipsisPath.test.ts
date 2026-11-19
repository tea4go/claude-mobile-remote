import { expect, test } from 'vitest'
import { fitPath } from './MiddleEllipsisPath'

const measure = (value: string) => Array.from(value).length

test.each([
  ['/Users/person/project/test-connect', '/Users/.../test-connect'],
  ['/home/person/work/data', '/home/.../data'],
  [String.raw`C:\Users\person\work\data`, String.raw`C:\...\data`],
  [String.raw`\\server\share\dir\project`, String.raw`\\server\...\project`],
])('路径按宽度保留首尾目录 %s', (path, compact) => {
  expect(fitPath(path, measure(path), measure)).toBe(path)
  expect(fitPath(path, measure(compact), measure)).toBe(compact)
  const narrow = fitPath(path, 8, measure)
  expect(narrow).toContain('...')
  expect(measure(narrow)).toBeLessThanOrEqual(8)
  expect(path.endsWith(narrow.split('...')[1]!)).toBe(true)
})
