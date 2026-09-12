import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, shallowMount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

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
    setLocale('fr')
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

describe('UptimeDetailSection — heartbeat bar and latency chart', () => {
  beforeEach(() => {
    setLocale('fr')
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'down', consecutive_failures: 3 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 98.5, successful_checks: 9, total_checks: 10, avg_latency_ms: 42, p95_latency_ms: 80 } })
    getUptimeHistoryBuckets.mockResolvedValue({
      data: {
        buckets: [
          { bucket_start: '2026-08-24T10:00:00Z', total_checks: 0, up_checks: 0, down_checks: 0, avg_latency_ms: 0 },
          { bucket_start: '2026-08-24T11:00:00Z', total_checks: 5, up_checks: 5, down_checks: 0, avg_latency_ms: 30 },
          { bucket_start: '2026-08-24T12:00:00Z', total_checks: 5, up_checks: 2, down_checks: 3, avg_latency_ms: 55 },
        ],
      },
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  // shallowMount stubs the async ApexChart child (real ApexCharts SVG
  // rendering needs a real browser — see frontend/CLAUDE.md's browser-mode
  // note) while still exercising this component's own buildChartOptions()/
  // bucketColorClass()/bucketTitle() logic, which runs independently of
  // whether the chart itself renders.
  it('builds the latency chart series from buckets and colors/labels the heartbeat bar per bucket', async () => {
    const wrapper = shallowMount(UptimeDetailSection, {
      props: { probeId: 'probe-1' },
    })
    await flushPromises()
    await flushPromises()

    // DOWN status + consecutive failures.
    expect(wrapper.text()).toContain('DOWN')
    expect(wrapper.text()).toContain('98.50%')
    expect(wrapper.text()).toContain('9 OK / 10 checks')

    const buckets = wrapper.findAll('.tracking > div')
    expect(buckets).toHaveLength(3)
    // No checks at all -> neutral.
    expect(buckets[0].classes()).toContain('bg-secondary-lt')
    expect(buckets[0].attributes('title')).toContain('aucun check')
    // All checks succeeded -> success color.
    expect(buckets[1].classes()).toContain('bg-success')
    expect(buckets[1].attributes('title')).toContain('5 OK')
    // At least one failure in the bucket -> danger color, even though some succeeded.
    expect(buckets[2].classes()).toContain('bg-danger')
    expect(buckets[2].attributes('title')).toContain('3 KO / 5')

    // The chart placeholder (stubbed) replaces the loading skeleton once
    // chartData/chartOptions are both populated.
    expect(wrapper.find('.chart-body').html()).not.toContain('loading-skeleton')
  })

  it('re-derives chart categories (not just the initial build) when the stats window changes', async () => {
    // Custom stub exposing the imperative updateOptions() the component
    // calls on a data refresh after the first build — the default
    // shallowMount auto-stub has no such method.
    const wrapper = shallowMount(UptimeDetailSection, {
      props: { probeId: 'probe-1' },
      global: {
        stubs: {
          ApexChart: { template: '<div class="apex-chart-stub" />', methods: { updateOptions: () => Promise.resolve() } },
        },
      },
    })
    await flushPromises()
    await flushPromises()
    getUptimeHistoryBuckets.mockClear()

    const sevenDayButton = wrapper.findAll('button').find((b) => b.text() === '7j')
    await sevenDayButton?.trigger('click')
    await flushPromises()

    expect(getUptimeHistoryBuckets).toHaveBeenCalledWith('probe-1', 168, expect.anything())
  })
})

describe('UptimeDetailSection — error state', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('shows the error alert instead of the KPI cards when the fetch fails', async () => {
    getUptimeProbe.mockRejectedValue(new Error('boom'))
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: null })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })

    const wrapper = mount(UptimeDetailSection, { props: { probeId: 'probe-1' } })
    await flushPromises()

    expect(wrapper.find('.alert-danger').exists()).toBe(true)
    expect(wrapper.text()).toContain('boom')
    vi.clearAllMocks()
  })
})
