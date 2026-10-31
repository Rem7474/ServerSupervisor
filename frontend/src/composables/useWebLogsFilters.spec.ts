import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
const replace = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace }) }))

import { useWebLogsFilters } from './useWebLogsFilters'

function mountFilters() {
  let api!: ReturnType<typeof useWebLogsFilters>
  mount(defineComponent({ setup() { api = useWebLogsFilters(); return () => h('div') } }))
  return api
}

describe('useWebLogsFilters', () => {
  beforeEach(() => {
    route.query = {}
    replace.mockReset()
    setLocale('fr')
  })

  it('starts from the URL query, in custom mode when a from/to range is present', () => {
    route.query = { period: '168h', source: 'npm', host_id: 'h1', from: '2026-09-01T00:00:00Z', to: '2026-09-02T00:00:00Z' }
    const f = mountFilters()
    expect(f.timeRange.value).toMatchObject({ mode: 'custom', period: '168h', from: '2026-09-01T00:00:00Z' })
    expect(f.detailFilters()).toEqual({ period: '168h', hostId: 'h1', source: 'npm', from: '2026-09-01T00:00:00Z', to: '2026-09-02T00:00:00Z' })
  })

  it('defaults to the last 24 h with no host or source', () => {
    const f = mountFilters()
    expect(f.timeRange.value.mode).toBe('preset')
    expect(f.detailFilters()).toEqual({ period: '24h', hostId: undefined, source: undefined, from: undefined, to: undefined })
  })

  it('applies a custom range, then a preset that clears it', () => {
    const f = mountFilters()
    f.timeRange.value = { mode: 'custom', period: '24h', from: 'a', to: 'b' }
    f.applyTimeRange()
    expect([f.from.value, f.to.value]).toEqual(['a', 'b'])

    f.timeRange.value = { mode: 'preset', period: '720h', from: null, to: null }
    f.applyTimeRange()
    expect([f.period.value, f.from.value, f.to.value]).toEqual(['720h', null, null])
  })

  it('mirrors filter changes into the URL, dropping empty values', async () => {
    const f = mountFilters()
    f.hostId.value = 'h2'
    await nextTick()
    expect(replace).toHaveBeenLastCalledWith({ query: { period: '24h', source: undefined, host_id: 'h2', from: undefined, to: undefined } })
  })

  it('labels the multi-day presets in the UI language', () => {
    const f = mountFilters()
    expect(f.periodOptions.value.map((o) => o.label)).toEqual(['1h', '24h', '7j', '30j'])
    setLocale('en')
    expect(f.periodOptions.value.map((o) => o.label)).toEqual(['1h', '24h', '7d', '30d'])
  })
})
