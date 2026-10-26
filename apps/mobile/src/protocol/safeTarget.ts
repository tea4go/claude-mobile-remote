export function safeNetworkTarget(raw: string): string {
  try { const url = new URL(raw); return `${url.protocol}//${url.host}` } catch { return '地址无效' }
}
