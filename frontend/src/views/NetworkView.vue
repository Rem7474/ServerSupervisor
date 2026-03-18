<template>
  <div>
    <!-- Page Header -->
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>Architecture réseau</span>
      </div>
      <h2 class="page-title">Architecture réseau logique</h2>
      <div class="text-secondary">Relations entre services, reverse proxy, Authelia et exposition Internet</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <!-- KPI Cards -->
    <div class="row row-cards mb-4">
      <!-- Hosts & Containers: context cards, visually de-emphasized -->
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100 kpi-context">
          <div class="card-body">
            <div class="subheader text-secondary">Hôtes</div>
            <div class="h2 mb-0 text-secondary">{{ hosts.length }}</div>
            <div class="text-muted small">{{ hostsOnline }} en ligne</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100 kpi-context">
          <div class="card-body">
            <div class="subheader text-secondary">Conteneurs</div>
            <div class="h2 mb-0 text-secondary">{{ containers.length }}</div>
            <div class="text-muted small">{{ containersRunning }} actifs</div>
          </div>
        </div>
      </div>
      <!-- Network-focused KPIs -->
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Ports visibles</div>
            <div class="h1 mb-0">{{ totalPorts }}</div>
            <div class="text-secondary small">{{ combinedServices.length }} services logiques</div>
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
                <svg width="12" height="12" fill="currentColor" viewBox="0 0 16 16">
                  <path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z"/>
                  <path d="m8.93 6.588-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588zM9 4.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0z"/>
                </svg>
              </span>
            </div>
            <div class="h1 mb-0">{{ trafficDelta.intervalSec > 0 ? formatBytes(trafficDelta.rx + trafficDelta.tx) : '—' }}</div>
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

    <!-- View Mode Toggle -->
    <div class="card mb-4">
      <div class="card-body py-2">
        <div class="d-flex align-items-center gap-3 flex-wrap">
          <div class="btn-group" role="group">
            <input type="radio" class="btn-check" id="viewGraph" value="graph" v-model="viewMode" />
            <label
              class="btn btn-sm"
              :class="viewMode === 'graph' ? 'btn-primary' : 'btn-outline-secondary'"
              for="viewGraph"
            >
              <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1">
                <path d="M0 2a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V2zm2.5 7a.5.5 0 0 0-.5.5v1a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-1a.5.5 0 0 0-.5-.5h-1zm2-4a.5.5 0 0 0-.5.5v5a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5v-5a.5.5 0 0 0-.5-.5h-1zm2-2a.5.5 0 0 0-.5.5v8a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V3.5a.5.5 0 0 0-.5-.5h-1zm2-1a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 .5.5h1a.5.5 0 0 0 .5-.5V2.5a.5.5 0 0 0-.5-.5h-1z"/>
              </svg>
              Graphe
            </label>

            <input type="radio" class="btn-check" id="viewCards" value="cards" v-model="viewMode" />
            <label
              class="btn btn-sm"
              :class="viewMode === 'cards' ? 'btn-primary' : 'btn-outline-secondary'"
              for="viewCards"
            >
              <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1">
                <path d="M1 1a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V1zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1V1zM1 11a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1v-4zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1v-4z"/>
              </svg>
              Cards
            </label>
          </div>
          <span class="text-secondary small d-none d-sm-inline">
            <strong>Graphe</strong> : vue d'architecture logique &nbsp;•&nbsp;
            <strong>Cards</strong> : vue détaillée par hôte / port
          </span>
        </div>
      </div>
    </div>

    <!-- ══════════════════ GRAPH VIEW ══════════════════ -->
    <div v-if="viewMode === 'graph'" class="card mb-4 network-topology-card">

      <!-- Card header -->
      <div class="card-header d-flex align-items-start justify-content-between flex-wrap gap-2">
        <div>
          <h3 class="card-title mb-1">Topologie réseau</h3>
          <div class="text-secondary small">{{ hosts.length }} hôtes · {{ combinedServices.length }} services logiques · {{ totalPorts }} ports mappés</div>
          <!-- Inline legend strip -->
          <div class="topology-legend-strip mt-2">
            <span class="leg-item">
              <span class="leg-box" style="border-color:#94a3b8; background:rgba(15,23,42,0.5)"></span>
              Reverse proxy
            </span>
            <span class="leg-item">
              <span class="leg-box" style="border-color:rgba(148,163,184,0.35); background:rgba(15,23,42,0.42)"></span>
              Hôte
            </span>
            <span class="leg-item">
              <span class="leg-dot" style="background:#38bdf8"></span>
              Service
            </span>
            <span class="leg-item">
              <span class="leg-dot" style="background:#60a5fa"></span>
              Port TCP
            </span>
            <span class="leg-item">
              <span class="leg-dot" style="background:#fb923c"></span>
              Port UDP
            </span>
            <span class="leg-item">
              <span class="leg-dash" style="border-color:#8b5cf6"></span>
              Authelia
            </span>
            <span class="leg-item">
              <span class="leg-dash" style="border-color:#fb923c"></span>
              Internet
            </span>
          </div>
        </div>

        <!-- Right: status + toolbar -->
        <div class="d-flex align-items-center gap-2 flex-wrap">
          <div v-if="saveStatus !== 'idle'" class="d-flex align-items-center gap-2">
            <span v-if="saveStatus === 'saving'" class="spinner-border spinner-border-sm text-secondary"></span>
            <span v-else-if="saveStatus === 'saved'" class="text-success small">✓ Enregistré</span>
            <span v-else-if="saveStatus === 'error'" class="text-danger small">✗ Erreur</span>
          </div>
          <!-- Zoom / layout toolbar (visible in topology tab only) -->
          <div v-if="networkTab === 'topology'" class="btn-group btn-group-sm">
            <button class="btn btn-outline-secondary" title="Zoom +" @click="networkGraphRef?.zoomIn()">
              <svg width="12" height="12" fill="currentColor" viewBox="0 0 16 16">
                <path d="M6.5 1a5.5 5.5 0 1 0 0 11A5.5 5.5 0 0 0 6.5 1zm-4.5 5.5a4.5 4.5 0 1 1 9 0 4.5 4.5 0 0 1-9 0z"/>
                <path d="M6.5 3.5a.5.5 0 0 1 .5.5V6h2a.5.5 0 0 1 0 1H7v2a.5.5 0 0 1-1 0V7H4a.5.5 0 0 1 0-1h2V4a.5.5 0 0 1 .5-.5zm5.35 4.85a.5.5 0 0 1 .707 0l3.5 3.5a.5.5 0 0 1-.707.707l-3.5-3.5a.5.5 0 0 1 0-.707z"/>
              </svg>
            </button>
            <button class="btn btn-outline-secondary" title="Zoom −" @click="networkGraphRef?.zoomOut()">
              <svg width="12" height="12" fill="currentColor" viewBox="0 0 16 16">
                <path d="M6.5 1a5.5 5.5 0 1 0 0 11A5.5 5.5 0 0 0 6.5 1zm-4.5 5.5a4.5 4.5 0 1 1 9 0 4.5 4.5 0 0 1-9 0z"/>
                <path d="M4 6.5a.5.5 0 0 1 .5-.5h4a.5.5 0 0 1 0 1h-4a.5.5 0 0 1-.5-.5zm5.35 1.85a.5.5 0 0 1 .707 0l3.5 3.5a.5.5 0 0 1-.707.707l-3.5-3.5a.5.5 0 0 1 0-.707z"/>
              </svg>
            </button>
            <button class="btn btn-outline-secondary" title="Ajuster à l'écran" @click="networkGraphRef?.fitView()">
              <svg width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path d="M3 3h6M3 3v6M21 3h-6M21 3v6M3 21h6M3 21v-6M21 21h-6M21 21v-6"/>
              </svg>
            </button>
            <button class="btn btn-outline-secondary" title="Réinitialiser la disposition" @click="handleResetLayout">
              <svg width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path d="M20 11a8.1 8.1 0 0 0-15.5-2m-.5-4v4h4"/>
                <path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4"/>
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <ul class="nav nav-tabs px-3 mb-0">
        <li class="nav-item">
          <button class="nav-link" :class="{ active: networkTab === 'topology' }" @click="networkTab = 'topology'">
            <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1">
              <path d="M1 4.5A1.5 1.5 0 0 1 2.5 3h11A1.5 1.5 0 0 1 15 4.5v7a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 1 11.5v-7zm13 0a.5.5 0 0 0-.5-.5h-11a.5.5 0 0 0-.5.5v7a.5.5 0 0 0 .5.5h11a.5.5 0 0 0 .5-.5v-7zm-6 3.5a.5.5 0 0 1 .5-.5h2a.5.5 0 0 1 0 1h-2a.5.5 0 0 1-.5-.5zm-4 0a.5.5 0 0 1 .5-.5h2a.5.5 0 0 1 0 1h-2a.5.5 0 0 1-.5-.5z"/>
            </svg>
            Topologie
          </button>
        </li>
        <li class="nav-item">
          <button class="nav-link" :class="{ active: networkTab === 'config' }" @click="networkTab = 'config'">
            <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1">
              <path d="M9.405 1.05c-.413-1.4-2.397-1.4-2.81 0l-.1.34a1.464 1.464 0 0 1-2.105.872l-.31-.17c-1.283-.698-2.686.264-2.17 1.655l.119.355a1.464 1.464 0 0 1-1.738 1.738l-.355-.119c-1.39-.516-2.353 1.102-1.656 2.17l.17.31a1.464 1.464 0 0 1-.872 2.105l-.34.1c-1.4.413-1.4 2.397 0 2.81l.34.1a1.464 1.464 0 0 1 .872 2.105l-.17.31c-.697 1.283.264 2.686 1.655 2.17l.355-.119a1.464 1.464 0 0 1 1.738 1.738l-.119.355c-.516 1.39 1.102 2.353 2.17 1.656l.31-.17a1.464 1.464 0 0 1 2.105.872l.1.34c.413 1.4 2.397 1.4 2.81 0l.1-.34a1.464 1.464 0 0 1 2.105-.872l.31.17c1.283.697 2.686-.264 2.17-1.655l-.119-.355a1.464 1.464 0 0 1 1.738-1.738l.355.119c1.39.516 2.353-1.102 1.656-2.17l-.17-.31a1.464 1.464 0 0 1 .872-2.105l.34-.1c1.4-.413 1.4-2.397 0-2.81l-.34-.1a1.464 1.464 0 0 1-.872-2.105l.17-.31c.697-1.283-.264-2.686-1.655-2.17l-.355.119a1.464 1.464 0 0 1-1.738-1.738l.119-.355c.516-1.39-1.102-2.353-2.17-1.656l-.31.17a1.464 1.464 0 0 1-2.105-.872l-.1-.34zM8 10.93a2.929 2.929 0 1 1 0-5.86 2.929 2.929 0 0 1 0 5.858z"/>
            </svg>
            Configuration
          </button>
        </li>
      </ul>

      <div class="card-body network-topology-body">
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
              <input type="checkbox" class="form-check-input" v-model="filterInternetOnly" />
              <span class="form-check-label small">Internet uniquement</span>
            </label>
            <label class="form-check form-switch mb-0 d-flex align-items-center gap-2">
              <input
                type="checkbox"
                class="form-check-input"
                v-model="filterHideInternal"
                :disabled="filterInternetOnly"
              />
              <span class="form-check-label small" :class="{ 'text-muted': filterInternetOnly }">
                Masquer les ports internes
              </span>
            </label>
            <span v-if="filterInternetOnly || filterHideInternal" class="badge bg-blue-lt text-blue small">
              Filtre actif
            </span>
          </div>

          <!-- Graph + detail panel layout -->
          <div class="network-topology-graph-layout">
            <div ref="graphSurfaceRef" class="network-graph-surface" :style="{ height: graphHeight }">
              <!-- Loading state -->
              <div v-if="!topologyConfigLoaded" class="graph-loading">
                <span class="spinner-border spinner-border-sm me-2"></span>
                Chargement de la topologie…
              </div>
              <!-- Empty state -->
              <div v-else-if="hosts.length === 0" class="graph-empty-state">
                <svg width="40" height="40" fill="none" stroke="currentColor" stroke-width="1.2" viewBox="0 0 24 24" class="mb-3">
                  <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
                </svg>
                <div class="fw-semibold mb-1">Aucun nœud réseau détecté</div>
                <div class="text-secondary small">Ajoute des hôtes ou configure ta topologie pour voir le diagramme.</div>
              </div>
              <!-- Graph -->
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

            <!-- Draggable splitter -->
            <div class="graph-splitter" @mousedown="onSplitterMouseDown" title="Redimensionner"></div>

            <!-- Detail panel -->
            <NetworkNodeDetail
              :selected-node="selectedNode"
              :hosts="hosts"
              :containers="containers"
              :host-port-overrides="hostPortOverrides"
              :combined-services="combinedServices"
              :discovered-ports-by-host="discoveredPortsByHost"
              :style="{ flexBasis: detailPanelWidth + 'px', flexShrink: 0 }"
            />
          </div>
        </template>
      </div>
    </div>

    <!-- ══════════════════ CARDS VIEW ══════════════════ -->
    <NetworkPortList v-if="viewMode === 'cards'" :hosts="hosts" :containers="containers" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, watchEffect, nextTick } from 'vue'
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
const graphSurfaceRef = ref(null)
const graphHeight = ref('auto')
const selectedNode = ref(null)
const networkGraphRef = ref(null)

// Graph filters
const filterInternetOnly = ref(false)
const filterHideInternal = ref(false)

// Detail panel resizing
const detailPanelWidth = ref(300)
let isDraggingSplitter = false
let splitterStartX = 0
let splitterStartWidth = 0

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

// ─── Layout reset (toolbar button) ────────────────────────────────────────
function handleResetLayout() {
  nodePositions.value = {}
  nextTick(() => {
    networkGraphRef.value?.resetLayout()
    debouncedSave()
  })
}

// ─── Splitter drag ─────────────────────────────────────────────────────────
function onSplitterMouseDown(e) {
  isDraggingSplitter = true
  splitterStartX = e.clientX
  splitterStartWidth = detailPanelWidth.value
  document.addEventListener('mousemove', onSplitterMouseMove)
  document.addEventListener('mouseup', onSplitterMouseUp)
  e.preventDefault()
}

function onSplitterMouseMove(e) {
  if (!isDraggingSplitter) return
  const delta = splitterStartX - e.clientX // dragging left → panel grows
  detailPanelWidth.value = Math.max(200, Math.min(560, splitterStartWidth + delta))
}

function onSplitterMouseUp() {
  isDraggingSplitter = false
  document.removeEventListener('mousemove', onSplitterMouseMove)
  document.removeEventListener('mouseup', onSplitterMouseUp)
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

// ─── Filtered graph data ───────────────────────────────────────────────────
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

// ─── Graph surface auto-height ────────────────────────────────────────────
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

onUnmounted(() => {
  if (resizeObserver) resizeObserver.disconnect()
  document.removeEventListener('mousemove', onSplitterMouseMove)
  document.removeEventListener('mouseup', onSplitterMouseUp)
})
</script>

<style scoped>
.network-topology-card {
  overflow: hidden;
}

/* De-emphasized context KPI cards */
.kpi-context {
  opacity: 0.75;
}

/* Inline legend strip in card header */
.topology-legend-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 11px;
  color: #94a3b8;
}

.leg-item {
  display: flex;
  align-items: center;
  gap: 5px;
}

.leg-box {
  display: inline-block;
  width: 18px;
  height: 10px;
  border: 1.5px solid;
  border-radius: 3px;
  flex-shrink: 0;
}

.leg-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.leg-dash {
  display: inline-block;
  width: 18px;
  height: 0;
  border-top: 2px dashed;
  flex-shrink: 0;
}

/* Filter bar */
.graph-filters {
  background: rgba(15, 23, 42, 0.3);
  font-size: 13px;
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

.network-graph-surface {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 400px;
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
}

/* Loading / empty state overlays */
.graph-loading,
.graph-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex: 1;
  text-align: center;
  padding: 40px 24px;
  color: #64748b;
  min-height: 300px;
}

.graph-loading {
  flex-direction: row;
  font-size: 14px;
}

.graph-empty-state .fw-semibold {
  color: #94a3b8;
  font-size: 16px;
}

/* Draggable splitter */
.graph-splitter {
  width: 5px;
  cursor: col-resize;
  background: rgba(148, 163, 184, 0.1);
  border-left: 1px solid rgba(148, 163, 184, 0.15);
  border-right: 1px solid rgba(148, 163, 184, 0.15);
  flex-shrink: 0;
  transition: background 0.15s;
}

.graph-splitter:hover {
  background: rgba(96, 165, 250, 0.25);
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
    flex: none;
  }

  .graph-splitter {
    display: none;
  }
}
</style>
