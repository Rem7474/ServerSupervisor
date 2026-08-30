import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// Real-browser test: ApexCharts (SVG-based) actually renders here (Chromium
// provides real layout), unlike happy-dom. Only the API is mocked.
vi.mock('../../api', () => ({
  default: {
    getNetworkFlowsSummary: vi.fn(async () => ({
      data: {
        points: [
          { timestamp: '2026-05-29T10:00:00Z', rx_bytes: 1024, tx_bytes: 512 },
          { timestamp: '2026-05-29T11:00:00Z', rx_bytes: 2048, tx_bytes: 1024 },
          { timestamp: '2026-05-29T12:00:00Z', rx_bytes: 4096, tx_bytes: 2048 },
        ],
      },
    })),
  },
}))

import NetworkFlowsHistoryChart from './NetworkFlowsHistoryChart.vue'

describe('NetworkFlowsHistoryChart (browser / real render)', () => {
  it('renders a real ApexCharts SVG with RX/TX series once summary data has loaded', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    mount(NetworkFlowsHistoryChart, {
      attachTo: host,
      props: { hostId: 'host-1', mode: 'summary' },
    })

    await flushPromises()

    await expect.poll(() => host.querySelectorAll('svg.apexcharts-svg').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(1)
    const svg = host.querySelector('svg.apexcharts-svg') as SVGSVGElement
    expect(svg.clientWidth).toBeGreaterThan(0)

    // RX + TX are two distinct series — two drawn series paths proves both
    // made it into the chart, not just one flattened line.
    await expect.poll(() => host.querySelectorAll('svg .apexcharts-series path').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(2)
  })
})
