// A bounded, text-only Markdown subset. HTML and remote images are never executed or fetched.
export interface InlineText { text: string; style: string; url: string }
export interface MarkdownBlock { kind: string; text: string; level: number; rows: Array<Array<string>> }
export function inlineMarkdown(text: string): Array<InlineText> {
  const result: Array<InlineText> = []
  const pattern = /(`[^`]+`|\*\*[^*]+\*\*|~~[^~]+~~|\[[^\]]+\]\(https?:\/\/[^\s)]+\))/g
  let cursor = 0
  let match: RegExpExecArray | null
  while ((match = pattern.exec(text)) !== null) {
    if (match.index > cursor) result.push({ text: text.slice(cursor, match.index), style: '', url: '' })
    const raw = match[0]
    if (raw.startsWith('`')) result.push({ text: raw.slice(1, -1), style: 'code', url: '' })
    else if (raw.startsWith('**')) result.push({ text: raw.slice(2, -2), style: 'bold', url: '' })
    else if (raw.startsWith('~~')) result.push({ text: raw.slice(2, -2), style: 'strike', url: '' })
    else {
      const separator = raw.indexOf('](')
      result.push({ text: raw.slice(1, separator), style: 'link', url: raw.slice(separator + 2, -1) })
    }
    cursor = match.index + raw.length
  }
  if (cursor < text.length) result.push({ text: text.slice(cursor), style: '', url: '' })
  return result
}
function tableRow(line: string): Array<string> { return line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((cell: string) => cell.trim()) }
export function markdownBlocks(text: string): Array<MarkdownBlock> {
  const blocks: Array<MarkdownBlock> = []
  const lines = text.split('\n')
  let paragraph: Array<string> = []
  let code: Array<string> = []
  let fenced = false
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]
    if (/^\s*```/.test(line)) {
      if (paragraph.length > 0) { blocks.push({ kind: 'text', text: paragraph.join('\n'), level: 0, rows: [] }); paragraph = [] }
      if (fenced) { blocks.push({ kind: 'code', text: code.join('\n'), level: 0, rows: [] }); code = [] }
      fenced = !fenced
      continue
    }
    if (fenced) { code.push(line); continue }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line)
    const isTable = index + 1 < lines.length && line.includes('|') && /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(lines[index + 1])
    if (heading !== null || isTable || line.trim().length === 0) {
      if (paragraph.length > 0) { blocks.push({ kind: 'text', text: paragraph.join('\n'), level: 0, rows: [] }); paragraph = [] }
      if (heading !== null) blocks.push({ kind: 'heading', text: heading[2], level: heading[1].length, rows: [] })
      else if (isTable) {
        const rows: Array<Array<string>> = [tableRow(line)]
        index += 1
        while (index + 1 < lines.length && lines[index + 1].includes('|')) { index += 1; rows.push(tableRow(lines[index])) }
        blocks.push({ kind: 'table', text: '', level: 0, rows })
      }
    } else paragraph.push(line.replace(/^\s*[-*+]\s+/, '• '))
  }
  if (paragraph.length > 0) blocks.push({ kind: 'text', text: paragraph.join('\n'), level: 0, rows: [] })
  if (fenced || code.length > 0) blocks.push({ kind: 'code', text: code.join('\n'), level: 0, rows: [] })
  return blocks
}
