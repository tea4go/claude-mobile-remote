import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import QuickPairForm, { quickPairUrl } from './QuickPairForm'
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
test('iOS connection failure shows toast',async()=>{
 vi.stubGlobal('WebSocket',Socket);render(<QuickPairForm iosDesign onPair={vi.fn()}/>);
 fireEvent.change(screen.getByRole('textbox',{name:'PC IP'}),{target:{value:'10.0.0.2'}});fireEvent.click(screen.getByText('连接'));act(()=>Socket.last.onerror?.());
 expect(await screen.findByText(/连接失败/)).toBeInTheDocument();expect(screen.queryByText('确定')).not.toBeInTheDocument();
})

test('iOS verification opens a cancellable dialog',()=>{
 vi.stubGlobal('WebSocket',Socket);render(<QuickPairForm iosDesign onPair={vi.fn()}/>);fireEvent.change(screen.getByRole('textbox',{name:'PC IP'}),{target:{value:'10.0.0.2'}});fireEvent.click(screen.getByText('连接'));Socket.last.receive({version:1,type:'ready',requestId:'request',payload:{expiresAt:new Date(Date.now()+120000).toISOString()}});expect(screen.getByRole('dialog',{name:'输入验证码'})).toBeInTheDocument();fireEvent.click(screen.getByRole('dialog',{name:'输入验证码'}).querySelector('button')!);expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
})
