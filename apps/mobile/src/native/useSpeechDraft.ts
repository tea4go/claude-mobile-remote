import { useEffect, useRef, useState } from 'react'
import type { ExperiencePort } from './experience'

export function useSpeechDraft(port: ExperiencePort | null, enabled: boolean, append: (text: string) => void) {
  const [recording, setRecording] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const generation = useRef(0)
  const active = useRef(false)
  const appendRef = useRef(append)
  useEffect(() => { appendRef.current = append }, [append])

  useEffect(() => {
    if (!enabled && active.current) {
      generation.current += 1
      active.current = false
      setRecording(false)
      void port?.cancelSpeech().catch(() => undefined)
    }
  }, [enabled, port])
  useEffect(() => () => {
    generation.current += 1
    if (active.current) void port?.cancelSpeech().catch(() => undefined)
    active.current = false
  }, [port])

  const toggle = async () => {
    if (port === null || !enabled) return
    if (active.current) {
      try { await port.stopSpeech() } catch { setError('无法停止语音，请重试') }
      return
    }
    const current = ++generation.current
    active.current = true
    setRecording(true)
    setError(null)
    try {
      const result = await port.recognize()
      if (generation.current !== current) return
      if (result.text.trim() !== '') appendRef.current(result.text)
      else setError('未识别到文字，请重试')
    } catch {
      if (generation.current === current) setError('语音不可用，请检查系统麦克风、语音识别权限及网络后重试')
    } finally {
      if (generation.current === current) {
        active.current = false
        setRecording(false)
      }
    }
  }
  return { recording, error, toggle }
}
