import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'
import { PCEditor, PCList, PCConnectionFeedback, type PCFeedback } from './PCManagement'
import PairingForm from './PairingForm'
import { createInitialState } from '../session/reducer'
const session = { sessionId: 'a', pcId: 'pc', pcName: 'Mac', wsUrl: 'ws://10.0.0.1:8765/ws', pairToken: 'fixture', expiresAt: '2099-01-01T00:00:00Z', name: 'a', lastConnectedAt: null, status: 'connected' as const, runtimeState: { ...createInitialState(), connection: 'disconnected' as const } }
test('PC 目录只展示末级名称，兼容 Windows、UNC、末尾分隔符和 Unix 路径', () => {
  const paths = ['/Users/test/run1/', 'C:\\Users\\test\\code\\whale-term', '\\\\server\\share\\folder\\', 'C:\\Users\\test\\AppData\\releases\\712bffe6fe4944d68fed7e9c6fae0dcf', '/']
  render(<PCList sessions={paths.map((projectCwd, index) => ({ ...session, sessionId: `${index}`, runtimeState: { ...session.runtimeState, projectCwd } }))} onSelect={vi.fn()} onEdit={vi.fn()} onRemove={vi.fn()} onTest={vi.fn()}/> )
  for (const name of ['run1', 'whale-term', 'folder', '712bffe6fe4944d68fed7e9c6fae0dcf', '/']) expect(screen.getByText(name)).toBeInTheDocument()
  expect(screen.queryByText(paths[1]!)).not.toBeInTheDocument()
})
test('长列表的测试摘要在固定操作上方，详情自动打开且可关闭后重开', async () => {
  let finish!: (results: { sessionId: string; ok: boolean; message: string; port: string; token: string }[]) => void
  const probe = vi.fn(() => new Promise<Parameters<typeof finish>[0]>(resolve => { finish = resolve }))
  function Page() {
    const [feedback, setFeedback] = useState<PCFeedback | null>(null)
    return <PairingForm iosDesign initialError={null} projectCwd="/work/target" onSubmit={vi.fn()} onScan={vi.fn()} pcFeedback={<PCConnectionFeedback feedback={feedback}/>} pcList={<PCList sessions={Array.from({ length: 30 }, (_, index) => ({ ...session, pcId: `${index}`, pcName: `PC ${index}`, sessionId: `${index}` }))} onSelect={vi.fn()} onEdit={vi.fn()} onRemove={vi.fn()} onTest={probe} onFeedback={setFeedback}/>}/>
  }
  render(<Page/>)
  fireEvent.click(screen.getByRole('button', { name: '测试连接 PC 0' }))
  const footer = document.querySelector('.quick-pair-ios__footer') as HTMLElement
  expect(within(footer).queryByText('正在测试 PC 0…')).not.toBeInTheDocument()
  expect(within(footer).queryByText('/work/target')).not.toBeInTheDocument()
  finish([{ sessionId: 'a', ok: true, message: '认证成功', port: '8765', token: '••••1234' }, { sessionId: 'b', ok: false, message: '连接失败', port: '8766', token: '••••5678' }])
  const dialog = await screen.findByRole('dialog', { name: 'PC 0 · 连接测试' })
  expect(within(dialog).getByText('端口：8765')).toBeInTheDocument()
  expect(within(dialog).getByText('连接失败')).toBeInTheDocument()
  expect(within(footer).getByRole('status')).toHaveTextContent('PC 0：连接失败')
  expect(document.querySelector('.pairing-pc-list')).not.toContainElement(within(footer).getByRole('status'))
  fireEvent.click(within(dialog).getByRole('button', { name: '关闭' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  fireEvent.click(within(footer).getByRole('button', { name: '查看详情' }))
  expect(await screen.findByRole('dialog', { name: 'PC 0 · 连接测试' })).toBeInTheDocument()
})
test('离页取消测试，不发布迟到的结果', async () => {
  let finish!: (results: []) => void
  const feedback = vi.fn()
  const probe = vi.fn((_pc: unknown, _ip: string, _signal: AbortSignal) => new Promise<[]>(resolve => { finish = resolve }))
  const view = render(<PCList sessions={[session]} onSelect={vi.fn()} onEdit={vi.fn()} onRemove={vi.fn()} onTest={probe} onFeedback={feedback}/> )
  fireEvent.click(screen.getByRole('button', { name: '测试连接 Mac' }))
  view.unmount()
  expect(probe.mock.calls[0]?.[2].aborted).toBe(true)
  finish([])
  await Promise.resolve()
  expect(feedback.mock.calls.map(([value]) => value?.testing)).toEqual([true, undefined])
})
test('测试异常显示可重试的摘要，不回显异常内容', async () => {
  render(<PCList sessions={[session]} onSelect={vi.fn()} onEdit={vi.fn()} onRemove={vi.fn()} onTest={vi.fn(async () => { throw new Error('private-transport-detail') })}/> )
  fireEvent.click(screen.getByRole('button', { name: '测试连接 Mac' }))
  expect(await screen.findByText('测试未完成，请重试')).toBeInTheDocument()
  expect(screen.getByRole('status')).toHaveTextContent('Mac：连接失败')
  expect(document.body.textContent).not.toContain('private-transport-detail')
})
test('PC 项点击切换、测试不切换，实时状态跟随会话', async () => {
  const select = vi.fn(), probe = vi.fn(async () => [{ sessionId: '', ok: true, port: '7330', token: '', message: 'PC 服务连接成功' }])
  const props = { sessions: [session], onSelect: select, onEdit: vi.fn(), onRemove: vi.fn(), onTest: probe }
  const view = render(<PCList {...props}/>)
  expect(screen.getByText('不可用')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '测试连接 Mac' }))
  await waitFor(() => expect(screen.getByText('可用')).toBeInTheDocument())
  expect(select).not.toHaveBeenCalled()
  fireEvent.click(screen.getByText('Mac'))
  expect(select).toHaveBeenCalledWith('pc:pc')
  view.rerender(<PCList {...props} sessions={[{ ...session, runtimeState: { ...session.runtimeState, connection: 'connected' } }]}/> )
  expect(screen.getByText('可用')).toBeInTheDocument()
})
test('编辑必填、检测输入 IP、修改后清空结果、确认才保存', async () => {
  const save = vi.fn(async () => {}), probe = vi.fn(async (_pc: unknown, _ip: string, _signal: AbortSignal) => [{ sessionId: 'a', ok: true, port: '8765', token: '••••', message: '认证成功' }])
  render(<PCEditor pc={{ key: 'pc:pc', label: 'Mac', legacy: false, sessions: [session] }} onBack={vi.fn()} onSave={save} onTest={probe}/>)
  fireEvent.change(screen.getByRole('textbox', { name: 'PC 名称' }), { target: { value: '' } })
  fireEvent.click(screen.getByText('确认')); expect(screen.getByRole('alert')).toHaveTextContent('请输入 PC 名称')
  fireEvent.change(screen.getByRole('textbox', { name: 'PC 名称' }), { target: { value: 'Work' } })
  fireEvent.change(screen.getByRole('textbox', { name: 'PC IP地址' }), { target: { value: '10.0.0.2' } })
  fireEvent.click(screen.getByText('测试连接'))
  await screen.findByText('认证成功')
  expect(probe.mock.calls[0]?.[1]).toBe('10.0.0.2')
  expect(save).not.toHaveBeenCalled()
  fireEvent.change(screen.getByRole('textbox', { name: 'PC IP地址' }), { target: { value: '10.0.0.3' } })
  expect(screen.queryByText('认证成功')).not.toBeInTheDocument()
  fireEvent.click(screen.getByText('确认'))
  await waitFor(() => expect(save).toHaveBeenCalledWith({ name: 'Work', ip: '10.0.0.3' }))
})


test('退出中的配对页面不会自动重新打开测试抽屉', async () => {
  render(<div inert><PCConnectionFeedback feedback={{ pcName: 'Mac', ip: '10.0.0.1', testing: false, results: [{ sessionId: '', ok: true, port: '7330', token: '', message: 'PC 服务连接成功' }] }}/></div>)
  await Promise.resolve()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('最新的本地错误替换 PC 检测摘要', async () => {
  const result = { sessionId: '', ok: true, port: '7330', token: '', message: 'PC 服务连接成功' }
  function Page() {
    const [feedback, setFeedback] = useState<PCFeedback | null>(null)
    return <PairingForm iosDesign initialError={null} onSubmit={vi.fn()} pcFeedback={<PCConnectionFeedback feedback={feedback}/>} pcList={<PCList sessions={[session]} onSelect={vi.fn()} onEdit={vi.fn()} onRemove={vi.fn()} onTest={vi.fn(async () => [result])} onFeedback={setFeedback}/>}/>
  }
  render(<Page/>)
  fireEvent.click(screen.getByRole('button', { name: '测试连接 Mac' }))
  const dialog = await screen.findByRole('dialog', { name: 'Mac · 连接测试' })
  expect(within(dialog).getByText('IP：10.0.0.1')).toBeInTheDocument()
  expect(within(dialog).getByText('端口：7330')).toBeInTheDocument()
  fireEvent.click(within(dialog).getByRole('button', { name: '关闭' }))
  fireEvent.click(screen.getByRole('button', { name: '连接' }))
  expect(screen.getByText('请输入有效的 IP 地址或配对信息')).toBeInTheDocument()
  expect(screen.queryByText('Mac：连接成功')).not.toBeInTheDocument()
})

test('刷新期间显示 loading，完成后只更新电脑状态', async () => {
  let finish!: (value: { sessionId: string; ok: boolean; port: string; token: string; message: string }[]) => void
  const onFeedback = vi.fn()
  const onTest = vi.fn(() => new Promise<Parameters<typeof finish>[0]>(resolve => { finish = resolve }))
  render(<PCList sessions={[session]} onSelect={vi.fn()} onEdit={vi.fn()} onRemove={vi.fn()} onTest={onTest} onFeedback={onFeedback}/> )
  const refresh = screen.getByRole('button', { name: '刷新电脑状态' })
  fireEvent.click(refresh)
  expect(refresh).toBeDisabled()
  expect(refresh.querySelector('.pc-testing')).not.toBeNull()
  finish([{ sessionId: '', ok: true, port: '7330', token: '', message: 'PC 服务连接成功' }])
  await waitFor(() => expect(refresh).toBeEnabled())
  expect(screen.getByText('可用')).toBeInTheDocument()
  expect(onFeedback).not.toHaveBeenCalled()
})

test('进入配对列表时自动批量检测，只在刷新按钮显示加载', async () => {
  let finish!: (results: { sessionId: string; ok: boolean; message: string; port: string; token: string }[]) => void
  const probe = vi.fn(() => new Promise<Parameters<typeof finish>[0]>(resolve => { finish = resolve }))
  const props = { refreshOnEntry: true, sessions: [session], onSelect: vi.fn(), onEdit: vi.fn(), onRemove: vi.fn(), onTest: probe }
  const view = render(<PCList {...props} disabled/>)
  expect(probe).not.toHaveBeenCalled()
  view.rerender(<PCList {...props}/>)
  expect(probe).toHaveBeenCalledTimes(1)
  expect(screen.getByRole('button', { name: '刷新电脑状态' }).querySelector('.pc-testing')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '测试连接 Mac' }).querySelector('.pc-testing')).toBeNull()
  finish([{ sessionId: '', ok: true, message: '连接成功', port: '7330', token: '' }])
  expect(await screen.findByText('可用')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(probe).toHaveBeenCalledTimes(1)
})
