import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'

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
      api = useProxmoxGuest(ref(null))
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
})
