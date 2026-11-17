import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import QuickPairForm, { quickPairUrl } from './QuickPairForm'
import * as history from '../native/recentPairingAddresses'
class Socket {
 static OPEN=1;static last:Socket;readyState=1;sent:string[]=[]
 onopen:(()=>void)|null=null;onmessage:((e:{data:string})=>void)|null=null;onclose:(()=>void)|null=null;onerror:(()=>void)|null=null
 constructor(){Socket.last=this} send(s:string){this.sent.push(s)} close(){} receive(m:object){act(()=>this.onmessage?.({data:JSON.stringify(m)}))}
}
afterEach(()=>vi.unstubAllGlobals())
test('default port and IPv6, reject URL injection',()=>{expect(quickPairUrl('10.0.0.1').href).toBe('ws://10.0.0.1:7330/quick-pair/v1');expect(quickPairUrl('[::1]:7330').hostname).toBe('[::1]');expect(()=>quickPairUrl('host/path')).toThrow()})
test('six digit code yields existing pairing with reachable host',()=>{
 vi.stubGlobal('WebSocket',Socket);const submit=vi.fn();render(<QuickPairForm onPair={submit}/>);
 fireEvent.change(screen.getByRole('textbox', { name: 'PC IP' }),{target:{value:'10.0.0.2'}});fireEvent.click(screen.getByText('连接'));
 const socket=Socket.last;act(()=>socket.onopen?.());expect(JSON.parse(socket.sent[0]!).type).toBe('begin');
 socket.receive({version:1,type:'ready',requestId:'request',payload:{expiresAt:new Date(Date.now()+120000).toISOString()}});
 fireEvent.change(screen.getByRole('textbox', { name: '六位验证码' }),{target:{value:'001234'}});fireEvent.click(screen.getByText('验证并连接'));expect(JSON.parse(socket.sent[1]!).payload.code).toBe('001234');
 socket.receive({version:1,type:'paired',requestId:'request',payload:{version:1,sessionId:'fixture',pairToken:'fixture',wsUrl:'ws://192.168.1.1:7332/ws',expiresAt:'2099-01-01T00:00:00Z'}});
 expect(JSON.parse(submit.mock.calls[0]![0]).wsUrl).toBe('ws://10.0.0.2:7332/ws');
})
test('iOS pasted legacy pairing displays only IP and uses old pairing, editing clears credential',async()=>{
 const submit=vi.fn();render(<QuickPairForm iosDesign onPair={submit}/>);const input=screen.getByRole('textbox',{name:'PC IP'});
 const pairing=JSON.stringify({version:1,sessionId:'fixture',pairToken:'secret',wsUrl:'ws://10.0.0.2:7332/ws',expiresAt:'2099-01-01T00:00:00Z'});
 fireEvent.paste(input,{clipboardData:{getData:()=>pairing}});expect(input).toHaveValue('10.0.0.2');expect(document.body.textContent).not.toContain('secret');fireEvent.click(screen.getByText('连接'));expect(submit).toHaveBeenCalledWith(pairing);
 fireEvent.change(input,{target:{value:'invalid'}});fireEvent.click(screen.getByText('连接'));expect(submit).toHaveBeenCalledTimes(1);expect(await screen.findByText(/请输入有效的 IP 地址/)).toHaveTextContent('有效');
})
test('iOS pasted IP starts quick pairing and invalid paste reports error; scan unchanged',async()=>{
 vi.stubGlobal('WebSocket',Socket);const scan=vi.fn();render(<QuickPairForm iosDesign onPair={vi.fn()} onScan={scan}/>);const input=screen.getByRole('textbox',{name:'PC IP'});
 fireEvent.paste(input,{clipboardData:{getData:()=> 'hello'}});expect(await screen.findByText(/请输入有效的 IP 地址/)).toHaveTextContent('有效');
 fireEvent.paste(input,{clipboardData:{getData:()=> '10.0.0.2'}});fireEvent.click(screen.getByText('扫描二维码'));expect(scan).toHaveBeenCalledOnce();fireEvent.click(screen.getByText('连接'));expect(Socket.last).toBeDefined();
})

test('iOS connection shows blocking progress and cancel',async()=>{
 vi.stubGlobal('WebSocket',Socket); render(<QuickPairForm iosDesign onPair={vi.fn()}/>);
 fireEvent.change(screen.getByRole('textbox',{name:'PC IP'}),{target:{value:'10.0.0.2'}});fireEvent.click(screen.getByText('连接'));
 expect(await screen.findByText('正在连接…')).toHaveTextContent('正在连接');fireEvent.click(screen.getByText('取消'));expect(screen.getByRole('textbox',{name:'PC IP'})).toBeEnabled();
})
test('iOS connection failure shows inline feedback',async()=>{
 vi.stubGlobal('WebSocket',Socket);render(<QuickPairForm iosDesign onPair={vi.fn()}/>);
 fireEvent.change(screen.getByRole('textbox',{name:'PC IP'}),{target:{value:'10.0.0.2'}});fireEvent.click(screen.getByText('连接'));act(()=>Socket.last.onerror?.());
 expect(await screen.findByText(/连接失败/)).toBeInTheDocument();expect(screen.queryByText('确定')).not.toBeInTheDocument();
})

test('iOS verification opens a cancellable dialog',()=>{
 vi.stubGlobal('WebSocket',Socket);render(<QuickPairForm iosDesign onPair={vi.fn()}/>);fireEvent.change(screen.getByRole('textbox',{name:'PC IP'}),{target:{value:'10.0.0.2'}});fireEvent.click(screen.getByText('连接'));Socket.last.receive({version:1,type:'ready',requestId:'request',payload:{expiresAt:new Date(Date.now()+120000).toISOString()}});expect(screen.getByRole('dialog',{name:'输入验证码'})).toBeInTheDocument();fireEvent.click(screen.getByRole('dialog',{name:'输入验证码'}).querySelector('button')!);expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
})

test('地址建议按输入过滤，选择后填入并关闭，失焦也关闭', async () => {
 const read = vi.spyOn(history, 'readRecentPairingAddresses').mockResolvedValue(['192.168.1.1', '10.0.0.2:7331'])
 render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
 const input = screen.getByRole('textbox', { name: 'PC IP' })
 fireEvent.focus(input)
 await waitFor(() => expect(screen.getAllByRole('option')).toHaveLength(2))
 fireEvent.change(input, { target: { value: '10.' } })
 expect(screen.getAllByRole('option')).toHaveLength(1)
 fireEvent.click(screen.getByRole('option', { name: '10.0.0.2:7331' }))
 expect(input).toHaveValue('10.0.0.2:7331')
 expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
 fireEvent.focus(input)
 expect(screen.getByRole('listbox')).toBeInTheDocument()
 fireEvent.blur(input)
 await waitFor(() => expect(screen.queryByRole('listbox')).not.toBeInTheDocument())
 read.mockRestore()
})

test('键盘 input 事件逐次筛选，删除扩大范围，清空恢复全部，无匹配隐藏', async () => {
 const read = vi.spyOn(history, 'readRecentPairingAddresses').mockResolvedValue(['192.168.1.1', '192.168.2.1', '10.0.0.2:7331'])
 try {
  render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
  const input = screen.getByRole('textbox', { name: 'PC IP' })
  fireEvent.focus(input)
  await waitFor(() => expect(screen.getAllByRole('option')).toHaveLength(3))
  const options = () => screen.queryAllByRole('option').map(item => item.textContent)
  for (const value of ['19', '192.', '192.168.']) {
   fireEvent.input(input, { target: { value }, inputType: 'insertText' })
   expect(options()).toEqual(['192.168.1.1', '192.168.2.1'])
  }
  fireEvent.input(input, { target: { value: '192.168.2' }, inputType: 'insertText' })
  expect(options()).toEqual(['192.168.2.1'])
  fireEvent.input(input, { target: { value: '192.168.' }, inputType: 'deleteContentBackward' })
  expect(options()).toHaveLength(2)
  fireEvent.input(input, { target: { value: '172.' }, inputType: 'insertReplacementText' })
  expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  fireEvent.input(input, { target: { value: '' }, inputType: 'deleteContentBackward' })
  expect(options()).toHaveLength(3)
 } finally { read.mockRestore() }
})

test('组合输入期间按可见值筛选，异步历史加载后仍使用最新输入', async () => {
 let resolveRead!: (addresses: string[]) => void
 const read = vi.spyOn(history, 'readRecentPairingAddresses').mockImplementation(() => new Promise(resolve => { resolveRead = resolve }))
 try {
  render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
  const input = screen.getByRole('textbox', { name: 'PC IP' }) as HTMLInputElement
  fireEvent.focus(input)
  fireEvent.compositionStart(input)
  // 原生值已改变时也应由 input 事件同步，不依赖额外的 change 通知。
  input.value = '10.'
  fireEvent.input(input, { inputType: 'insertCompositionText', isComposing: true })
  await act(async () => resolveRead(['192.168.1.1', '10.0.0.2:7331']))
  expect(screen.getAllByRole('option').map(item => item.textContent)).toEqual(['10.0.0.2:7331'])
  fireEvent.compositionEnd(input)
  expect(input).toHaveValue('10.')
 } finally { read.mockRestore() }
})

test('首次输入有效地址后失焦即可保存，无需连接；重开页面可恢复并筛选', async () => {
 let saved: string[] = []
 const read = vi.spyOn(history, 'readRecentPairingAddresses').mockImplementation(async () => saved)
 const write = vi.spyOn(history, 'rememberPairingAddress').mockImplementation(async (address) => { saved = [address] })
 const view = render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
 const input = screen.getByRole('textbox', { name: 'PC IP' })
 fireEvent.focus(input)
 fireEvent.change(input, { target: { value: '192.168.1.23' } })
 fireEvent.blur(input)
 await waitFor(() => expect(write).toHaveBeenCalledWith('192.168.1.23'))
 view.unmount()
 render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
 const restored = screen.getByRole('textbox', { name: 'PC IP' })
 fireEvent.focus(restored)
 expect(await screen.findByRole('option', { name: '192.168.1.23' })).toBeInTheDocument()
 fireEvent.change(restored, { target: { value: '192.168' } })
 expect(screen.getByRole('option', { name: '192.168.1.23' })).toBeInTheDocument()
 read.mockRestore(); write.mockRestore()
})

test('输入框先失焦后点击建议仍能回填', async () => {
 const read = vi.spyOn(history, 'readRecentPairingAddresses').mockResolvedValue(['10.0.0.2'])
 render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
 const input = screen.getByRole('textbox', {name: 'PC IP'})
 fireEvent.focus(input)
 const option = await screen.findByRole('option', {name: '10.0.0.2'})
 fireEvent.blur(input)
 fireEvent.click(option)
 expect(input).toHaveValue('10.0.0.2')
 read.mockRestore()
})

test('清空历史后失焦不会重新保存当前地址', async () => {
 const read = vi.spyOn(history, 'readRecentPairingAddresses').mockResolvedValue(['10.0.0.2'])
 const clear = vi.spyOn(history, 'clearRecentPairingAddresses').mockResolvedValue()
 const write = vi.spyOn(history, 'rememberPairingAddress').mockResolvedValue()
 render(<QuickPairForm iosDesign onPair={vi.fn()}/>)
 const input = screen.getByRole('textbox', {name: 'PC IP'})
 fireEvent.focus(input)
 await screen.findByRole('option', {name: '10.0.0.2'})
 fireEvent.change(input, {target: {value: '10.0.0.2'}})
 fireEvent.click(screen.getByRole('button', {name: '清空历史记录'}))
 await waitFor(() => expect(clear).toHaveBeenCalledOnce())
 fireEvent.blur(input)
 expect(write).not.toHaveBeenCalled()
 read.mockRestore(); clear.mockRestore(); write.mockRestore()
})
