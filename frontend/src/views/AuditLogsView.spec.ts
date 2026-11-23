import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
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

const route = reactive({ query: {} as Record<string, string> })
const routerReplace = vi.fn((loc: { query?: Record<string, string> }) => {
  if (loc?.query) route.query = { ...loc.query }
})
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ replace: routerReplace }),
}))

import AuditLogsView from './AuditLogsView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

const wrappers: VueWrapper[] = []
function mountAuditView() {
  const wrapper = mount(AuditLogsView, mountOpts)
  wrappers.push(wrapper)
  return wrapper
}

describe('AuditLogsView', () => {
  afterEach(() => {
    while (wrappers.length) wrappers.pop()?.unmount()
  })

  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    route.query = {}
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getCommandsHistory.mockResolvedValue({ data: { commands: [], total: 0 } })
    getLoginEventsAdmin.mockResolvedValue({ data: { events: [], total: 0 } })
    getSecuritySummary.mockResolvedValue({ data: { stats: null, blocked_ips: [], top_failed_ips: [] } })
    getAuditLogs.mockResolvedValue({ data: { logs: [] } })
  })

  it('renders the translated header, tabs and commands table', async () => {
    const wrapper = mountAuditView()
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
    const wrapper = mountAuditView()
    await flushPromises()
    expect(wrapper.text()).toContain('2 commandes — page 1 / 1')
  })

  it('shows the translated status/module filter options', async () => {
    const wrapper = mountAuditView()
    await flushPromises()

    expect(wrapper.text()).toContain('Tous les états')
    expect(wrapper.text()).toContain('En attente')
    expect(wrapper.text()).toContain('Terminé')
    expect(wrapper.text()).toContain('Tous les modules')
    expect(wrapper.text()).toContain('Docker')
  })

  it('renders the translated connexions tab', async () => {
    const wrapper = mountAuditView()
    await flushPromises()
    await wrapper.findAll('a.nav-link')[1].trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Toutes les connexions')
    expect(wrapper.text()).toContain('Page 1 / 1')
  })

  it('renders the translated journal tab with category filter options', async () => {
    const wrapper = mountAuditView()
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
    const wrapper = mountAuditView()
    await flushPromises()

    expect(wrapper.text()).toContain('History of actions, connections and commands')
    expect(wrapper.text()).toContain('Commands')
    expect(wrapper.text()).toContain('No command recorded')
  })

  it('loads the journal tab immediately on mount when ?tab=audit-log', async () => {
    route.query = { tab: 'audit-log' }
    const wrapper = mountAuditView()
    await flushPromises()

    expect(getAuditLogs).toHaveBeenCalled()
    expect(getCommandsHistory).not.toHaveBeenCalled()
    expect(wrapper.find('a.nav-link.active').text()).toBe('Journal')
  })

  it('loads the logins tab immediately on mount when ?tab=logins', async () => {
    route.query = { tab: 'logins' }
    const wrapper = mountAuditView()
    await flushPromises()

    expect(getLoginEventsAdmin).toHaveBeenCalledTimes(1)
    expect(wrapper.find('a.nav-link.active').text()).toBe('Connexions')
  })

  it('switches tabs reactively when route.query.tab changes while already on /audit', async () => {
    route.query = { tab: 'commands' }
    const wrapper = mountAuditView()
    await flushPromises()
    expect(wrapper.find('a.nav-link.active').text()).toBe('Commandes')

    route.query.tab = 'audit-log'
    await flushPromises()
    expect(getAuditLogs).toHaveBeenCalledTimes(1)
    expect(wrapper.find('a.nav-link.active').text()).toBe('Journal')

    route.query.tab = 'logins'
    await flushPromises()
    expect(getLoginEventsAdmin).toHaveBeenCalledTimes(1)
    expect(wrapper.find('a.nav-link.active').text()).toBe('Connexions')

    route.query.tab = 'commands'
    await flushPromises()
    expect(wrapper.find('a.nav-link.active').text()).toBe('Commandes')
  })

  it('restricts operators to the commandes tab even if ?tab=audit-log is requested', async () => {
    useAuthStore().setAuth({ role: 'operator', username: 'op' } as never, 'op')
    route.query = { tab: 'audit-log' }
    const wrapper = mountAuditView()
    await flushPromises()

    expect(wrapper.findAll('a.nav-link').map((a) => a.text())).toEqual(['Commandes'])
    expect(wrapper.find('a.nav-link.active').text()).toBe('Commandes')
    expect(getCommandsHistory).toHaveBeenCalledTimes(1)
    expect(getAuditLogs).not.toHaveBeenCalled()
  })
})
