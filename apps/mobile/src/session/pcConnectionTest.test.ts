import { expect, test, vi } from 'vitest'
import { testPCConnection } from './pcConnectionTest'
import type { SocketLike } from '../protocol/client'
function fixture() {
  const socket: SocketLike = { onopen: null, onerror: null, onclose: null, onmessage: null, send: vi.fn(), close: vi.fn() }
  return { socket, factory: vi.fn(() => socket) }
}
test('只连接 PC 固定入口，成功后关闭且不发送配对或凭证', async () => {
  const {socket, factory} = fixture()
  const result = testPCConnection('10.0.0.2', factory, new AbortController().signal)
  expect(factory).toHaveBeenCalledWith('ws://10.0.0.2:7330/quick-pair/v1')
  socket.onopen?.()
  expect(await result).toMatchObject({ok: true, message: 'PC 服务连接成功'})
  expect(socket.send).not.toHaveBeenCalled()
  expect(socket.close).toHaveBeenCalledOnce()
})
test('超时和取消均及时释放连接', async () => {
  vi.useFakeTimers()
  const {socket, factory} = fixture()
  const controller = new AbortController()
  const cancelled = testPCConnection('10.0.0.2', factory, controller.signal)
  controller.abort()
  expect(await cancelled).toMatchObject({message: '测试已取消'})
  const timed = testPCConnection('10.0.0.2', factory, new AbortController().signal)
  await vi.advanceTimersByTimeAsync(3000)
  expect(await timed).toMatchObject({ok: false, message: expect.stringContaining('超时')})
  expect(socket.close).toHaveBeenCalledTimes(2)
  vi.useRealTimers()
})
