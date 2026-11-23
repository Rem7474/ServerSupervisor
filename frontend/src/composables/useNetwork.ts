import { ref, computed, onMounted, watch } from 'vue'
import apiClient from '../api'
import { useWebSocket } from './useWebSocket'
import type { WSNetworkSnapshot } from '../types/ws'
import type { NetworkProxmoxGuestIP, NetworkNPMEntry } from '../types/network'
import type { NetworkHost, NetworkContainer } from '../types/generated'
import type { NetworkGuestNode, NetworkService } from '../components/network/buildNetworkElements'
import { formatBytes as formatBytesShared } from '../utils/formatters'
import { buildDiscoveredPortsByHost, ensureHostPortConfig, type HostPortEntry, type PortSetting } from '../utils/networkPorts'

/** A host's ports as consumed by the graph: discovered from container port mappings. */
interface GraphPort { port: number; protocol: string; containers: string[] }
type GraphHost = NetworkHost & { ports: GraphPort[] }

/** A manually configured service (persisted in the topology config's manual_services). */
interface ManualService {
  id: string
  name: string
  domain: string
  path: string
  internalPort: number | null
  externalPort: number | null
  hostId: string
  tags: string
  linkToProxy: boolean
  linkToAuthelia: boolean
  exposedToInternet: boolean
}

interface HostPortOverride {
  excludedPorts: number[]
  portMap: Record<number, string>
  proxyPorts: Set<number>
  autheliaPortNumbers: Set<number>
  internetExposedPorts: Record<number, number | null>
}

export function useNetwork() {
  const hosts = ref<NetworkHost[]>([])
  const containers = ref<NetworkContainer[]>([])
  const proxmoxGuestIPs = ref<NetworkProxmoxGuestIP[]>([])
  const npmEntries = ref<NetworkNPMEntry[]>([])
  const ipInventoryLoading = ref(false)
  const ipInventoryError = ref(false)
  const viewMode = ref(localStorage.getItem('networkViewMode') || 'graph')
  const networkTab = ref('topology')
  const rootNodeName = ref('Infrastructure')
  const rootNodeIp = ref('')
  const autheliaLabel = ref('Authelia')
  const autheliaIp = ref('')
  const internetLabel = ref('Internet')
  const internetIp = ref('')
  const networkServices = ref<ManualService[]>([])
  const hostPortConfig = ref<HostPortEntry[]>([])
  const nodePositions = ref<Record<string, { x: number; y: number }>>({})
  const topologyConfigLoaded = ref(false)
  const saveStatus = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const selectedNode = ref<Record<string, unknown> | null>(null)
  const rootHostId = ref('')
  const autheliaHostId = ref('')
  const rootPortId = ref('')
  const autheliaPortId = ref('')

  const filterInternetOnly = ref(false)
  const filterHideInternal = ref(false)

  // ─── Persist view mode ────────────────────────────────────────────────────
  watch(viewMode, (newMode) => {
    localStorage.setItem('networkViewMode', newMode)
  })

  // ─── Debounced save ───────────────────────────────────────────────────────
  let saveTimeout: ReturnType<typeof setTimeout> | null = null
  const debouncedSave = () => {
    if (saveTimeout) clearTimeout(saveTimeout)
    saveTimeout = setTimeout(async () => {
      await saveTopologyConfig()
    }, 500)
  }

  watch(rootNodeName, () => debouncedSave())
  watch(rootNodeIp, () => debouncedSave())
  watch(autheliaLabel, () => debouncedSave())
  watch(autheliaIp, () => debouncedSave())
  watch(internetLabel, () => debouncedSave())
  watch(internetIp, () => debouncedSave())
  watch(networkServices, () => debouncedSave(), { deep: true })
  watch(hostPortConfig, () => debouncedSave(), { deep: true })
  watch(rootHostId, () => debouncedSave())
  watch(autheliaHostId, () => debouncedSave())
  watch(rootPortId, () => debouncedSave())
  watch(autheliaPortId, () => debouncedSave())

  // ─── Topology config load/save ────────────────────────────────────────────
  async function loadTopologyConfig(): Promise<void> {
    try {
      const res = await apiClient.getTopologyConfig()
      if (res.data) {
        const cfg = res.data
        rootNodeName.value = cfg.root_label || 'Infrastructure'
        rootNodeIp.value = cfg.root_ip || ''
        autheliaLabel.value = cfg.authelia_label || 'Authelia'
        autheliaIp.value = cfg.authelia_ip || ''
        internetLabel.value = cfg.internet_label || 'Internet'
        internetIp.value = cfg.internet_ip || ''
        networkServices.value = cfg.manual_services ? JSON.parse(cfg.manual_services) : []
        rootHostId.value     = cfg.root_host_id      || ''
        autheliaHostId.value = cfg.authelia_host_id  || ''
        rootPortId.value     = cfg.root_port_id      || ''
        autheliaPortId.value = cfg.authelia_port_id  || ''
        if (cfg.node_positions) {
          try { nodePositions.value = JSON.parse(cfg.node_positions) } catch { nodePositions.value = {} }
        }
        if (cfg.host_overrides) {
          try { hostPortConfig.value = JSON.parse(cfg.host_overrides) } catch { hostPortConfig.value = [] }
        }
      }
    } catch (e) {
      console.warn('Failed to load topology config from server:', e)
    } finally {
      topologyConfigLoaded.value = true
    }
  }

  async function saveTopologyConfig(): Promise<void> {
    if (!topologyConfigLoaded.value) return
    try {
      saveStatus.value = 'saving'
      const config = {
        root_label: rootNodeName.value,
        root_ip: rootNodeIp.value,
        excluded_ports: [],
        service_map: '{}',
        host_overrides: JSON.stringify(hostPortConfig.value),
        manual_services: JSON.stringify(networkServices.value),
        node_positions: JSON.stringify(nodePositions.value),
        authelia_label: autheliaLabel.value || 'Authelia',
        authelia_ip: autheliaIp.value || '',
        internet_label: internetLabel.value || 'Internet',
        internet_ip: internetIp.value || '',
        root_host_id:      rootHostId.value,
        authelia_host_id:  autheliaHostId.value,
        root_port_id:      rootPortId.value,
        authelia_port_id:  autheliaPortId.value,
      }
      await apiClient.saveTopologyConfig(config)
      saveStatus.value = 'saved'
      setTimeout(() => { if (saveStatus.value === 'saved') saveStatus.value = 'idle' }, 3000)
    } catch (e) {
      console.warn('Failed to save topology config:', e)
      saveStatus.value = 'error'
      setTimeout(() => { if (saveStatus.value === 'error') saveStatus.value = 'idle' }, 3000)
    }
  }

  // ─── Computed: port discovery ──────────────────────────────────────────────
  const discoveredPortsByHost = computed(() => buildDiscoveredPortsByHost(containers.value, hosts.value))

  const hostPortOverrides = computed<Record<string, HostPortOverride>>(() => {
    const overrides: Record<string, HostPortOverride> = {}
    for (const entry of hostPortConfig.value) {
      if (!entry.hostId) continue
      const excludedPortsList: number[] = []
      const portMap: Record<number, string> = {}
      const proxyPorts = new Set<number>()
      const autheliaPortNumbers = new Set<number>()
      const internetExposedPorts: Record<number, number | null> = {}
      for (const [port, settings] of Object.entries(entry.ports || {}) as [string, PortSetting | undefined][]) {
        const portNumber = Number(port)
        if (!settings?.enabled) excludedPortsList.push(portNumber)
        if (settings?.name) portMap[portNumber] = settings.name
        if (settings?.linkToProxy && settings?.enabled) proxyPorts.add(portNumber)
        if (settings?.linkToAuthelia && settings?.enabled) autheliaPortNumbers.add(portNumber)
        if (settings?.exposedToInternet && settings?.enabled) internetExposedPorts[portNumber] = settings?.externalPort || null
      }
      overrides[entry.hostId] = { excludedPorts: excludedPortsList, portMap, proxyPorts, autheliaPortNumbers, internetExposedPorts }
    }
    return overrides
  })

  const combinedServices = computed<NetworkService[]>(() => {
    const linkedServices: NetworkService[] = []
    for (const entry of hostPortConfig.value) {
      if (!entry.hostId) continue
      for (const [port, settings] of Object.entries(entry.ports || {}) as [string, PortSetting | undefined][]) {
        if (!settings?.linkToProxy) continue
        const portNumber = Number(port)
        if (!portNumber) continue
        linkedServices.push({
          id: `linked-${entry.hostId}-${portNumber}`,
          name: settings.name || `Port ${portNumber}`,
          domain: settings.domain || '',
          path: settings.path || '/',
          internalPort: portNumber,
          externalPort: settings.externalPort || null,
          hostId: entry.hostId,
          tags: 'proxy',
          linkToProxy: true,
          linkToAuthelia: settings.linkToAuthelia || false,
          exposedToInternet: settings.exposedToInternet || false,
        })
      }
    }
    return [...(networkServices.value as NetworkService[]), ...linkedServices]
  })

  // ─── Proxmox guest nodes (no agent) auto-detected via NPM correlation ─────
  // The topology graph's nodes otherwise come only from agent-reporting Hosts
  // (see fetchSnapshot below) — a domain routed to a Proxmox VM/CT with no
  // agent had no node to attach to. Restricted to guests that actually have
  // an NPM-matched entry (rather than every known guest) so the graph stays
  // focused on what's relevant here instead of duplicating the full Proxmox
  // inventory already available on its own page.
  const guestNodes = computed<NetworkGuestNode[]>(() => {
    const seen = new Map<string, NetworkGuestNode>()
    for (const entry of npmEntries.value) {
      if (entry.matched_type !== 'proxmox_guest' || !entry.matched_id || seen.has(entry.matched_id)) continue
      const guest = proxmoxGuestIPs.value.find((g) => g.guest_id === entry.matched_id)
      seen.set(entry.matched_id, {
        id: entry.matched_id,
        label: entry.matched_name || guest?.name || entry.matched_id,
        sublabel: guest ? [guest.node, guest.ip_addresses?.[0]].filter(Boolean).join(' · ') : '',
        status: guest?.status,
      })
    }
    return Array.from(seen.values())
  })

  const guestServices = computed<NetworkService[]>(() =>
    npmEntries.value
      .filter((n) => n.matched_type === 'proxmox_guest' && n.matched_id)
      .map((n) => ({
        id: `npm-${n.proxy_host_id}`,
        guestId: n.matched_id,
        name: (n.domain_names || []).join(', ') || n.forward_host,
        domain: (n.domain_names || [])[0] || '',
        path: '/',
        internalPort: n.forward_port,
        linkToProxy: true,
        tags: 'npm-auto',
      }))
  )

  const graphHosts = computed<GraphHost[]>(() => {
    const portsByHost = new Map<string, Map<string, GraphPort>>()
    for (const container of containers.value) {
      const mappings = container.port_mappings || []
      for (const mapping of mappings) {
        const hostId = container.host_id
        if (!hostId) continue
        const portNumber = mapping.host_port || 0
        if (!portNumber) continue
        const protocol = (mapping.protocol || 'tcp').toLowerCase()
        const key = `${portNumber}-${protocol}`
        if (!portsByHost.has(hostId)) portsByHost.set(hostId, new Map())
        const hostPorts = portsByHost.get(hostId)!
        if (!hostPorts.has(key)) {
          hostPorts.set(key, { port: portNumber, protocol, containers: [] })
        }
        hostPorts.get(key)!.containers.push(container.name)
      }
    }
    return hosts.value.map((host) => ({
      ...host,
      ports: portsByHost.has(host.id) ? Array.from(portsByHost.get(host.id)!.values()) : [],
    }))
  })

  const filteredGraphHosts = computed(() => {
    if (!filterInternetOnly.value && !filterHideInternal.value) return graphHosts.value
    return graphHosts.value.map((host) => {
      const override = hostPortOverrides.value[host.id] || {}
      const proxyPorts = override.proxyPorts || new Set<number>()
      const internetPorts = override.internetExposedPorts || {}
      let ports = host.ports || []
      if (filterInternetOnly.value) {
        ports = ports.filter((p) => Number(p.port) in internetPorts)
      } else if (filterHideInternal.value) {
        ports = ports.filter((p) => {
          const pn = Number(p.port)
          return proxyPorts.has(pn) || pn in internetPorts
        })
      }
      return { ...host, ports }
    })
  })

  const filteredServices = computed(() => {
    const all = [...combinedServices.value, ...guestServices.value]
    if (!filterInternetOnly.value) return all
    return all.filter((s) => s.exposedToInternet)
  })

  const totalPorts = computed(() => graphHosts.value.reduce((sum, host) => sum + (host.ports?.length || 0), 0))
  const hostsOnline = computed(() => hosts.value.filter((h) => h.status === 'online').length)
  const containersRunning = computed(() => containers.value.filter((c) => c.state === 'running').length)

  const trafficDelta = ref({ rx: 0, tx: 0, intervalSec: 0 })
  const prevTrafficByHost = ref<Record<string, { rx: number; tx: number }>>({})
  const prevTrafficTime = ref<number | null>(null)

  function formatBytes(bytes: number | undefined): string {
    return formatBytesShared(bytes)
  }

  function onNodePositionsUpdate(positions: Record<string, { x: number; y: number }>): void {
    nodePositions.value = positions
    debouncedSave()
  }

  // ─── Data fetch ───────────────────────────────────────────────────────────
  async function fetchSnapshot(): Promise<void> {
    try {
      const res = await apiClient.getNetworkSnapshot()
      hosts.value = res.data?.hosts || []
      containers.value = res.data?.containers || []
      ensureHostPortConfig(hostPortConfig.value, hosts.value, discoveredPortsByHost.value)
    } catch {
      // ignore
    }
  }

  // ─── IP inventory (Proxmox guests + NPM domains, live/non-persisted) ──────
  async function fetchIPInventory(): Promise<void> {
    ipInventoryLoading.value = true
    ipInventoryError.value = false
    try {
      const res = await apiClient.getIPInventory()
      proxmoxGuestIPs.value = res.data?.proxmox_guests || []
      npmEntries.value = res.data?.npm_hosts || []
    } catch {
      ipInventoryError.value = true
    } finally {
      ipInventoryLoading.value = false
    }
  }

  // ─── WebSocket ────────────────────────────────────────────────────────────
  const { wsStatus, wsError, retryCount, reconnect } = useWebSocket<WSNetworkSnapshot>('/api/v1/ws/network', (payload) => {
    if (payload.type !== 'network') return
    const now = Date.now()
    const newHosts = payload.hosts || []

    if (prevTrafficTime.value !== null) {
      const intervalSec = Math.max(1, Math.round((now - prevTrafficTime.value) / 1000))
      let deltaRx = 0, deltaTx = 0
      for (const h of newHosts) {
        const prev = prevTrafficByHost.value[h.id]
        if (prev) {
          const drx = (h.network_rx_bytes || 0) - prev.rx
          const dtx = (h.network_tx_bytes || 0) - prev.tx
          if (drx >= 0) deltaRx += drx
          if (dtx >= 0) deltaTx += dtx
        }
      }
      trafficDelta.value = { rx: deltaRx, tx: deltaTx, intervalSec }
    }

    const snap: Record<string, { rx: number; tx: number }> = {}
    for (const h of newHosts) {
      snap[h.id] = { rx: h.network_rx_bytes || 0, tx: h.network_tx_bytes || 0 }
    }
    prevTrafficByHost.value = snap
    prevTrafficTime.value = now
    hosts.value = newHosts
    containers.value = payload.containers || []
    ensureHostPortConfig(hostPortConfig.value, hosts.value, discoveredPortsByHost.value)
  })

  // ─── Lifecycle ────────────────────────────────────────────────────────────
  onMounted(async () => {
    await loadTopologyConfig()
    await fetchSnapshot()
    // Fire-and-forget: live Proxmox calls can be slow, must not delay the
    // hosts/containers snapshot above.
    void fetchIPInventory()
  })

  return {
    hosts,
    containers,
    proxmoxGuestIPs,
    npmEntries,
    ipInventoryLoading,
    ipInventoryError,
    viewMode,
    networkTab,
    rootNodeName,
    rootNodeIp,
    autheliaLabel,
    autheliaIp,
    internetLabel,
    internetIp,
    networkServices,
    hostPortConfig,
    nodePositions,
    topologyConfigLoaded,
    saveStatus,
    selectedNode,
    rootHostId,
    autheliaHostId,
    rootPortId,
    autheliaPortId,
    filterInternetOnly,
    filterHideInternal,
    debouncedSave,
    discoveredPortsByHost,
    hostPortOverrides,
    combinedServices,
    guestNodes,
    filteredGraphHosts,
    filteredServices,
    totalPorts,
    hostsOnline,
    containersRunning,
    trafficDelta,
    formatBytes,
    onNodePositionsUpdate,
    wsStatus,
    wsError,
    retryCount,
    reconnect,
  }
}
