import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { fetchMetricsHistory } = vi.hoisted(() => ({ fetchMetricsHistory: vi.fn() }))

vi.mock('../../composables/useHostMetricsHistory', () => ({ fetchMetricsHistory }))

import HostMetricsPanel from './HostMetricsPanel.vue'

const mountOpts = { global: { stubs: { ApexChart: true } } }

const baseMetrics = {
  cpu_cores: 4,
  cpu_usage_percent: 42.567,
  cpu_model: 'Intel Xeon',
  memory_percent: 55.4,
  memory_used: 4 * 1024 * 1024 * 1024,
  memory_total: 8 * 1024 * 1024 * 1024,
  uptime: 90000,
  load_avg_1: 1.234,
  load_avg_5: 0.98,
  load_avg_15: 0.5,
}

describe('HostMetricsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    fetchMetricsHistory.mockResolvedValue([])
  })

  it('renders the CPU/RAM/uptime/load-avg metric cards', async () => {
    const wrapper = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: baseMetrics },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('CPU (4 CORES)')
    expect(wrapper.text()).toContain('42.6%')
    expect(wrapper.text()).toContain('Intel Xeon')
    expect(wrapper.text()).toContain('55.4%')
    expect(wrapper.text()).toContain('4 GiB / 8 GiB')
    expect(wrapper.text()).toContain('1j 1h')
    expect(wrapper.text()).toContain('1.23')
  })

  it('formats uptime as hours/minutes when under a day', async () => {
    const wrapper = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: { ...baseMetrics, uptime: 3900 } },
      ...mountOpts,
    })
    await flushPromises()
    expect(wrapper.text()).toContain('1h 5m')
  })

  it('shows N/A when uptime is missing', async () => {
    const wrapper = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: { ...baseMetrics, uptime: undefined } },
      ...mountOpts,
    })
    await flushPromises()
    expect(wrapper.text()).toContain('N/A')
  })

  it('shows the CPU temperature block only when a temperature is reported', async () => {
    const withTemp = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: { ...baseMetrics, cpu_temperature: 62.3 } },
      ...mountOpts,
    })
    await flushPromises()
    expect(withTemp.text()).toContain('62.3°C')

    const withoutTemp = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: baseMetrics },
      ...mountOpts,
    })
    await flushPromises()
    expect(withoutTemp.text()).not.toContain('°C')
  })

  it('shows the "no data" placeholder for both charts when history is empty', async () => {
    const wrapper = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: baseMetrics },
      ...mountOpts,
    })
    await flushPromises()
    const noDataBlocks = wrapper.findAll('.text-secondary').filter((el) => el.text() === 'Aucune donnée')
    expect(noDataBlocks.length).toBe(2)
    expect(wrapper.text()).toContain('Mémoire')
  })

  it('loads a new time range when a chart-range button is clicked', async () => {
    const wrapper = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: baseMetrics },
      ...mountOpts,
    })
    await flushPromises()
    fetchMetricsHistory.mockClear()

    const sevenDayButton = wrapper.findAll('button').find((b) => b.text() === '7d')
    await sevenDayButton?.trigger('click')
    await flushPromises()

    expect(fetchMetricsHistory).toHaveBeenCalledWith('h1', 168, 'agent', null)
  })

  it('renders charts once history data is available', async () => {
    fetchMetricsHistory.mockResolvedValue([
      { timestamp: new Date().toISOString(), cpu_usage_percent: 10, memory_percent: 20, memory_used: 1000, memory_total: 2000 },
    ])
    const wrapper = mount(HostMetricsPanel, {
      props: { hostId: 'h1', metrics: baseMetrics },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.findAllComponents({ name: 'ApexChart' }).length).toBe(2)
  })
})
