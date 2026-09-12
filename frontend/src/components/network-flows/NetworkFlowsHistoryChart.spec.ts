import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { fetchNetworkFlowsSummary, fetchNetworkFlowsHistory } = vi.hoisted(() => ({
  fetchNetworkFlowsSummary: vi.fn(),
  fetchNetworkFlowsHistory: vi.fn(),
}))

vi.mock('../../composables/useNetworkFlowsHistory', () => ({ fetchNetworkFlowsSummary, fetchNetworkFlowsHistory }))

import NetworkFlowsHistoryChart from './NetworkFlowsHistoryChart.vue'

const mountOpts = { global: { stubs: { ApexChart: true } } }

describe('NetworkFlowsHistoryChart', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('shows the summary-mode title and calls fetchNetworkFlowsSummary', async () => {
    fetchNetworkFlowsSummary.mockResolvedValue([])
    const wrapper = mount(NetworkFlowsHistoryChart, {
      props: { hostId: 'h1', mode: 'summary' },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Bande passante trackée')
    expect(fetchNetworkFlowsSummary).toHaveBeenCalledWith('h1', '24h', undefined)
  })

  it('shows the talker-mode title and calls fetchNetworkFlowsHistory with the talker params', async () => {
    fetchNetworkFlowsHistory.mockResolvedValue([])
    const wrapper = mount(NetworkFlowsHistoryChart, {
      props: { hostId: 'h1', mode: 'talker', remoteIp: '1.2.3.4', remotePort: 443, protocol: 'tcp' },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Historique du talker')
    expect(fetchNetworkFlowsHistory).toHaveBeenCalledWith('h1', '1.2.3.4', 443, 'tcp', '24h', undefined)
  })

  it('shows the "no data" placeholder when there are no points', async () => {
    fetchNetworkFlowsSummary.mockResolvedValue([])
    const wrapper = mount(NetworkFlowsHistoryChart, {
      props: { hostId: 'h1', mode: 'summary' },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucune donnée')
  })

  it('renders the chart once data is available', async () => {
    fetchNetworkFlowsSummary.mockResolvedValue([
      { timestamp: new Date().toISOString(), rx_bytes: 100, tx_bytes: 200 },
    ])
    const wrapper = mount(NetworkFlowsHistoryChart, {
      props: { hostId: 'h1', mode: 'summary' },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.findComponent({ name: 'ApexChart' }).exists()).toBe(true)
    expect(wrapper.find('.h-100.d-flex').exists()).toBe(false)
  })

  it('surfaces a fetch failure by showing the "no data" placeholder', async () => {
    fetchNetworkFlowsSummary.mockRejectedValue(new Error('boom'))
    const wrapper = mount(NetworkFlowsHistoryChart, {
      props: { hostId: 'h1', mode: 'summary' },
      ...mountOpts,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucune donnée')
  })

  it('re-fetches on a time-range change, passing a custom from/to range', async () => {
    fetchNetworkFlowsSummary.mockResolvedValue([])
    const wrapper = mount(NetworkFlowsHistoryChart, {
      props: { hostId: 'h1', mode: 'summary' },
      ...mountOpts,
    })
    await flushPromises()
    fetchNetworkFlowsSummary.mockClear()

    await wrapper.findComponent({ name: 'TimeRangePicker' }).vm.$emit(
      'update:modelValue',
      { mode: 'custom', period: '', from: '2026-01-01T00:00:00Z', to: '2026-01-02T00:00:00Z' },
    )
    await wrapper.findComponent({ name: 'TimeRangePicker' }).vm.$emit('change')
    await flushPromises()

    expect(fetchNetworkFlowsSummary).toHaveBeenCalledWith('h1', '24h', { from: '2026-01-01T00:00:00Z', to: '2026-01-02T00:00:00Z' })
  })
})
