import type { Attachment } from '../protocol/types'

export const ATTACHMENT_BYTES = 4 * 1024 * 1024
export const ATTACHMENT_TOTAL_BYTES = 8 * 1024 * 1024
export function utf8Bytes(text: string): number {
  let size = 0
  for (const ch of text) {
    const code = ch.codePointAt(0) ?? 0
    size += code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4
  }
  return size
}
export function attachmentSize(data: string): number {
  return data.length / 4 * 3 - (data.endsWith('==') ? 2 : data.endsWith('=') ? 1 : 0)
}
export function validateAttachments(items: Array<Attachment>): string {
  if (items.length === 0 || items.length > 4) return '每次请选择 1–4 项附件'
  let total = 0
  for (const item of items) {
    if (!item.name || item.name.length > 255 || /[\\/\x00-\x1f\x7f]/.test(item.name)) return '附件名称无效'
    if (!/^[\w.+-]+\/[\w.+-]+$/.test(item.mediaType)) return '附件格式无效'
    if (item.data.length > Math.ceil(ATTACHMENT_BYTES / 3) * 4 || item.data.length % 4 !== 0 ||
      !/^[A-Za-z0-9+/]*={0,2}$/.test(item.data)) return '附件数据无效或超过 4 MiB'
    const size = attachmentSize(item.data)
    if (size <= 0 || size > ATTACHMENT_BYTES) return '附件为空或超过 4 MiB'
    total += size
  }
  return total > ATTACHMENT_TOTAL_BYTES ? '附件总量超过 8 MiB' : ''
}

export interface ComposerDraft {
  text: string
  attachments: Array<Attachment>
  sending: boolean
  error: string
}
export function emptyDraft(): ComposerDraft { return { text: '', attachments: [], sending: false, error: '' } }
