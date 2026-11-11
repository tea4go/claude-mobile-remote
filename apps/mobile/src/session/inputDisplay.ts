// Imported prompts can contain escaped paragraph breaks. Keep isolated escapes
// and code examples intact; this changes presentation only.
export function inputDisplayText(text: string): string {
  if (/[\r\n]/.test(text)) return text
  const parts = text.split(/(```[\s\S]*?(?:```|$)|`[^`]*`)/g)
  const prose = parts.filter((_, index) => index % 2 === 0).join('')
  if (!/\\n\\n/.test(prose) || (prose.match(/(?<!\\)\\(?:r\\n|n)/g) ?? []).length < 3) return text
  return parts.map((part, index) => index % 2 ? part : part.replace(/(?<!\\)\\(?:r\\n|n)/g, '\n')).join('')
}
