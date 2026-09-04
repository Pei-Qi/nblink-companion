import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import SettingsView from '../src/components/SettingsView.vue'

describe('settings view', () => {
  it('renders settings from the reactive store snapshot', () => {
    const wrapper = mount(SettingsView, {
      global: { plugins: [createPinia()] },
    })

    expect(wrapper.get('h1').text()).toBe('设置')
    expect(wrapper.text()).toContain('登录系统后自动启动')
    expect(wrapper.text()).toContain('客户端与数据')
    expect(wrapper.get('button[type="submit"]').text()).toContain('保存设置')
  })
})
