import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { getUptimeProbe, getUptimeHistory, getUptimeStats, getUptimeHistoryBuckets } = vi.hoisted(() => ({
  getUptimeProbe: vi.fn(),
  getUptimeHistory: vi.fn(),
  getUptimeStats: vi.fn(),
  getUptimeHistoryBuckets: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'probe-1' } }),
}))

vi.mock('../api', () => ({
  default: { getUptimeProbe, getUptimeHistory, getUptimeStats, getUptimeHistoryBuckets },
}))

import { useUptimeProbeDetail } from './useUptimeProbeDetail'

// useI18n() needs an active component instance.
function mountHost() {
  let api: ReturnType<typeof useUptimeProbeDetail> | undefined
  const wrapper = mount(defineComponent({
    setup() {
      api = useUptimeProbeDetail()
      return () => h('div')
    },
  }))
  return { wrapper, api: api! }
}

describe('useUptimeProbeDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('falls back to the translated "unknown" status label for a probe with neither up nor down status', async () => {
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'pending' } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: {} })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })

    const { api } = mountHost()
    await vi.waitFor(() => expect(api.probe.value).not.toBeNull())

    expect(api.statusLabel.value).toBe('Inconnue')
  })

  it('surfaces the translated fallback error when the initial load fails', async () => {
    getUptimeProbe.mockRejectedValue(new Error())
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: {} })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })

    const { api } = mountHost()
    await vi.waitFor(() => expect(api.loading.value).toBe(false))

    expect(api.error.value).toBe('Impossible de charger la sonde')
  })

  it('surfaces the translated fallback error when switching the stats window fails', async () => {
    getUptimeProbe.mockResolvedValue({ data: { id: 'probe-1', last_status: 'up' } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getUptimeStats.mockResolvedValue({ data: {} })
    getUptimeHistoryBuckets.mockResolvedValue({ data: { buckets: [] } })

    const { api } = mountHost()
    await vi.waitFor(() => expect(api.loading.value).toBe(false))

    getUptimeStats.mockRejectedValue(new Error())
    await api.setStatsWindow(24)

    expect(api.error.value).toBe('Impossible de charger les statistiques')
  })
})
