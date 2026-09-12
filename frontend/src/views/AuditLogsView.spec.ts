import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'

const { getCommandsHistory, getLoginEventsAdmin, getSecuritySummary, getAuditLogs } = vi.hoisted(() => ({
  getCommandsHistory: vi.fn(),
  getLoginEventsAdmin: vi.fn(),
  getSecuritySummary: vi.fn(),
  getAuditLogs: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getCommandsHistory, getLoginEventsAdmin, getSecuritySummary, getAuditLogs },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))

import AuditLogsView from './AuditLogsView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

describe('AuditLogsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getCommandsHistory.mockResolvedValue({ data: { commands: [], total: 0 } })
    getLoginEventsAdmin.mockResolvedValue({ data: { events: [], total: 0 } })
    getSecuritySummary.mockResolvedValue({ data: { stats: null, blocked_ips: [], top_failed_ips: [] } })
    getAuditLogs.mockResolvedValue({ data: { logs: [] } })
  })

  it('renders the translated header, tabs and commands table', async () => {
    const wrapper = mount(AuditLogsView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Audit')
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Historique des actions, connexions et commandes')
    expect(wrapper.text()).toContain('Commandes')
    expect(wrapper.text()).toContain('Connexions')
    expect(wrapper.text()).toContain('Journal')
    expect(wrapper.text()).toContain('Date')
    expect(wrapper.text()).toContain('Hôte')
    expect(wrapper.text()).toContain('Aucune commande enregistrée')
    expect(wrapper.text()).toContain('0 commande')
  })

  it('pluralizes the commands-count footer label for 2+ results', async () => {
    getCommandsHistory.mockResolvedValue({ data: { commands: [], total: 2 } })
    const wrapper = mount(AuditLogsView, mountOpts)
    await flushPromises()
    expect(wrapper.text()).toContain('2 commandes — page 1 / 1')
  })

  it('shows the translated status/module filter options', async () => {
    const wrapper = mount(AuditLogsView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Tous les états')
    expect(wrapper.text()).toContain('En attente')
    expect(wrapper.text()).toContain('Terminé')
    expect(wrapper.text()).toContain('Tous les modules')
    expect(wrapper.text()).toContain('Docker')
  })

  it('renders the translated connexions tab', async () => {
    const wrapper = mount(AuditLogsView, mountOpts)
    await flushPromises()
    await wrapper.findAll('a.nav-link')[1].trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Toutes les connexions')
    expect(wrapper.text()).toContain('Page 1 / 1')
  })

  it('renders the translated journal tab with category filter options', async () => {
    const wrapper = mount(AuditLogsView, mountOpts)
    await flushPromises()
    await wrapper.findAll('a.nav-link')[2].trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain("Journal d'audit")
    expect(wrapper.text()).toContain('Exporter CSV')
    expect(wrapper.text()).toContain('Catégorie')
    expect(wrapper.text()).toContain("Aucune entrée dans le journal d'audit")
    expect(wrapper.text()).toContain('Toutes catégories')
    expect(wrapper.text()).toContain('Alertes')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(AuditLogsView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('History of actions, connections and commands')
    expect(wrapper.text()).toContain('Commands')
    expect(wrapper.text()).toContain('No command recorded')
  })
})
