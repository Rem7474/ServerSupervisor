import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Real-browser test: Chart.js + the D3/topojson world map actually render here
// (Chromium provides a real 2D canvas context + layout), unlike happy-dom.
// Only the API is mocked; charting and map libraries run for real.
const summaryData = {
  data: {
    traffic: {
      total_requests: 4096,
      total_bytes: 10485760,
      ratio_5xx: 0.02,
      status_distribution: { '2xx': 300, '3xx': 20, '4xx': 40, '5xx': 8 },
      top_domains: [{ domain: 'example.com', hits: 200 }],
      country_distribution: [
        { country: 'France', country_code: 'FR', hits: 120 },
        { country: 'United States of America', country_code: 'US', hits: 80 },
      ],
      top_client_ips: [],
    },
    threats: { suspicious_ips: 5, top_ips: [] },
    compare: { delta_percent: {} },
  },
}

vi.mock('../api', () => ({
  default: {
    getWebLogsSummary: vi.fn(async () => summaryData),
    getWebLogsTimeseries: vi.fn(async () => ({
      data: {
        points: [
          { ts: '2026-05-29T10:00:00Z', total: 100, bots: 10 },
          { ts: '2026-05-29T11:00:00Z', total: 140, bots: 25 },
        ],
      },
    })),
    getWebLogsLive: vi.fn(async () => ({ data: { requests: [] } })),
    getDomainDetails: vi.fn(async () => ({ data: {} })),
  },
}))

import TrafficView from './TrafficView.vue'

describe('TrafficView (browser / real render)', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    // TrafficView calls useHostsStore() in setup; install a fresh Pinia so the
    // store resolves without the app having to register the plugin.
    setActivePinia(createPinia())
  })

  it('renders chart canvases and the SVG world map for real', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    mount(TrafficView, {
      attachTo: host,
      global: { stubs: { 'router-link': true } },
    })

    await flushPromises()
    // Initial render draws the map only once the content (not the skeleton) is
    // mounted; the component redraws on resize. Trigger it deterministically.
    await new Promise((r) => setTimeout(r, 50))
    window.dispatchEvent(new Event('resize'))

    // The two Chart.js canvases (requests + status) are present and laid out
    // with real dimensions — proof the extracted chart sub-components rendered.
    await expect.poll(() => host.querySelectorAll('canvas').length, { timeout: 8000 })
      .toBeGreaterThanOrEqual(2)
    const canvasesSized = Array.from(host.querySelectorAll('canvas'))
      .every((c) => (c as HTMLCanvasElement).width > 0)
    expect(canvasesSized).toBe(true)

    // The D3 world map renders one <path class="country"> per country feature
    // (≈177). This is the assertion happy-dom cannot satisfy — it proves the
    // topojson → geoPath pipeline actually drew into the SVG.
    await expect.poll(() => host.querySelectorAll('svg path.country').length, { timeout: 8000 })
      .toBeGreaterThan(50)

    // At least one country is shaded (has a non-default fill) from the data.
    const filled = Array.from(host.querySelectorAll('svg path.country'))
      .some((p) => {
        const fill = p.getAttribute('fill') || ''
        return fill !== '' && fill !== '#e9edf2'
      })
    expect(filled).toBe(true)
  })
})
