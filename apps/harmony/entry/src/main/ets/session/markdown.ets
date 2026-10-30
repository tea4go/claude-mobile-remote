// Native, text-only Markdown. Never execute HTML or fetch Markdown images.
export interface InlineText {
  text: string; style: string; url: string
  bold: boolean; italic: boolean; strike: boolean; code: boolean
}
export interface MarkdownBlock {
  kind: string; text: string; level: number; rows: Array<Array<string>>
  alignments: Array<string>; indent: number; quote: number; marker: string; language: string
}

export function safeMarkdownUrl(url: string): string {
  const value = url.trim()
  if (/[\u0000-\u0020\u007f]/.test(value)) return ''
  if (/^https?:\/\/[^/?#]+/i.test(value) || /^mailto:[^@]+@[^@]+$/i.test(value)) return value
  return ''
}
function entityText(text: string): string {
  return text.replace(/&(#x[0-9a-f]+|#\d+|amp|lt|gt|quot|apos|nbsp);/gi, (raw: string, body: string): string => {
    if (body[0] === '#') {
      const number = body[1].toLowerCase() === 'x' ? parseInt(body.slice(2), 16) : parseInt(body.slice(1), 10)
      return number > 0 && number <= 0x10ffff && !(number >= 0xd800 && number <= 0xdfff) ? String.fromCodePoint(number) : '\ufffd'
    }
    if (body === 'amp') return '&'
    if (body === 'lt') return '<'
    if (body === 'gt') return '>'
    if (body === 'quot') return '"'
    if (body === 'apos') return "'"
    return '\u00a0'
  })
}
function span(text: string, bold: boolean, italic: boolean, strike: boolean, code: boolean, url: string): InlineText {
  return { text, bold, italic, strike, code, url, style: url.length > 0 ? 'link' : code ? 'code' : bold ? 'bold' : strike ? 'strike' : italic ? 'italic' : '' }
}
function delimiterEnd(text: string, from: number, delimiter: string): number {
  let offset = from
  while (offset < text.length) {
    const found = text.indexOf(delimiter, offset)
    if (found < 0) return -1
    if (found > 0 && text[found - 1] === '\\') { offset = found + delimiter.length; continue }
    if (delimiter === '*' || delimiter === '_') {
      let run = 1
      while (text[found + run] === delimiter) run += 1
      if (run === 2) { offset = found + run; continue }
      if (run >= 3) return found + run - 1
    }
    if (delimiter === '**' || delimiter === '__') {
      // A closing triple run ends the inner emphasis before the outer strong.
      if (text[found + 2] === delimiter.charAt(0) && text[from] !== delimiter.charAt(0)) return found + 1
    }
    return found
  }
  return -1
}
interface MarkdownLink { label: string; destination: string; end: number }
function readLink(text: string, offset: number): MarkdownLink | null {
  const close = text.indexOf('](', offset + 1)
  if (close < 0) return null
  let depth = 1
  let cursor = close + 2
  for (; cursor < text.length; cursor += 1) {
    if (text[cursor] === '\\') { cursor += 1; continue }
    if (text[cursor] === '(') depth += 1
    if (text[cursor] === ')') { depth -= 1; if (depth === 0) break }
  }
  if (depth !== 0) return null
  const raw = text.slice(close + 2, cursor).trim()
  const destination = /^<([^<>]*)>(?:\s+["'][\s\S]*["'])?$/.exec(raw)
  const plain = /^(\S+?)(?:\s+["'][\s\S]*["'])?$/.exec(raw)
  return { label: text.slice(offset + 1, close), destination: entityText(destination !== null ? destination[1] : plain !== null ? plain[1] : raw), end: cursor + 1 }
}
function parseInline(text: string, bold: boolean, italic: boolean, strike: boolean, url: string, depth: number): Array<InlineText> {
  if (depth > 12) return [span(text, bold, italic, strike, false, url)]
  if (!/[\\`[!*_~<&]|https?:\/\/|www\./i.test(text)) return [span(entityText(text), bold, italic, strike, false, url)]
  const result: Array<InlineText> = []
  let plain = ''
  let cursor = 0
  while (cursor < text.length) {
    const rest = text.slice(cursor)
    const escape = /^\\([!"#$%&'()*+,\-./:;<=>?@[\]\\^_`{|}~])/.exec(rest)
    if (escape !== null) { plain += escape[1]; cursor += 2; continue }
    const ticks = /^`+/.exec(rest)
    if (ticks !== null) {
      let end = text.indexOf(ticks[0], cursor + ticks[0].length)
      while (end >= 0 && (text[end - 1] === '`' || text[end + ticks[0].length] === '`')) end = text.indexOf(ticks[0], end + ticks[0].length)
      if (end >= 0) {
        if (plain.length > 0) { result.push(span(entityText(plain), bold, italic, strike, false, url)); plain = '' }
        let code = text.slice(cursor + ticks[0].length, end).replace(/\n/g, ' ')
        if (/^ .+ $/.test(code) && code.trim().length > 0) code = code.slice(1, -1)
        result.push(span(code, bold, italic, strike, true, url)); cursor = end + ticks[0].length; continue
      }
    }
    const image = rest.startsWith('![')
    if (image || rest[0] === '[') {
      const link = readLink(text, cursor + (image ? 1 : 0))
      if (link !== null) {
        if (plain.length > 0) { result.push(span(entityText(plain), bold, italic, strike, false, url)); plain = '' }
        // Match mobile: Markdown images are removed, including their link target.
        if (!image) result.push(...parseInline(link.label, bold, italic, strike, safeMarkdownUrl(link.destination), depth + 1))
        cursor = link.end; continue
      }
    }
    const auto = /^(?:<(https?:\/\/[^<>\s]+|mailto:[^<>\s]+|[^<>\s@]+@[^<>\s@]+)>|(https?:\/\/[^\s<>]+|www\.[^\s<>]+))/i.exec(rest)
    if (auto !== null && url.length === 0) {
      let label = auto[1] ?? auto[2]
      if (auto[1] === undefined) {
        label = label.replace(/[.,!?:;]+$/, '')
        while (label.endsWith(')') && label.split('(').length < label.split(')').length) label = label.slice(0, -1)
      }
      let destination = label
      if (/^www\./i.test(label)) destination = 'https://' + label
      else if (!/^[a-z]+:/i.test(label)) destination = 'mailto:' + label
      if (plain.length > 0) { result.push(span(entityText(plain), bold, italic, strike, false, url)); plain = '' }
      result.push(span(entityText(label.replace(/^mailto:/i, '')), bold, italic, strike, false, safeMarkdownUrl(entityText(destination))))
      cursor += auto[1] !== undefined ? auto[0].length : label.length; continue
    }
    let delimiter = ''
    for (const candidate of ['***', '___', '**', '__', '~~', '*', '_']) {
      if (!rest.startsWith(candidate) || /\s/.test(rest[candidate.length] ?? ' ')) continue
      if (candidate[0] === '_' && cursor > 0 && /[\p{L}\p{N}]/u.test(text[cursor - 1])) continue
      delimiter = candidate; break
    }
    if (delimiter.length > 0) {
      const end = delimiterEnd(text, cursor + delimiter.length, delimiter)
      if (end > cursor + delimiter.length && !/\s/.test(text[end - 1])) {
        if (plain.length > 0) { result.push(span(entityText(plain), bold, italic, strike, false, url)); plain = '' }
        result.push(...parseInline(text.slice(cursor + delimiter.length, end), bold || delimiter.length >= 2 && delimiter !== '~~',
          italic || delimiter.length === 1 || delimiter.length === 3, strike || delimiter === '~~', url, depth + 1))
        cursor = end + delimiter.length; continue
      }
    }
    plain += text[cursor]; cursor += 1
  }
  if (plain.length > 0) result.push(span(entityText(plain), bold, italic, strike, false, url))
  return result
}
export function inlineMarkdown(text: string): Array<InlineText> { return parseInline(text, false, false, false, '', 0) }

function block(kind: string, text: string, indent: number, quote: number): MarkdownBlock {
  return { kind, text, indent, quote, level: 0, rows: [], alignments: [], marker: '', language: '' }
}
function tableRow(line: string): Array<string> {
  const source = line.trim().replace(/^\|/, '').replace(/(?<!\\)\|$/, '')
  const cells: Array<string> = []
  let cell = ''; let ticks = 0
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index]
    if (char === '\\' && source[index + 1] === '|') { cell += '|'; index += 1; continue }
    if (char === '`') {
      let count = 1
      while (source[index + count] === '`') count += 1
      if (ticks === 0) ticks = count
      else if (ticks === count) ticks = 0
      cell += '`'.repeat(count); index += count - 1; continue
    }
    if (char === '|' && ticks === 0) { cells.push(cell.trim()); cell = '' }
    else cell += char
  }
  cells.push(cell.trim()); return cells
}
function tableAlignment(line: string): Array<string> {
  const cells = tableRow(line)
  if (!cells.every((cell: string) => /^:?-{1,}:?$/.test(cell))) return []
  return cells.map((cell: string) => cell.startsWith(':') && cell.endsWith(':') ? 'center' : cell.endsWith(':') ? 'right' : 'left')
}
function isRule(line: string): boolean { return /^ {0,3}(?:(?:\*\s*){3,}|(?:-\s*){3,}|(?:_\s*){3,})$/.test(line) }
function listMatch(line: string): RegExpExecArray | null { return /^ {0,3}([-+*]|\d{1,9}[.)])(?:[ \t]+(.*)|$)/.exec(line) }
function startsBlock(line: string): boolean {
  return line.trim().length === 0 || /^ {0,3}(?:#{1,6}\s|>|`{3,}|~{3,})/.test(line) || isRule(line) || listMatch(line) !== null
}
function parseBlocks(lines: Array<string>, indent: number, quote: number, depth: number): Array<MarkdownBlock> {
  if (depth > 12) return [block('text', lines.join('\n'), indent, quote)]
  const blocks: Array<MarkdownBlock> = []
  let index = 0
  while (index < lines.length) {
    const line = lines[index]
    if (line.trim().length === 0) { index += 1; continue }
    const fence = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(line)
    if (fence !== null && !(fence[1][0] === '`' && fence[2].includes('`'))) {
      const code: Array<string> = []; index += 1
      const closing = new RegExp('^ {0,3}' + fence[1][0] + '{' + fence[1].length + ',}\\s*$')
      const spaces = line.length - line.trimStart().length
      while (index < lines.length && !closing.test(lines[index])) { code.push(lines[index].replace(new RegExp('^ {0,' + spaces + '}'), '')); index += 1 }
      const item = block('code', code.join('\n'), indent, quote); item.language = fence[2].trim().split(/\s+/)[0]; blocks.push(item)
      if (index < lines.length) index += 1
      continue
    }
    if (/^ {0,3}>/.test(line)) {
      const nested: Array<string> = []
      while (index < lines.length && /^ {0,3}>/.test(lines[index])) { nested.push(lines[index].replace(/^ {0,3}> ?/, '')); index += 1 }
      blocks.push(...parseBlocks(nested, indent, quote + 1, depth + 1)); continue
    }
    if (isRule(line)) { blocks.push(block('rule', '', indent, quote)); index += 1; continue }
    const heading = /^ {0,3}(#{1,6})(?:\s+(.*)|\s*)$/.exec(line)
    if (heading !== null) {
      const item = block('heading', (heading[2] ?? '').replace(/\s+#+\s*$/, ''), indent, quote); item.level = heading[1].length; blocks.push(item); index += 1; continue
    }
    const list = listMatch(line)
    if (list !== null) {
      const content = list[2] ?? ''
      const contentOffset = line.length - content.length
      const nested = [content]; index += 1
      while (index < lines.length) {
        if (lines[index].trim().length === 0) {
          if (index + 1 < lines.length && lines[index + 1].startsWith(' '.repeat(contentOffset))) { nested.push(''); index += 1; continue }
          break
        }
        if (!lines[index].startsWith(' '.repeat(contentOffset))) break
        nested.push(lines[index].slice(contentOffset)); index += 1
      }
      const items = parseBlocks(nested, indent + 1, quote, depth + 1)
      if (items.length === 0) items.push(block('text', '', indent + 1, quote))
      const task = /^\[([ xX])\](?:\s+|$)/.exec(items[0].text)
      items[0].marker = task !== null ? task[1] === ' ' ? '☐' : '☑' : /^\d/.test(list[1]) ? list[1].replace(')', '.') : '•'
      if (task !== null) items[0].text = items[0].text.slice(task[0].length)
      blocks.push(...items); continue
    }
    const alignments = index + 1 < lines.length && line.includes('|') ? tableAlignment(lines[index + 1]) : []
    const header = alignments.length > 0 ? tableRow(line) : []
    if (alignments.length > 0 && alignments.length === header.length) {
      const item = block('table', '', indent, quote); item.rows = [header]; item.alignments = alignments; index += 2
      while (index < lines.length && lines[index].includes('|') && !startsBlock(lines[index])) {
        const row = tableRow(lines[index]).slice(0, header.length)
        while (row.length < header.length) row.push('')
        item.rows.push(row); index += 1
      }
      blocks.push(item); continue
    }
    if (/^( {4}|\t)/.test(line)) {
      const code: Array<string> = []
      while (index < lines.length && (/^( {4}|\t)/.test(lines[index]) || lines[index].trim().length === 0)) { code.push(lines[index].replace(/^( {4}|\t)/, '')); index += 1 }
      blocks.push(block('code', code.join('\n').trimEnd(), indent, quote)); continue
    }
    const paragraph = [line]; index += 1
    let level = 0
    while (index < lines.length) {
      if (/^ {0,3}(=+|-+)\s*$/.test(lines[index])) { level = lines[index].trim()[0] === '=' ? 1 : 2; index += 1; break }
      if (startsBlock(lines[index])) break
      // A table can begin immediately after an ordinary paragraph.
      if (index + 1 < lines.length && lines[index].includes('|') && tableAlignment(lines[index + 1]).length > 0) break
      paragraph.push(lines[index]); index += 1
    }
    const item = block(level > 0 ? 'heading' : 'text', paragraph.join('\n').replace(/ {2,}\n|\\\n/g, '\n'), indent, quote)
    item.level = level; blocks.push(item)
  }
  return blocks
}
export function markdownBlocks(text: string): Array<MarkdownBlock> {
  return parseBlocks(text.replace(/\r\n?/g, '\n').split('\n'), 0, 0, 0)
}
