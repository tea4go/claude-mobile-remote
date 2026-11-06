import { afterEach, expect, test, vi } from 'vitest'
import { clipFor, graphemeBoundaries, indexMessage, measureSelection, moveBoundary, offsetAtPoint } from './messageSelection'

const box = (left: number, top: number, width: number, height: number) => ({ left, top, right: left + width, bottom: top + height, x: left, y: top, width, height, toJSON() {} })
function root(text: string) {
  const node = document.createElement('div')
  node.textContent = text
  document.body.append(node)
  vi.spyOn(node, 'getBoundingClientRect').mockReturnValue(box(0, 100, 300, 300))
  return node
}
function geometry(rect: (range: Range) => ReturnType<typeof box>[]) {
  const createRange = document.createRange.bind(document)
  vi.spyOn(document, 'createRange').mockImplementation(() => {
    const range = createRange()
    Object.defineProperty(range, 'getClientRects', { value: () => rect(range) })
    return range
  })
}
afterEach(() => { document.body.replaceChildren(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

test('保留 Markdown 段落、换行、表格单元格和代码内容，排除隐藏内容与按钮', () => {
  const node = root('')
  const first = document.createElement('p')
  first.append('第一段', document.createElement('br'), '换行')
  const second = document.createElement('p')
  const strong = document.createElement('strong')
  strong.textContent = '强调'
  second.append(strong, '正文')
  const pre = document.createElement('pre')
  pre.textContent = 'a\n  b\n'
  const table = document.createElement('table')
  const row = table.insertRow()
  row.insertCell().textContent = '单元格1'
  row.insertCell().textContent = '单元格2'
  const hidden = document.createElement('span')
  hidden.hidden = true
  hidden.textContent = '隐藏'
  const button = document.createElement('button')
  button.textContent = '操作'
  node.append(first, second, pre, table, hidden, button)
  const index = indexMessage(node)
  expect(index.text).toBe('第一段\n换行\n强调正文\na\n  b\n单元格1\t单元格2')
  for (const run of index.runs) expect(index.text.slice(run.start, run.end)).toBe(run.node.data)
})

test('字素边界包含完整 emoji、组合字符，起止不能交叉或变为空', () => {
  const node = root('甲👨‍👩‍👧‍👦é乙')
  const index = indexMessage(node)
  expect(index.boundaries).toEqual([0, 1, 12, 14, 15])
  expect(moveBoundary(index, 0, 15, 'start', 9)).toEqual([12, 15])
  expect(moveBoundary(index, 12, 15, 'end', 0)).toEqual([12, 14])
  expect(moveBoundary(index, 0, 14, 'start', 15)).toEqual([12, 14])
  expect(index.text.slice(1, 14)).toBe('👨‍👩‍👧‍👦é')
})

test('旧引擎的字素降级保留 emoji 链、肤色、旗帜和组合重音', () => {
  vi.stubGlobal('Intl', Object.defineProperty(Object.create(Intl), 'Segmenter', { value: undefined }))
  expect(graphemeBoundaries('👩🏽‍💻🇨🇳é')).toEqual([0, 7, 11, 13])
})

test('手柄端点位于虚拟段落换行时仍锚定相邻的真实字形', () => {
  const node = root('')
  const first = document.createElement('p')
  const second = document.createElement('p')
  first.textContent = '甲'
  second.textContent = '乙'
  node.append(first, second)
  geometry((range) => [box(20, range.startContainer.parentElement === first ? 120 : 160, 10, 20)])
  const index = indexMessage(node)
  expect(index.text).toBe('甲\n乙')
  expect(measureSelection(index, 1, 3).start).toEqual({ x: 20, top: 160, bottom: 180 })
  expect(measureSelection(index, 0, 2).end).toEqual({ x: 30, top: 120, bottom: 140 })
})

test('高亮按外层滚动区及内部代码横向滚动区裁剪，不覆盖输入框', () => {
  const scroll = document.createElement('div')
  scroll.style.overflowY = 'auto'
  vi.spyOn(scroll, 'getBoundingClientRect').mockReturnValue(box(0, 100, 300, 200))
  Object.defineProperties(scroll, { clientHeight: { value: 200 }, clientWidth: { value: 300 } })
  document.body.append(scroll)
  const node = root('')
  scroll.append(node)
  const pre = document.createElement('pre')
  pre.style.overflowX = 'auto'
  pre.textContent = '很长的代码'
  vi.spyOn(pre, 'getBoundingClientRect').mockReturnValue(box(20, 120, 180, 300))
  Object.defineProperties(pre, { clientWidth: { value: 180 }, clientHeight: { value: 300 } })
  node.append(pre)
  geometry(() => [box(10, 120, 500, 20), box(10, 290, 500, 30), box(10, 350, 500, 20)])
  const index = indexMessage(node)
  const result = measureSelection(index, 0, index.text.length)
  expect(result.rects).toEqual([
    { left: 20, top: 120, right: 200, bottom: 140 },
    { left: 20, top: 290, right: 200, bottom: 300 },
  ])
  expect(result.start).toBeNull()
  expect(result.end).toBeNull()
  expect(clipFor(node).bottom).toBe(300)
})

test('系统 caret API 不返回禁选文字时通过 Range 几何找到拖动边界', () => {
  const node = root('甲乙丙丁')
  geometry((range) => [box(20 + range.startOffset * 10, 120, (range.endOffset - range.startOffset) * 10, 20)])
  const index = indexMessage(node)
  expect(offsetAtPoint(index, { x: 40, y: 130 })).toBe(2)
  expect(offsetAtPoint(index, { x: 21, y: 130 })).toBe(0)
  expect(offsetAtPoint(index, { x: 59, y: 130 })).toBe(4)
})

test('多行文字的几何降级优先命中触点所在行', () => {
  const node = root('甲乙丙丁戊己')
  geometry((range) => {
    const result = []
    for (let i = range.startOffset; i < range.endOffset; i++) result.push(box(20 + (i % 3) * 10, 120 + Math.floor(i / 3) * 24, 10, 20))
    return result
  })
  expect(offsetAtPoint(indexMessage(node), { x: 36, y: 154 })).toBe(5)
})

test('Range 高亮不注册系统选区，内联相邻字形合并避免加深', () => {
  const node = root('')
  node.append('甲', document.createElement('strong'))
  node.lastChild!.textContent = '乙'
  geometry((range) => [box(range.startContainer.parentElement?.tagName === 'STRONG' ? 29 : 20, 120, 10, 20)])
  const addRange = vi.spyOn(window.getSelection()!, 'addRange')
  const index = indexMessage(node)
  expect(measureSelection(index, 0, index.text.length).rects).toEqual([{ left: 20, top: 120, right: 39, bottom: 140 }])
  expect(addRange).not.toHaveBeenCalled()
})
