import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { getUptimeProbe, getUptimeHistory, getUptimeStats, getUptimeHistoryBuckets } = vi.hoisted(() => ({
  getUptimeProbe: vi.fn(),
  getUptimeHistory: vi.fn(),
  getUptimeStats: vi.fn(),
  getUptimeHistoryBuckets: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'probe-1' } }),
}))

vi.mock('../../api', () => ({
  default: { getUptimeProbe, getUptimeHistory, getUptimeStats, getUptimeHistoryBuckets },
  getApiErrorMessage: (e: unknown) => String(e),
  isApiAbort: () => false,
}))

import UptimeDetailSection from './UptimeDetailSection.vue'

describe('UptimeDetailSection — autoRefresh when mounted without override props', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'up', consecutive_failures: 0 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 100, successful_checks: 1, total_checks: 1, avg_latency_ms: 10, p95_latency_ms: 10 } })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  // Regression test: UptimeProbeDetailView.vue (the standalone
  // /monitoring/probes/:id route) mounts this component with neither
  // hideRefreshBar nor autoRefresh bound at all — not even `undefined`
  // explicitly. Vue casts an unbound optional `boolean` prop to `false`
  // (not `undefined`), which used to be misread as "an active override
  // pinned to false", permanently freezing auto-refresh off outside of
  // MonitoringHostDetailView's merged-bar case.
  it('still auto-refreshes on its own timer when no autoRefresh/hideRefreshBar prop is passed', async () => {
    const wrapper = mount(UptimeDetailSection, {
      props: { probeId: 'probe-1' },
    })

    await flushPromises()
    expect(getUptimeProbe).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.page-refresh-bar').exists()).toBe(true)
    expect(wrapper.text()).toContain('Auto (30s)')
    expect(wrapper.text()).not.toContain('Pause')

    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()

    expect(getUptimeProbe).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
