import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'
import type { NPMProxyHostEnriched } from '../types/npm'

const { getUptimeProbe, getUptimeHistory, getUptimeStats, getUptimeHistoryBuckets, getSSLCertificate, getSSLCertificateHistory } = vi.hoisted(() => ({
  getUptimeProbe: vi.fn(),
  getUptimeHistory: vi.fn(),
  getUptimeStats: vi.fn(),
  getUptimeHistoryBuckets: vi.fn(),
  getSSLCertificate: vi.fn(),
  getSSLCertificateHistory: vi.fn(),
}))

const { listAllProxyHosts } = vi.hoisted(() => ({ listAllProxyHosts: vi.fn() }))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'proxy-1' } }),
}))

vi.mock('../api/npm', () => ({
  npmApi: { listAllProxyHosts },
}))

vi.mock('../api', () => ({
  default: { getUptimeProbe, getUptimeHistory, getUptimeStats, getUptimeHistoryBuckets, getSSLCertificate, getSSLCertificateHistory },
  getApiErrorMessage: (e: unknown) => String(e),
  isApiAbort: () => false,
}))

import MonitoringHostDetailView from './MonitoringHostDetailView.vue'

function makeHost(overrides: Partial<NPMProxyHostEnriched> = {}): NPMProxyHostEnriched {
  return {
    id: 'proxy-1',
    connection_id: 'conn-1',
    npm_id: 1,
    domain_names: ['app.example.com'],
    forward_host: '10.0.0.5',
    forward_port: 8080,
    ssl_enabled: false,
    npm_enabled: true,
    monitoring_enabled: true,
    uptime_monitoring_enabled: true,
    ssl_monitoring_enabled: false,
    last_seen_at: '2026-08-14T00:00:00Z',
    created_at: '2026-08-14T00:00:00Z',
    updated_at: '2026-08-14T00:00:00Z',
    connection_name: 'main',
    ...overrides,
  } as NPMProxyHostEnriched
}

beforeEach(() => {
  setLocale('fr')
})

describe('MonitoringHostDetailView — auto-refresh when only uptime is configured', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [makeHost({ uptime_probe_id: 'probe-1' })] } })
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'up', consecutive_failures: 0 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 100, successful_checks: 1, total_checks: 1, avg_latency_ms: 10, p95_latency_ms: 10 } })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('fetches the probe again on the next auto-refresh tick (PROBE_REFRESH_SEC)', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    const wrapper = mount(MonitoringHostDetailView, {
      global: { stubs: { 'router-link': true } },
    })

    await flushPromises() // useMonitoringHostDetail's load()
    await flushPromises() // UptimeDetailSection's onMounted fetchAll()

    expect(getUptimeProbe).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(30_000) // PROBE_REFRESH_SEC
    await flushPromises()

    expect(getUptimeProbe).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    errorSpy.mockRestore()
    warnSpy.mockRestore()
  })
})

describe('MonitoringHostDetailView — combined uptime + SSL summary', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows the merged summary row with probe status and cert days remaining', async () => {
    listAllProxyHosts.mockResolvedValue({
      data: { proxy_hosts: [makeHost({ uptime_probe_id: 'probe-1', ssl_certificate_id: 'cert-1', ssl_enabled: true, ssl_monitoring_enabled: true })] },
    })
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'up', consecutive_failures: 0 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 100, successful_checks: 1, total_checks: 1, avg_latency_ms: 10, p95_latency_ms: 10 } })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })
    getSSLCertificate.mockResolvedValue({ data: { id: 'cert-1', days_remaining: 20, valid_to: '2026-09-20T00:00:00Z' } })
    getSSLCertificateHistory.mockResolvedValue({ data: { results: [] } })

    const wrapper = mount(MonitoringHostDetailView, { global: { stubs: { 'router-link': true } } })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('UP')
    expect(wrapper.text()).toContain('Sonde uptime')
    expect(wrapper.text()).toContain('20 jours')
  })

  it('shows the singular day phrasing and the expired phrasing for the cert countdown', async () => {
    listAllProxyHosts.mockResolvedValue({
      data: { proxy_hosts: [makeHost({ uptime_probe_id: 'probe-1', ssl_certificate_id: 'cert-1', ssl_enabled: true, ssl_monitoring_enabled: true })] },
    })
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'down', consecutive_failures: 3 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 0, successful_checks: 0, total_checks: 1, avg_latency_ms: 0, p95_latency_ms: 0 } })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })
    getSSLCertificate.mockResolvedValue({ data: { id: 'cert-1', days_remaining: -2, valid_to: '2026-08-20T00:00:00Z' } })
    getSSLCertificateHistory.mockResolvedValue({ data: { results: [] } })

    const wrapper = mount(MonitoringHostDetailView, { global: { stubs: { 'router-link': true } } })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('DOWN')
    expect(wrapper.text()).toContain('Expiré (2j)')
  })
})

describe('MonitoringHostDetailView — no active tracking', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows the no-tracking banner when neither uptime nor SSL is configured', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [makeHost()] } })

    const wrapper = mount(MonitoringHostDetailView, { global: { stubs: { 'router-link': true } } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun suivi actif pour ce proxy host')
  })

  it('shows a translated error when the host is not found', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [] } })

    const wrapper = mount(MonitoringHostDetailView, { global: { stubs: { 'router-link': true } } })
    await flushPromises()

    expect(wrapper.find('.alert-danger').exists()).toBe(true)
  })
})
