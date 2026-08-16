import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
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
    console.log('--- DOM after initial load ---')
    console.log(wrapper.html())
    console.log('--- console.error calls ---', errorSpy.mock.calls)
    console.log('--- console.warn calls ---', warnSpy.mock.calls)

    await vi.advanceTimersByTimeAsync(30_000) // PROBE_REFRESH_SEC
    await flushPromises()

    expect(getUptimeProbe).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    errorSpy.mockRestore()
    warnSpy.mockRestore()
  })
})
