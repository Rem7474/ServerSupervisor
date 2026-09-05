import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'

const summaryData = {
  data: {
    traffic: {
      total_requests: 1234,
      total_bytes: 2048,
      ratio_5xx: 0.01,
      status_distribution: { '2xx': 10, '3xx': 1, '4xx': 2, '5xx': 0 },
      top_domains: [{ domain: 'example.com', hits: 100 }],
      country_distribution: [{ country: 'France', country_code: 'FR', hits: 50 }],
      top_client_ips: [],
    },
    threats: { suspicious_ips: 3, top_ips: [] },
    compare: { delta_percent: { total_requests: 5, total_bytes: -2, ratio_5xx: 0, suspicious_ips: 1 } },
  },
}

vi.mock('../../api', () => ({
  default: {
    getWebLogsSummary: vi.fn(async () => summaryData),
    getWebLogsTimeseries: vi.fn(async () => ({ data: { points: [] } })),
    getWebLogsLive: vi.fn(async () => ({ data: { requests: [] } })),
    getDomainDetails: vi.fn(async () => ({ data: {} })),
  },
}))

// topojson is imperative/canvas-bound (D3 world map); stub it so mounting in
// happy-dom does not throw (no world atlas). The two ApexCharts-based traffic
// charts are stubbed at the component level below instead of mocking
// vue3-apexcharts here — TrafficRequestsChart/TrafficStatusChart's own
// <script setup> never runs, so there's nothing chart-library-specific to mock.
vi.mock('topojson-client', () => ({
  feature: () => ({ features: [] }),
}))

import { setLocale } from '../../i18n'
import apiClient from '../../api'
import TrafficOverviewPanel from './TrafficOverviewPanel.vue'

describe('TrafficOverviewPanel (characterization)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    // TrafficOverviewPanel calls useHostsStore() in setup; install a fresh Pinia
    // so the store resolves without the app having to register the plugin.
    setActivePinia(createPinia())
  })

  // useTraffic() now reads/writes the URL query (period/source/host_id/from/to
  // sync, see useTraffic.ts) — a real router instance is required for
  // useRoute()/useRouter() to resolve during setup, a plain 'router-link' stub
  // is not enough for that.
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div/>' } }],
  })

  // Charts (ApexCharts, SVG-based) + world map (D3, canvas-adjacent) need a
  // real browser to render meaningfully and are verified in the real-browser
  // test; stub them here so the happy-dom shell test stays clean.
  const mountOpts = {
    global: {
      plugins: [router],
      stubs: {
        TrafficRequestsChart: true,
        TrafficStatusChart: true,
        TrafficWorldMap: true,
        'router-link': true,
      },
    },
  }

  it('fetches web-logs summary on mount', async () => {
    mount(TrafficOverviewPanel, mountOpts)
    await flushPromises()
    expect(apiClient.getWebLogsSummary).toHaveBeenCalled()
    expect(apiClient.getWebLogsTimeseries).toHaveBeenCalled()
    expect(apiClient.getWebLogsLive).toHaveBeenCalled()
  })

  it('renders the KPI labels once data has loaded', async () => {
    const wrapper = mount(TrafficOverviewPanel, mountOpts)
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Requêtes totales')
    expect(text).toContain('Bande passante')
    expect(text).toContain('Taux 5xx')
    expect(text).toContain('IPs suspectes')
  })

  it('renders the formatted total requests KPI value', async () => {
    const wrapper = mount(TrafficOverviewPanel, mountOpts)
    await flushPromises()
    await flushPromises()
    // fr-FR grouping uses a (narrow) no-break space; strip whitespace first.
    const compact = wrapper.text().replace(/\s/g, '')
    expect(compact).toContain('1234')
  })

  it('renders the translated section titles, table headers and non-empty rows', async () => {
    const wrapper = mount(TrafficOverviewPanel, mountOpts)
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Trafic - requêtes par tranche')
    expect(text).toContain('Distribution HTTP')
    expect(text).toContain('Top endpoints')
    expect(text).toContain('Top IPs suspectes')
    expect(text).toContain('Carte mondiale des IP clientes')
    expect(text).toContain('Pays les plus actifs')
    expect(text).toContain('Répartition du trafic par domaine')
    expect(text).toContain('Top domaines cibles')
    expect(text).toContain('Flux temps réel - dernières requêtes')
    expect(text).toContain('auto-refresh')
    expect(text).toContain('France')
    expect(text).toContain('example.com')
  })

  it('shows the translated empty states when there is no data at all', async () => {
    vi.mocked(apiClient.getWebLogsSummary).mockResolvedValue({
      data: {
        traffic: { total_requests: 0, total_bytes: 0, ratio_5xx: 0, status_distribution: {}, top_domains: [], country_distribution: [], top_client_ips: [] },
        threats: { suspicious_ips: 0, top_ips: [] },
        compare: { delta_percent: {} },
      },
    } as never)
    const wrapper = mount(TrafficOverviewPanel, mountOpts)
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Aucun endpoint sur la période.')
    expect(text).toContain('Aucune IP suspecte.')
    expect(text).toContain('Aucune donnée pays.')
    expect(text).toContain('Aucune donnée domaine.')
    expect(text).toContain('Aucune donnée de trafic.')
    expect(text).toContain('Aucune requête récente.')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(TrafficOverviewPanel, mountOpts)
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Total requests')
    expect(text).toContain('Top target domains')
  })
})
