import { afterEach, describe, expect, it, vi } from 'vitest'

afterEach(() => {
  delete (window as Window & { _wails?: unknown })._wails
  Reflect.deleteProperty(window, 'chrome')
  Reflect.deleteProperty(window, 'webkit')
  vi.resetModules()
})

describe('desktop bridge detection', () => {
  it('uses mock data in a regular browser', async () => {
    const { bridge } = await import('../src/bridge')
    expect(bridge.isDesktop).toBe(false)
  })

  it('uses generated bindings with the macOS Wails transport', async () => {
    Object.defineProperty(window, 'webkit', {
      configurable: true,
      value: { messageHandlers: { external: { postMessage: vi.fn() } } },
    })
    const { bridge } = await import('../src/bridge')
    expect(bridge.isDesktop).toBe(true)
  })

  it('uses generated bindings with the Windows Wails transport', async () => {
    Object.defineProperty(window, 'chrome', {
      configurable: true,
      value: { webview: { postMessage: vi.fn() } },
    })
    const { bridge } = await import('../src/bridge')
    expect(bridge.isDesktop).toBe(true)
  })
})
