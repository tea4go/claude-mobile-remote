import { nativeExperience } from './experience'

// Feedback is optional: an older shell or unavailable hardware must not block selection.
export async function selectionFeedback(): Promise<void> {
  try {
    await nativeExperience?.selectionFeedback?.()
  } catch {
    // Capacitor proxies can expose a method that the installed native shell lacks.
  }
}
