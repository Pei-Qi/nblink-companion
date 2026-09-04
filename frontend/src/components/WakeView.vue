<script setup lang="ts">
import { Power } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'
import IconButton from './IconButton.vue'

const store = useAppStore()
</script>

<template>
  <section class="content-section">
    <header class="page-header">
      <div><span class="eyebrow">设备管理</span><h1>远程唤醒</h1><p>向当前节点小宝账号中已发现的设备发送唤醒请求。</p></div>
    </header>
    <div class="table-shell wake-table-shell">
      <table class="service-table wake-table">
        <thead><tr><th>设备</th><th>网卡</th><th>当前状态</th><th class="actions-column">操作</th></tr></thead>
        <tbody>
          <tr v-for="target in store.snapshot.wakeTargets" :key="target.targetKey">
            <td><strong>{{ target.name }}</strong></td>
            <td><span class="mono">{{ target.maskedMAC }}</span></td>
            <td><span class="status-pill" :class="target.online ? 'online' : 'offline'">{{ target.online ? '在线' : '离线或未知' }}</span></td>
            <td class="actions-column"><IconButton :icon="Power" label="唤醒设备" :disabled="store.busyKeys.has(`wake:${target.targetKey}`)" @click="store.wake(target.targetKey)" /></td>
          </tr>
        </tbody>
      </table>
      <div v-if="store.snapshot.wakeTargets.length === 0" class="state-panel"><strong>未发现可唤醒设备</strong><p>仅展示带有效 MAC 地址的设备。</p></div>
    </div>
  </section>
</template>
