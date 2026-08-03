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
                style="cursor:help; color:var(--ss-text-subtle-on-dark);"
                title="Delta calculé entre les deux dernières mises à jour WebSocket. Les deltas négatifs (reset de compteur après redémarrage agent) sont ignorés."
              >
                <IconInfoCircle :size="12" />
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
              type="button"
              class="btn"
              :class="viewMode === 'graph' ? 'btn-primary' : 'btn-outline-secondary'"
              @click="viewMode = 'graph'"
            >
              <IconChartBar
                :size="14"
                class="me-1"
              />
              Graphe
            </button>
            <button
              type="button"
              class="btn"
              :class="viewMode === 'cards' ? 'btn-primary' : 'btn-outline-secondary'"
              @click="viewMode = 'cards'"
            >
              <IconLayoutGrid
                :size="14"
                class="me-1"
              />
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
            type="button"
            class="nav-link"
            :class="{ active: networkTab === 'topology' }"
            @click="networkTab = 'topology'"
          >
            <IconSitemap
              :size="14"
              class="me-1"
            />
            Topologie
          </button>
        </li>
        <li class="nav-item">
          <button
            type="button"
            class="nav-link"
            :class="{ active: networkTab === 'config' }"
            @click="networkTab = 'config'"
          >
            <IconSettings
              :size="14"
              class="me-1"
            />
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
            v-model:root-host-id="rootHostId"
            v-model:authelia-host-id="autheliaHostId"
            v-model:root-port-id="rootPortId"
            v-model:authelia-port-id="autheliaPortId"
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
                class="badge bg-primary-lt text-primary small"
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
                <LoadingSkeleton variant="chart" />
              </div>
              <div
                v-else-if="hosts.length === 0"
                class="graph-state-overlay graph-state-empty"
              >
                <IconStack2
                  :size="40"
                  class="mb-3"
                  :stroke-width="1.2"
                />
                <div class="fw-semibold mb-1">
                  Aucun nœud réseau détecté
                </div>
                <div class="text-secondary small">
                  Ajoute des hôtes ou configure ta topologie pour voir le diagramme.
                </div>
              </div>
              <ErrorBoundary
                v-else
                title="Erreur lors du rendu du graphe réseau"
              >
                <NetworkGraph
                  ref="networkGraphRef"
                  :data="filteredGraphHosts"
                  :root-label="rootNodeName"
                  :root-ip="rootNodeIp"
                  :services="filteredServices"
                  :guests="guestNodes"
                  :host-port-overrides="hostPortOverrides"
                  :authelia-label="autheliaLabel"
                  :authelia-ip="autheliaIp"
                  :internet-label="internetLabel"
                  :internet-ip="internetIp"
                  :node-positions="nodePositions"
                  :root-host-id="rootHostId"
                  :authelia-host-id="autheliaHostId"
                  :root-port-id="rootPortId"
                  :authelia-port-id="autheliaPortId"
                  @node-select="selectedNode = $event"
                  @update:node-positions="onNodePositionsUpdate"
                />
              </ErrorBoundary>
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
            :proxmox-guests="proxmoxGuestIPs"
            :npm-entries="npmEntries"
            :ip-inventory-loading="ipInventoryLoading"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { IconInfoCircle, IconChartBar, IconLayoutGrid, IconSitemap, IconSettings, IconStack2 } from '@tabler/icons-vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import NetworkGraph from '../components/network/NetworkGraph.vue'
import ErrorBoundary from '../components/common/ErrorBoundary.vue'
import NetworkNodeDetail from '../components/network/NetworkNodeDetail.vue'
import NetworkPortList from '../components/network/NetworkPortList.vue'
import NetworkTopologyConfig from '../components/network/NetworkTopologyConfig.vue'
import { useNetwork } from '../composables/useNetwork'

const {
  hosts,
  containers,
  proxmoxGuestIPs,
  npmEntries,
  ipInventoryLoading,
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
} = useNetwork()

// Template ref to the NetworkGraph component instance — kept local per the
// DOM/component-ref convention (composables own state, not component refs).
const networkGraphRef = ref<any>(null)

// ─── Layout reset ─────────────────────────────────────────────────────────
function handleResetLayout(): void {
  nodePositions.value = {}
  networkGraphRef.value?.resetLayout()
  debouncedSave()
}

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
  background: var(--ss-panel-soft);
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
  color: var(--ss-text-subtle-on-dark);
  z-index: 2;
}

.graph-state-empty {
  flex-direction: column;
  text-align: center;
  padding: 40px 24px;
}

.graph-state-empty .fw-semibold {
  color: var(--ss-text-muted-on-dark);
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
