import type { AppSnapshot, ServiceSnapshot } from './types'

const serviceSeed = [
  ['家庭存储管理后台', '192.168.10.15', 443, 18443, 'web', 'https'],
  ['书房 Windows 工作站', '192.168.10.21', 3389, 13389, 'rdp', ''],
  ['开发数据库 PostgreSQL', '192.168.10.32', 5432, 15432, 'tcp', ''],
  ['Home Assistant', '192.168.10.40', 8123, 18123, 'web', 'http'],
  ['媒体中心 Jellyfin', '192.168.10.16', 8096, 18096, 'web', 'http'],
  ['设计工作站远程桌面与素材同步服务超长名称', '192.168.10.28', 5900, 15900, 'vnc', ''],
] as const

function makeService(index: number): ServiceSnapshot {
  const seed = serviceSeed[index % serviceSeed.length]
  const running = index % 3 !== 2
  const state = !running ? 'disabled' : index % 5 === 2 ? 'waiting' : index % 7 === 4 ? 'error' : 'ready'
  const activeConnections = state === 'ready' ? index % 4 : 0
  return {
    endpointKey: `mock-${index}`,
    name: index < serviceSeed.length ? seed[0] : `${seed[0]} ${index + 1}`,
    host: seed[1],
    targetPort: seed[2],
    targetAddress: `${seed[1]}:${seed[2]}`,
    listenPort: seed[3] + index,
    localAddress: `127.0.0.1:${seed[3] + index}`,
    kind: seed[4],
    webScheme: seed[5],
    favorite: index % 3 === 0,
    available: index !== 5,
    running,
    state,
    stateLabel: state === 'ready' ? '已就绪' : state === 'waiting' ? '等待节点小宝' : state === 'error' ? '转发错误' : '已停止',
    message: state === 'error' ? '后端映射暂时不可用' : '',
    activeConnections,
    canOpen: state === 'ready' && seed[4] !== 'tcp',
  }
}

export function createMockSnapshot(scenario = new URLSearchParams(location.search).get('scenario') || 'default'): AppSnapshot {
  const count = scenario === 'empty' ? 0 : scenario === 'many' ? 24 : 6
  const services = Array.from({ length: count }, (_, index) => makeService(index))
  const errors = services.filter((item) => item.running && (item.state === 'error' || item.state === 'waiting')).length
  const syncState = scenario === 'loading' ? 'syncing' : scenario === 'error' ? 'error' : errors ? 'partial' : 'ready'
  return {
    revision: 1,
    version: '0.3.0',
    syncState,
    syncMessage: scenario === 'loading' ? '正在同步节点小宝服务...' : scenario === 'error' ? '服务同步失败，请检查节点小宝登录状态' : errors ? '服务已同步，部分转发正在等待恢复' : '服务已同步',
    lastSyncedAt: scenario === 'loading' ? '' : new Date('2026-09-03T20:36:00+08:00').toISOString(),
    node: {
      connected: scenario !== 'error',
      version: scenario === 'error' ? '' : '3.8.2',
      apiBase: scenario === 'error' ? '' : 'http://127.0.0.1:4080',
      message: scenario === 'error' ? '节点小宝本地服务未运行' : '本地服务已连接',
    },
    summary: {
      total: services.length,
      running: services.filter((item) => item.running).length,
      favorites: services.filter((item) => item.favorite).length,
      errors,
      activeConnections: services.reduce((total, item) => total + item.activeConnections, 0),
    },
    services,
    wakeTargets: [
      { targetKey: 'wake-1', name: '书房 Windows 工作站', maskedMAC: '**:**:**:9A:2F:10', online: false },
      { targetKey: 'wake-2', name: '客厅媒体服务器', maskedMAC: '**:**:**:7C:18:42', online: true },
      { targetKey: 'wake-3', name: '设计工作站', maskedMAC: '**:**:**:51:AA:08', online: false },
    ],
    settings: {
      launchAtLogin: true,
      startFavoritesOnLaunch: true,
      credentialFile: '/Users/example/Library/Application Support/nblink/user.db',
      rdpLauncher: 'Microsoft Remote Desktop',
      vncLauncher: '',
      refreshMinutes: 5,
      themeMode: 'system',
    },
  }
}
