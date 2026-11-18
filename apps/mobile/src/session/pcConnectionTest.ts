import type { SocketFactory, SocketLike } from '../protocol/client'
import { replacePCIP } from '../native/pcProfiles'

export interface PCConnectionResult { sessionId: string; ok: boolean; port: string; token: string; message: string }
export function testPCConnection(ip: string, factory: SocketFactory, signal: AbortSignal): Promise<PCConnectionResult> {
  const url = replacePCIP('ws://127.0.0.1:7330/quick-pair/v1', ip)
  return new Promise(resolve => {
    let socket: SocketLike | undefined
    let done = false
    const finish = (ok: boolean, message: string) => {
      if (done) return
      done = true
      clearTimeout(timeout)
      signal.removeEventListener('abort', abort)
      if (socket) { socket.onopen = socket.onerror = socket.onclose = null; socket.close() }
      resolve({ sessionId: '', ok, port: '7330', token: '', message })
    }
    const abort = () => finish(false, '测试已取消')
    const timeout = setTimeout(() => finish(false, '连接超时，请检查 PC 服务和网络'), 3000)
    signal.addEventListener('abort', abort, { once: true })
    if (signal.aborted) { abort(); return }
    try {
      socket = factory(url)
      socket.onopen = () => finish(true, 'PC 服务连接成功')
      socket.onerror = socket.onclose = () => finish(false, '连接失败，请检查 IP、PC 服务和网络')
    } catch { finish(false, '连接失败，请检查 IP、PC 服务和网络') }
  })
}
