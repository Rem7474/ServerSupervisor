import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../api', () => ({
  default: {
    getSettings: vi.fn().mockResolvedValue({ data: { settings: {}, db_status: {} } }),
  },
  getApiErrorMessage: (e: unknown) => String(e),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))

import SettingsView from './SettingsView.vue'
import { useAuthStore } from '../stores/auth'
import { setLocale } from '../i18n'

function mountView() {
  setActivePinia(createPinia())
  useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
  return mount(SettingsView, {
    global: {
      stubs: {
        SettingsSystemInfoCard: true,
        SettingsDatabaseCard: true,
        SettingsSmtpCard: true,
        SettingsNotificationsCard: true,
        SettingsProxmoxCard: true,
        SettingsNPMCard: true,
        SettingsRegistryCredentialsCard: true,
        SettingsRetentionCard: true,
        SettingsThreatDetectionCard: true,
        SettingsMaintenanceCard: true,
        'router-link': { template: '<a><slot /></a>' },
      },
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsView', () => {
  it('shows the translated page title and tab labels', () => {
    const wrapper = mountView()
    expect(wrapper.text()).toContain('Paramètres')
    expect(wrapper.text()).toContain('Général')
    expect(wrapper.text()).toContain('Détection de menaces')
    expect(wrapper.text()).toContain('Zone sensible')
  })

  it('switches the active tab on click', async () => {
    const wrapper = mountView()
    const buttons = wrapper.findAll('.list-group-item')
    const notifTab = buttons.find((b) => b.text() === 'Notifications')!
    await notifTab.trigger('click')
    expect(notifTab.classes()).toContain('active')
  })

  it('translates the tabs in English', () => {
    setLocale('en')
    const wrapper = mountView()
    expect(wrapper.text()).toContain('Settings')
    expect(wrapper.text()).toContain('Threat detection')
    expect(wrapper.text()).toContain('Danger zone')
  })
})
