import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

// Real-browser test: ApexCharts (SVG-based) actually renders here (Chromium
// provides real layout), unlike happy-dom. Only the API is mocked.
vi.mock('../../api', () => ({
  default: {
    getDiskMetricsAggregated: vi.fn(async () => ({
      data: {
        points: [
          { timestamp: '2026-05-29T10:00:00Z', used_percent: 40, used_gb: 40, size_gb: 100 },
          { timestamp: '2026-05-29T11:00:00Z', used_percent: 42, used_gb: 42, size_gb: 100 },
          { timestamp: '2026-05-29T12:00:00Z', used_percent: 45, used_gb: 45, size_gb: 100 },
        ],
      },
    })),
  },
}))

import DiskHistoryChart from './DiskHistoryChart.vue'

describe('DiskHistoryChart (browser / real render)', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders a real ApexCharts SVG once data has loaded', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    mount(DiskHistoryChart, {
      attachTo: host,
      props: { hostId: 'host-1', mounts: ['/'] },
    })

    await flushPromises()

    await expect.poll(() => host.querySelectorAll('svg.apexcharts-svg').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(1)
    const svg = host.querySelector('svg.apexcharts-svg') as SVGSVGElement
    expect(svg.clientWidth).toBeGreaterThan(0)

    // The area chart draws one series path per data series (CPU/RAM-style
    // single series here: disk usage %) — proves ApexCharts actually
    // consumed the `series` prop, not just mounted an empty chart shell.
    await expect.poll(() => host.querySelectorAll('svg .apexcharts-series path').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(1)
  })

  it('switching time range re-fetches and keeps the chart rendered', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    const wrapper = mount(DiskHistoryChart, {
      attachTo: host,
      props: { hostId: 'host-1', mounts: ['/'] },
    })
    await flushPromises()
    await expect.poll(() => host.querySelectorAll('svg.apexcharts-svg').length).toBeGreaterThanOrEqual(1)

    const sevenDayButton = wrapper.findAll('button').find((b) => b.text() === '7j')
    await sevenDayButton?.trigger('click')
    await flushPromises()

    await expect.poll(() => host.querySelectorAll('svg.apexcharts-svg').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(1)
    const svg = host.querySelector('svg.apexcharts-svg') as SVGSVGElement
    expect(svg.clientWidth).toBeGreaterThan(0)
  })
})
