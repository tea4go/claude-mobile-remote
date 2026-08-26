import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import MessageMarkdown from './MessageMarkdown'

describe('MessageMarkdown：受限语法', () => {
  test('渲染标题', () => {
    render(<MessageMarkdown text={'# 一级标题'} />)
    expect(screen.getByRole('heading', { name: '一级标题' })).toBeInTheDocument()
  })

  test('渲染粗体与行内代码', () => {
    const { container } = render(<MessageMarkdown text={'这里**很重要**，用 `go test` 验证'} />)
    expect(container.querySelector('strong')?.textContent).toBe('很重要')
    expect(container.querySelector('code')?.textContent).toBe('go test')
  })

  test('渲染无序列表', () => {
    const { container } = render(<MessageMarkdown text={'- 第一项\n- 第二项'} />)
    expect(container.querySelectorAll('li')).toHaveLength(2)
  })

  test('渲染 GFM 删除线与表格', () => {
    const { container } = render(
      <MessageMarkdown text={'~~废弃~~\n\n| 列 A | 列 B |\n| --- | --- |\n| 1 | 2 |'} />,
    )
    expect(container.querySelector('del')?.textContent).toBe('废弃')
    expect(container.querySelector('table')).not.toBeNull()
    expect(container.querySelectorAll('th')).toHaveLength(2)
  })

  test('渲染代码块并保留原文', () => {
    const { container } = render(
      <MessageMarkdown text={'```go\nfunc main() {}\n```'} />,
    )
    const block = container.querySelector('pre code')
    expect(block).not.toBeNull()
    expect(block?.textContent).toContain('func main() {}')
  })

  test('保留段落内换行的可读性', () => {
    const { container } = render(<MessageMarkdown text={'第一行\n第二行'} />)
    expect(container.textContent).toContain('第一行')
    expect(container.textContent).toContain('第二行')
  })

  test('代码块内长行不擅自折行，容器提供横向滚动', () => {
    const longLine = 'const x = ' + 'a'.repeat(200) + ';'
    const { container } = render(<MessageMarkdown text={'```\n' + longLine + '\n```'} />)
    const pre = container.querySelector('pre')
    expect(pre).not.toBeNull()
    expect(pre?.style.overflowX).toBe('auto')
    expect(pre?.style.whiteSpace).toBe('pre')
  })

  test('表格单元格不擅自折行，容器提供横向滚动', () => {
    const { container } = render(
      <MessageMarkdown text={'| 列 A | 列 B |\n| --- | --- |\n| 很长很长很长很长很长很长很长很长很长很长很长很长 | 2 |'} />,
    )
    // 表格外层包裹提供 overflowX: auto，作为 <table> 的父级 div。
    const table = container.querySelector('table')
    expect(table).not.toBeNull()
    const wrapper = table?.parentElement as HTMLElement
    expect(wrapper?.style.overflowX).toBe('auto')
    const td = container.querySelector('td')
    expect(td).not.toBeNull()
    expect(td?.style.whiteSpace).toBe('nowrap')
  })

  test('列表保持随宽度自然折行，不走横向滚动', () => {
    // 列表属于正文：逐条横向拉动反而更难阅读，用户实测后回退。
    const { container } = render(
      <MessageMarkdown text={'- 很长很长很长很长很长很长很长很长很长很长很长很长的条目\n- 第二条'} />,
    )
    const ul = container.querySelector('ul')
    expect(ul).not.toBeNull()
    const wrapper = (ul as HTMLElement).parentElement as HTMLElement
    expect(wrapper.style.overflowX).toBe('')
    const li = container.querySelector('li')
    expect(li?.style.whiteSpace).toBe('')
  })

  test('顶层段落仍保留软换行', () => {
    const { container } = render(<MessageMarkdown text="普通段落文本" />)
    const p = container.querySelector('p')
    expect(p).not.toBeNull()
    expect((p as HTMLElement).style.whiteSpace).toBe('pre-wrap')
  })

  test('空文本不渲染额外容器内容', () => {
    const { container } = render(<MessageMarkdown text="" />)
    expect(container.textContent).toBe('')
  })
})

describe('MessageMarkdown：安全边界', () => {
  test('恶意 img 标签不进入 DOM', () => {
    const { container } = render(
      <MessageMarkdown text={'正文 <img src=x onerror="globalThis.__pwned = true">'} />,
    )
    expect(container.querySelector('img')).toBeNull()
    // 原始标签只能作为纯文本出现，不能被解析成元素。
    expect(container.textContent).toContain('<img')
  })

  test('script 标签不进入 DOM 也不执行', () => {
    const { container } = render(
      <MessageMarkdown text={'<script>globalThis.__pwned = true</script>'} />,
    )
    expect(container.querySelector('script')).toBeNull()
    expect((globalThis as Record<string, unknown>).__pwned).toBeUndefined()
  })

  test('iframe 与事件属性不进入 DOM', () => {
    const { container } = render(
      <MessageMarkdown text={'<iframe src="https://example.com"></iframe><div onclick="x()">点</div>'} />,
    )
    expect(container.querySelector('iframe')).toBeNull()
    expect(container.querySelector('[onclick]')).toBeNull()
  })

  test('javascript: 链接不产生可执行 href', () => {
    const { container } = render(<MessageMarkdown text={'[点我](javascript:alert(1))'} />)
    const anchor = container.querySelector('a')
    expect(anchor).not.toBeNull()
    expect(anchor?.getAttribute('href') ?? '').not.toContain('javascript')
  })

  test('普通 http 链接保留地址并禁止引用来源泄漏', () => {
    const { container } = render(<MessageMarkdown text={'[文档](https://example.com/a)'} />)
    const anchor = container.querySelector('a')
    expect(anchor?.getAttribute('href')).toBe('https://example.com/a')
    expect(anchor?.getAttribute('rel')).toContain('noreferrer')
  })

  test('代码块中的 HTML 保持纯文本', () => {
    const { container } = render(
      <MessageMarkdown text={'```html\n<script>alert(1)</script>\n```'} />,
    )
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('pre code')?.textContent).toContain('<script>alert(1)</script>')
  })

  test('图片语法不加载远程资源', () => {
    const { container } = render(<MessageMarkdown text={'![图](https://example.com/a.png)'} />)
    expect(container.querySelector('img')).toBeNull()
  })
})
