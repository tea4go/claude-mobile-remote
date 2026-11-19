import { useEffect, useRef, useState, isValidElement, type ReactNode } from 'react'
import { Button, Input } from 'antd-mobile'
import { parsePairingInput, parsePairingJson } from '../protocol/pairing'
import { nativeExperience } from '../native/experience'
import pasteIcon from '../assets/icons/svgs/session/paste_pari.svg'
import SessionIcon from './SessionIcon'
import { PairingCodeDialog, PairingLoading } from './PairingFeedback'
import { clearRecentPairingAddresses, readRecentPairingAddresses, rememberPairingAddress, validPairingAddress } from '../native/recentPairingAddresses'

export function quickPairUrl(input: string): URL {
  const text = input.trim()
  if (!text || /[/?#@\s]/.test(text)) throw new Error('请输入 PC IP，可附带端口')
  const url = new URL(`ws://${text}`)
  if (!url.port) url.port = '7330'
  url.pathname = '/quick-pair/v1'
  return url
}
const unavailable = '连接失败，请检查 PC 服务和网络'
export default function QuickPairForm({ onPair, disabled = false, iosDesign = false, onScan, feedback, pcFeedback, onDismissFeedback, pcList, externalFeedbackKey }: { externalFeedbackKey?: string | null; pcList?: ReactNode; pcFeedback?: ReactNode; onDismissFeedback?: () => void; feedback?: ReactNode; onPair: (text: string) => void; disabled?: boolean; iosDesign?: boolean; onScan?: () => void }) {
  const [ip,setIP]=useState(''), [code,setCode]=useState(''), [phase,setPhase]=useState('idle'), [error,setError]=useState('')
  const [toast, setToast] = useState('')
  const [latest, setLatest] = useState<'local' | 'pc' | 'external'>('external')
  const pcValue = isValidElement<{ feedback?: unknown }>(pcFeedback) ? pcFeedback.props.feedback : null
  useEffect(() => { if (pcValue) setLatest('pc') }, [pcValue])
  useEffect(() => { if (error || toast) setLatest('local') }, [error, toast])
  useEffect(() => { if (externalFeedbackKey) setLatest('external') }, [externalFeedbackKey])
  const [recent, setRecent] = useState<string[]>([])
  const [focused, setFocused] = useState(false)
  const legacy=useRef<string|null>(null)
  const blurTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const suppressRemember = useRef(false)
  const touchingSuggestions = useRef(false)
  const matchingAddresses = recent.filter(address => address.toLowerCase().includes(ip.trim().toLowerCase()))
  const socket=useRef<WebSocket|null>(null), timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined), request=useRef('')
  function rememberAddress(value: string) {
    const address = value.trim()
    if (suppressRemember.current || legacy.current || !validPairingAddress(address)) return
    void rememberPairingAddress(address).then(() => readRecentPairingAddresses()).then(setRecent).catch(() => {})
  }
  function stop() { clearTimeout(timer.current);const s=socket.current;socket.current=null;s?.close();request.current='';setPhase('idle') }
  useEffect(()=>{if(disabled)stop()},[disabled])
  useEffect(()=>()=>{clearTimeout(timer.current);clearTimeout(blurTimer.current);socket.current?.close();socket.current=null},[])
  useEffect(() => { let active = true; void readRecentPairingAddresses().then((items) => { if (active) setRecent(items) }).catch(() => {}); return () => { active = false } }, [])
  useEffect(() => {
    if (!iosDesign || !focused || !validPairingAddress(ip.trim()) || legacy.current) return
    const timeout = setTimeout(() => rememberAddress(ip), 1000)
    return () => clearTimeout(timeout)
  }, [ip, focused, iosDesign])
  useEffect(()=>{const hide=()=>{if(document.hidden)stop()};document.addEventListener('visibilitychange',hide);return()=>document.removeEventListener('visibilitychange',hide)},[])
  function validIP(value: string): boolean {
    return validPairingAddress(value)
  }
  function editAddress(value: string) {
    suppressRemember.current = false
    legacy.current = null
    setIP(value)
    setError('')
    setToast('')
    onDismissFeedback?.()
  }
  function acceptInput(text: string) {
    const value = text.trim(), parsed = parsePairingInput(value, Date.now())
    if (parsed.ok) { legacy.current = value; setIP(new URL(parsed.value.wsUrl).hostname); setError(''); return }
    legacy.current = null
    if (validIP(value)) { setIP(value); setError('') }
    else { setIP(''); if (parsed.error === '配对凭证已过期，请重新配对') setError(parsed.error); else { setLatest('local'); setToast('请输入有效的 IP 地址或配对信息') } }
  }
  async function paste() {
    try { const text = nativeExperience?.readPasteboard ? (await nativeExperience.readPasteboard()).text : await navigator.clipboard.readText(); acceptInput(text) }
    catch { setError('无法读取剪贴板，请使用系统粘贴') }
  }
  function start() {
    stop();setError('');setCode('')
    if (iosDesign && legacy.current) { onPair(legacy.current); return }
    if (iosDesign && !validIP(ip)) { setLatest('local'); setToast('请输入有效的 IP 地址或配对信息'); return }
    let url:URL;try{url=quickPairUrl(ip)}catch{setError('请输入有效 PC IP，可附带端口');return}
    let s:WebSocket;try{s=new WebSocket(url)}catch{setError(unavailable);return}
    socket.current=s;setPhase('connecting')
    const fail=(message:string)=>{if(socket.current!==s)return;stop();setError(message)}
    timer.current=setTimeout(()=>fail(unavailable),8000)
    s.onopen=()=>{if(socket.current===s)s.send(JSON.stringify({version:1,type:'begin',requestId:'',payload:{}}))}
    s.onerror=()=>fail(unavailable);s.onclose=()=>fail('配对已结束或连接断开，请重试')
    s.onmessage=event=>{
      if(socket.current!==s)return
      try {
        const m=JSON.parse(event.data)
        if(m.version!==1 || typeof m.type!=='string')throw new Error()
        if(m.type==='ready') {
          if(typeof m.requestId!=='string'||!m.requestId)throw new Error()
          const ttl=Date.parse(m.payload.expiresAt)-Date.now();if(!Number.isFinite(ttl)||ttl<=0||ttl>125000)throw new Error()
          rememberAddress(ip)
          request.current=m.requestId;clearTimeout(timer.current);timer.current=setTimeout(()=>fail('验证码已过期，请重新连接'),ttl);setPhase('code')
        } else if(m.type==='paired') {
          if(!request.current||m.requestId!==request.current)throw new Error()
          const parsed=parsePairingJson(JSON.stringify(m.payload),Date.now());if(!parsed.ok)throw new Error()
          const target=new URL(parsed.value.wsUrl);target.hostname=url.hostname;parsed.value.wsUrl=target.toString()
          stop();onPair(JSON.stringify(parsed.value))
        } else if(m.type==='error') {
          const c=m.payload.code
          if(c==='invalid_code'||c==='target_required'){setError(c==='invalid_code'?'验证码不正确':'请先在 PC 选择会话');setPhase('code')}
          else fail(c==='busy'?'PC 正在处理另一条配对请求，请稍后重试':'配对已失效，请重新连接')
        } else throw new Error()
      } catch { fail('配对响应无效，请重试或扫码') }
    }
  }
  if (iosDesign) return <div className="quick-pair-ios">
    <div className="quick-pair-ios__body">
    <div className="quick-pair-ios__address">
    <div className="quick-pair-ios__input" onPaste={event => { event.preventDefault(); acceptInput(event.clipboardData.getData('text')) }}>
      <input className="quick-pair-ios__ip" aria-label="PC IP" placeholder="请输入IP" type="text" inputMode="url" autoCapitalize="none" autoCorrect="off" autoComplete="off" spellCheck={false} value={ip} onInput={event => editAddress(event.currentTarget.value)} onChange={event => { if (event.currentTarget.value !== ip) editAddress(event.currentTarget.value) }} onCompositionEnd={event => editAddress(event.currentTarget.value)} onFocus={() => { clearTimeout(blurTimer.current); setFocused(true) }} onBlur={event => { rememberAddress(event.currentTarget.value); blurTimer.current = setTimeout(() => { if (!touchingSuggestions.current) setFocused(false) }, 300) }} disabled={phase !== 'idle' || disabled}/>
      <button type="button" aria-label="粘贴配对信息" onClick={() => void paste()} disabled={phase !== 'idle' || disabled}><img src={pasteIcon} alt="" /></button>
    </div>
    {focused && phase === 'idle' && !disabled && matchingAddresses.length > 0 && <div className="quick-pair-ios__suggestions" role="listbox" aria-label="最近连接地址" onTouchStart={() => { touchingSuggestions.current = true }} onTouchEnd={() => { touchingSuggestions.current = false; clearTimeout(blurTimer.current); blurTimer.current = setTimeout(() => setFocused(false), 300) }} onTouchCancel={() => { touchingSuggestions.current = false; setFocused(false) }}>
      <div className="quick-pair-ios__history-header"><span>历史记录</span><button type="button" aria-label="清空历史记录" onMouseDown={event => event.preventDefault()} onClick={() => { suppressRemember.current = true; void clearRecentPairingAddresses().then(() => setRecent([])).catch(() => setError('清空历史记录失败，请重试')) }}>清空</button></div>
      {matchingAddresses.map((address) => <button type="button" role="option" aria-selected="false" key={address} onMouseDown={(event) => event.preventDefault()} onClick={() => { clearTimeout(blurTimer.current); suppressRemember.current = false; legacy.current = null; setIP(address); setError(''); setToast(''); setFocused(false) }}>{address}</button>)}
    </div>}
    </div>
    <p>请与 PC 配对窗口核对；手机所在网络需要能访问此地址。不同路由器共用宽带不代表设备能够互通。</p>
    <div className="quick-pair-ios__pc-scroll">{pcList}</div>
    </div>
    {phase === 'code' && <PairingCodeDialog code={code} error={error} onChange={setCode} onCancel={stop} onVerify={() => { if (socket.current?.readyState === WebSocket.OPEN) { setError(''); setPhase('verifying'); socket.current.send(JSON.stringify({version:1,type:'verify',requestId:request.current,payload:{code}})) } }}/>}
    {(phase === 'connecting' || phase === 'verifying') && <PairingLoading message={phase === 'connecting' ? '正在连接…' : '正在验证…'} onCancel={stop}/>}
    <div className="quick-pair-ios__footer">
    <div className="quick-pair-ios__feedback">
    {latest === 'local' && phase !== 'code' && (error || toast) && <div className="pairing-notice pairing-notice--error" role="alert">{error || toast}</div>}
    {latest === 'pc' && pcFeedback}
    {latest === 'external' && feedback}
    </div>
    <div className="quick-pair-ios__actions" data-debug-bottom-actions>
      <Button className="pairing-form__primary" color="primary" disabled={disabled || phase !== 'idle'} onClick={start}>连接</Button>
      {onScan && <Button className="pairing-form__scan" disabled={disabled || phase !== 'idle'} color="primary" fill="outline" onClick={onScan}><SessionIcon name="scan" size={16} style={{ marginRight: 6 }} />扫描二维码</Button>}
    </div>
    </div>
  </div>
  return <div style={{display:'grid',gap:16}}>
    <Input aria-label="PC IP" placeholder="请输入 PC IP" value={ip} onChange={setIP} disabled={phase!=='idle'||disabled}/>
    {phase==='code' && <><div>请查看 WhaleTerm 上的六位验证码</div><Input aria-label="六位验证码" type="tel" maxLength={6} value={code} onChange={v=>setCode(v.replace(/\D/g,''))}/></>}
    {error && <div role="alert" style={{color:'#c0392b'}}>{error}</div>}
    <Button color="primary" disabled={disabled||phase==='connecting'||phase==='verifying'||(phase==='code'&&code.length!==6)} onClick={()=>{if(phase==='idle')start();else if(socket.current?.readyState===WebSocket.OPEN){setError('');setPhase('verifying');socket.current.send(JSON.stringify({version:1,type:'verify',requestId:request.current,payload:{code}}))}}}>{phase==='connecting'?'正在连接…':phase==='verifying'?'正在验证…':phase==='code'?'验证并连接':'连接'}</Button>
    {phase!=='idle'&&<Button onClick={stop}>取消连接</Button>}
  </div>
}
