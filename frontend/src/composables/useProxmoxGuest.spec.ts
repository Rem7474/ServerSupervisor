import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const {
  getProxmoxGuests, getProxmoxGuestLink, getProxmoxGuestMetrics,
  getProxmoxNodes, getProxmoxNodeGuestNetworks,
} = vi.hoisted(() => ({
  getProxmoxGuests: vi.fn(),
  getProxmoxGuestLink: vi.fn(),
  getProxmoxGuestMetrics: vi.fn(),
  getProxmoxNodes: vi.fn(),
  getProxmoxNodeGuestNetworks: vi.fn(),
}))

let routeQuery: Record<string, string> = {}

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'guest-1' }, query: routeQuery }),
}))

vi.mock('../api', () => ({
  default: {
    getProxmoxGuests, getProxmoxGuestLink, getProxmoxGuestMetrics,
    getProxmoxNodes, getProxmoxNodeGuestNetworks,
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
  })

  it('still prefers the ?nodeId= query param when present, without an extra lookup', async () => {
    routeQuery = { nodeId: 'node-from-query' }

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxNodes).not.toHaveBeenCalled()
    expect(getProxmoxNodeGuestNetworks).toHaveBeenCalledWith('node-from-query')
    expect(api.guestNetworks.value).toEqual([{ name: 'eth0', ips: ['10.0.0.5'] }])
  })

  it('leaves guestNetworks empty (not an error) when the node cannot be resolved', async () => {
    getProxmoxNodes.mockResolvedValue({ data: [] })

    const { api } = mountUseProxmoxGuest()
    await flushPromises()

    expect(getProxmoxNodeGuestNetworks).not.toHaveBeenCalled()
    expect(api.guestNetworks.value).toEqual([])
  })
})
