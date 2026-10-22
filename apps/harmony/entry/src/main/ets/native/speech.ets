import { abilityAccessCtrl, common } from '@kit.AbilityKit'
import { speechRecognizer } from '@kit.CoreSpeechKit'
import { audio } from '@kit.AudioKit'

// Audio and recognition are bound to one draft generation; late callbacks cannot edit another Session.
export class SpeechDraft {
  private engine: speechRecognizer.SpeechRecognitionEngine | null = null
  private capturer: audio.AudioCapturer | null = null
  private generation: number = 0
  private sessionId: string = ''
  private timer: number = -1
  private listener: ((text: string, active: boolean, error: string) => void) | null = null
  private text: string = ''
  async start(context: common.UIAbilityContext, listener: (text: string, active: boolean, error: string) => void): Promise<void> {
    const cancelling = this.cancel()
    const generation = this.generation
    await cancelling
    if (generation !== this.generation) return
    this.listener = listener; this.text = ''
    listener('', true, '')
    try {
      const permission = await abilityAccessCtrl.createAtManager().requestPermissionsFromUser(context, ['ohos.permission.MICROPHONE'])
      if (generation !== this.generation) return
      if (permission.authResults[0] !== 0) throw new Error('麦克风权限未获授权')
      const engine = await speechRecognizer.createEngine({ language: 'zh-CN', online: 1 })
      if (generation !== this.generation) { engine.shutdown(); return }
      this.engine = engine
      this.sessionId = 'cmr-speech-' + generation + '-' + Date.now()
      const id = this.sessionId
      const callbacks: speechRecognizer.RecognitionListener = {
        onStart: (_id: string, _message: string) => {},
        onEvent: (_id: string, _code: number, _message: string) => {},
        onResult: (_id: string, result: speechRecognizer.SpeechRecognitionResult) => {
          if (generation !== this.generation) return
          this.text = result.result
          this.listener?.(this.text, true, '')
          if (result.isLast) void this.complete('')
        },
        onComplete: (_id: string, _message: string) => { if (generation === this.generation) void this.complete('') },
        onError: (_id: string, _code: number, _message: string) => {
          if (generation === this.generation) void this.complete('语音识别失败，请检查权限、网络或系统语音服务')
        },
      }
      engine.setListener(callbacks)
      const options: audio.AudioCapturerOptions = {
        streamInfo: { samplingRate: audio.AudioSamplingRate.SAMPLE_RATE_16000, channels: audio.AudioChannel.CHANNEL_1,
          sampleFormat: audio.AudioSampleFormat.SAMPLE_FORMAT_S16LE, encodingType: audio.AudioEncodingType.ENCODING_TYPE_RAW },
        capturerInfo: { source: audio.SourceType.SOURCE_TYPE_MIC, capturerFlags: 0 },
      }
      const capturer = await audio.createAudioCapturer(options)
      if (generation !== this.generation) { await capturer.release(); return }
      this.capturer = capturer
      capturer.on('readData', (buffer: ArrayBuffer) => {
        if (generation === this.generation) {
          try { engine.writeAudio(id, new Uint8Array(buffer)) } catch (_) { void this.complete('语音服务中断') }
        }
      })
      capturer.on('audioInterrupt', () => { if (generation === this.generation) void this.complete('录音被系统中断') })
      engine.startListening({ sessionId: id,
        audioInfo: { audioType: 'pcm', sampleRate: 16000, soundChannel: 1, sampleBit: 16 },
        extraParams: { recognitionMode: 0, vadBegin: 5000, vadEnd: 3000, maxAudioDuration: 60000 } })
      if (generation !== this.generation) return
      await capturer.start()
      if (generation !== this.generation) return
      this.timer = setTimeout(() => { void this.stop() }, 60000)
    } catch (_) {
      if (generation === this.generation) await this.complete('无法开始语音输入，请检查麦克风权限和系统语音服务')
    }
  }
  async stop(): Promise<void> {
    if (this.engine === null) { await this.cancel(); return }
    const generation = this.generation
    if (this.timer >= 0) clearTimeout(this.timer)
    const capturer = this.capturer; this.capturer = null
    if (capturer !== null) { await capturer.stop().catch(() => {}); await capturer.release().catch(() => {}) }
    if (generation !== this.generation) return
    try { this.engine?.finish(this.sessionId) } catch (_) { await this.complete('语音识别停止失败') }
    if (this.engine !== null) this.timer = setTimeout(() => { void this.complete('') }, 5000)
  }
  async cancel(): Promise<void> {
    this.listener?.('', false, '')
    this.listener = null
    await this.release()
  }
  private async complete(error: string): Promise<void> {
    const listener = this.listener; this.listener = null
    const text = this.text
    const releasing = this.release()
    const generation = this.generation
    await releasing
    if (generation === this.generation) listener?.(text, false, error)
  }
  private async release(): Promise<void> {
    this.generation += 1
    if (this.timer >= 0) clearTimeout(this.timer)
    this.timer = -1
    const engine = this.engine; this.engine = null
    const capturer = this.capturer; this.capturer = null
    try { engine?.cancel(this.sessionId) } catch (_) {}
    try { engine?.shutdown() } catch (_) {}
    if (capturer !== null) { await capturer.stop().catch(() => {}); await capturer.release().catch(() => {}) }
  }
}
