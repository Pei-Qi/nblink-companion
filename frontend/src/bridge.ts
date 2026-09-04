import { Events } from '@wailsio/runtime'
import * as Controller from '../bindings/github.com/local/nblink-companion/internal/appservice/controller'
import type * as BindingModels from '../bindings/github.com/local/nblink-companion/internal/appservice/models'
import { createMockSnapshot } from './mock'
import type { AppSnapshot, RulePatch, SettingsInput, ToastEvent } from './types'

type RuntimeWindow = Window & {
  chrome?: { webview?: { postMessage?: (message: unknown) => void } }
  webkit?: { messageHandlers?: { external?: { postMessage?: (message: unknown) => void } } }
  wails?: { invoke?: (message: unknown) => void }
}

function hasNativeWailsTransport(): boolean {
  if (typeof window === 'undefined') return false
  const runtimeWindow = window as RuntimeWindow
  return typeof runtimeWindow.chrome?.webview?.postMessage === 'function'
    || typeof runtimeWindow.webkit?.messageHandlers?.external?.postMessage === 'function'
    || typeof runtimeWindow.wails?.invoke === 'function'
}

const isDesktop = hasNativeWailsTransport()
let mock = createMockSnapshot()

function cloneMock(): AppSnapshot {
  return structuredClone(mock)
}

export const bridge = {
  isDesktop,
  onSnapshot(handler: (snapshot: AppSnapshot) => void): () => void {
    if (!isDesktop) return () => undefined
    return Events.On('app:snapshot', (event: AppSnapshot | { data: AppSnapshot }) => {
      handler('data' in event ? event.data : event)
    })
  },
  onToast(handler: (toast: ToastEvent) => void): () => void {
    if (!isDesktop) return () => undefined
    return Events.On('app:toast', (event: ToastEvent | { data: ToastEvent }) => {
      handler('data' in event ? event.data : event)
    })
  },
  onNavigate(handler: (page: string) => void): () => void {
    if (!isDesktop) return () => undefined
    return Events.On('ui:navigate', (event: string | { data: string }) => handler(typeof event === 'string' ? event : event.data))
  },
  bootstrap: () => isDesktop ? Controller.Bootstrap() as Promise<AppSnapshot> : Promise.resolve(cloneMock()),
  refresh: async () => {
    if (isDesktop) return Controller.Refresh()
    mock = { ...mock, revision: mock.revision + 1, syncState: 'ready', syncMessage: '服务已同步', lastSyncedAt: new Date().toISOString() }
  },
  setFavorite: async (key: string, favorite: boolean) => {
    if (isDesktop) return Controller.SetFavorite(key, favorite)
    mock.services = mock.services.map((item) => item.endpointKey === key ? { ...item, favorite } : item)
    mock.summary.favorites = mock.services.filter((item) => item.favorite).length
    mock.revision++
  },
  toggleRule: async (key: string) => {
    if (isDesktop) return Controller.ToggleRule(key)
    mock.services = mock.services.map((item) => item.endpointKey === key ? { ...item, running: !item.running, state: item.running ? 'disabled' : 'ready', stateLabel: item.running ? '已停止' : '已就绪' } : item)
    mock.summary.running = mock.services.filter((item) => item.running).length
    mock.revision++
  },
  stopAll: () => isDesktop ? Controller.StopAll() : Promise.resolve(),
  updateRule: (key: string, patch: RulePatch) => isDesktop ? Controller.UpdateRule(key, patch as BindingModels.RulePatch) : Promise.resolve(),
  openRule: (key: string) => isDesktop ? Controller.OpenRule(key) : Promise.resolve(),
  copyAddress: async (key: string) => {
    if (isDesktop) return Controller.CopyAddress(key)
    const item = mock.services.find((service) => service.endpointKey === key)
    if (item && navigator.clipboard) await navigator.clipboard.writeText(item.localAddress)
  },
  wake: (key: string) => isDesktop ? Controller.Wake(key) : Promise.resolve(),
  saveSettings: async (settings: SettingsInput) => {
    if (isDesktop) return Controller.SaveSettings(settings as BindingModels.SettingsInput)
    mock.settings = structuredClone(settings)
    mock.revision++
  },
  chooseFile: (kind: 'credential' | 'rdp' | 'vnc') => isDesktop ? Controller.ChooseFile(kind) : Promise.resolve(''),
  openLogs: () => isDesktop ? Controller.OpenLogs() : Promise.resolve(),
  copyDiagnostics: () => isDesktop ? Controller.CopyDiagnostics() : Promise.resolve(),
}
