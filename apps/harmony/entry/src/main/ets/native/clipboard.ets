import { pasteboard } from '@kit.BasicServicesKit'
import { promptAction } from '@kit.ArkUI'

export async function copyVisibleText(text: string): Promise<void> {
  try {
    if (text.length === 0) throw new Error('empty text')
    await pasteboard.getSystemPasteboard().setData(pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, text))
    promptAction.showToast({ message: '已复制' })
  } catch (_) {
    promptAction.showToast({ message: '复制失败，请重试' })
  }
}
