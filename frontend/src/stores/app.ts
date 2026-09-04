import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { bridge } from '../bridge'
import { createMockSnapshot } from '../mock'
import type { AppSnapshot, RulePatch, SettingsInput, ToastEvent } from '../types'

export const useAppStore = defineStore('app', () => {
  const snapshot = ref<AppSnapshot>(createMockSnapshot('loading'))
  const page = ref<'services' | 'wake' | 'settings'>('services')
  const toast = ref<ToastEvent | null>(null)
  const busyKeys = ref(new Set<string>())
  let toastTimer = 0

  const services = computed(() => snapshot.value.services)

  function acceptSnapshot(next: AppSnapshot) {
    if (next.revision <= snapshot.value.revision && snapshot.value.revision !== 1) return
    snapshot.value = next
  }

  function showToast(next: ToastEvent) {
    toast.value = next
    window.clearTimeout(toastTimer)
    toastTimer = window.setTimeout(() => { toast.value = null }, 3600)
  }

  async function initialize() {
    bridge.onSnapshot(acceptSnapshot)
    bridge.onToast(showToast)
    bridge.onNavigate((next) => {
      if (next === 'services' || next === 'wake' || next === 'settings') page.value = next
    })
    snapshot.value = await bridge.bootstrap()
  }

  async function run(key: string, action: () => Promise<unknown>, success?: string) {
    busyKeys.value.add(key)
    busyKeys.value = new Set(busyKeys.value)
    try {
      await action()
      if (!bridge.isDesktop) snapshot.value = await bridge.bootstrap()
      if (success) showToast({ kind: 'success', title: '操作完成', message: success })
    } catch (error) {
      showToast({ kind: 'error', title: '操作失败', message: error instanceof Error ? error.message : String(error) })
    } finally {
      busyKeys.value.delete(key)
      busyKeys.value = new Set(busyKeys.value)
    }
  }

  const refresh = () => run('refresh', bridge.refresh)
  const setFavorite = (key: string, favorite: boolean) => run(`favorite:${key}`, () => bridge.setFavorite(key, favorite))
  const toggleRule = (key: string) => run(`toggle:${key}`, () => bridge.toggleRule(key))
  const stopAll = () => run('stop-all', bridge.stopAll, '全部转发已停止，常用设置未更改')
  const updateRule = (key: string, patch: RulePatch) => run(`edit:${key}`, () => bridge.updateRule(key, patch), '固定端口设置已保存')
  const openRule = (key: string) => run(`open:${key}`, () => bridge.openRule(key))
  const copyAddress = (key: string) => run(`copy:${key}`, () => bridge.copyAddress(key), '固定地址已复制')
  const wake = (key: string) => run(`wake:${key}`, () => bridge.wake(key), '唤醒请求已发送')
  const saveSettings = (input: SettingsInput) => run('save-settings', () => bridge.saveSettings(input), '设置已保存')
  const openLogs = () => run('open-logs', bridge.openLogs)
  const copyDiagnostics = () => run('copy-diagnostics', bridge.copyDiagnostics, '脱敏诊断信息已复制')

  return {
    snapshot, page, toast, busyKeys, services,
    initialize, acceptSnapshot, showToast, refresh, setFavorite, toggleRule, stopAll,
    updateRule, openRule, copyAddress, wake, saveSettings, openLogs, copyDiagnostics,
  }
})
