<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>Réseau</span>
      </div>
      <h2 class="page-title">Réseau</h2>
      <div class="text-secondary">Ports exposés et trafic par hôte</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Hôtes</div>
            <div class="h1 mb-0">{{ hosts.length }}</div>
            <div class="text-secondary small">{{ hostsOnline }} en ligne</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Conteneurs</div>
            <div class="h1 mb-0">{{ containers.length }}</div>
            <div class="text-secondary small">{{ containersRunning }} actifs</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Ports visibles</div>
            <div class="h1 mb-0">{{ totalPorts }}</div>
            <div class="text-secondary small">sur {{ hosts.length }} hôte{{ hosts.length > 1 ? 's' : '' }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Trafic (intervalle)</div>
            <div class="h1 mb-0">{{ trafficDelta.intervalSec > 0 ? formatBytes(trafficDelta.rx + trafficDelta.tx) : '-' }}</div>
            <div class="text-secondary small">
              <span v-if="trafficDelta.intervalSec > 0">↓ {{ formatBytes(trafficDelta.rx) }} / ↑ {{ formatBytes(trafficDelta.tx) }}</span>
              <span v-else>En attente de données…</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- View Mode Toggle -->
    <div class="card mb-4">
      <div class="card-body">
        <div class="btn-group" role="group">
          <input type="radio" class="btn-check" id="viewCards" value="cards" v-model="viewMode" />
          <label class="btn btn-outline-primary" for="viewCards">
            <svg width="18" height="18" fill="currentColor" viewBox="0 0 16 16" class="me-1">
              <path d="M1 1a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V1zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1V1zM1 11a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1v-4zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1v-4z"/>
            </svg>
            <span class="d-none d-sm-inline">Cards</span>
          </label>
          
          <input type="radio" class="btn-check" id="viewGraph" value="graph" v-model="viewMode" />
          <label class="btn btn-outline-primary" for="viewGraph">
            <svg width="18" height="18" fill="currentColor" viewBox="0 0 16 16" class="me-1">
              <path d="M0 2a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V2zm2.5 7a.5.5 0 0 0-.5.5v1a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-1a.5.5 0 0 0-.5-.5h-1zm2-4a.5.5 0 0 0-.5.5v5a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-5a.5.5 0 0 0-.5-.5h-1zm2-2a.5.5 0 0 0-.5.5v8a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V3.5a.5.5 0 0 0-.5-.5h-1zm2-1a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V2.5a.5.5 0 0 0-.5-.5h-1z"/>
            </svg>
            <span class="d-none d-sm-inline">Graph</span>
          </label>
        </div>
      </div>
    </div>

    <!-- Graph View -->
    <div v-if="viewMode === 'graph'" class="card mb-4 network-topology-card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <div>
          <h3 class="card-title mb-1">Network Topology</h3>
          <div class="text-secondary small">Glisser pour reordonner, scroll pour zoomer</div>
        </div>
        <div class="d-flex align-items-center gap-3">
          <div v-if="saveStatus !== 'idle'" class="d-flex align-items-center gap-2">
            <span v-if="saveStatus === 'saving'" class="spinner-border spinner-border-sm"></span>
            <span v-else-if="saveStatus === 'saved'" class="text-success small">✓ Enregistré</span>
            <span v-else-if="saveStatus === 'error'" class="text-danger small">✗ Erreur</span>
          </div>
          <div class="text-secondary small">
            {{ hosts.length }} hôtes • {{ totalPorts }} ports publiés
          </div>
        </div>
      </div>
      <ul class="nav nav-tabs px-3 mb-0">
        <li class="nav-item">
          <button class="nav-link" :class="{ active: networkTab === 'topology' }" @click="networkTab = 'topology'">
            <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1"><path d="M2 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v13.5a.5.5 0 0 1-.777.416L8 13.71l-5.223 2.206A.5.5 0 0 1 2 15.5V2zm2-1a1 1 0 0 0-1 1v12.566l4.723-2.482a.5.5 0 0 1 .554 0L13 14.566V2a1 1 0 0 0-1-1H4z"/></svg>
            Topology
          </button>
        </li>
        <li class="nav-item">
          <button class="nav-link" :class="{ active: networkTab === 'config' }" @click="networkTab = 'config'">
            <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1"><path d="M9.405 1.05c-.413-1.4-2.397-1.4-2.81 0l-.1.34a1.464 1.464 0 0 1-2.105.872l-.31-.17c-1.283-.698-2.686.264-2.17 1.655l.119.355a1.464 1.464 0 0 1-1.738 1.738l-.355-.119c-1.39-.516-2.353 1.102-1.656 2.17l.17.31a1.464 1.464 0 0 1-.872 2.105l-.34.1c-1.4.413-1.4 2.397 0 2.81l.34.1a1.464 1.464 0 0 1 .872 2.105l-.17.31c-.697 1.283.264 2.686 1.655 2.17l.355-.119a1.464 1.464 0 0 1 1.738 1.738l-.119.355c-.516 1.39 1.102 2.353 2.17 1.656l.31-.17a1.464 1.464 0 0 1 2.105.872l.1.34c.413 1.4 2.397 1.4 2.81 0l.1-.34a1.464 1.464 0 0 1 2.105-.872l.31.17c1.283.697 2.686-.264 2.17-1.655l-.119-.355a1.464 1.464 0 0 1 1.738-1.738l.355.119c1.39.516 2.353-1.102 1.656-2.17l-.17-.31a1.464 1.464 0 0 1 .872-2.105l.34-.1c1.4-.413 1.4-2.397 0-2.81l-.34-.1a1.464 1.464 0 0 1-.872-2.105l.17-.31c.697-1.283-.264-2.686-1.655-2.17l-.355.119a1.464 1.464 0 0 1-1.738-1.738l.119-.355c.516-1.39-1.102-2.353-2.17-1.656l-.31.17a1.464 1.464 0 0 1-2.105-.872l-.1-.34zM8 10.93a2.929 2.929 0 1 1 0-5.86 2.929 2.929 0 0 1 0 5.858z"/></svg>
            Configuration
          </button>
        </li>
      </ul>
      <div class="card-body network-topology-body">
        <NetworkTopologyConfig
          v-if="networkTab === 'config'"
          v-model:root-node-name="rootNodeName"
          v-model:root-node-ip="rootNodeIp"
          v-model:authelia-label="autheliaLabel"
          v-model:authelia-ip="autheliaIp"
          v-model:internet-label="internetLabel"
          v-model:internet-ip="internetIp"
          v-model:network-services="networkServices"
          v-model:host-port-config="hostPortConfig"
          :hosts="hosts"
          :containers="containers"
        />
        <div v-else class="network-topology-graph-layout">
          <div ref="graphSurfaceRef" class="network-graph-surface" :style="{ height: graphHeight }">
            <NetworkGraph
              v-if="topologyConfigLoaded"
              :data="graphHosts"
              :root-label="rootNodeName"
              :root-ip="rootNodeIp"
              :services="combinedServices"
              :host-port-overrides="hostPortOverrides"
              :authelia-label="autheliaLabel"
              :authelia-ip="autheliaIp"
              :internet-label="internetLabel"
              :internet-ip="internetIp"
              :node-positions="nodePositions"
              @node-select="selectedNode = $event"
              @update:node-positions="onNodePositionsUpdate"
            />
          </div>
          <NetworkNodeDetail :selected-node="selectedNode" :hosts="hosts" :containers="containers" />
        </div>
      </div>
    </div>

    <NetworkPortList v-if="viewMode === 'cards'" :hosts="hosts" :containers="containers" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, watchEffect } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import NetworkNodeDetail from '../components/network/NetworkNodeDetail.vue'
import NetworkPortList from '../components/network/NetworkPortList.vue'
import NetworkTopologyConfig from '../components/network/NetworkTopologyConfig.vue'
import apiClient from '../api'

const hosts = ref([])
const containers = ref([])
const viewMode = ref(localStorage.getItem('networkViewMode') || 'cards')
const networkTab = ref('topology')
const rootNodeName = ref('Infrastructure')
const rootNodeIp = ref('')
const autheliaLabel = ref('Authelia')
const autheliaIp = ref('')
const internetLabel = ref('Internet')
const internetIp = ref('')
const networkServices = ref([])
const hostPortConfig = ref([])
const nodePositions = ref({})
const topologyConfigLoaded = ref(false)
const saveStatus = ref('idle') // 'idle' | 'saving' | 'saved' | 'error'
const graphSurfaceRef = ref(null)
const graphHeight = ref('auto')
const selectedNode = ref(null)

// Save view mode to localStorage only (local UI preference)
watch(viewMode, (newMode) => {
  localStorage.setItem('networkViewMode', newMode)
})

// Debounced save function (500ms debounce)
let saveTimeout = null
const debouncedSave = () => {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(async () => {
    await saveTopologyConfig()
  }, 500)
}

// Watch for changes and trigger save
watch(rootNodeName, () => debouncedSave())
watch(rootNodeIp, () => debouncedSave())
watch(autheliaLabel, () => debouncedSave())
watch(autheliaIp, () => debouncedSave())
watch(internetLabel, () => debouncedSave())
watch(internetIp, () => debouncedSave())
watch(networkServices, () => debouncedSave(), { deep: true })
watch(hostPortConfig, () => debouncedSave(), { deep: true })

// Load topology configuration from database
async function loadTopologyConfig() {
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
      if (cfg.node_positions) {
        try { nodePositions.value = JSON.parse(cfg.node_positions) } catch { nodePositions.value = {} }
      }
      if (cfg.host_overrides) {
        try {
          hostPortConfig.value = JSON.parse(cfg.host_overrides)
        } catch {
          hostPortConfig.value = []
        }
      }
      topologyConfigLoaded.value = true
    }
  } catch (e) {
    console.warn('Failed to load topology config from server:', e)
    topologyConfigLoaded.value = true
  }
}

// Save topology configuration to database
async function saveTopologyConfig() {
  if (!topologyConfigLoaded.value) return // Don't save until fully loaded
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
    }
    await apiClient.saveTopologyConfig(config)
    saveStatus.value = 'saved'
    // Auto-reset to idle after 3 seconds
    setTimeout(() => {
      if (saveStatus.value === 'saved') saveStatus.value = 'idle'
    }, 3000)
  } catch (e) {
    console.warn('Failed to save topology config:', e)
    saveStatus.value = 'error'
    setTimeout(() => {
      if (saveStatus.value === 'error') saveStatus.value = 'idle'
    }, 3000)
  }
}


const discoveredPortsByHost = computed(() => {
  const map = {}
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
      const existing = map[hostId].find(entry => entry.key === key)
      if (existing) {
        if (container.name && !existing.containers.includes(container.name)) existing.containers.push(container.name)
        continue
      }

      // internal: port exists only inside Docker (no host binding)
      map[hostId].push({ key, port: portNumber, protocol, internal: hostPort === 0, containers: container.name ? [container.name] : [] })
    }
  }

  for (const host of hosts.value) {
    if (!map[host.id]) map[host.id] = []
  }

  return map
})

const hostPortOverrides = computed(() => {
  const overrides = {}
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    const excludedPortsList = []
    const portMap = {}
    const proxyPorts = new Set()
    const autheliaPortNumbers = new Set()
    const internetExposedPorts = {}
    for (const [port, settings] of Object.entries(entry.ports || {})) {
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

const totalPorts = computed(() => graphHosts.value.reduce((sum, host) => sum + (host.ports?.length || 0), 0))
const hostsOnline = computed(() => hosts.value.filter(h => h.status === 'online').length)
const containersRunning = computed(() => containers.value.filter(c => c.state === 'running').length)

// Traffic delta: difference between the last two WS updates (not cumulative since boot)
const trafficDelta = ref({ rx: 0, tx: 0, intervalSec: 0 })
const prevTrafficByHost = ref({}) // host_id → { rx, tx }
const prevTrafficTime = ref(null)

const combinedServices = computed(() => {
  const linkedServices = []
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    for (const [port, settings] of Object.entries(entry.ports || {})) {
      if (!settings?.linkToProxy) continue
      const portNumber = Number(port)
      if (!portNumber) continue
      const name = settings.name || `Port ${portNumber}`
      const domain = settings.domain || ''
      const path = settings.path || '/'
      linkedServices.push({
        id: `linked-${entry.hostId}-${portNumber}`,
        name,
        domain,
        path,
        internalPort: portNumber,
        externalPort: settings.externalPort || null,
        hostId: entry.hostId,
        tags: 'proxy',
        linkToProxy: true,
        linkToAuthelia: settings.linkToAuthelia || false,
        exposedToInternet: settings.exposedToInternet || false
      })
    }
  }
  return [...networkServices.value, ...linkedServices]
})

const graphHosts = computed(() => {
  const portsByHost = new Map()
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostId = container.host_id
      if (!hostId) continue

      const portNumber = mapping.host_port || 0
      if (!portNumber) continue  // only host-exposed ports

      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      const key = `${portNumber}-${protocol}`

      if (!portsByHost.has(hostId)) {
        portsByHost.set(hostId, new Map())
      }

      const hostPorts = portsByHost.get(hostId)
      if (!hostPorts.has(key)) {
        hostPorts.set(key, {
          port: portNumber,
          protocol,
          containers: []
        })
      }

      const entry = hostPorts.get(key)
      entry.containers.push(container.name)
    }
  }

  return hosts.value.map((host) => {
    const hostPorts = portsByHost.get(host.id)
    return {
      ...host,
      ports: hostPorts ? Array.from(hostPorts.values()) : []
    }
  })
})

function formatBytes(bytes) {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(1)} ${units[idx]}`
}

function ensureHostPortConfig() {
  const known = new Set(hostPortConfig.value.map((item) => item.hostId))
  for (const host of hosts.value) {
    if (known.has(host.id)) continue
    hostPortConfig.value.push({ hostId: host.id, ports: {} })
  }
  // Pre-initialize all discovered ports
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

function getHostPortEntry(hostId) {
  let entry = hostPortConfig.value.find((item) => item.hostId === hostId)
  if (!entry) {
    entry = { hostId, ports: {} }
    hostPortConfig.value.push(entry)
  }
  if (!entry.ports) entry.ports = {}
  return entry
}

function onNodePositionsUpdate(positions) {
  nodePositions.value = positions
  debouncedSave()
}

async function fetchSnapshot() {
  try {
    const res = await apiClient.getNetworkSnapshot()
    hosts.value = res.data?.hosts || []
    containers.value = res.data?.containers || []
    ensureHostPortConfig()
  } catch (e) {
    // ignore
  }
}

// Setup ResizeObserver with watchEffect to handle dynamic mounting/unmounting
let resizeObserver = null
watchEffect(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (graphSurfaceRef.value) {
    resizeObserver = new ResizeObserver(() => {
      const rect = graphSurfaceRef.value?.getBoundingClientRect()
      if (rect) {
        const availableHeight = window.innerHeight - rect.top - 20
        graphHeight.value = Math.max(400, availableHeight) + 'px'
      }
    })
    resizeObserver.observe(graphSurfaceRef.value)
  }
})

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/network', (payload) => {
  if (payload.type !== 'network') return

  const now = Date.now()
  const newHosts = payload.hosts || []

  // Compute traffic delta between this update and the previous one
  if (prevTrafficTime.value !== null) {
    const intervalSec = Math.max(1, Math.round((now - prevTrafficTime.value) / 1000))
    let deltaRx = 0, deltaTx = 0
    for (const h of newHosts) {
      const prev = prevTrafficByHost.value[h.id]
      if (prev) {
        const drx = (h.network_rx_bytes || 0) - prev.rx
        const dtx = (h.network_tx_bytes || 0) - prev.tx
        // Ignore negative deltas (counter reset / agent restart)
        if (drx >= 0) deltaRx += drx
        if (dtx >= 0) deltaTx += dtx
      }
    }
    trafficDelta.value = { rx: deltaRx, tx: deltaTx, intervalSec }
  }

  // Save current values for next delta computation
  const snap = {}
  for (const h of newHosts) {
    snap[h.id] = { rx: h.network_rx_bytes || 0, tx: h.network_tx_bytes || 0 }
  }
  prevTrafficByHost.value = snap
  prevTrafficTime.value = now

  hosts.value = newHosts
  containers.value = payload.containers || []

  // Config is loaded only via REST API (loadTopologyConfig), not from WebSocket

  ensureHostPortConfig()
})

onMounted(async () => {
  // Load topology config from server first
  await loadTopologyConfig()
  // Then fetch snapshot to populate real hosts/containers
  await fetchSnapshot()
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})
</script>

<style scoped>
.network-topology-card {
  overflow: hidden;
}

/* Network port row coloring */
.port-row-proxy {
  background-color: rgba(var(--tblr-cyan-rgb, 23, 162, 184), 0.07) !important;
}
.port-row-authelia {
  background-color: rgba(var(--tblr-purple-rgb, 132, 90, 223), 0.08) !important;
}

.network-subnav {
  display: flex;
  gap: 8px;
  padding: 14px 18px 0;
  background: rgba(15, 23, 42, 0.45);
}

.network-topology-body {
  height: auto;
  min-height: 600px;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.network-topology-graph-layout {
  display: flex;
  flex: 1;
  min-height: 0;
  align-items: stretch;
}

.network-config {
  padding: 16px 18px 24px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.45);
  overflow-y: auto;
  max-height: calc(100vh - 260px);
}

.network-config-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-bottom: 12px;
  align-items: start;
}

@media (max-width: 900px) {
  .network-config-row {
    grid-template-columns: 1fr;
  }
}

.network-config-item .form-label {
  font-size: 12px;
  color: #cbd5f5;
}

.network-config-item textarea,
.network-config-item input:not([type="checkbox"]):not([type="radio"]) {
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.4);
  color: #e2e8f0;
}

.network-config-table {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 12px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.55);
}

.network-config-table table {
  margin: 0;
}

.network-config-table thead th {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: #94a3b8;
  background: rgba(15, 23, 42, 0.7);
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.network-config-table tbody td {
  border-top: 1px solid rgba(148, 163, 184, 0.1);
  vertical-align: middle;
}

.network-config-table .form-control,
.network-config-table .form-select {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(148, 163, 184, 0.3);
  color: #e2e8f0;
}

.network-config-item textarea::placeholder,
.network-config-item input::placeholder {
  color: rgba(226, 232, 240, 0.55);
}

.network-graph-surface {
  flex: 1;
  min-height: 400px;
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  overflow-x: hidden;
}

@media (max-width: 991px) {
  .network-topology-body {
    min-height: 420px;
  }

  .network-topology-graph-layout {
    flex-direction: column;
  }

  .network-graph-surface {
    height: 52vh;
  }

  .network-config-row {
    grid-template-columns: 1fr;
  }
}
</style>
