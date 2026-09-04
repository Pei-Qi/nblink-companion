export type ThemeMode = 'system' | 'light' | 'dark'
export type SyncState = 'idle' | 'syncing' | 'ready' | 'partial' | 'error'
export type RuleState = 'disabled' | 'waiting' | 'mapping' | 'ready' | 'error'
export type ServiceKind = 'tcp' | 'web' | 'rdp' | 'vnc'

export interface NodeSnapshot {
  connected: boolean
  version: string
  apiBase: string
  message: string
}

export interface AppSummary {
  total: number
  running: number
  favorites: number
  errors: number
  activeConnections: number
}

export interface ServiceSnapshot {
  endpointKey: string
  name: string
  host: string
  targetPort: number
  targetAddress: string
  listenPort: number
  localAddress: string
  kind: ServiceKind
  webScheme: string
  favorite: boolean
  available: boolean
  running: boolean
  state: RuleState
  stateLabel: string
  message: string
  activeConnections: number
  canOpen: boolean
}

export interface WakeTargetSnapshot {
  targetKey: string
  name: string
  maskedMAC: string
  online: boolean
}

export interface SettingsSnapshot {
  launchAtLogin: boolean
  startFavoritesOnLaunch: boolean
  credentialFile: string
  rdpLauncher: string
  vncLauncher: string
  refreshMinutes: number
  themeMode: ThemeMode
}

export interface AppSnapshot {
  revision: number
  version: string
  syncState: SyncState
  syncMessage: string
  lastSyncedAt: string
  node: NodeSnapshot
  summary: AppSummary
  services: ServiceSnapshot[]
  wakeTargets: WakeTargetSnapshot[]
  settings: SettingsSnapshot
}

export interface RulePatch {
  listenPort: number
  kind: ServiceKind
  webScheme: string
}

export interface SettingsInput extends SettingsSnapshot {}

export interface ToastEvent {
  kind: 'success' | 'info' | 'warning' | 'error'
  title: string
  message: string
}
