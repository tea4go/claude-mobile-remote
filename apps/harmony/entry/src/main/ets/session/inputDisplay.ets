export function inputDisplayText(text: string): string {
  if (/[\r\n]/.test(text)) return text
  const parts = text.split(/(```[\s\S]*?(?:```|$)|`[^`]*`)/g)
  const prose = parts.filter((_part: string, index: number): boolean => index % 2 === 0).join('')
  if (!/\\n\\n/.test(prose) || (prose.match(/(?<!\\)\\(?:r\\n|n)/g) ?? []).length < 3) return text
  return parts.map((part: string, index: number): string => index % 2 === 1 ? part : part.replace(/(?<!\\)\\(?:r\\n|n)/g, '\n')).join('')
}
