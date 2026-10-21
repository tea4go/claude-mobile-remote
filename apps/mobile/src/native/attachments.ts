import type { Attachment } from '../protocol/types'
export const MAX_FILES = 4
export const MAX_FILE_BYTES = 4 * 1024 * 1024
export const MAX_FILES_BYTES = 8 * 1024 * 1024

export function validateFiles(files: File[]): void {
  if (files.length > MAX_FILES) throw new Error('最多添加 4 个附件')
  if (files.some((file) => file.size === 0 || file.size > MAX_FILE_BYTES)) throw new Error('附件不能为空，每个附件最多 4 MiB')
  if (files.reduce((sum, file) => sum + file.size, 0) > MAX_FILES_BYTES) throw new Error('附件合计最多 8 MiB')
  if (files.some((file) => new TextEncoder().encode(file.name).length > 255 || (/[\\/]/.test(file.name) || [...file.name].some((value) => { const code = value.codePointAt(0) ?? 0; return code < 32 || code === 127 })) || !file.name || file.name === '.' || file.name === '..')) throw new Error('附件名称无效或过长，请重命名后重试')
}

export function encodeFile(file: File): Promise<Attachment> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('读取附件失败，请重新选择'))
    reader.onabort = () => reject(new Error('附件读取已取消'))
    reader.onload = () => {
      if (typeof reader.result !== 'string' || !reader.result.includes(',')) { reject(new Error('读取附件失败')); return }
      resolve({ name: file.name, mediaType: file.type || 'application/octet-stream', data: reader.result.slice(reader.result.indexOf(',') + 1) })
    }
    reader.readAsDataURL(file)
  })
}

// iOS commonly supplies HEIC photos. Convert these, and oversized photos, into
// bounded JPEGs before the shared upload validation; ordinary files stay intact.
export function needsPhotoPreparation(file: File): boolean {
  const image = file.type.startsWith('image/') || /\.(heic|heif)$/i.test(file.name)
  return image && (/heic|heif/i.test(file.type) || /\.(heic|heif)$/i.test(file.name) || file.size > MAX_FILE_BYTES)
}

export async function preparePickedFile(file: File): Promise<File> {
  if (!needsPhotoPreparation(file)) return file
  if (file.size > 20 * 1024 * 1024) throw new Error('原始照片超过 20 MiB，请先缩小后重试')
  const url = URL.createObjectURL(file)
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const value = new Image()
      value.onload = () => resolve(value)
      value.onerror = () => reject(new Error('无法转换此照片，请导出为 JPEG 或 PNG 后重试'))
      value.src = url
    })
    if (!image.naturalWidth || !image.naturalHeight) throw new Error('图片尺寸无效')
    const scale = Math.min(1, 2048 / Math.max(image.naturalWidth, image.naturalHeight))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
    const context = canvas.getContext('2d')
    if (context === null) throw new Error('无法处理照片，请缩小或转换为 JPEG 后重试')
    context.fillStyle = '#fff'
    context.fillRect(0, 0, canvas.width, canvas.height)
    context.drawImage(image, 0, 0, canvas.width, canvas.height)
    const blob = await new Promise<Blob>((resolve, reject) => canvas.toBlob((value) => value === null ? reject(new Error('照片转换失败')) : resolve(value), 'image/jpeg', 0.85))
    return new File([blob], file.name.replace(/\.[^.]+$/, '') + '.jpg', { type: 'image/jpeg' })
  } finally { URL.revokeObjectURL(url) }
}
