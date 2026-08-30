import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const {
  getProxmoxGuests, getProxmoxGuestLink, getProxmoxGuestMetrics,
  getProxmoxNodes, getProxmoxNodeGuestNetworks, getProxmoxInstance,
} = vi.hoisted(() => ({
  getProxmoxGuests: vi.fn(),
  getProxmoxGuestLink: vi.fn(),
  getProxmoxGuestMetrics: vi.fn(),
  getProxmoxNodes: vi.fn(),
  getProxmoxNodeGuestNetworks: vi.fn(),
  getProxmoxInstance: vi.fn(),
}))

let routeQuery: Record<string, string> = {}

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'guest-1' }, query: routeQuery }),
}))

vi.mock('../api', () => ({
  default: {
    getProxmoxGuests, getProxmoxGuestLink, getProxmoxGuestMetrics,
    getProxmoxNodes, getProxmoxNodeGuestNetworks, getProxmoxInstance,
  },
  getApiErrorMessage: (e: unknown) => String(e),
  isApiAbort: () => false,
}))

import { useProxmoxGuest } from './useProxmoxGuest'

const guest = {
  id: 'guest-1',
  connection_id: 'conn-1',
  node_name: 'pve-node-a',
  guest_type: 'lxc',
  vmid: 101,
  name: 'web-lxc',
  status: 'running',
  cpu_alloc: 2,
  cpu_usage: 0.1,
  mem_alloc: 1024,
  mem_usage: 512,
  disk_alloc: 1024,
  disk_usage: 512,
  uptime: 1000,
}

function mountUseProxmoxGuest() {
  let api!: ReturnType<typeof useProxmoxGuest>
  const wrapper = mount({
    setup() {
      api = useProxmoxGuest()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

beforeEach(() => {
  vi.clearAllMocks()
  routeQuery = {}
  getProxmoxGuests.mockResolvedValue({ data: [guest] })
  getProxmoxGuestLink.mockResolvedValue({ data: null })
  getProxmoxGuestMetrics.mockResolvedValue({ data: [] })
  getProxmoxNodeGuestNetworks.mockResolvedValue({ data: { 101: [{ name: 'eth0', ips: ['10.0.0.5'] }] } })
  getProxmoxInstance.mockResolvedValue({ data: { console_configured: true } })
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('useProxmoxGuest — CPU/RAM chart time axis', () => {
  it('pins xaxis to the requested window rather than the span of returned points', async () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.spyOn(Date, 'now').mockReturnValue(now)
    // Sparse data covering only the last 5 minutes — before the fix, xaxis
    // used to shrink to this span instead of the full default 24h range.
    getProxmoxGuestMetrics.mockResolvedValue({
      data: [
        { timestamp: new Date(now - 5 * 60 * 1000).toISOString(), cpu_avg: 10, memory_avg: 20 },
        { timestamp: new Date(now - 1 * 60 * 1000).toISOString(), cpu_avg: 12, memory_avg: 22 },
      ],
    })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(api.chartOptions.value?.xaxis?.min).toBe(now - 24 * 60 * 60 * 1000)
    expect(api.chartOptions.value?.xaxis?.max).toBe(now)
  })

  it('breaks the CPU/RAM line across a real gap instead of interpolating across it', async () => {
    const now = Date.now()
    // Default range is 24h (5-minute buckets) — a ~59-minute gap between
    // samples is well past the 3-bucket tolerance and must break the line.
    getProxmoxGuestMetrics.mockResolvedValue({
      data: [
        { timestamp: new Date(now - 60 * 60 * 1000).toISOString(), cpu_avg: 0, memory_avg: 0 },
        { timestamp: new Date(now - 1 * 60 * 1000).toISOString(), cpu_avg: 50, memory_avg: 60 },
      ],
    })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    const cpuSeries = api.series.value?.find((s) => s.name === 'CPU %')
    expect(cpuSeries?.data.some((p) => p.y === null)).toBe(true)
  })

  it('updates xaxis to the new window on changeRange(), not just on the initial load', async () => {
    // Regression test: loadGuestSummary sets summaryLoading true for its
    // whole duration, and the view only renders <ApexChart> while
    // !summaryLoading — so a prior version that pushed xaxis.min/max via
    // the chart instance's exposed updateOptions() method reached an
    // already-unmounted (ref already null) instance on every range change,
    // silently leaving the axis frozen at whatever the very first load
    // computed it as, regardless of which range button was clicked
    // afterwards. chartOptions must instead be rebuilt directly.
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.spyOn(Date, 'now').mockReturnValue(now)
    getProxmoxGuestMetrics.mockResolvedValue({
      data: [{ timestamp: new Date(now - 1 * 60 * 1000).toISOString(), cpu_avg: 10, memory_avg: 20 }],
    })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()
    expect(api.chartOptions.value?.xaxis?.min).toBe(now - 24 * 60 * 60 * 1000)

    api.changeRange(6)
    await flushPromises()

    expect(api.chartOptions.value?.xaxis?.min).toBe(now - 6 * 60 * 60 * 1000)
    expect(api.chartOptions.value?.xaxis?.max).toBe(now)
  })
})

describe('useProxmoxGuest — guest network IPs without a ?nodeId= query param', () => {
  it('resolves the node id from the guest itself when reached without ?nodeId= (dashboard/host-detail/bookmark links)', async () => {
    getProxmoxNodes.mockResolvedValue({ data: [
      { id: 'node-db-id', connection_id: 'conn-1', node_name: 'pve-node-a' },
      { id: 'other-node', connection_id: 'conn-1', node_name: 'pve-node-b' },
    ] })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxNodes).toHaveBeenCalledWith('conn-1', expect.anything())
    expect(getProxmoxNodeGuestNetworks).toHaveBeenCalledWith('node-db-id')
    expect(api.guestNetworks.value).toEqual([{ name: 'eth0', ips: ['10.0.0.5'] }])
    // The resolved node id is also exposed for the view's "back to node"
    // breadcrumb link — it used to read route.query.nodeId directly there
    // and break the same way the network fetch did.
    expect(api.nodeId.value).toBe('node-db-id')
  })

  it('still prefers the ?nodeId= query param when present, without an extra lookup', async () => {
    routeQuery = { nodeId: 'node-from-query' }

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxNodes).not.toHaveBeenCalled()
    expect(getProxmoxNodeGuestNetworks).toHaveBeenCalledWith('node-from-query')
    expect(api.guestNetworks.value).toEqual([{ name: 'eth0', ips: ['10.0.0.5'] }])
    expect(api.nodeId.value).toBe('node-from-query')
  })

  it('leaves guestNetworks empty (not an error) when the node cannot be resolved', async () => {
    getProxmoxNodes.mockResolvedValue({ data: [] })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxNodeGuestNetworks).not.toHaveBeenCalled()
    expect(api.guestNetworks.value).toEqual([])
    expect(api.nodeId.value).toBe('')
  })
})

describe('useProxmoxGuest — chart time formatting', () => {
  it('formats the x-axis/tooltip time as HH:mm for windows under 24h, and DD/MM HH:mm at/above 24h', async () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.spyOn(Date, 'now').mockReturnValue(now)
    getProxmoxGuestMetrics.mockResolvedValue({
      data: [{ timestamp: new Date(now - 60_000).toISOString(), cpu_avg: 10, memory_avg: 20 }],
    })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    // Default range is 24h.
    const xaxisFormatter = api.chartOptions.value?.xaxis?.labels?.formatter as (v: string) => string
    expect(xaxisFormatter(String(now))).toMatch(/^\d{2}\/\d{2} \d{2}:\d{2}$/)

    api.changeRange(6)
    await flushPromises()
    const shortRangeFormatter = api.chartOptions.value?.xaxis?.labels?.formatter as (v: string) => string
    expect(shortRangeFormatter(String(now))).toMatch(/^\d{2}:\d{2}$/)

    // Tooltip x/y formatters delegate to the same helper / a plain % format.
    const tooltip = api.chartOptions.value?.tooltip as { x?: { formatter: (v: number) => string }; y?: { formatter: (v: number | null) => string } }
    expect(tooltip.x?.formatter(now)).toMatch(/^\d{2}:\d{2}$/)
    expect(tooltip.y?.formatter(42.567)).toBe('42.6%')
    expect(tooltip.y?.formatter(null as unknown as number)).toBe('—')
  })

  it('returns an empty string from the time formatter for an invalid/non-finite timestamp', async () => {
    getProxmoxGuestMetrics.mockResolvedValue({
      data: [{ timestamp: new Date().toISOString(), cpu_avg: 10, memory_avg: 20 }],
    })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    const formatter = api.chartOptions.value?.xaxis?.labels?.formatter as (v: string) => string
    expect(formatter('not-a-number')).toBe('')
  })
})

describe('useProxmoxGuest — summary fetch failure', () => {
  it('clears the series (not a thrown error) when the metrics fetch rejects', async () => {
    getProxmoxGuestMetrics.mockRejectedValue(new Error('boom'))

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(api.series.value).toBeNull()
    // Regression: chartOptions used to stay stale (from a prior successful
    // load's time window) when a reload failed — the template's
    // `v-else-if="series && chartOptions"` masked it today, but only by
    // accident of both being checked together.
    expect(api.chartOptions.value).toBeNull()
    expect(api.summaryLoading.value).toBe(false)
  })

  it('clears both series and chartOptions when a reload returns zero points', async () => {
    const now = Date.now()
    getProxmoxGuestMetrics.mockResolvedValueOnce({
      data: [{ timestamp: new Date(now).toISOString(), cpu_avg: 10, memory_avg: 20 }],
    })
    const { api } = mountUseProxmoxGuest()
    await flushPromises()
    expect(api.series.value).not.toBeNull()
    expect(api.chartOptions.value).not.toBeNull()

    getProxmoxGuestMetrics.mockResolvedValueOnce({ data: [] })
    api.changeRange(1)
    await flushPromises()

    expect(api.series.value).toBeNull()
    expect(api.chartOptions.value).toBeNull()
  })
})

describe('useProxmoxGuest — console configuration check', () => {
  it('looks up console_configured on the guest\'s own connection once loaded', async () => {
    getProxmoxInstance.mockResolvedValue({ data: { console_configured: false } })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxInstance).toHaveBeenCalledWith('conn-1')
    expect(api.consoleConfigured.value).toBe(false)
    expect(api.consoleButtonTitle.value).toContain('non configurée')
  })

  it('fails open (null) when the console check itself errors, without blocking the button', async () => {
    getProxmoxInstance.mockRejectedValue(new Error('network error'))

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(api.consoleConfigured.value).toBeNull()
    expect(api.consoleButtonTitle.value).toBe('Ouvrir une console interactive')
  })

  it('offers the console once configured is confirmed true for an lxc guest', async () => {
    getProxmoxInstance.mockResolvedValue({ data: { console_configured: true } })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(api.consoleButtonTitle.value).toBe('Ouvrir une console interactive')
  })

  it('never checks console config for a non-lxc (QEMU) guest and shows the "coming soon" title', async () => {
    getProxmoxGuests.mockResolvedValue({ data: [{ ...guest, guest_type: 'qemu' }] })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxInstance).not.toHaveBeenCalled()
    expect(api.consoleButtonTitle.value).toBe('VM QEMU : bientôt disponible')
  })
})
