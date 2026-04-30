<template>
  <div>
    <!-- Page Header -->
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Architecture réseau</span>
      </div>
      <h2 class="page-title">
        Architecture réseau logique
      </h2>
      <div class="text-secondary">
        Relations entre services, reverse proxy, Authelia et exposition Internet
      </div>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      @reconnect="reconnect"
    />

    <!-- KPI Cards -->
    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Hôtes
            </div>
            <div class="h2 mb-0">
              {{ hosts.length }}
            </div>
            <div class="text-muted small">
              {{ hostsOnline }} en ligne
            </div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Conteneurs
            </div>
            <div class="h2 mb-0">
              {{ containers.length }}
            </div>
            <div class="text-muted small">
              {{ containersRunning }} actifs
            </div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Ports visibles
            </div>
            <div class="h1 mb-0">
              {{ totalPorts }}
            </div>
            <div class="text-secondary small">
              {{ combinedServices.length }} services logiques
            </div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="d-flex align-items-center gap-1 subheader">
              Trafic réseau
              <span
                class="ms-1"
                style="cursor:help; color:#64748b;"
                title="Delta calculé entre les deux dernières mises à jour WebSocket. Les deltas négatifs (reset de compteur après redémarrage agent) sont ignorés."
              >
                <svg
                  width="12"
                  height="12"
                  fill="currentColor"
                  viewBox="0 0 16 16"
                >
                  <path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z" />
                  <path d="m8.93 6.588-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588zM9 4.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0z" />
                </svg>
              </span>
            </div>
            <div class="h1 mb-0">
              {{ trafficDelta.intervalSec > 0 ? formatBytes(trafficDelta.rx + trafficDelta.tx) : '—' }}
            </div>
            <div class="text-secondary small">
              <span v-if="trafficDelta.intervalSec > 0">
                sur {{ trafficDelta.intervalSec }}s · ↓ {{ formatBytes(trafficDelta.rx) }} / ↑ {{ formatBytes(trafficDelta.tx) }}
              </span>
              <span v-else>En attente de données…</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ══════════════════ UNIFIED NETWORK CARD ══════════════════ -->
    <div class="card mb-4 network-topology-card">
      <!-- Card header -->
      <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
        <div>
          <h3 class="card-title mb-0">
            {{ viewMode === 'graph' ? 'Topologie réseau' : 'Ports &amp; conteneurs' }}
          </h3>
          <div class="text-secondary small mt-1">
            {{ hosts.length }} hôtes · {{ combinedServices.length }} services logiques · {{ totalPorts }} ports mappés
          </div>
        </div>

        <div class="d-flex align-items-center gap-2 flex-wrap">
          <!-- Save status (graph mode only) -->
          <div
            v-if="viewMode === 'graph' && saveStatus !== 'idle'"
            class="d-flex align-items-center gap-2"
          >
            <span
              v-if="saveStatus === 'saving'"
              class="spinner-border spinner-border-sm text-secondary"
            />
            <span
              v-else-if="saveStatus === 'saved'"
              class="text-success small"
            >✓ Enregistré</span>
            <span
              v-else-if="saveStatus === 'error'"
              class="text-danger small"
            >✗ Erreur</span>
          </div>

          <!-- View mode toggle -->
          <div
            class="btn-group btn-group-sm"
            role="group"
          >
            <button
              class="btn"
              :class="viewMode === 'graph' ? 'btn-primary' : 'btn-outline-secondary'"
              @click="viewMode = 'graph'"
            >
              <svg
                width="14"
                height="14"
                fill="currentColor"
                viewBox="0 0 16 16"
                class="me-1"
              >
                <path d="M0 2a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V2zm2.5 7a.5.5 0 0 0-.5.5v1a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-1a.5.5 0 0 0-.5-.5h-1zm2-4a.5.5 0 0 0-.5.5v5a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-5a.5.5 0 0 0-.5-.5h-1zm2-2a.5.5 0 0 0-.5.5v8a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V3.5a.5.5 0 0 0-.5-.5h-1zm2-1a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V2.5a.5.5 0 0 0-.5-.5h-1z" />
              </svg>
              Graphe
            </button>
            <button
              class="btn"
              :class="viewMode === 'cards' ? 'btn-primary' : 'btn-outline-secondary'"
              @click="viewMode = 'cards'"
            >
              <svg
                width="14"
                height="14"
                fill="currentColor"
                viewBox="0 0 16 16"
                class="me-1"
              >
                <path d="M1 1a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V1zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1V1zM1 11a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1v-4zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1v-4z" />
              </svg>
              Cartes
            </button>
          </div>
        </div>
      </div>

      <!-- Tabs: only in graph mode -->
      <ul
        v-if="viewMode === 'graph'"
        class="nav nav-tabs px-3 mb-0"
      >
        <li class="nav-item">
          <button
            class="nav-link"
            :class="{ active: networkTab === 'topology' }"
            @click="networkTab = 'topology'"
          >
            <svg
              width="14"
              height="14"
              fill="currentColor"
              viewBox="0 0 16 16"
              class="me-1"
            >
              <path d="M1 4.5A1.5 1.5 0 0 1 2.5 3h11A1.5 1.5 0 0 1 15 4.5v7a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 1 11.5v-7zm13 0a.5.5 0 0 0-.5-.5h-11a.5.5 0 0 0-.5.5v7a.5.5 0 0 0 .5.5h11a.5.5 0 0 0 .5-.5v-7zm-6 3.5a.5.5 0 0 1 .5-.5h2a.5.5 0 0 1 0 1h-2a.5.5 0 0 1-.5-.5zm-4 0a.5.5 0 0 1 .5-.5h2a.5.5 0 0 1 0 1h-2a.5.5 0 0 1-.5-.5z" />
            </svg>
            Topologie
          </button>
        </li>
        <li class="nav-item">
          <button
            class="nav-link"
            :class="{ active: networkTab === 'config' }"
            @click="networkTab = 'config'"
          >
            <svg
              width="14"
              height="14"
              fill="currentColor"
              viewBox="0 0 16 16"
              class="me-1"
            >
              <path d="M9.405 1.05c-.413-1.4-2.397-1.4-2.81 0l-.1.34a1.464 1.464 0 0 1-2.105.872l-.31-.17c-1.283-.698-2.686.264-2.17 1.655l.119.355a1.464 1.464 0 0 1-1.738 1.738l-.355-.119c-1.39-.516-2.353 1.102-1.656 2.17l.17.31a1.464 1.464 0 0 1-.872 2.105l-.34.1c-1.4.413-1.4 2.397 0 2.81l.34.1a1.464 1.464 0 0 1 .872 2.105l-.17.31c-.697 1.283.264 2.686 1.655 2.17l.355-.119a1.464 1.464 0 0 1 1.738 1.738l-.119.355c-.516 1.39 1.102 2.353 2.17 1.656l.31-.17a1.464 1.464 0 0 1 2.105.872l.1.34c.413 1.4 2.397 1.4 2.81 0l.1-.34a1.464 1.464 0 0 1 2.105-.872l.31.17c1.283.697 2.686-.264 2.17-1.655l-.119-.355a1.464 1.464 0 0 1 1.738-1.738l.355.119c1.39.516 2.353-1.102 1.656-2.17l-.17-.31a1.464 1.464 0 0 1 .872-2.105l.34-.1c1.4-.413 1.4-2.397 0-2.81l-.34-.1a1.464 1.464 0 0 1-.872-2.105l.17-.31c.697-1.283-.264-2.686-1.655-2.17l-.355.119a1.464 1.464 0 0 1-1.738-1.738l.119-.355c.516-1.39-1.102-2.353-2.17-1.656l-.31.17a1.464 1.464 0 0 1-2.105-.872l-.1-.34zM8 10.93a2.929 2.929 0 1 1 0-5.86 2.929 2.929 0 0 1 0 5.858z" />
            </svg>
            Configuration
          </button>
        </li>
      </ul>

      <!-- Card body -->
      <div class="network-card-body">

        <!-- ── GRAPH MODE ── -->
        <template v-if="viewMode === 'graph'">

          <!-- Configuration tab -->
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

          <!-- Topology tab -->
          <template v-else>
            <!-- Filters bar -->
            <div class="graph-filters d-flex align-items-center gap-4 px-3 py-2 border-bottom flex-wrap">
              <label class="form-check form-switch mb-0 d-flex align-items-center gap-2">
                <input
                  v-model="filterInternetOnly"
                  type="checkbox"
                  class="form-check-input"
                >
                <span class="form-check-label small">Internet uniquement</span>
              </label>
              <label class="form-check form-switch mb-0 d-flex align-items-center gap-2">
                <input
                  v-model="filterHideInternal"
                  type="checkbox"
                  class="form-check-input"
                  :disabled="filterInternetOnly"
                >
                <span
                  class="form-check-label small"
                  :class="{ 'text-muted': filterInternetOnly }"
                >
                  Masquer les ports internes
                </span>
              </label>
              <span
                v-if="filterInternetOnly || filterHideInternal"
                class="badge bg-blue-lt text-blue small"
              >
                Filtre actif
              </span>
            </div>

            <!-- Graph: full width -->
            <div class="network-graph-surface">
              <div
                v-if="!topologyConfigLoaded"
                class="graph-state-overlay"
              >
                <span class="spinner-border spinner-border-sm me-2" />
                Chargement de la topologie…
              </div>
              <div
                v-else-if="hosts.length === 0"
                class="graph-state-overlay graph-state-empty"
              >
                <svg
                  width="40"
                  height="40"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.2"
                  viewBox="0 0 24 24"
                  class="mb-3"
                >
                  <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
                </svg>
                <div class="fw-semibold mb-1">
                  Aucun nœud réseau détecté
                </div>
                <div class="text-secondary small">
                  Ajoute des hôtes ou configure ta topologie pour voir le diagramme.
                </div>
              </div>
              <NetworkGraph
                v-else
                ref="networkGraphRef"
                :data="filteredGraphHosts"
                :root-label="rootNodeName"
                :root-ip="rootNodeIp"
                :services="filteredServices"
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

            <!-- Detail panel: full width, below graph, dismissible -->
            <Transition name="detail-slide">
              <NetworkNodeDetail
                v-if="selectedNode"
                :selected-node="selectedNode"
                :hosts="hosts"
                :containers="containers"
                :host-port-overrides="hostPortOverrides"
                :combined-services="combinedServices"
                :discovered-ports-by-host="discoveredPortsByHost"
                @close="selectedNode = null"
              />
            </Transition>
          </template>
        </template>

        <!-- ── CARDS MODE ── -->
        <div
          v-else
          class="p-3"
        >
          <NetworkPortList
            :hosts="hosts"
            :containers="containers"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import NetworkNodeDetail from '../components/network/NetworkNodeDetail.vue'
import NetworkPortList from '../components/network/NetworkPortList.vue'
import NetworkTopologyConfig from '../components/network/NetworkTopologyConfig.vue'
import apiClient from '../api'

// ─── State ────────────────────────────────────────────────────────────────
const hosts = ref([])
const containers = ref([])
const viewMode = ref(localStorage.getItem('networkViewMode') || 'graph')
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
const selectedNode = ref(null)
const networkGraphRef = ref(null)

// Graph filters
const filterInternetOnly = ref(false)
const filterHideInternal = ref(false)

// ─── Persist view mode ────────────────────────────────────────────────────
watch(viewMode, (newMode) => {
  localStorage.setItem('networkViewMode', newMode)
})

// ─── Debounced save ───────────────────────────────────────────────────────
let saveTimeout = null
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

// ─── Topology config load/save ────────────────────────────────────────────
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
        try { hostPortConfig.value = JSON.parse(cfg.host_overrides) } catch { hostPortConfig.value = [] }
      }
    }
  } catch (e) {
    console.warn('Failed to load topology config from server:', e)
  } finally {
    topologyConfigLoaded.value = true
  }
}

async function saveTopologyConfig() {
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

// ─── Layout reset ─────────────────────────────────────────────────────────
function handleResetLayout() {
  nodePositions.value = {}
  networkGraphRef.value?.resetLayout()
  debouncedSave()
}

// ─── Computed: port discovery ──────────────────────────────────────────────
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

const combinedServices = computed(() => {
  const linkedServices = []
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    for (const [port, settings] of Object.entries(entry.ports || {})) {
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

const graphHosts = computed(() => {
  const portsByHost = new Map()
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
      const hostPorts = portsByHost.get(hostId)
      if (!hostPorts.has(key)) {
        hostPorts.set(key, { port: portNumber, protocol, containers: [] })
      }
      hostPorts.get(key).containers.push(container.name)
    }
  }
  return hosts.value.map((host) => ({
    ...host,
    ports: portsByHost.has(host.id) ? Array.from(portsByHost.get(host.id).values()) : [],
  }))
})

const filteredGraphHosts = computed(() => {
  if (!filterInternetOnly.value && !filterHideInternal.value) return graphHosts.value
  return graphHosts.value.map(host => {
    const override = hostPortOverrides.value[host.id] || {}
    const proxyPorts = override.proxyPorts || new Set()
    const internetPorts = override.internetExposedPorts || {}
    let ports = host.ports || []
    if (filterInternetOnly.value) {
      ports = ports.filter(p => Number(p.port) in internetPorts)
    } else if (filterHideInternal.value) {
      ports = ports.filter(p => {
        const pn = Number(p.port)
        return proxyPorts.has(pn) || pn in internetPorts
      })
    }
    return { ...host, ports }
  })
})

const filteredServices = computed(() => {
  if (!filterInternetOnly.value) return combinedServices.value
  return combinedServices.value.filter(s => s.exposedToInternet)
})

// ─── KPI computeds ────────────────────────────────────────────────────────
const totalPorts = computed(() => graphHosts.value.reduce((sum, host) => sum + (host.ports?.length || 0), 0))
const hostsOnline = computed(() => hosts.value.filter(h => h.status === 'online').length)
const containersRunning = computed(() => containers.value.filter(c => c.state === 'running').length)

const trafficDelta = ref({ rx: 0, tx: 0, intervalSec: 0 })
const prevTrafficByHost = ref({})
const prevTrafficTime = ref(null)

// ─── Helpers ──────────────────────────────────────────────────────────────
function formatBytes(bytes) {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) { value /= 1024; idx++ }
  return `${value.toFixed(1)} ${units[idx]}`
}

function ensureHostPortConfig() {
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

// ─── Data fetch ───────────────────────────────────────────────────────────
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

// ─── WebSocket ────────────────────────────────────────────────────────────
const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/network', (payload) => {
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

  const snap = {}
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

// expose for toolbar reset button (used nowhere else now — kept for NetworkGraph ref)
defineExpose({ handleResetLayout })
</script>

<style scoped>
.network-topology-card {
  overflow: hidden;
}

.network-card-body {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* Filter bar */
.graph-filters {
  background: rgba(15, 23, 42, 0.3);
  font-size: 13px;
  flex-shrink: 0;
}

/* Graph canvas: full width, CSS-driven height */
.network-graph-surface {
  position: relative;
  width: 100%;
  height: calc(100vh - 380px);
  min-height: 480px;
  display: flex;
  flex-direction: column;
}

/* Loading / empty overlays */
.graph-state-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  color: #64748b;
  z-index: 2;
}

.graph-state-empty {
  flex-direction: column;
  text-align: center;
  padding: 40px 24px;
}

.graph-state-empty .fw-semibold {
  color: #94a3b8;
  font-size: 16px;
}

/* Detail panel slide-in transition */
.detail-slide-enter-active,
.detail-slide-leave-active {
  transition: max-height 0.25s ease, opacity 0.2s ease;
  overflow: hidden;
}

.detail-slide-enter-from,
.detail-slide-leave-to {
  max-height: 0;
  opacity: 0;
}

.detail-slide-enter-to,
.detail-slide-leave-from {
  max-height: 320px;
  opacity: 1;
}

@media (max-width: 991px) {
  .network-graph-surface {
    height: 52vh;
    min-height: 360px;
  }
}
</style>
