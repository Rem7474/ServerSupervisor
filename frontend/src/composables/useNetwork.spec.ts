import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

const { wsHandler } = vi.hoisted(() => ({
  wsHandler: { current: null as ((payload: unknown) => void) | null },
}))
vi.mock('./useWebSocket', () => ({
  useWebSocket: (_url: string, onMessage: (payload: unknown) => void) => {
    wsHandler.current = onMessage
    return { wsStatus: { value: 'connected' }, wsError: { value: '' }, retryCount: { value: 0 }, reconnect: vi.fn() }
  },
}))

const api = vi.hoisted(() => ({
  getTopologyConfig: vi.fn(),
  saveTopologyConfig: vi.fn(),
  getNetworkSnapshot: vi.fn(),
  getIPInventory: vi.fn(),
}))
vi.mock('../api', () => ({ default: api }))

import { useNetwork } from './useNetwork'

function mountUseNetwork() {
  let result!: ReturnType<typeof useNetwork>
  const wrapper = mount({
    setup() {
      result = useNetwork()
      return () => null
    },
  })
  return { wrapper, n: result }
}

async function mountLoaded() {
  const ctx = mountUseNetwork()
  await flushPromises()
  return ctx
}

const PORT = { name: '', domain: '', path: '/', enabled: true, linkToProxy: false, linkToAuthelia: false, exposedToInternet: false, externalPort: null }

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  wsHandler.current = null
  api.getTopologyConfig.mockResolvedValue({ data: null })
  api.saveTopologyConfig.mockResolvedValue({})
  api.getNetworkSnapshot.mockResolvedValue({ data: { hosts: [], containers: [] } })
  api.getIPInventory.mockResolvedValue({ data: { proxmox_guests: [], npm_hosts: [] } })
})

afterEach(() => { vi.useRealTimers() })

describe('useNetwork — view mode', () => {
  it('defaults to the graph view and restores the stored mode', () => {
    expect(mountUseNetwork().n.viewMode.value).toBe('graph')
    localStorage.setItem('networkViewMode', 'table')
    expect(mountUseNetwork().n.viewMode.value).toBe('table')
  })

  it('persists the view mode when it changes', async () => {
    const { n } = await mountLoaded()
    n.viewMode.value = 'table'
    await nextTick()
    expect(localStorage.getItem('networkViewMode')).toBe('table')
  })
})

describe('useNetwork — topology config loading', () => {
  it('applies the saved config and parses its JSON fields', async () => {
    api.getTopologyConfig.mockResolvedValue({ data: {
      root_label: 'Core', root_ip: '10.0.0.1', authelia_label: 'Auth', authelia_ip: '10.0.0.2',
      internet_label: 'WAN', internet_ip: '1.2.3.4',
      manual_services: JSON.stringify([{ id: 's1', name: 'manual' }]),
      root_host_id: 'h1', authelia_host_id: 'h2', root_port_id: '80', authelia_port_id: '9091',
      node_positions: JSON.stringify({ a: { x: 1, y: 2 } }),
      host_overrides: JSON.stringify([{ hostId: 'h1', ports: {} }]),
    } })
    const { n } = await mountLoaded()
    expect(n.rootNodeName.value).toBe('Core')
    expect(n.rootNodeIp.value).toBe('10.0.0.1')
    expect(n.autheliaLabel.value).toBe('Auth')
    expect(n.internetLabel.value).toBe('WAN')
    expect(n.internetIp.value).toBe('1.2.3.4')
    expect(n.networkServices.value).toEqual([{ id: 's1', name: 'manual' }])
    expect([n.rootHostId.value, n.autheliaHostId.value, n.rootPortId.value, n.autheliaPortId.value]).toEqual(['h1', 'h2', '80', '9091'])
    expect(n.nodePositions.value).toEqual({ a: { x: 1, y: 2 } })
    expect(n.hostPortConfig.value).toEqual([{ hostId: 'h1', ports: {} }])
    expect(n.topologyConfigLoaded.value).toBe(true)
  })

  it('falls back to the default labels and empty values for an empty config', async () => {
    api.getTopologyConfig.mockResolvedValue({ data: {} })
    const { n } = await mountLoaded()
    expect(n.rootNodeName.value).toBe('Infrastructure')
    expect(n.autheliaLabel.value).toBe('Authelia')
    expect(n.internetLabel.value).toBe('Internet')
    expect(n.rootNodeIp.value).toBe('')
    expect(n.networkServices.value).toEqual([])
    expect(n.rootHostId.value).toBe('')
    expect(n.nodePositions.value).toEqual({})
  })

  it('resets positions and overrides when their stored JSON is corrupted', async () => {
    api.getTopologyConfig.mockResolvedValue({ data: { node_positions: '{not json', host_overrides: '[oops' } })
    const { n } = await mountLoaded()
    expect(n.nodePositions.value).toEqual({})
    expect(n.hostPortConfig.value).toEqual([])
    expect(n.topologyConfigLoaded.value).toBe(true)
  })

  it('keeps the defaults but still marks the config loaded when the request fails', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    api.getTopologyConfig.mockRejectedValue(new Error('boom'))
    const { n } = await mountLoaded()
    expect(n.rootNodeName.value).toBe('Infrastructure')
    expect(n.topologyConfigLoaded.value).toBe(true)
    expect(warn).toHaveBeenCalled()
    warn.mockRestore()
  })

  it('leaves defaults untouched when the response has no data', async () => {
    api.getTopologyConfig.mockResolvedValue({})
    const { n } = await mountLoaded()
    expect(n.rootNodeName.value).toBe('Infrastructure')
    expect(n.topologyConfigLoaded.value).toBe(true)
  })
})

describe('useNetwork — snapshot and inventory loading', () => {
  it('loads hosts and containers and creates a config entry per host and discovered port', async () => {
    api.getNetworkSnapshot.mockResolvedValue({ data: {
      hosts: [{ id: 'h1', name: 'one', status: 'online' }, { id: 'h2', name: 'two', status: 'offline' }],
      containers: [{ id: 'c1', name: 'web', host_id: 'h1', state: 'running', port_mappings: [{ host_port: 8080, protocol: 'TCP' }] }],
    } })
    const { n } = await mountLoaded()
    expect(n.hosts.value).toHaveLength(2)
    expect(n.hostPortConfig.value.map((e) => e.hostId)).toEqual(['h1', 'h2'])
    expect(n.hostPortConfig.value[0]!.ports['8080']).toEqual(PORT)
    expect(n.hostsOnline.value).toBe(1)
    expect(n.containersRunning.value).toBe(1)
  })

  it('treats a snapshot without data as empty and swallows request errors', async () => {
    api.getNetworkSnapshot.mockResolvedValue({})
    expect((await mountLoaded()).n.hosts.value).toEqual([])
    api.getNetworkSnapshot.mockRejectedValue(new Error('down'))
    const { n } = await mountLoaded()
    expect(n.hosts.value).toEqual([])
    expect(n.containers.value).toEqual([])
  })

  it('loads the IP inventory without blocking, and clears the loading flag', async () => {
    api.getIPInventory.mockResolvedValue({ data: {
      proxmox_guests: [{ guest_id: 'g1', name: 'vm1' }],
      npm_hosts: [{ proxy_host_id: 1, matched_type: 'proxmox_guest', matched_id: 'g1' }],
    } })
    const { n } = await mountLoaded()
    expect(n.proxmoxGuestIPs.value).toHaveLength(1)
    expect(n.npmEntries.value).toHaveLength(1)
    expect(n.ipInventoryLoading.value).toBe(false)
    expect(n.ipInventoryError.value).toBe(false)
  })

  it('falls back to empty lists for a partial inventory and flags errors', async () => {
    api.getIPInventory.mockResolvedValue({ data: {} })
    const ok = await mountLoaded()
    expect(ok.n.proxmoxGuestIPs.value).toEqual([])
    expect(ok.n.npmEntries.value).toEqual([])

    api.getIPInventory.mockRejectedValue(new Error('proxmox down'))
    const { n } = await mountLoaded()
    expect(n.ipInventoryError.value).toBe(true)
    expect(n.ipInventoryLoading.value).toBe(false)
  })
})

describe('useNetwork — Proxmox guest nodes and services', () => {
  const guests = [
    { guest_id: 'g1', name: 'vm-one', node: 'pve1', ip_addresses: ['10.0.0.5', '10.0.0.6'], status: 'running' },
    { guest_id: 'g2', name: 'vm-two', node: 'pve2', ip_addresses: [], status: 'stopped' },
  ]

  it('derives one node per matched guest, deduplicated, with fallbacks for label and sublabel', async () => {
    api.getIPInventory.mockResolvedValue({ data: { proxmox_guests: guests, npm_hosts: [
      { proxy_host_id: 1, matched_type: 'proxmox_guest', matched_id: 'g1', matched_name: 'custom' },
      { proxy_host_id: 2, matched_type: 'proxmox_guest', matched_id: 'g1' },
      { proxy_host_id: 3, matched_type: 'proxmox_guest', matched_id: 'g2' },
      { proxy_host_id: 4, matched_type: 'proxmox_guest', matched_id: 'ghost' },
      { proxy_host_id: 5, matched_type: 'host', matched_id: 'h1' },
      { proxy_host_id: 6, matched_type: 'proxmox_guest' },
    ] } })
    const { n } = await mountLoaded()
    expect(n.guestNodes.value).toEqual([
      { id: 'g1', label: 'custom', sublabel: 'pve1 · 10.0.0.5', status: 'running' },
      { id: 'g2', label: 'vm-two', sublabel: 'pve2', status: 'stopped' },
      { id: 'ghost', label: 'ghost', sublabel: '', status: undefined },
    ])
  })

  it('exposes guest-linked NPM entries as services, named by domains or forward host', async () => {
    api.getIPInventory.mockResolvedValue({ data: { proxmox_guests: guests, npm_hosts: [
      { proxy_host_id: 1, matched_type: 'proxmox_guest', matched_id: 'g1', domain_names: ['a.fr', 'b.fr'], forward_host: 'x', forward_port: 80 },
      { proxy_host_id: 2, matched_type: 'proxmox_guest', matched_id: 'g2', forward_host: '10.0.0.9', forward_port: 8080 },
      { proxy_host_id: 3, matched_type: 'host', matched_id: 'h1', domain_names: ['skip.fr'] },
    ] } })
    const { n } = await mountLoaded()
    expect(n.filteredServices.value).toEqual([
      { id: 'npm-1', guestId: 'g1', name: 'a.fr, b.fr', domain: 'a.fr', path: '/', internalPort: 80, linkToProxy: true, tags: 'npm-auto' },
      { id: 'npm-2', guestId: 'g2', name: '10.0.0.9', domain: '', path: '/', internalPort: 8080, linkToProxy: true, tags: 'npm-auto' },
    ])
  })
})

describe('useNetwork — port overrides and services', () => {
  async function withConfig() {
    const ctx = await mountLoaded()
    ctx.n.hostPortConfig.value = [
      { hostId: '', ports: { '1': { ...PORT } } },
      { hostId: 'h1', ports: {
        '80': { ...PORT, name: 'http', linkToProxy: true, domain: 'a.fr', path: '/app', externalPort: 8080, exposedToInternet: true, linkToAuthelia: true },
        '81': { ...PORT, enabled: false, name: 'off', linkToProxy: true, exposedToInternet: true },
        '82': { ...PORT, exposedToInternet: true },
        '0': { ...PORT, linkToProxy: true },
      } },
    ]
    return ctx
  }

  it('computes excluded ports, names, proxy / authelia / internet sets per host', async () => {
    const { n } = await withConfig()
    const o = n.hostPortOverrides.value
    expect(Object.keys(o)).toEqual(['h1'])
    expect(o.h1.excludedPorts).toEqual([81])
    expect(o.h1.portMap).toEqual({ 80: 'http', 81: 'off' })
    expect([...o.h1.proxyPorts].sort()).toEqual([0, 80])
    expect([...o.h1.autheliaPortNumbers]).toEqual([80])
    expect(o.h1.internetExposedPorts).toEqual({ 80: 8080, 82: null })
  })

  it('handles an entry without a ports map', async () => {
    const { n } = await mountLoaded()
    n.hostPortConfig.value = [{ hostId: 'h1' } as never]
    expect(n.hostPortOverrides.value.h1.excludedPorts).toEqual([])
    expect(n.combinedServices.value).toEqual([])
  })

  it('appends proxy-linked ports to the manual services, skipping port 0 and unlinked ports', async () => {
    const { n } = await withConfig()
    n.networkServices.value = [{ id: 'manual', name: 'm' }] as never
    const linked = n.combinedServices.value.filter((s) => String(s.id).startsWith('linked-'))
    expect(n.combinedServices.value[0]).toEqual({ id: 'manual', name: 'm' })
    expect(linked.map((s) => s.id)).toEqual(['linked-h1-80', 'linked-h1-81'])
    expect(linked[0]).toMatchObject({ name: 'http', domain: 'a.fr', path: '/app', externalPort: 8080, linkToAuthelia: true, exposedToInternet: true, tags: 'proxy' })
    expect(linked[1]).toMatchObject({ name: 'off', domain: '', path: '/', externalPort: null, linkToAuthelia: false, exposedToInternet: true })
  })

  it('names an unnamed linked port after its number', async () => {
    const { n } = await mountLoaded()
    n.hostPortConfig.value = [{ hostId: 'h1', ports: { '9000': { ...PORT, linkToProxy: true } } }]
    expect(n.combinedServices.value[0]!.name).toBe('Port 9000')
  })
})

describe('useNetwork — graph hosts and filters', () => {
  async function withTopology() {
    api.getNetworkSnapshot.mockResolvedValue({ data: {
      hosts: [{ id: 'h1', name: 'one', status: 'online' }, { id: 'h2', name: 'two', status: 'online' }],
      containers: [
        { name: 'web', host_id: 'h1', state: 'running', port_mappings: [{ host_port: 80, protocol: 'TCP' }, { host_port: 443 }, { host_port: 0 }, { host_port: 22 }] },
        { name: 'web2', host_id: 'h1', state: 'exited', port_mappings: [{ host_port: 80, protocol: 'tcp' }] },
        { name: 'orphan', state: 'running', port_mappings: [{ host_port: 99 }] },
        { name: 'bare', host_id: 'h2', state: 'running' },
      ],
    } })
    const ctx = await mountLoaded()
    ctx.n.hostPortConfig.value = [{ hostId: 'h1', ports: {
      '80': { ...PORT, linkToProxy: true },
      '443': { ...PORT, exposedToInternet: true, externalPort: 443 },
      '22': { ...PORT },
    } }]
    return ctx
  }

  it('groups container ports per host, merging containers on the same port and protocol', async () => {
    const { n } = await withTopology()
    const h1 = n.filteredGraphHosts.value.find((h) => h.id === 'h1')!
    expect(h1.ports).toEqual([
      { port: 80, protocol: 'tcp', containers: ['web', 'web2'] },
      { port: 443, protocol: 'tcp', containers: ['web'] },
      { port: 22, protocol: 'tcp', containers: ['web'] },
    ])
    expect(n.filteredGraphHosts.value.find((h) => h.id === 'h2')!.ports).toEqual([])
    expect(n.totalPorts.value).toBe(3)
  })

  it('keeps only internet-exposed ports and services with the internet-only filter', async () => {
    const { n } = await withTopology()
    n.networkServices.value = [{ id: 'm1', exposedToInternet: true }, { id: 'm2' }] as never
    expect(n.filteredServices.value.map((s) => s.id)).toContain('m2')
    n.filterInternetOnly.value = true
    expect(n.filteredGraphHosts.value.find((h) => h.id === 'h1')!.ports.map((p) => p.port)).toEqual([443])
    expect(n.filteredServices.value.map((s) => s.id)).toEqual(['m1'])
  })

  it('hides internal-only ports with the hide-internal filter', async () => {
    const { n } = await withTopology()
    n.filterHideInternal.value = true
    expect(n.filteredGraphHosts.value.find((h) => h.id === 'h1')!.ports.map((p) => p.port)).toEqual([80, 443])
    // a host without overrides has no proxy/internet ports left
    expect(n.filteredGraphHosts.value.find((h) => h.id === 'h2')!.ports).toEqual([])
  })

  it('gives the internet-only filter precedence over hide-internal', async () => {
    const { n } = await withTopology()
    n.filterInternetOnly.value = true
    n.filterHideInternal.value = true
    expect(n.filteredGraphHosts.value.find((h) => h.id === 'h1')!.ports.map((p) => p.port)).toEqual([443])
  })
})

describe('useNetwork — saving the topology config', () => {
  it('debounces edits into a single save of the serialized config', async () => {
    const { n } = await mountLoaded()
    vi.useFakeTimers()
    n.rootNodeName.value = 'A'
    await nextTick()
    n.rootNodeIp.value = '10.0.0.1'
    await nextTick()
    vi.advanceTimersByTime(499)
    expect(api.saveTopologyConfig).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(api.saveTopologyConfig).toHaveBeenCalledTimes(1)
    const sent = api.saveTopologyConfig.mock.calls[0][0]
    expect(sent).toMatchObject({ root_label: 'A', root_ip: '10.0.0.1', authelia_label: 'Authelia', internet_label: 'Internet', excluded_ports: [], service_map: '{}' })
    expect(JSON.parse(sent.host_overrides)).toEqual([])
    expect(n.saveStatus.value).toBe('saved')
    await vi.advanceTimersByTimeAsync(3000)
    expect(n.saveStatus.value).toBe('idle')
  })

  it('substitutes default labels for blank authelia / internet fields', async () => {
    const { n } = await mountLoaded()
    vi.useFakeTimers()
    n.autheliaLabel.value = ''
    n.internetLabel.value = ''
    await nextTick()
    await vi.advanceTimersByTimeAsync(500)
    expect(api.saveTopologyConfig.mock.calls[0][0]).toMatchObject({ authelia_label: 'Authelia', internet_label: 'Internet', authelia_ip: '', internet_ip: '' })
  })

  it('does not save before the config has been loaded', async () => {
    let release!: () => void
    api.getTopologyConfig.mockReturnValue(new Promise((r) => { release = () => r({ data: null }) }))
    const { n } = mountUseNetwork()
    vi.useFakeTimers()
    n.debouncedSave()
    await vi.advanceTimersByTimeAsync(600)
    expect(api.saveTopologyConfig).not.toHaveBeenCalled()
    release()
  })

  it('reports an error status when saving fails, then goes back to idle', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    api.saveTopologyConfig.mockRejectedValue(new Error('500'))
    const { n } = await mountLoaded()
    vi.useFakeTimers()
    n.debouncedSave()
    await vi.advanceTimersByTimeAsync(500)
    expect(n.saveStatus.value).toBe('error')
    await vi.advanceTimersByTimeAsync(3000)
    expect(n.saveStatus.value).toBe('idle')
    warn.mockRestore()
  })

  it('stores dragged node positions and schedules a save', async () => {
    const { n } = await mountLoaded()
    vi.useFakeTimers()
    n.onNodePositionsUpdate({ a: { x: 1, y: 2 } })
    expect(n.nodePositions.value).toEqual({ a: { x: 1, y: 2 } })
    await vi.advanceTimersByTimeAsync(500)
    expect(JSON.parse(api.saveTopologyConfig.mock.calls[0][0].node_positions)).toEqual({ a: { x: 1, y: 2 } })
  })
})

describe('useNetwork — live WebSocket updates', () => {
  const snapshot = (hosts: unknown[], containers?: unknown[]) => ({ type: 'network', hosts, containers })

  it('ignores messages that are not network snapshots', async () => {
    const { n } = await mountLoaded()
    wsHandler.current!({ type: 'ping', hosts: [{ id: 'x' }] })
    expect(n.hosts.value).toEqual([])
  })

  it('replaces hosts and containers and creates their config entries', async () => {
    const { n } = await mountLoaded()
    wsHandler.current!(snapshot([{ id: 'h1', status: 'online' }], [{ name: 'c', host_id: 'h1', state: 'running', port_mappings: [{ host_port: 80 }] }]))
    expect(n.hosts.value).toHaveLength(1)
    expect(n.containers.value).toHaveLength(1)
    expect(n.hostPortConfig.value[0]!.ports['80']).toBeDefined()
    wsHandler.current!({ type: 'network' })
    expect(n.hosts.value).toEqual([])
    expect(n.containers.value).toEqual([])
  })

  it('computes the traffic delta between two snapshots, ignoring counter resets and unknown hosts', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'))
    const { n } = await mountLoaded()

    wsHandler.current!(snapshot([
      { id: 'h1', network_rx_bytes: 1000, network_tx_bytes: 500 },
      { id: 'h2', network_rx_bytes: 5000, network_tx_bytes: 5000 },
    ]))
    expect(n.trafficDelta.value).toEqual({ rx: 0, tx: 0, intervalSec: 0 })

    vi.setSystemTime(new Date('2026-01-01T00:00:10Z'))
    wsHandler.current!(snapshot([
      { id: 'h1', network_rx_bytes: 1600, network_tx_bytes: 700 },
      { id: 'h2', network_rx_bytes: 100, network_tx_bytes: 5400 }, // rx counter reset
      { id: 'h3', network_rx_bytes: 9999, network_tx_bytes: 9999 }, // no previous sample
      { id: 'h4' }, // no counters
    ]))
    expect(n.trafficDelta.value).toEqual({ rx: 600, tx: 600, intervalSec: 10 })
  })

  it('never reports an interval shorter than one second', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'))
    const { n } = await mountLoaded()
    wsHandler.current!(snapshot([{ id: 'h1', network_rx_bytes: 1, network_tx_bytes: 1 }]))
    wsHandler.current!(snapshot([{ id: 'h1', network_rx_bytes: 11, network_tx_bytes: 6 }]))
    expect(n.trafficDelta.value).toEqual({ rx: 10, tx: 5, intervalSec: 1 })
  })

  it('formats byte counts through the shared formatter', async () => {
    const { n } = await mountLoaded()
    expect(n.formatBytes(1024)).toMatch(/1(\.0)?\s?(KB|Ko|KiB)/i)
    expect(typeof n.formatBytes(undefined)).toBe('string')
  })
})
