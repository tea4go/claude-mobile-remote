import { common } from '@kit.AbilityKit'
import { notificationManager } from '@kit.NotificationKit'
import { cryptoFramework } from '@kit.CryptoArchitectureKit'
import { util } from '@kit.ArkTS'
import { http } from '@kit.NetworkKit'
import { JPushInterface, CallBackMsg, JMessage } from '@jg/push'
import { NativeStorage } from './storage'
import { nativeLifecycle } from './lifecycle'
import type { SessionRecord } from '../session/records'

interface PushRoute { topic: string; session: string; expires: string }
interface AppliedRoute { route: PushRoute; rid: string; confirmed?: boolean }
interface PushState { routes: PushRoute[]; applied: AppliedRoute[] }
interface RegistrationBody { platform: string; topic: string; registrationId: string; expiresAt?: string }

class PushCallbacks extends CallBackMsg {
  private owner: SessionPush
  constructor(owner: SessionPush) { super(); this.owner = owner }
  onRegister(_id: string): void { void this.owner.refresh() }
  onConnected(connected: boolean): void { if (connected) void this.owner.refresh() }
  onClickMessage(message: JMessage): void {
    const topic = message.extras?.['topic']
    if (typeof topic === 'string') this.owner.open(topic)
  }
}

export class SessionPush {
  private readonly context: common.UIAbilityContext
  private readonly storage: NativeStorage
  private routes: PushRoute[] = []
  private applied: AppliedRoute[] = []
  private started: boolean = false
  private starting: boolean = false
  private syncing: boolean = false
  private resyncRequested: boolean = false
  private revision: number = 0
  private retries: number = 0
  private retryTimer: number = -1
  private expiryTimer: number = -1
  private disposed: boolean = false
  private consent: boolean = false
  private notificationsEnabled: boolean = false
  ready: boolean = false
  error: string = ''
  onStatus: () => void = () => {}
  constructor(context: common.UIAbilityContext, storage: NativeStorage) {
    this.context = context; this.storage = storage
  }
  async initialize(): Promise<void> {
    this.consent = await this.storage.readPushConsent()
    this.notificationsEnabled = await this.storage.readNotificationsEnabled()
    try {
      const saved = JSON.parse(await this.storage.readPushState()) as PushState
      if (Array.isArray(saved.routes) && Array.isArray(saved.applied) && saved.routes.length <= 1000 && saved.applied.length <= 2000) {
        this.routes = saved.routes.filter((route: PushRoute) => this.valid(route))
        this.applied = saved.applied.filter((item: AppliedRoute) => this.valid(item.route) && /^[a-zA-Z0-9]{8,64}$/.test(item.rid))
      }
    } catch (_) {}
    this.prune()
  }
  private valid(route: PushRoute): boolean {
    return route !== null && typeof route.topic === 'string' && /^cmr_[0-9a-f]{32}$/.test(route.topic) &&
      typeof route.session === 'string' && route.session.length > 0 && route.session.length <= 256 && Number.isFinite(Date.parse(route.expires))
  }
  get enabled(): boolean { return this.notificationsEnabled && this.consent }
  private save(): void { const state: PushState = { routes: this.routes, applied: this.applied }; void this.storage.writePushState(JSON.stringify(state)).catch(() => { this.error = 'push_storage_failed'; this.onStatus() }) }
  private prune(): void {
    this.routes = this.routes.filter((route: PushRoute) => Date.parse(route.expires) > Date.now())
    if (this.expiryTimer >= 0) clearTimeout(this.expiryTimer)
    if (this.routes.length > 0 && !this.disposed) {
      const next = Math.min(...this.routes.map((route: PushRoute) => Date.parse(route.expires)))
      this.expiryTimer = setTimeout(() => { this.prune(); void this.refresh() }, Math.max(1000, Math.min(86400000, next - Date.now())))
    }
    this.save()
  }
  async syncSessions(records: SessionRecord[]): Promise<void> {
    const revision = ++this.revision
    const next: PushRoute[] = []
    for (const record of records) {
      if (Date.parse(record.expiresAt) <= Date.now() || record.pairToken.length === 0) continue
      const md = cryptoFramework.createMd('SHA256')
      await md.update({ data: new util.TextEncoder().encodeInto('cmr-push-v1\0' + record.sessionId + '\0' + record.pairToken) })
      const hash = await md.digest()
      const topic = 'cmr_' + Array.from(hash.data.slice(0, 16)).map((value: number) => value.toString(16).padStart(2, '0')).join('')
      next.push({ topic, session: record.sessionId, expires: record.expiresAt })
    }
    if (revision !== this.revision || this.disposed) return
    this.routes = next; this.retries = 0; this.prune(); await this.refresh()
  }
  async setEnabled(enabled: boolean): Promise<void> {
    this.consent = enabled
    this.notificationsEnabled = await this.storage.readNotificationsEnabled()
    await this.storage.setPushConsent(enabled)
    this.ready = false; this.retries = 0
    await this.refresh()
  }
  async refresh(): Promise<void> {
    if (this.disposed || this.starting) return
    this.prune(); this.ready = false
    this.notificationsEnabled = await this.storage.readNotificationsEnabled()
    const allowed = await notificationManager.isNotificationEnabled()
    if (this.enabled && allowed && !this.started) {
      this.starting = true
      try {
        JPushInterface.setDebug(false)
        JPushInterface.setAppKey('ade0811aea90ae3052eb7397')
        JPushInterface.setChannel('cmr')
        JPushInterface.setCallBackMsg(new PushCallbacks(this))
        await JPushInterface.init(this.context)
        this.started = true
      } catch (_) { this.error = 'push_registration_failed'; this.onStatus(); return }
      finally { this.starting = false }
    }
    if (this.started && this.enabled && allowed) {
      await JPushInterface.turnOnPush(this.context)
      if (nativeLifecycle.pendingPushWant !== null) {
        const want = nativeLifecycle.pendingPushWant; nativeLifecycle.pendingPushWant = null
        await JPushInterface.setClickWant(want, this.context.getApplicationContext()).catch(() => undefined)
      }
    } else if (this.started) await JPushInterface.turnOffPush(this.context).catch(() => {})
    await this.reconcile(this.started && this.enabled && allowed ? await JPushInterface.getRegistrationId() : '')
  }
  private async reconcile(rid: string): Promise<void> {
    if (this.disposed) return
    if (this.syncing) { this.resyncRequested = true; return }
    this.syncing = true
    try {
      // Re-read desired state after every await so deletion/off cannot leave a late PUT active.
      while (!this.disposed) {
        const activeID = this.enabled && await notificationManager.isNotificationEnabled() ? rid : ''
        const stale = this.applied.find((item: AppliedRoute) => item.rid !== activeID || !this.routes.some((route: PushRoute) =>
          route.topic === item.route.topic && route.expires === item.route.expires && Date.parse(route.expires) > Date.now()))
        const changed = activeID.length === 0 ? undefined : this.routes.find((route: PushRoute) => Date.parse(route.expires) > Date.now() &&
          !this.applied.some((item: AppliedRoute) => item.rid === activeID && item.route.topic === route.topic && item.route.expires === route.expires && item.confirmed === true))
        if (stale === undefined && changed === undefined) { this.ready = activeID.length > 0; this.error = ''; break }
        if (stale !== undefined) {
          await this.send('DELETE', stale)
          this.applied = this.applied.filter((item: AppliedRoute) => item !== stale)
        } else if (changed !== undefined) {
          let value = this.applied.find((item: AppliedRoute) => item.rid === activeID && item.route.topic === changed.topic && item.route.expires === changed.expires)
          if (value === undefined) {
            value = { route: changed, rid: activeID, confirmed: false }; this.applied.push(value)
            const pending: PushState = { routes: this.routes, applied: this.applied }
            await this.storage.writePushState(JSON.stringify(pending))
          }
          await this.send('PUT', value)
          value.confirmed = true
        }
        this.save(); this.retries = 0
      }
    } catch (_) {
      this.error = 'push_subscription_failed'
      if (this.retries++ < 3 && !this.disposed) {
        if (this.retryTimer >= 0) clearTimeout(this.retryTimer)
        this.retryTimer = setTimeout(() => { void this.refresh() }, 2000)
      }
    } finally {
      this.syncing = false; this.onStatus()
      if (this.resyncRequested && !this.disposed) { this.resyncRequested = false; void this.refresh() }
    }
  }
  private async send(method: string, item: AppliedRoute): Promise<void> {
    const body: RegistrationBody = { platform: 'hmos', topic: item.route.topic, registrationId: item.rid }
    if (method === 'PUT') body.expiresAt = item.route.expires
    const request = http.createHttp()
    try {
      const response = await request.request('https://edo.iwhalecloud.com/verfiles/notify/v1/events', {
        method: method === 'PUT' ? http.RequestMethod.PUT : http.RequestMethod.DELETE,
        header: { 'Content-Type': 'application/json' }, extraData: JSON.stringify(body),
        connectTimeout: 10000, readTimeout: 10000, maxRedirects: 0,
      })
      if (response.responseCode !== 204) throw new Error('push_subscription_failed')
    } finally { request.destroy() }
  }
  session(topic: string): string {
    if (!this.enabled) return ''
    return this.routes.find((route: PushRoute) => route.topic === topic && Date.parse(route.expires) > Date.now())?.session ?? ''
  }
  open(topic: string): void {
    const id = this.session(topic)
    if (id.length > 0) nativeLifecycle.open({ parameters: { cmrSessionId: id } })
  }
  remove(id: string): void {
    this.revision++
    this.routes = this.routes.filter((route: PushRoute) => route.session !== id); this.save(); void this.refresh()
  }
  dispose(): void { this.disposed = true; clearTimeout(this.retryTimer); clearTimeout(this.expiryTimer); this.onStatus = () => {} }
}
