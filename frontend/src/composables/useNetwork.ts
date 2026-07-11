/* eslint-disable @typescript-eslint/no-explicit-any --
 * Verbatim move of NetworkView.vue's business logic (see eslint.config.js's
 * TEMP Phase 7 exemption for the view/NetworkGraph/TrafficWorldMap). The
 * topology config + discovered-port shapes are runtime, cytoscape-facing
 * structures with no Go model; typed in the dedicated cytoscape/d3 follow-up. */
import { ref, computed, onMounted, watch } from 'vue'
import apiClient from '../api'
import { useWebSocket } from './useWebSocket'
import type { WSNetworkSnapshot } from '../types/ws'

export function useNetwork() {
  const hosts = ref<any[]>([])
  const containers = ref<any[]>([])
  const viewMode = ref(localStorage.getItem('networkViewMode') || 'graph')
  const networkTab = ref('topology')
  const rootNodeName = ref('Infrastructure')
  const rootNodeIp = ref('')
  const autheliaLabel = ref('Authelia')
  const autheliaIp = ref('')
  const internetLabel = ref('Internet')
  const internetIp = ref('')
  const networkServices = ref<any[]>([])
  const hostPortConfig = ref<any[]>([])
  const nodePositions = ref<Record<string, { x: number; y: number }>>({})
  const topologyConfigLoaded = ref(false)
  const saveStatus = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const selectedNode = ref<any>(null)
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
  const discoveredPortsByHost = computed<Record<string, any[]>>(() => {
    const map: Record<string, any[]> = {}
    for (const container of containers.value) {
      const mappings = container.port_mappings || []
      for (const mapping of mappings) {
        const hostId = container.host_id
        if (!hostId) continue
        const hostPort = mapping.host_port || 0
        const containerPort = mapping.container_port || 0
        const portNumber = hostPort || containerPort
        if (!portNumber) continue
        const protocol = (mapping.protocol || 'tcp').toLowerCase()
        if (!map[hostId]) map[hostId] = []
        const key = `${portNumber}-${protocol}`
        const existing = map[hostId].find((entry: any) => entry.key === key)
        if (existing) {
          if (container.name && !existing.containers.includes(container.name)) existing.containers.push(container.name)
          continue
        }
        map[hostId].push({ key, port: portNumber, protocol, internal: hostPort === 0, containers: container.name ? [container.name] : [] })
      }
    }
    for (const host of hosts.value) {
      if (!map[host.id]) map[host.id] = []
    }
    return map
  })

  const hostPortOverrides = computed<Record<string, any>>(() => {
    const overrides: Record<string, any> = {}
    for (const entry of hostPortConfig.value) {
      if (!entry.hostId) continue
      const excludedPortsList: number[] = []
      const portMap: Record<number, string> = {}
      const proxyPorts = new Set<number>()
      const autheliaPortNumbers = new Set<number>()
      const internetExposedPorts: Record<number, number | null> = {}
      for (const [port, settings] of Object.entries(entry.ports || {}) as [string, any][]) {
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

  const combinedServices = computed<any[]>(() => {
    const linkedServices: any[] = []
    for (const entry of hostPortConfig.value) {
      if (!entry.hostId) continue
      for (const [port, settings] of Object.entries(entry.ports || {}) as [string, any][]) {
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
    return [...networkServices.value, ...linkedServices]
  })

  const graphHosts = computed<any[]>(() => {
    const portsByHost = new Map<string, Map<string, any>>()
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
    return hosts.value.map((host: any) => ({
      ...host,
      ports: portsByHost.has(host.id) ? Array.from(portsByHost.get(host.id)!.values()) : [],
    }))
  })

  const filteredGraphHosts = computed(() => {
    if (!filterInternetOnly.value && !filterHideInternal.value) return graphHosts.value
    return graphHosts.value.map((host: any) => {
      const override = hostPortOverrides.value[host.id] || {}
      const proxyPorts = override.proxyPorts || new Set<number>()
      const internetPorts = override.internetExposedPorts || {}
      let ports = host.ports || []
      if (filterInternetOnly.value) {
        ports = ports.filter((p: any) => Number(p.port) in internetPorts)
      } else if (filterHideInternal.value) {
        ports = ports.filter((p: any) => {
          const pn = Number(p.port)
          return proxyPorts.has(pn) || pn in internetPorts
        })
      }
      return { ...host, ports }
    })
  })

  const filteredServices = computed(() => {
    if (!filterInternetOnly.value) return combinedServices.value
    return combinedServices.value.filter((s: any) => s.exposedToInternet)
  })

  const totalPorts = computed(() => graphHosts.value.reduce((sum, host: any) => sum + (host.ports?.length || 0), 0))
  const hostsOnline = computed(() => hosts.value.filter((h: any) => h.status === 'online').length)
  const containersRunning = computed(() => containers.value.filter((c: any) => c.state === 'running').length)

  const trafficDelta = ref({ rx: 0, tx: 0, intervalSec: 0 })
  const prevTrafficByHost = ref<Record<string, { rx: number; tx: number }>>({})
  const prevTrafficTime = ref<number | null>(null)

  function formatBytes(bytes: number | undefined): string {
    if (!bytes && bytes !== 0) return '-'
    if (bytes < 1024) return `${bytes} B`
    const units = ['KB', 'MB', 'GB', 'TB']
    let value = bytes / 1024
    let idx = 0
    while (value >= 1024 && idx < units.length - 1) { value /= 1024; idx++ }
    return `${value.toFixed(1)} ${units[idx]}`
  }

  function ensureHostPortConfig(): void {
    const known = new Set(hostPortConfig.value.map((item) => item.hostId))
    for (const host of hosts.value) {
      if (known.has(host.id)) continue
      hostPortConfig.value.push({ hostId: host.id, ports: {} })
    }
    for (const [hostId, ports] of Object.entries(discoveredPortsByHost.value)) {
      const entry = getHostPortEntry(hostId)
      for (const port of ports) {
        const portKey = String(port.port)
        if (!entry.ports[portKey]) {
          entry.ports[portKey] = { name: '', domain: '', path: '/', enabled: true, linkToProxy: false, linkToAuthelia: false, exposedToInternet: false, externalPort: null }
        }
      }
    }
  }

  function getHostPortEntry(hostId: string): any {
    let entry = hostPortConfig.value.find((item) => item.hostId === hostId)
    if (!entry) {
      entry = { hostId, ports: {} }
      hostPortConfig.value.push(entry)
    }
    if (!entry.ports) entry.ports = {}
    return entry
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
      ensureHostPortConfig()
    } catch {
      // ignore
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
    ensureHostPortConfig()
  })

  // ─── Lifecycle ────────────────────────────────────────────────────────────
  onMounted(async () => {
    await loadTopologyConfig()
    await fetchSnapshot()
  })

  return {
    hosts,
    containers,
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
