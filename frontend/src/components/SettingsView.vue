<script setup lang="ts">
import { reactive, watch } from 'vue'
import { ClipboardCopy, FolderOpen, Save } from 'lucide-vue-next'
import { bridge } from '../bridge'
import { useAppStore } from '../stores/app'
import type { SettingsInput } from '../types'

const store = useAppStore()
const form = reactive<SettingsInput>({ ...store.snapshot.settings })

watch(() => store.snapshot.settings, (settings) => Object.assign(form, settings), { deep: true })

async function choose(kind: 'credential' | 'rdp' | 'vnc') {
  const path = await bridge.chooseFile(kind)
  if (!path) return
  if (kind === 'credential') form.credentialFile = path
  if (kind === 'rdp') form.rdpLauncher = path
  if (kind === 'vnc') form.vncLauncher = path
}
</script>

<template>
  <section class="content-section settings-view">
    <header class="page-header"><div><span class="eyebrow">偏好设置</span><h1>设置</h1><p>管理启动行为、刷新周期、主题和外部客户端。</p></div></header>
    <form class="settings-form" @submit.prevent="store.saveSettings({ ...form })">
      <section class="settings-group">
        <div class="group-heading"><h2>常规</h2><p>应用启动和后台同步行为。</p></div>
        <div class="field-list">
          <label class="toggle-row"><span><strong>登录系统后自动启动</strong><small>在后台启动，不主动显示窗口。</small></span><input v-model="form.launchAtLogin" type="checkbox" role="switch" /></label>
          <label class="toggle-row"><span><strong>启动常用服务</strong><small>应用启动后自动运行标记为常用的规则。</small></span><input v-model="form.startFavoritesOnLaunch" type="checkbox" role="switch" /></label>
          <label class="field-row"><span><strong>刷新周期</strong><small>同步节点与服务列表的间隔。</small></span><select v-model.number="form.refreshMinutes"><option v-for="minute in [1,2,5,10,15,30]" :key="minute" :value="minute">{{ minute }} 分钟</option></select></label>
          <label class="field-row"><span><strong>外观</strong><small>默认跟随操作系统外观。</small></span><select v-model="form.themeMode"><option value="system">跟随系统</option><option value="light">原生轻盈白</option><option value="dark">原生轻盈黑</option></select></label>
        </div>
      </section>
      <section class="settings-group">
        <div class="group-heading"><h2>客户端与数据</h2><p>覆盖自动发现的数据文件或应用。</p></div>
        <div class="field-list path-fields">
          <label><span>节点小宝数据</span><div class="path-input"><input v-model="form.credentialFile" placeholder="留空时自动查找节点小宝数据文件" /><button type="button" title="选择数据文件" @click="choose('credential')"><FolderOpen :size="18" /></button></div></label>
          <label><span>RDP 客户端</span><div class="path-input"><input v-model="form.rdpLauncher" placeholder="可选：客户端路径或应用名" /><button type="button" title="选择 RDP 客户端" @click="choose('rdp')"><FolderOpen :size="18" /></button></div></label>
          <label><span>VNC 客户端</span><div class="path-input"><input v-model="form.vncLauncher" placeholder="Windows 使用 VNC 时需要配置" /><button type="button" title="选择 VNC 客户端" @click="choose('vnc')"><FolderOpen :size="18" /></button></div></label>
        </div>
      </section>
      <section class="settings-group diagnostics-group">
        <div class="group-heading"><h2>诊断</h2><p>日志与诊断信息不包含节点小宝凭据。</p></div>
        <div class="button-row"><button type="button" class="secondary-button" @click="store.openLogs"><FolderOpen :size="17" />打开日志目录</button><button type="button" class="secondary-button" @click="store.copyDiagnostics"><ClipboardCopy :size="17" />复制诊断信息</button></div>
      </section>
      <footer class="settings-footer"><button type="submit" class="primary-button" :disabled="store.busyKeys.has('save-settings')"><Save :size="17" />保存设置</button></footer>
    </form>
  </section>
</template>
