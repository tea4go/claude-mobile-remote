import { common } from '@kit.AbilityKit'
import { picker, fileIo as fs } from '@kit.CoreFileKit'
import { photoAccessHelper } from '@kit.MediaLibraryKit'
import { camera, cameraPicker } from '@kit.CameraKit'
import { image } from '@kit.ImageKit'
import { util } from '@kit.ArkTS'
import { scanBarcode, scanCore } from '@kit.ScanKit'
import type { Attachment } from '../protocol/types'
import { ATTACHMENT_BYTES, validateAttachments } from '../session/attachments'

export async function scanPairing(context: common.UIAbilityContext): Promise<string> {
  const result = await scanBarcode.startScanForResult(context, { scanTypes: [scanCore.ScanType.QR_CODE], enableMultiMode: false })
  return result.originalValue
}
function mediaType(bytes: Uint8Array, name: string): string {
  if (bytes[0] === 0xff && bytes[1] === 0xd8) return 'image/jpeg'
  if (bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4e && bytes[3] === 0x47) return 'image/png'
  if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) return 'image/gif'
  if (bytes[0] === 0x52 && bytes[8] === 0x57 && bytes[9] === 0x45) return 'image/webp'
  const extension = name.toLowerCase().split('.').pop() ?? ''
  if (extension === 'pdf') return 'application/pdf'
  if (extension === 'txt' || extension === 'md' || extension === 'csv') return 'text/plain'
  if (extension === 'json') return 'application/json'
  return 'application/octet-stream'
}
async function readAttachment(uri: string, photo: boolean): Promise<Attachment> {
  let name = 'attachment'
  try { name = decodeURIComponent(uri.split('/').pop() ?? name).replace(/[\\/\x00-\x1f\x7f]/g, '_').slice(0, 240) } catch (_) {}
  const file = await fs.open(uri, fs.OpenMode.READ_ONLY)
  let bytes: Uint8Array
  try {
    const size = (await fs.stat(file.fd)).size
    if (size <= 0 || size > (photo ? 20 * 1024 * 1024 : ATTACHMENT_BYTES)) throw new Error('文件为空或超过大小限制')
    const buffer = new ArrayBuffer(size)
    if (await fs.read(file.fd, buffer) !== size) throw new Error('文件读取不完整，请重新选择')
    bytes = new Uint8Array(buffer)
  } finally { await fs.close(file) }
  let type = mediaType(bytes, name)
  if (photo && (bytes.length > ATTACHMENT_BYTES || type === 'application/octet-stream')) {
    const source = image.createImageSource(bytes.buffer)
    let pixels: image.PixelMap | null = null
    const packer = image.createImagePacker()
    try {
      const info = await source.getImageInfo()
      const scale = Math.min(1, 2048 / Math.max(info.size.width, info.size.height))
      pixels = await source.createPixelMap({ desiredSize: { width: Math.max(1, Math.round(info.size.width * scale)), height: Math.max(1, Math.round(info.size.height * scale)) } })
      bytes = new Uint8Array(await packer.packing(pixels, { format: 'image/jpeg', quality: 85 }))
      type = 'image/jpeg'; name = name.replace(/\.[^.]+$/, '') + '.jpg'
    } catch (_) { throw new Error('图片转换失败，请选择 JPEG 或 PNG 图片') }
    finally { await pixels?.release(); await source.release(); await packer.release() }
  }
  const attachment: Attachment = { name, mediaType: type, data: new util.Base64Helper().encodeToStringSync(bytes) }
  const error = validateAttachments([attachment])
  if (error.length > 0) throw new Error(error)
  return attachment
}
export async function selectAttachments(context: common.UIAbilityContext, kind: string, remaining: number): Promise<Array<Attachment>> {
  if (remaining <= 0) throw new Error('每次最多 4 项附件')
  let uris: Array<string> = []
  if (kind === 'photo') {
    const result = await new photoAccessHelper.PhotoViewPicker().select({ MIMEType: photoAccessHelper.PhotoViewMIMETypes.IMAGE_TYPE, maxSelectNumber: remaining })
    uris = result.photoUris
  } else if (kind === 'camera') {
    const result = await cameraPicker.pick(context, [cameraPicker.PickerMediaType.PHOTO], { cameraPosition: camera.CameraPosition.CAMERA_POSITION_BACK })
    if (result.resultCode === 0 && result.resultUri.length > 0) uris = [result.resultUri]
  } else {
    uris = await new picker.DocumentViewPicker(context).select({ maxSelectNumber: remaining })
  }
  const items: Array<Attachment> = []
  for (const uri of uris) items.push(await readAttachment(uri, kind !== 'file'))
  return items
}
