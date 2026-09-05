import { describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeChartsPanel from './ProxmoxNodeChartsPanel.vue'
import type { RRDChartSeries } from './RRDChartCard.vue'

// Real-browser test: ApexCharts (SVG-based) actually renders here (Chromium
// provides real layout), unlike happy-dom. This panel receives its series
// as plain props (no internal fetch), so no API mocking is needed.
const series = (name: string): RRDChartSeries => [
  { name, data: [{ x: 1748509200000, y: 20 }, { x: 1748512800000, y: 35 }, { x: 1748516400000, y: 28 }] },
]

describe('ProxmoxNodeChartsPanel (browser / real render)', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders one real ApexCharts SVG per non-empty RRD series (CPU/RAM/network)', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    mount(ProxmoxNodeChartsPanel, {
      attachTo: host,
      props: {
        cpuChart: series('CPU'),
        ramChart: series('RAM'),
        netChart: series('Réseau'),
      },
    })

    await flushPromises()

    // CPU, RAM and network each got real data — iowait/temp/fan were left
    // null, so their RRDChartCard falls back to its empty-text branch
    // instead of mounting a chart.
    await expect.poll(() => host.querySelectorAll('svg.apexcharts-svg').length, { timeout: 8000 })
      .toBe(3)
    const chartsSized = Array.from(host.querySelectorAll('svg.apexcharts-svg'))
      .every((svg) => (svg as SVGSVGElement).clientWidth > 0)
    expect(chartsSized).toBe(true)

    expect(host.textContent).toContain('Aucune donnée')
  })
})
