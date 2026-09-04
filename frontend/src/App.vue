<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CircleStop, Moon, Network, Power, RefreshCw, Search, Settings, Sun } from 'lucide-vue-next'
import ServiceTable from './components/ServiceTable.vue'
import SettingsView from './components/SettingsView.vue'
import WakeView from './components/WakeView.vue'
import { resolveTheme, type ServiceFilter } from './lib/presentation'
import { useAppStore } from './stores/app'

const store = useAppStore()
const query = ref('')
const filter = ref<ServiceFilter>('all')
const systemDark = ref(matchMedia('(prefers-color-scheme: dark)').matches)
const media = matchMedia('(prefers-color-scheme: dark)')
const effectiveTheme = computed(() => resolveTheme(store.snapshot.settings.themeMode, systemDark.value))

const filters: Array<{ value: ServiceFilter; label: string }> = [
  { value: 'all', label: '全部' }, { value: 'favorite', label: '常用' }, { value: 'running', label: '运行中' }, { value: 'error', label: '异常' },
]

function applyTheme() {
  document.documentElement.dataset.theme = effectiveTheme.value
  document.documentElement.style.colorScheme = effectiveTheme.value
}

function toggleTheme() {
  const next = effectiveTheme.value === 'dark' ? 'light' : 'dark'
  store.saveSettings({ ...store.snapshot.settings, themeMode: next })
}

function onSystemTheme(event: MediaQueryListEvent) { systemDark.value = event.matches }

watch(effectiveTheme, applyTheme, { immediate: true })
onMounted(() => { media.addEventListener('change', onSystemTheme); store.initialize() })
onBeforeUnmount(() => media.removeEventListener('change', onSystemTheme))
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand"><img src="/app-icon.svg" alt="" /><div><strong>Nblink</strong><span>Companion</span></div></div>
      <nav aria-label="主导航">
        <button :class="{ active: store.page === 'services' }" @click="store.page = 'services'"><Network :size="19" />固定端口</button>
        <button :class="{ active: store.page === 'wake' }" @click="store.page = 'wake'"><Power :size="19" />远程唤醒</button>
        <button :class="{ active: store.page === 'settings' }" @click="store.page = 'settings'"><Settings :size="19" />设置</button>
      </nav>
      <div class="sidebar-status">
        <div class="node-line"><span class="status-dot" :class="store.snapshot.node.connected ? 'ready' : 'error'" /><strong>{{ store.snapshot.node.connected ? '节点已连接' : '节点未连接' }}</strong></div>
        <span>{{ store.snapshot.node.connected ? `节点小宝 ${store.snapshot.node.version}` : store.snapshot.node.message }}</span>
      </div>
      <div class="sidebar-footer"><span>v{{ store.snapshot.version }}</span><button type="button" :title="effectiveTheme === 'dark' ? '切换浅色主题' : '切换深色主题'" @click="toggleTheme"><Sun v-if="effectiveTheme === 'dark'" :size="18" /><Moon v-else :size="18" /></button></div>
    </aside>

    <main>
      <section v-if="store.page === 'services'" class="content-section services-view">
        <header class="page-header services-header">
          <div><span class="eyebrow">本地转发</span><h1>固定端口</h1><p>{{ store.snapshot.syncMessage }}</p></div>
          <div class="header-actions"><button class="secondary-button" type="button" :disabled="store.busyKeys.has('stop-all')" @click="store.stopAll"><CircleStop :size="17" />停止全部</button><button class="icon-button" type="button" title="刷新服务" aria-label="刷新服务" :disabled="store.busyKeys.has('refresh')" @click="store.refresh"><RefreshCw :size="18" :class="{ spinning: store.snapshot.syncState === 'syncing' }" /></button></div>
        </header>
        <div class="summary-line" data-testid="summary-line">
          <span>共 <strong>{{ store.snapshot.summary.total }}</strong> 个服务</span><i />
          <span>运行 <strong>{{ store.snapshot.summary.running }}</strong></span><i />
          <span>常用 <strong>{{ store.snapshot.summary.favorites }}</strong></span><i />
          <span :class="{ 'error-text': store.snapshot.summary.errors > 0 }">异常 <strong>{{ store.snapshot.summary.errors }}</strong></span><i />
          <span>活动连接 <strong>{{ store.snapshot.summary.activeConnections }}</strong></span>
          <time v-if="store.snapshot.lastSyncedAt">最近同步 {{ new Date(store.snapshot.lastSyncedAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }}</time>
        </div>
        <div class="toolbar">
          <label class="search-box"><Search :size="18" /><input v-model="query" type="search" placeholder="搜索服务、目标或固定端口" aria-label="搜索服务" /></label>
          <div class="segmented" aria-label="服务筛选"><button v-for="item in filters" :key="item.value" type="button" :class="{ active: filter === item.value }" @click="filter = item.value">{{ item.label }}</button></div>
        </div>
        <ServiceTable :query="query" :filter="filter" />
      </section>
      <WakeView v-else-if="store.page === 'wake'" />
      <SettingsView v-else />
    </main>

    <transition name="toast"><div v-if="store.toast" class="toast" :class="store.toast.kind" role="status"><strong>{{ store.toast.title }}</strong><span>{{ store.toast.message }}</span></div></transition>
  </div>
</template>
