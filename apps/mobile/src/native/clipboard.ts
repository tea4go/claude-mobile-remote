import { nativeExperience } from './experience'

// Prefer the native bridge in packaged apps; browsers use their own clipboard.
export async function copyText(text: string): Promise<void> {
  if (!text) throw new Error('empty text')
  if (nativeExperience?.copyText) {
    try {
      await nativeExperience.copyText({ text })
      return
    } catch {
      // Older native shells can still use a browser clipboard if available.
    }
  }
  if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable')
  await navigator.clipboard.writeText(text)
}
