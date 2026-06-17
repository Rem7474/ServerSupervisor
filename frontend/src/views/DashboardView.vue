<template>
  <div>
    <!-- ─── En-tête ─────────────────────────────────────────────────────────── -->
    <div
      class="page-header d-flex flex-column flex-md-row align-items-md-center
                justify-content-between gap-3 mb-4"
    >
      <div>
        <div class="page-pretitle">
          ServerSupervisor
        </div>
        <h2 class="page-title">
          Dashboard
        </h2>
        <div class="text-secondary">
          Vue d'ensemble de l'infrastructure
        </div>
      </div>
      <router-link
        to="/hosts/new"
        class="btn btn-primary btn-sm"
      >
        <svg
          class="icon"
          width="20"
          height="20"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 6v6m0 0v6m0-6h6m-6 0H6"
          />
        </svg>
        <span class="d-none d-sm-inline ms-1">Ajouter un hôte</span>
      </router-link>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      :data-stale-alert="dataStaleAlert"
      @reconnect="reconnect"
      @dismiss-stale-alert="dataStaleAlert = false"
    />

    <!-- ─── Bannières d'alerte ───────────────────────────────────────────────── -->

    <div
      v-if="cveSummary && ((cveSummary.critical_count || 0) > 0 || (cveSummary.hosts_with_critical || 0) > 0)"
      class="alert alert-danger mb-3 d-flex align-items-center gap-3"
    >
      <AppIcon
        name="warning"
        :size="24"
        css-class="icon icon-lg icon-responsive-lg flex-shrink-0"
      />
      <div class="flex-grow-1">
        <div class="fw-semibold">
          Vulnérabilités critiques détectées
        </div>
        <div class="text-secondary small">
          <span class="badge bg-red-lt text-red me-1">CRITICAL</span>
          {{ cveSummary.critical_count || 0 }} CVE
          <span v-if="(cveSummary.hosts_with_critical || 0) > 0"> sur {{ cveSummary.hosts_with_critical || 0 }} hôte{{ pluralize(cveSummary.hosts_with_critical) }}</span>
        </div>
      </div>
      <router-link
        to="/apt"
        class="btn btn-sm btn-danger"
      >
        Voir les mises à jour
      </router-link>
    </div>

    <div
      v-if="proxmoxSummary && ((proxmoxSummary.nodes_down ?? 0) > 0 || (proxmoxSummary.recent_failed_tasks ?? 0) > 0 || (proxmoxSummary.storage_near_full ?? 0) > 0 || (proxmoxSummary.storage_offline ?? 0) > 0)"
      class="alert alert-warning mb-3 d-flex align-items-center gap-3"
    >
      <AppIcon
        name="warning"
        :size="24"
        css-class="icon icon-lg icon-responsive-lg flex-shrink-0"
      />
      <div class="flex-grow-1">
        <div class="fw-semibold">
          Alertes Proxmox
        </div>
        <div class="text-secondary small d-flex flex-wrap gap-2">
          <span v-if="(proxmoxSummary.nodes_down ?? 0) > 0">{{ proxmoxSummary.nodes_down }} nœud{{ pluralize(proxmoxSummary.nodes_down) }} hors ligne</span>
          <span v-if="(proxmoxSummary.storage_near_full ?? 0) > 0">{{ proxmoxSummary.storage_near_full }} stockage{{ pluralize(proxmoxSummary.storage_near_full) }} presque plein{{ pluralize(proxmoxSummary.storage_near_full) }}</span>
          <span v-if="(proxmoxSummary.storage_offline ?? 0) > 0">{{ proxmoxSummary.storage_offline }} stockage{{ pluralize(proxmoxSummary.storage_offline) }} hors ligne</span>
          <span v-if="(proxmoxSummary.recent_failed_tasks ?? 0) > 0">{{ proxmoxSummary.recent_failed_tasks }} tâche{{ pluralize(proxmoxSummary.recent_failed_tasks) }} échouée{{ pluralize(proxmoxSummary.recent_failed_tasks) }} (24h)</span>
        </div>
      </div>
      <router-link
        to="/proxmox"
        class="btn btn-sm btn-warning"
      >
        Voir Proxmox
      </router-link>
    </div>

    <!-- ─── KPIs ─────────────────────────────────────────────────────────────── -->
    <LoadingSkeleton
      v-if="loading"
      variant="kpi"
      :lines="4"
    />
    <DashboardKPIs
      v-else
      :cve-summary="(cveSummary as any)"
    />

    <!-- ─── Cluster Proxmox (conditionnel) ──────────────────────────────────── -->
    <LoadingSkeleton
      v-if="loading && hasProxmox"
      variant="proxmox-cluster"
      :lines="3"
    />
    <ProxmoxClusterCard
      v-else-if="hasProxmox && proxmoxNodes.length"
      :nodes="(proxmoxNodes as any)"
    />

    <!-- ─── Graphiques de tendance ───────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="card-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <h3 class="card-title">
            Tendance CPU / RAM
          </h3>
          <div class="text-secondary small">
            <template v-if="hasProxmox">
              <div
                class="summary-source-switch"
                role="group"
                aria-label="Source des métriques du graphe"
              >
                <button
                  v-for="src in chartSources"
                  :key="src.key"
                  type="button"
                  :class="chartSource === src.key ? 'btn btn-sm btn-primary' : 'btn btn-sm btn-outline-secondary'"
                  :aria-pressed="chartSource === src.key"
                  @click="chartSource = src.key; fetchSummary()"
                >
                  {{ src.label }}
                </button>
              </div>
            </template>
            <template v-else>
              Moyenne sur tous les hôtes
            </template>
          </div>
        </div>
        <div class="btn-group btn-group-sm">
          <button
            v-for="h in [1, 6, 24, 168, 720]"
            :key="h"
            type="button"
            :class="summaryHours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'"
            @click="changeSummaryRange(h)"
          >
            {{ h >= 24 ? (h / 24) + 'j' : h + 'h' }}
          </button>
        </div>
      </div>
      <div
        ref="chartContainerRef"
        class="card-body summary-chart-body"
      >
        <div
          v-if="summaryLoading || !chartVisible"
          class="h-100 d-flex align-items-center justify-content-center"
        >
          <div
            class="spinner-border text-secondary"
            role="status"
          />
        </div>
        <Line
          v-else-if="summaryChartData"
          :data="(summaryChartData as any)"
          :options="(summaryChartOptions as any)"
          class="h-100"
        />
        <div
          v-else
          class="h-100 d-flex align-items-center justify-content-center text-secondary"
        >
          Aucune donnée
        </div>
      </div>
    </div>

    <!-- ─── Recherche / filtre ───────────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3 align-items-center">
          <div class="col-12 col-lg">
            <label
              class="form-label"
              for="dashboard-search"
            >Recherche d'hôte</label>
            <input
              id="dashboard-search"
              v-model="searchQuery"
              type="text"
              class="form-control"
              placeholder="Rechercher un hôte..."
            >
          </div>
          <div class="col-12 col-md-4 col-lg-2">
            <label
              class="form-label"
              for="dashboard-status-filter"
            >Filtre de statut</label>
            <select
              id="dashboard-status-filter"
              v-model="statusFilter"
              class="form-select"
            >
              <option value="all">
                Tous
              </option>
              <option value="online">
                En ligne
              </option>
              <option value="offline">
                Hors ligne
              </option>
              <option value="warning">
                Warning
              </option>
            </select>
          </div>
          <div
            v-if="canRunApt"
            class="col-12 col-md-auto d-flex align-items-end"
          >
            <button
              type="button"
              class="btn btn-outline-secondary btn-sm"
              @click="selectAllFiltered"
            >
              Tout sélectionner
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ─── Table des hôtes ──────────────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th class="host-selection-col" />
              <th>
                <SortableHeader
                  label="Nom"
                  :active="sortKey === 'name'"
                  :direction="sortDir"
                  @toggle="toggleSort('name')"
                />
              </th>
              <th>
                <SortableHeader
                  label="État"
                  :active="sortKey === 'status'"
                  :direction="sortDir"
                  @toggle="toggleSort('status')"
                />
              </th>
              <th>
                <SortableHeader
                  label="IP / OS"
                  :active="sortKey === 'ip_os'"
                  :direction="sortDir"
                  @toggle="toggleSort('ip_os')"
                />
              </th>
              <th>
                <SortableHeader
                  label="CPU"
                  :active="sortKey === 'cpu'"
                  :direction="sortDir"
                  @toggle="toggleSort('cpu')"
                />
              </th>
              <th>
                <SortableHeader
                  label="RAM"
                  :active="sortKey === 'ram'"
                  :direction="sortDir"
                  @toggle="toggleSort('ram')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Disque"
                  :active="sortKey === 'disk'"
                  :direction="sortDir"
                  @toggle="toggleSort('disk')"
                />
              </th>
              <th title="Paquets APT en attente">
                <SortableHeader
                  label="APT"
                  :active="sortKey === 'apt'"
                  :direction="sortDir"
                  @toggle="toggleSort('apt')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Uptime"
                  :active="sortKey === 'uptime'"
                  :direction="sortDir"
                  @toggle="toggleSort('uptime')"
                />
              </th>
              <th class="last-activity-col">
                <SortableHeader
                  label="Dernière activité"
                  :active="sortKey === 'last_seen'"
                  :direction="sortDir"
                  @toggle="toggleSort('last_seen')"
                />
              </th>
            </tr>
          </thead>
          <tbody>
            <template v-if="metricsReady">
              <tr
                v-for="host in paginatedHosts"
                :key="host.id"
                :class="{ 'table-active': selectedHostIds.includes(host.id) }"
              >
                <td>
                  <label class="form-check">
                    <input
                      v-model="selectedHostIds"
                      class="form-check-input"
                      type="checkbox"
                      :value="host.id"
                    >
                    <span class="form-check-label" />
                  </label>
                </td>
                <td>
                  <router-link
                    :to="`/hosts/${host.id}`"
                    class="fw-semibold text-decoration-none"
                  >
                    {{ host.name || host.hostname || 'Sans nom' }}
                  </router-link>
                  <div class="text-secondary small">
                    {{ host.hostname || 'Non connecté' }}
                  </div>
                  <div
                    v-if="proxmoxGuestPath(host.id)"
                    class="mt-1"
                  >
                    <router-link
                      :to="proxmoxGuestPath(host.id)"
                      class="badge bg-orange-lt text-orange text-decoration-none"
                    >
                      Stats Proxmox
                    </router-link>
                  </div>
                </td>
                <td class="status-col">
                  <span :class="hostStatusClass(host.status || '')">
                    <span :class="['status-dot', host.status === 'online' ? 'status-dot-animated' : '']" />
                    {{ formatHostStatus(host.status || '') }}
                  </span>
                </td>
                <td>
                  <div class="text-body">
                    {{ host.ip_address }}
                  </div>
                  <div class="text-secondary small">
                    {{ host.os || 'N/A' }}
                  </div>
                </td>
                <td>
                  <span
                    :class="cpuColor(effectiveMetricsByHost[host.id]?.cpu)"
                    :title="effectiveMetricsByHost[host.id]?.source === 'proxmox' ? 'proxmox' : ''"
                  >
                    {{ effectiveMetricsByHost[host.id]?.cpu != null ? effectiveMetricsByHost[host.id]!.cpu!.toFixed(1) + '%' : '-' }}
                  </span>
                </td>
                <td>
                  <span
                    :class="memColor(effectiveMetricsByHost[host.id]?.memPct)"
                    :title="effectiveMetricsByHost[host.id]?.source === 'proxmox' ? 'proxmox' : ''"
                  >
                    {{ effectiveMetricsByHost[host.id]?.memPct != null ? effectiveMetricsByHost[host.id]!.memPct!.toFixed(1) + '%' : '-' }}
                  </span>
                </td>
                <td>
                  <span :class="diskColor(diskUsage[host.id])">
                    {{ diskUsage[host.id] != null ? diskUsage[host.id].toFixed(1) + '%' : '-' }}
                  </span>
                </td>
                <td>
                  <span
                    v-if="aptPendingHosts[host.id]"
                    class="badge bg-yellow-lt text-yellow"
                  >{{ aptPendingHosts[host.id] }}</span>
                  <span
                    v-else
                    class="text-secondary"
                  >—</span>
                </td>
                <td>{{ hostMetrics[host.id] ? formatUptime(hostMetrics[host.id]!.uptime) : '-' }}</td>
                <td class="last-activity-col">
                  <RelativeTime :date="(host.last_seen as any) || ''" />
                </td>
              </tr>
              <tr v-if="hosts.length > 0 && sortedHosts.length === 0">
                <td
                  colspan="10"
                  class="text-center text-secondary py-4"
                >
                  Aucun hôte ne correspond à votre recherche.
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <div
        v-if="metricsReady && sortedHosts.length > 0"
        class="card-footer d-flex justify-content-end"
      >
        <PaginationNav
          :current-page="currentHostPage"
          :total-pages="totalHostPages"
          @select="setHostPage"
        />
      </div>

      <div
        v-if="!metricsReady"
        class="p-3"
      >
        <LoadingSkeleton
          :lines="8"
          variant="table"
        />
      </div>

      <div
        v-if="!loading && hosts.length === 0"
        class="py-3"
      >
        <EmptyState
          title="Aucun hôte enregistré"
          subtitle="Ajoutez votre premier hôte pour commencer à surveiller votre infrastructure."
          cta-label="Ajouter un hôte"
          cta-to="/hosts/new"
        />
      </div>
    </div>

    <!-- ─── Versions Docker (collapsible) ───────────────────────────────────── -->
    <DashboardDockerVersions :versions="(versionComparisons as any)" />

    <!-- ─── Barre d'actions groupées ──────────────────────────────────────────── -->
    <BulkActionBar
      v-if="canRunApt"
      :count="selectedCount"
      @clear="clearSelection"
    >
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary"
        :disabled="aptLoading !== ''"
        @click="sendBulkApt('update')"
      >
        <span
          v-if="aptLoading === 'update'"
          class="spinner-border spinner-border-sm me-1"
          role="status"
        />
        apt update
      </button>
      <button
        type="button"
        :class="selectedCount > 5 ? 'btn btn-sm btn-danger' : 'btn btn-sm btn-primary'"
        :disabled="aptLoading !== ''"
        @click="sendBulkApt('upgrade')"
      >
        <span
          v-if="aptLoading === 'upgrade'"
          class="spinner-border spinner-border-sm me-1"
          role="status"
        />
        apt upgrade
        <span
          v-if="selectedCount > 5"
          class="badge bg-danger-lt text-danger ms-1"
        >DANGER</span>
      </button>
    </BulkActionBar>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import RelativeTime from '../components/RelativeTime.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import ProxmoxClusterCard from '../components/proxmox/ProxmoxClusterCard.vue'
import DashboardKPIs from '../components/dashboard/DashboardKPIs.vue'
import DashboardDockerVersions from '../components/dashboard/DashboardDockerVersions.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PaginationNav from '../components/PaginationNav.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import BulkActionBar from '../components/BulkActionBar.vue'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'
import { pluralize } from '../utils/formatters'
import { useDashboard, type DashboardProxmoxLinkRecord } from '../composables/useDashboard'

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend)
  return Line
})
const {
  hosts,
  versionComparisons,
  proxmoxSummary,
  hasProxmox,
  cveSummary,
  proxmoxNodes,
  proxmoxLinks,
  hostMetrics,
  aptPendingHosts,
  diskUsage,
  loading,
  searchQuery,
  statusFilter,
  sortKey,
  sortDir,
  selectedHostIds,
  aptLoading,
  summaryHours,
  summaryChartData,
  summaryLoading,
  chartSource,
  chartSources,
  selectedCount,
  canRunApt,
  metricsReady,
  wsStatus,
  wsError,
  retryCount,
  dataStaleAlert,
  reconnect,
  effectiveMetricsByHost,
  sortedHosts,
  summaryChartOptions,
  fetchSummary,
  changeSummaryRange,
  selectAllFiltered,
  clearSelection,
  sendBulkApt,
  formatUptime,
  cpuColor,
  memColor,
  diskColor,
} = useDashboard()

const proxmoxLinkByHostId = computed<Record<string, DashboardProxmoxLinkRecord>>(() => {
  const map: Record<string, DashboardProxmoxLinkRecord> = {}
  for (const link of proxmoxLinks.value || []) {
    if (!link?.host_id || !link?.guest_id) continue
    map[link.host_id] = link
  }
  return map
})

function proxmoxGuestPath(hostId: string): string {
  const link = proxmoxLinkByHostId.value[hostId]
  if (!link || !link.guest_id || link.status === 'ignored') return ''
  return `/proxmox/guests/${link.guest_id}`
}

const hostsPerPage = 15
const currentHostPage = ref(1)

const totalHostPages = computed(() => {
  if (!sortedHosts.value.length) return 1
  return Math.ceil(sortedHosts.value.length / hostsPerPage)
})

const paginatedHosts = computed(() => {
  const start = (currentHostPage.value - 1) * hostsPerPage
  return sortedHosts.value.slice(start, start + hostsPerPage)
})

function setHostPage(page: number): void {
  currentHostPage.value = page
}

function toggleSort(key: string): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }

  sortKey.value = key
  sortDir.value = 'asc'
}

watch([searchQuery, statusFilter, sortKey, sortDir], () => {
  currentHostPage.value = 1
})

watch(totalHostPages, (pages) => {
  if (currentHostPage.value > pages) {
    currentHostPage.value = pages
  }
})

// Lazy-mount the chart: defer loading chart.js + vue-chartjs until the chart
// card is actually scrolled into view. The card is above the fold for most
// users, so the observer fires almost immediately — but on smaller viewports
// or when the user scrolls past quickly, we skip the work entirely.
const chartContainerRef = ref<HTMLElement | null>(null)
const chartVisible = ref(false)
let chartObserver: IntersectionObserver | null = null

onMounted(() => {
  if (typeof IntersectionObserver === 'undefined' || !chartContainerRef.value) {
    chartVisible.value = true
    return
  }
  chartObserver = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        chartVisible.value = true
        chartObserver?.disconnect()
        chartObserver = null
        break
      }
    }
  }, { rootMargin: '200px' })
  chartObserver.observe(chartContainerRef.value)
})

onBeforeUnmount(() => {
  chartObserver?.disconnect()
  chartObserver = null
})
</script>

<style scoped>
.summary-source-switch {
  display: inline-flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.summary-chart-body {
  height: 14rem;
}

.host-selection-col {
  width: 1%;
}

.empty-state-icon {
  opacity: 0.35;
}

.last-activity-col {
  min-width: 13rem;
  white-space: nowrap;
}

.status-col {
  min-width: 110px;
}
</style>
