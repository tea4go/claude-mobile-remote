import { connection } from '@kit.NetworkKit'
import { debugLog } from '../debug/log'
export class NetworkMonitor {
  private monitor: connection.NetConnection | null = null
  private generation: number = 0
  start(changed: (available: boolean) => void): void {
    this.stop()
    const generation = this.generation
    try {
      const monitor = connection.createNetConnection(); this.monitor = monitor
      monitor.on('netAvailable', () => { if (generation === this.generation) changed(true) })
      monitor.on('netLost', () => { if (generation === this.generation) changed(false) })
      monitor.register((error) => { if (error && generation === this.generation) debugLog.record('network_monitor_failed') })
    } catch (_) { debugLog.record('network_monitor_failed') }
  }
  stop(): void {
    this.generation += 1
    const monitor = this.monitor; this.monitor = null
    if (monitor !== null) { try { monitor.unregister(() => {}) } catch (_) {} }
  }
}
