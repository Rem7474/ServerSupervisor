import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

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

vi.mock('../api', () => ({
  default: {
    getWebLogsSummary: vi.fn(async () => summaryData),
    getWebLogsTimeseries: vi.fn(async () => ({ data: { points: [] } })),
    getWebLogsLive: vi.fn(async () => ({ data: { requests: [] } })),
    getDomainDetails: vi.fn(async () => ({ data: {} })),
  },
}))

// chart.js + topojson are imperative/canvas-bound; stub them so mounting in
// happy-dom does not throw (no real 2d context / no world atlas).
vi.mock('chart.js', () => ({
  Chart: class {
    static register = vi.fn()
    destroy = vi.fn()
    update = vi.fn()
  },
  registerables: [],
}))
vi.mock('topojson-client', () => ({
  feature: () => ({ features: [] }),
}))

import apiClient from '../api'
import TrafficView from './TrafficView.vue'

describe('TrafficView (characterization)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // TrafficView calls useHostsStore() in setup; install a fresh Pinia so the
    // store resolves without the app having to register the plugin.
    setActivePinia(createPinia())
  })

  // Charts + world map render imperatively (Chart.js / D3) and are verified in
  // the real-browser test; stub them here so the happy-dom shell test stays clean.
  const mountOpts = {
    global: {
      stubs: {
        TrafficRequestsChart: true,
        TrafficStatusChart: true,
        TrafficWorldMap: true,
        'router-link': true,
      },
    },
  }

  it('fetches web-logs summary on mount', async () => {
    mount(TrafficView, mountOpts)
    await flushPromises()
    expect(apiClient.getWebLogsSummary).toHaveBeenCalled()
    expect(apiClient.getWebLogsTimeseries).toHaveBeenCalled()
    expect(apiClient.getWebLogsLive).toHaveBeenCalled()
  })

  it('renders the page shell + KPI labels once data has loaded', async () => {
    const wrapper = mount(TrafficView, mountOpts)
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Stats web')
    expect(text).toContain('Requêtes totales')
    expect(text).toContain('Bande passante')
    expect(text).toContain('Taux 5xx')
    expect(text).toContain('IPs suspectes')
  })

  it('renders the formatted total requests KPI value', async () => {
    const wrapper = mount(TrafficView, mountOpts)
    await flushPromises()
    await flushPromises()
    // fr-FR grouping uses a (narrow) no-break space; strip whitespace first.
    const compact = wrapper.text().replace(/\s/g, '')
    expect(compact).toContain('1234')
  })
})
