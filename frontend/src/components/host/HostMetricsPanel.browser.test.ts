import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// Real-browser test: ApexCharts (SVG-based) actually renders here (Chromium
// provides real layout), unlike happy-dom. Only the API is mocked.
vi.mock('../../api', () => ({
  default: {
    getMetricsHistory: vi.fn(async () => ({
      data: [
        { timestamp: '2026-05-29T10:00:00Z', cpu_usage_percent: 20, memory_percent: 40, memory_used: 4000, memory_total: 10000 },
        { timestamp: '2026-05-29T11:00:00Z', cpu_usage_percent: 35, memory_percent: 45, memory_used: 4500, memory_total: 10000 },
        { timestamp: '2026-05-29T12:00:00Z', cpu_usage_percent: 50, memory_percent: 48, memory_used: 4800, memory_total: 10000 },
      ],
    })),
  },
}))

import HostMetricsPanel from './HostMetricsPanel.vue'

describe('HostMetricsPanel (browser / real render)', () => {
  it('renders both the CPU and memory ApexCharts SVGs once history has loaded', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    mount(HostMetricsPanel, {
      attachTo: host,
      props: { hostId: 'host-1', metrics: { cpu_usage_percent: 50, memory_percent: 48 } },
    })

    await flushPromises()

    // Two independent charts (CPU + memory) — both must render their own SVG.
    await expect.poll(() => host.querySelectorAll('svg.apexcharts-svg').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(2)
    const chartsSized = Array.from(host.querySelectorAll('svg.apexcharts-svg'))
      .every((svg) => (svg as SVGSVGElement).clientWidth > 0)
    expect(chartsSized).toBe(true)

    await expect.poll(() => host.querySelectorAll('svg .apexcharts-series path').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(2)
  })
})
