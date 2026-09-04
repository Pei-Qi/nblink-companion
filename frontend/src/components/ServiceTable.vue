<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy, Monitor, Pencil, Play, Square, Star } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'
import { serviceMatches, type ServiceFilter } from '../lib/presentation'
import type { RulePatch, ServiceSnapshot } from '../types'
import IconButton from './IconButton.vue'

const props = defineProps<{ query: string; filter: ServiceFilter }>()
const store = useAppStore()
const editing = ref<ServiceSnapshot | null>(null)
const draft = ref<RulePatch>({ listenPort: 18080, kind: 'tcp', webScheme: 'http' })

const visible = computed(() => store.services.filter((item) => serviceMatches(item, props.query, props.filter)))

function edit(service: ServiceSnapshot) {
  editing.value = service
  draft.value = { listenPort: service.listenPort, kind: service.kind, webScheme: service.webScheme || 'http' }
}

async function save() {
  if (!editing.value) return
  await store.updateRule(editing.value.endpointKey, draft.value)
  editing.value = null
}
</script>

<template>
  <div class="table-shell">
    <table class="service-table">
      <thead>
        <tr>
          <th>服务</th>
          <th>远端目标</th>
          <th>固定地址</th>
          <th class="connections-column">连接数</th>
          <th class="actions-column">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="service in visible" :key="service.endpointKey">
          <td>
            <div class="service-identity">
              <span class="status-dot" :class="[service.state, { unavailable: !service.available }]" />
              <div class="truncate-stack">
                <strong :title="service.name">{{ service.name }}</strong>
                <span :class="{ 'error-text': service.state === 'error' }">{{ service.stateLabel }}</span>
              </div>
            </div>
          </td>
          <td>
            <div class="truncate-stack">
              <span class="mono" :title="service.targetAddress">{{ service.targetAddress }}</span>
              <small>{{ service.kind === 'web' ? service.webScheme.toUpperCase() + ' 网页' : service.kind.toUpperCase() }}</small>
            </div>
          </td>
          <td>
            <div class="truncate-stack">
              <span class="mono" :title="service.localAddress">{{ service.localAddress }}</span>
              <small>{{ service.running ? '本机监听中' : '当前未监听' }}</small>
            </div>
          </td>
          <td class="connections-column"><span class="connection-count">{{ service.activeConnections }}</span></td>
          <td class="actions-column">
            <div class="row-actions">
              <IconButton :icon="Star" :active="service.favorite" :label="service.favorite ? '取消常用' : '设为常用'" @click="store.setFavorite(service.endpointKey, !service.favorite)" />
              <IconButton :icon="Copy" label="复制固定地址" @click="store.copyAddress(service.endpointKey)" />
              <IconButton :icon="Monitor" label="打开服务" :disabled="!service.canOpen" @click="store.openRule(service.endpointKey)" />
              <IconButton :icon="Pencil" label="编辑规则" @click="edit(service)" />
              <IconButton
                :icon="service.running ? Square : Play"
                :label="service.running ? '停止转发' : '启动转发'"
                :danger="service.running"
                :disabled="store.busyKeys.has(`toggle:${service.endpointKey}`)"
                @click="store.toggleRule(service.endpointKey)"
              />
            </div>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="store.snapshot.syncState === 'syncing'" class="state-panel" data-testid="loading-state">
      <span class="spinner" />
      <strong>正在同步服务</strong>
      <p>正在读取节点状态与远程服务列表。</p>
    </div>
    <div v-else-if="store.services.length === 0" class="state-panel" data-testid="empty-state">
      <strong>尚未发现服务</strong>
      <p>确认节点小宝已登录后重新刷新。</p>
    </div>
    <div v-else-if="visible.length === 0" class="state-panel">
      <strong>没有匹配结果</strong>
      <p>调整搜索内容或筛选条件。</p>
    </div>
  </div>

  <div v-if="editing" class="modal-backdrop" @click.self="editing = null">
    <form class="dialog" @submit.prevent="save">
      <header>
        <div>
          <span class="eyebrow">编辑固定端口</span>
          <h2>{{ editing.name }}</h2>
        </div>
        <button type="button" class="text-button" @click="editing = null">取消</button>
      </header>
      <label>
        <span>固定端口</span>
        <input v-model.number="draft.listenPort" type="number" min="1024" max="65535" required />
      </label>
      <label>
        <span>服务类型</span>
        <select v-model="draft.kind">
          <option value="tcp">TCP</option>
          <option value="web">网页</option>
          <option value="rdp">RDP</option>
          <option value="vnc">VNC</option>
        </select>
      </label>
      <label v-if="draft.kind === 'web'">
        <span>网页协议</span>
        <select v-model="draft.webScheme"><option value="http">HTTP</option><option value="https">HTTPS</option></select>
      </label>
      <footer>
        <button type="button" class="secondary-button" @click="editing = null">取消</button>
        <button class="primary-button" type="submit">保存规则</button>
      </footer>
    </form>
  </div>
</template>
