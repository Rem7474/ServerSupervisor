import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setLocale } from '../../i18n'

const threatsData = {
  data: {
    threats: {
      suspicious_requests: 42,
      suspicious_ips: 3,
      targeted_hosts: 2,
      blocked_ips: 1,
      top_ips: [{ ip: '1.2.3.4', hits: 100, unique_paths: 5, host_count: 2, level: 'HIGH' }],
      top_paths: [{ path: '/wp-login.php', category: 'CMS', hits: 50 }],
      country_distribution: [{ country: 'France', country_code: 'FR', hits: 30 }],
      most_targeted_hosts: [{ host_id: 'h1', host_name: 'app.example.com', hits: 80 }],
      ip_host_matrix: [{ ip: '5.6.7.8', host_count: 3, hits: 60 }],
      crowdsec_top_blocked: [],
      crowdsec_blocked_ips: 0,
    },
  },
}

vi.mock('../../api', () => ({
  default: {
    getWebLogsSummary: vi.fn(async () => threatsData),
    getIPTimeline: vi.fn(async () => ({ data: { requests: [] } })),
    blockCrowdSecIP: vi.fn(async () => ({ data: { command_id: 'cmd-1' } })),
    unblockCrowdSecIP: vi.fn(async () => ({ data: { command_id: 'cmd-2' } })),
    getCommand: vi.fn(async () => ({ data: { status: 'completed', output: '' } })),
    getDomainDetails: vi.fn(async () => ({ data: {} })),
  },
  getApiErrorMessage: (e: unknown) => String(e),
}))

vi.mock('topojson-client', () => ({
  feature: () => ({ features: [] }),
}))

import apiClient from '../../api'
import ThreatsPanel from './ThreatsPanel.vue'

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/', component: { template: '<div/>' } }],
})

const mountOpts = {
  global: {
    plugins: [router],
    stubs: { TrafficWorldMap: true, 'router-link': true },
  },
}

describe('ThreatsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
  })

  it('fetches the threats summary on mount', async () => {
    mount(ThreatsPanel, mountOpts)
    await flushPromises()
    expect(apiClient.getWebLogsSummary).toHaveBeenCalled()
  })

  it('renders the translated KPI labels and table headers/titles', async () => {
    const wrapper = mount(ThreatsPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Requêtes suspectes')
    expect(wrapper.text()).toContain('IPs suspectes')
    expect(wrapper.text()).toContain('Domaines ciblés')
    expect(wrapper.text()).toContain('IPs bloquées')
    expect(wrapper.text()).toContain('Top chemins scannés')
    expect(wrapper.text()).toContain('Pays des IPs suspectes')
    expect(wrapper.text()).toContain('Domaines les plus ciblés')
    expect(wrapper.text()).toContain('IP × Domaines (scan coordonné)')
  })

  it('shows a translated Timeline button per suspicious IP row', async () => {
    const wrapper = mount(ThreatsPanel, mountOpts)
    await flushPromises()

    const timelineButtons = wrapper.findAll('button').filter((b) => b.text() === 'Timeline')
    expect(timelineButtons.length).toBeGreaterThan(0)
  })

  it('shows the translated empty states when there is no data', async () => {
    vi.mocked(apiClient.getWebLogsSummary).mockResolvedValue({
      data: {
        threats: {
          suspicious_requests: 0, suspicious_ips: 0, targeted_hosts: 0, blocked_ips: 0,
          top_ips: [], top_paths: [], country_distribution: [], most_targeted_hosts: [],
          ip_host_matrix: [], crowdsec_top_blocked: [], crowdsec_blocked_ips: 0,
        },
      },
    } as never)
    const wrapper = mount(ThreatsPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Aucune IP suspecte sur la période.')
    expect(wrapper.text()).toContain('Aucun chemin suspect.')
    expect(wrapper.text()).toContain('Aucune donnée pays.')
    expect(wrapper.text()).toContain('Aucun domaine ciblé')
    expect(wrapper.text()).toContain('Pas de scan coordonné détecté')
  })

  it('shows the translated CrowdSec section and unblock control when there are blocked IPs', async () => {
    vi.mocked(apiClient.getWebLogsSummary).mockResolvedValue({
      data: {
        threats: {
          suspicious_requests: 0, suspicious_ips: 0, targeted_hosts: 0, blocked_ips: 1,
          top_ips: [], top_paths: [], country_distribution: [], most_targeted_hosts: [], ip_host_matrix: [],
          crowdsec_top_blocked: [{ ip: '9.9.9.9', type: 'ban', reason: 'manual scan', origin: 'cscli', country: 'US', blocked_until: '2099-01-01T00:00:00Z' }],
          crowdsec_blocked_ips: 1,
        },
      },
    } as never)
    const wrapper = mount(ThreatsPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('IPs bloquées par CrowdSec')
    expect(wrapper.text()).toContain('décisions actives')
    expect(wrapper.find('[aria-label="Débloquer cette IP"]').exists()).toBe(true)
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(ThreatsPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Suspicious requests')
    expect(wrapper.text()).toContain('Most targeted domains')
  })
})
