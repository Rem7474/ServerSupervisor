import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'

const {
  getAlertRules,
  getHosts,
  getAgentAlertRuleCapabilities,
  getProxmoxAlertRuleCapabilities,
  getSyntheticAlertRuleCapabilities,
  getDockerAlertCapabilities,
  getNotifications,
  getReleaseTrackers,
  getAlertRuleTemplates,
} = vi.hoisted(() => ({
  getAlertRules: vi.fn(),
  getHosts: vi.fn(),
  getAgentAlertRuleCapabilities: vi.fn(),
  getProxmoxAlertRuleCapabilities: vi.fn(),
  getSyntheticAlertRuleCapabilities: vi.fn(),
  getDockerAlertCapabilities: vi.fn(),
  getNotifications: vi.fn(),
  getReleaseTrackers: vi.fn(),
  getAlertRuleTemplates: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getAlertRules,
    getHosts,
    getAgentAlertRuleCapabilities,
    getProxmoxAlertRuleCapabilities,
    getSyntheticAlertRuleCapabilities,
    getDockerAlertCapabilities,
    getNotifications,
    getReleaseTrackers,
    getAlertRuleTemplates,
  },
  getApiErrorMessage: (e: unknown, fallback?: string) => fallback || String(e),
}))

vi.mock('../composables/useNotifications', () => ({
  onNotificationsMessage: vi.fn(() => vi.fn()),
}))

const route = reactive({ query: {} as Record<string, string> })
const routerReplace = vi.fn()
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ replace: routerReplace }),
}))

import AlertsView from './AlertsView.vue'

const wrappers: VueWrapper[] = []
function mountAlerts() {
  const wrapper = mount(AlertsView, {
    global: {
      stubs: {
        'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' },
        WarRoomPanel: { template: '<div data-testid="warroom-panel" />' },
        AlertRuleList: { template: '<div data-testid="rules-panel" />' },
        AlertReleaseSummary: { template: '<div data-testid="releases-panel" />' },
        AlertIncidentList: { template: '<div data-testid="incidents-panel" />' },
        MaintenanceWindowsPanel: { template: '<div data-testid="maintenance-panel" />' },
        AlertRuleTemplateList: { template: '<div data-testid="templates-panel" />' },
        AlertRuleModal: true,
        AlertRuleTemplateModal: true,
        AlertRuleTemplateApplyModal: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('AlertsView', () => {
  afterEach(() => {
    while (wrappers.length) wrappers.pop()?.unmount()
  })

  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' }, 'admin')
    route.query = {}

    getAlertRules.mockResolvedValue({ data: [] })
    getHosts.mockResolvedValue({ data: [] })
    getAgentAlertRuleCapabilities.mockResolvedValue({ data: { metrics: [] } })
    getProxmoxAlertRuleCapabilities.mockResolvedValue({ data: { proxmox_metrics: [], proxmox_scope: {} } })
    getSyntheticAlertRuleCapabilities.mockResolvedValue({ data: { metrics: [] } })
    getDockerAlertCapabilities.mockResolvedValue({ data: { metrics: [] } })
    getNotifications.mockResolvedValue({ data: { notifications: [] } })
    getReleaseTrackers.mockResolvedValue({ data: { trackers: [] } })
    getAlertRuleTemplates.mockResolvedValue({ data: [] })
  })

  it('defaults to the warroom tab when no ?tab query param is present', async () => {
    const wrapper = mountAlerts()
    await flushPromises()

    expect(wrapper.find('[data-testid="warroom-panel"]').exists()).toBe(true)
    expect(wrapper.find('button.nav-link.active').text()).toContain('Vue active')
  })

  it('honors ?tab=maintenance on mount and mounts the maintenance panel', async () => {
    route.query = { tab: 'maintenance' }
    const wrapper = mountAlerts()
    await flushPromises()

    expect(wrapper.find('[data-testid="maintenance-panel"]').exists()).toBe(true)
    expect(wrapper.find('button.nav-link.active').text()).toContain('Maintenance')
    expect(wrapper.find('[data-testid="warroom-panel"]').exists()).toBe(false)
  })

  it('honors ?tab=rules, ?tab=releases, ?tab=incidents, and ?tab=templates on mount', async () => {
    route.query = { tab: 'rules' }
    const rulesWrapper = mountAlerts()
    await flushPromises()
    expect(rulesWrapper.find('button.nav-link.active').text()).toContain('Règles')

    route.query = { tab: 'releases' }
    const releasesWrapper = mountAlerts()
    await flushPromises()
    expect(getReleaseTrackers).toHaveBeenCalled()
    expect(releasesWrapper.find('[data-testid="releases-panel"]').exists()).toBe(true)

    route.query = { tab: 'incidents' }
    const incidentsWrapper = mountAlerts()
    await flushPromises()
    expect(getNotifications).toHaveBeenCalled()
    expect(incidentsWrapper.find('[data-testid="incidents-panel"]').exists()).toBe(true)

    route.query = { tab: 'templates' }
    const templatesWrapper = mountAlerts()
    await flushPromises()
    expect(getAlertRuleTemplates).toHaveBeenCalled()
    expect(templatesWrapper.find('[data-testid="templates-panel"]').exists()).toBe(true)
  })
})
