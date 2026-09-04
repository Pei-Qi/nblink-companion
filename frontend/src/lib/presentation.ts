import type { ServiceSnapshot, ThemeMode } from '../types'

export type ServiceFilter = 'all' | 'favorite' | 'running' | 'error'

export function serviceMatches(service: ServiceSnapshot, query: string, filter: ServiceFilter): boolean {
  const normalized = query.trim().toLocaleLowerCase()
  if (normalized) {
    const haystack = [
      service.name,
      service.host,
      service.targetAddress,
      service.localAddress,
      String(service.targetPort),
      String(service.listenPort),
      service.kind,
    ].join(' ').toLocaleLowerCase()
    if (!haystack.includes(normalized)) return false
  }
  if (filter === 'favorite') return service.favorite
  if (filter === 'running') return service.running
  if (filter === 'error') return service.running && (service.state === 'error' || service.state === 'waiting')
  return true
}

export function resolveTheme(mode: ThemeMode, systemDark: boolean): 'light' | 'dark' {
  if (mode === 'system') return systemDark ? 'dark' : 'light'
  return mode
}
