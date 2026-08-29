import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

const {
  getAptCVESummary, getMetricsSummary, getProxmoxNodeMetrics,
  getProxmoxSummary, getSettings, sendAptCommand,
} = vi.hoisted(() => ({
  getAptCVESummary: vi.fn(),
  getMetricsSummary: vi.fn(),
  getProxmoxNodeMetrics: vi.fn(),
  getProxmoxSummary: vi.fn(),
  getSettings: vi.fn(),
  sendAptCommand: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getAptCVESummary, getMetricsSummary, getProxmoxNodeMetrics,
    getProxmoxSummary, getSettings, sendAptCommand,
  },
}))

// The real WebSocket connection/auth gating is irrelevant to the summary
// chart logic under test here — stub it out the same way a plain data
// object would satisfy the composable's destructuring.
vi.mock('./useWebSocket', () => ({
  useWebSocket: () => ({
    wsStatus: ref('connected'), wsError: ref(''), retryCount: ref(0),
    dataStaleAlert: ref(false), reconnect: vi.fn(),
  }),
  wsEvents: { on: vi.fn(), off: vi.fn() },
}))

import { useDashboard } from './useDashboard'

function mountUseDashboard() {
  let api!: ReturnType<typeof useDashboard>
  const wrapper = mount({
    setup() {
      api = useDashboard()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

describe('useDashboard — summary chart series/options', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
    getAptCVESummary.mockResolvedValue({ data: null })
    getProxmoxSummary.mockResolvedValue({ data: {} })
    getSettings.mockResolvedValue({ data: { settings: {} } })
    getMetricsSummary.mockResolvedValue({ data: [] })
    getProxmoxNodeMetrics.mockResolvedValue({ data: [] })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('builds CPU/RAM series from the agents endpoint by default, clamping a future (clock-skewed) point to now', async () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.useFakeTimers()
    vi.setSystemTime(now)

    getMetricsSummary.mockResolvedValue({
      data: [
        { timestamp: new Date(now - 60_000).toISOString(), cpu_avg: 10, memory_avg: 20 },
        { timestamp: new Date(now + 5 * 60_000).toISOString(), cpu_avg: 90, memory_avg: 95 }, // future -> clamped to now
      ],
    })

    const { api } = mountUseDashboard()
    await flushPromises()

    expect(getMetricsSummary).toHaveBeenCalled()
    expect(getProxmoxNodeMetrics).not.toHaveBeenCalled()
    const cpuSeries = api.summaryChartSeries.value?.find((s) => s.name === 'CPU %')
    expect(cpuSeries?.data).toEqual([
      { x: now - 60_000, y: 10 },
      { x: now, y: 90 },
    ])

    vi.useRealTimers()
  })

  it('switches to the Proxmox node-metrics endpoint when chartSource is "proxmox"', async () => {
    const { api } = mountUseDashboard()
    await flushPromises()
    getMetricsSummary.mockClear()
    getProxmoxNodeMetrics.mockClear()
    getProxmoxNodeMetrics.mockResolvedValue({ data: [{ timestamp: new Date().toISOString(), cpu_avg: 5, memory_avg: 6 }] })

    api.chartSource.value = 'proxmox'
    await api.fetchSummary()

    expect(getProxmoxNodeMetrics).toHaveBeenCalled()
    expect(getMetricsSummary).not.toHaveBeenCalled()
    expect(api.summaryChartSeries.value).not.toBeNull()
  })

  it('clears the series (not an error) when the endpoint returns no points', async () => {
    getMetricsSummary.mockResolvedValue({ data: [] })

    const { api } = mountUseDashboard()
    await flushPromises()

    expect(api.summaryChartSeries.value).toBeNull()
  })

  it('clears the series when the fetch itself fails', async () => {
    getMetricsSummary.mockRejectedValue(new Error('network down'))

    const { api } = mountUseDashboard()
    await flushPromises()

    expect(api.summaryChartSeries.value).toBeNull()
    expect(api.summaryLoading.value).toBe(false)
  })

  it('derives the xaxis min/max from the actual series data, not a fixed window', async () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.useFakeTimers()
    vi.setSystemTime(now)
    getMetricsSummary.mockResolvedValue({
      data: [
        { timestamp: new Date(now - 3 * 60_000).toISOString(), cpu_avg: 1, memory_avg: 2 },
        { timestamp: new Date(now - 1 * 60_000).toISOString(), cpu_avg: 3, memory_avg: 4 },
      ],
    })

    const { api } = mountUseDashboard()
    await flushPromises()

    expect(api.summaryChartOptions.value.xaxis?.min).toBe(now - 3 * 60_000)
    expect(api.summaryChartOptions.value.xaxis?.max).toBe(now - 1 * 60_000)

    vi.useRealTimers()
  })
})
