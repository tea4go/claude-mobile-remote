import { afterEach, expect, test, vi } from 'vitest'
import { needsPhotoPreparation, preparePickedFile, validateFiles } from './attachments'

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

test('ordinary files preserve exact bytes and shared limits apply', async () => {
  const file = new File(['bytes'], 'report.pdf', { type: 'application/pdf' })
  expect(await preparePickedFile(file)).toBe(file)
  expect(needsPhotoPreparation(new File(['photo'], 'photo.HEIC'))).toBe(true)
  expect(() => validateFiles(Array.from({ length: 5 }, () => file))).toThrow('最多添加 4 个附件')
  const large = new File([new Uint8Array(3 * 1024 * 1024)], 'x.txt')
  expect(() => validateFiles([large, large, large])).toThrow('附件合计最多 8 MiB')
})

test('HEIC conversion scales to 2048 and releases source resource', async () => {
  const revoke = vi.fn()
  vi.stubGlobal('URL', class extends URL { static createObjectURL = vi.fn(() => 'blob:photo'); static revokeObjectURL = revoke })
  vi.stubGlobal('Image', class {
    naturalWidth = 4000
    naturalHeight = 3000
    onload: (() => void) | null = null
    set src(_value: string) { queueMicrotask(() => this.onload?.()) }
  })
  const drawImage = vi.fn()
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ fillStyle: '', fillRect: vi.fn(), drawImage } as never)
  vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation((callback) => callback(new Blob(['jpeg bytes'], { type: 'image/jpeg' })))
  const file = await preparePickedFile(new File(['heic bytes'], 'photo.heic', { type: 'image/heic' }))
  expect(file.name).toBe('photo.jpg')
  expect(file.type).toBe('image/jpeg')
  expect(drawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, 2048, 1536)
  expect(revoke).toHaveBeenCalledWith('blob:photo')
})

test('unsupported photo conversion reports error and releases resource', async () => {
  const revoke = vi.fn()
  vi.stubGlobal('URL', class extends URL { static createObjectURL = vi.fn(() => 'blob:bad'); static revokeObjectURL = revoke })
  vi.stubGlobal('Image', class {
    onerror: (() => void) | null = null
    set src(_value: string) { queueMicrotask(() => this.onerror?.()) }
  })
  await expect(preparePickedFile(new File(['bad'], 'bad.heic'))).rejects.toThrow('无法转换此照片')
  expect(revoke).toHaveBeenCalledWith('blob:bad')
})
