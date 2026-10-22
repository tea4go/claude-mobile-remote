import { afterEach, expect, test, vi } from 'vitest'
const fixtures = vi.hoisted(() => ({ platform: 'web', bridge: { fixture: true } }))
vi.mock('@capacitor/core', () => ({
  Capacitor: { getPlatform: () => fixtures.platform },
  registerPlugin: () => fixtures.bridge,
}))
afterEach(() => { vi.resetModules() })
for (const platform of ['ios', 'android', 'web']) {
  test(`${platform} selects the correct native experience and file cache ports`, async () => {
    fixtures.platform = platform
    vi.resetModules()
    const [{ nativeExperience }, { nativeSessionCache }] = await Promise.all([import('./experience'), import('./sessionCache')])
    expect(nativeExperience).toBe(platform === 'web' ? null : fixtures.bridge)
    expect(nativeSessionCache).toBe(platform === 'web' ? null : fixtures.bridge)
  })
}
