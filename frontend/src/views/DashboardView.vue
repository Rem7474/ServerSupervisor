<template>
  <div>
    <!-- ─── Header ─────────────────────────────────────────────────────────── -->
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
          {{ t('dashboard.subtitle') }}
        </div>
      </div>
      <router-link
        to="/hosts/new"
        class="btn btn-primary btn-sm"
      >
        <IconPlus
          :size="14"
          class="icon"
        />
        <span class="d-none d-sm-inline ms-1">{{ t('dashboard.addHost') }}</span>
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

    <!-- ─── Points d'attention (CVE + Proxmox + Attention, un seul bandeau) ─── -->
    <div
      v-if="!loading && bannerItems.length > 0"
      class="card mb-3"
    >
      <div class="card-header">
        <h3 class="card-title mb-0">
          <IconListCheck
            :size="24"
            class="icon me-1"
          />
          {{ t('dashboard.attentionPoints') }}
        </h3>
      </div>
      <div class="list-group list-group-flush">
        <router-link
          v-for="item in bannerItems"
          :key="item.key"
          :to="item.to"
          class="list-group-item list-group-item-action d-flex align-items-center gap-2"
        >
          <span
            v-if="item.count"
            class="badge"
            :class="bannerBadgeClass(item.severity)"
          >{{ item.count }}</span>
          <IconAlertTriangle
            v-else
            :size="16"
            class="icon flex-shrink-0"
            :class="bannerIconClass(item.severity)"
          />
          <span class="flex-grow-1">{{ item.label }}</span>
          <IconChevronRight
            :size="16"
            class="icon text-secondary"
          />
        </router-link>
      </div>
    </div>

    <!-- ─── Explicit state when nothing needs attention ──────────────── -->
    <div
      v-else-if="!loading && hosts.length > 0"
      class="alert alert-success d-flex align-items-center gap-2 mb-3"
    >
      <IconCircleCheck
        :size="16"
        class="icon flex-shrink-0"
      />
      <span>{{ t('dashboard.allOperational') }}</span>
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

    <!-- ─── Recherche / filtre ───────────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3 align-items-center">
          <div class="col-12 col-lg">
            <label
              class="form-label"
              for="dashboard-search"
            >{{ t('dashboard.searchHostLabel') }}</label>
            <input
              id="dashboard-search"
              v-model="searchQuery"
              type="text"
              class="form-control"
              :placeholder="t('dashboard.searchHostPlaceholder')"
            >
          </div>
          <div class="col-12 col-md-4 col-lg-2">
            <label
              class="form-label"
              for="dashboard-status-filter"
            >{{ t('dashboard.statusFilterLabel') }}</label>
            <select
              id="dashboard-status-filter"
              v-model="statusFilter"
              class="form-select"
            >
              <option value="all">
                {{ t('common.all') }}
              </option>
              <option value="online">
                {{ t('common.statusOnline') }}
              </option>
              <option value="offline">
                {{ t('common.statusOffline') }}
              </option>
              <option value="warning">
                Warning
              </option>
            </select>
          </div>
          <div
            v-if="allTags.length > 0"
            class="col-12 col-md-4 col-lg-2"
          >
            <label
              class="form-label"
              for="dashboard-tag-filter"
            >{{ t('dashboard.tagLabel') }}</label>
            <select
              id="dashboard-tag-filter"
              v-model="tagFilter"
              class="form-select"
            >
              <option value="all">
                {{ t('common.all') }}
              </option>
              <option
                v-for="tag in allTags"
                :key="tag"
                :value="tag"
              >
                {{ tag }}
              </option>
            </select>
          </div>
        </div>
      </div>
    </div>

    <!-- ─── Hosts table ──────────────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th class="host-selection-col">
                <label class="form-check mb-0">
                  <input
                    class="form-check-input"
                    type="checkbox"
                    :checked="allVisibleHostsSelected"
                    :aria-label="t('dashboard.selectAllVisible')"
                    @change="toggleSelectAllVisibleHosts(($event.target as HTMLInputElement).checked)"
                  >
                </label>
              </th>
              <th>
                <SortableHeader
                  :label="t('dashboard.nameColumn')"
                  :active="sortKey === 'name'"
                  :direction="sortDir"
                  @toggle="toggleSort('name')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('dashboard.stateColumn')"
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
              <th :title="t('dashboard.cpuTooltip')">
                <SortableHeader
                  label="CPU"
                  :active="sortKey === 'cpu'"
                  :direction="sortDir"
                  @toggle="toggleSort('cpu')"
                />
              </th>
              <th :title="t('dashboard.ramDiskTooltip')">
                <SortableHeader
                  label="RAM"
                  :active="sortKey === 'ram'"
                  :direction="sortDir"
                  @toggle="toggleSort('ram')"
                />
              </th>
              <th :title="t('dashboard.ramDiskTooltip')">
                <SortableHeader
                  :label="t('dashboard.diskColumn')"
                  :active="sortKey === 'disk'"
                  :direction="sortDir"
                  @toggle="toggleSort('disk')"
                />
              </th>
              <th :title="t('dashboard.aptPendingTooltip')">
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
                  :label="t('dashboard.lastActivityColumn')"
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
                    {{ host.name || host.hostname || t('dashboard.unnamed') }}
                  </router-link>
                  <div class="text-secondary small">
                    {{ host.hostname || t('dashboard.notConnected') }}
                  </div>
                  <div
                    v-if="proxmoxGuestPath(host.id) || (host.tags && host.tags.length > 0) || isNeverConnectedHost(host)"
                    class="d-flex flex-wrap gap-1 mt-1"
                  >
                    <router-link
                      v-if="isNeverConnectedHost(host)"
                      :to="`/hosts/${host.id}`"
                      class="badge bg-warning-lt text-warning text-decoration-none badge-link"
                      :title="t('dashboard.neverConnectedTooltip')"
                    >
                      {{ t('dashboard.installPending') }}
                    </router-link>
                    <router-link
                      v-if="proxmoxGuestPath(host.id)"
                      :to="proxmoxGuestPath(host.id)"
                      class="badge bg-orange-lt text-orange text-decoration-none badge-link"
                    >
                      {{ t('dashboard.proxmoxStats') }}
                    </router-link>
                    <span
                      v-for="tag in host.tags"
                      :key="tag"
                      class="badge bg-secondary-lt text-secondary"
                    >{{ tag }}</span>
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
                    class="badge bg-warning-lt text-warning"
                  >{{ aptPendingHosts[host.id] }}</span>
                  <span
                    v-else
                    class="text-secondary"
                  >—</span>
                </td>
                <td>{{ hostMetrics[host.id] ? formatUptime(hostMetrics[host.id]!.uptime) : '-' }}</td>
                <td class="last-activity-col">
                  <span
                    v-if="isNeverConnectedHost(host)"
                    class="text-secondary small"
                  >{{ t('dashboard.neverConnectedShort') }}</span>
                  <RelativeTime
                    v-else
                    :date="(host.last_seen as any) || ''"
                  />
                </td>
              </tr>
              <tr v-if="hosts.length > 0 && sortedHosts.length === 0">
                <td colspan="10">
                  <EmptyState :title="t('dashboard.noHostsMatchSearch')" />
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
          :title="t('dashboard.noHostsRegistered')"
          :subtitle="t('dashboard.noHostsSubtitle')"
          :cta-label="t('dashboard.addHost')"
          cta-to="/hosts/new"
        />
      </div>
    </div>

    <!-- ─── Trend chart (collapsible, collapsed by default) ──────────── -->
    <div class="card mb-4">
      <div
        class="card-header dashboard-chart-header clickable-row"
        role="button"
        tabindex="0"
        :aria-expanded="chartOpen"
        aria-controls="dashboard-trend-chart-panel"
        @click="chartOpen = !chartOpen"
        @keydown.enter.prevent="chartOpen = !chartOpen"
        @keydown.space.prevent="chartOpen = !chartOpen"
      >
        <h3 class="card-title mb-0">
          {{ t('dashboard.trendTitle') }}
          <IconChevronDown
            :size="16"
            class="ms-auto chart-chevron"
            :class="{ 'is-open': chartOpen }"
          />
        </h3>
      </div>
      <div
        v-show="chartOpen"
        id="dashboard-trend-chart-panel"
      >
        <div class="card-body d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 border-bottom">
          <div class="text-secondary small">
            <template v-if="hasProxmox">
              <div
                class="summary-source-switch"
                role="group"
                :aria-label="t('dashboard.chartSourceAriaLabel')"
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
              {{ t('dashboard.averageAllHosts') }}
            </template>
          </div>
          <div class="btn-group btn-group-sm">
            <button
              v-for="h in [1, 6, 24, 168, 720]"
              :key="h"
              type="button"
              :class="summaryHours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'"
              @click="changeSummaryRange(h)"
            >
              {{ h >= 24 ? t('dashboard.rangeDays', { n: h / 24 }) : t('dashboard.rangeHours', { n: h }) }}
            </button>
          </div>
        </div>
        <div
          ref="chartContainerRef"
          class="card-body summary-chart-body"
        >
          <LoadingSkeleton
            v-if="summaryLoading || !chartVisible"
            variant="chart"
          />
          <ApexChart
            v-else-if="summaryChartSeries"
            type="area"
            height="100%"
            :options="summaryChartOptions"
            :series="summaryChartSeries"
          />
          <div
            v-else
            class="h-100 d-flex align-items-center justify-content-center text-secondary"
          >
            {{ t('common.noData') }}
          </div>
        </div>
      </div>
    </div>

    <!-- ─── Versions Docker (collapsible) ───────────────────────────────────── -->
    <DashboardDockerVersions :versions="(versionComparisons as any)" />

    <!-- ─── Bulk action bar ──────────────────────────────────────────── -->
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
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RelativeTime from '../components/RelativeTime.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import ProxmoxClusterCard from '../components/proxmox/ProxmoxClusterCard.vue'
import DashboardKPIs from '../components/dashboard/DashboardKPIs.vue'
import DashboardDockerVersions from '../components/dashboard/DashboardDockerVersions.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PaginationNav from '../components/PaginationNav.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import { IconAlertTriangle, IconPlus, IconListCheck, IconChevronRight, IconChevronDown, IconCircleCheck } from '@tabler/icons-vue'
import BulkActionBar from '../components/BulkActionBar.vue'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'
import { isNeverConnectedHost } from '../utils/hosts'
import { useDashboard, type DashboardProxmoxLinkRecord } from '../composables/useDashboard'
import { useAttentionCenter } from '../composables/useAttentionCenter'
import { AsyncApexChart as ApexChart } from '../utils/apexChartTheme'

const { t } = useI18n()

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
  tagFilter,
  allTags,
  sortKey,
  sortDir,
  selectedHostIds,
  aptLoading,
  summaryHours,
  summaryChartSeries,
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
  clearSelection,
  sendBulkApt,
  formatUptime,
  cpuColor,
  memColor,
  diskColor,
} = useDashboard()

const { items: attentionItems } = useAttentionCenter()

interface BannerItem {
  key: string
  label: string
  to: string
  severity: 'danger' | 'warning' | 'info'
  count?: number
}

// Merges the CVE banner, the Proxmox health banner and the Attention items
// into one condensed list — three stacked blocks before this, now one card.
const bannerItems = computed<BannerItem[]>(() => {
  const list: BannerItem[] = []

  const critical = cveSummary.value?.critical_count || 0
  const hostsWithCritical = cveSummary.value?.hosts_with_critical || 0
  if (critical > 0 || hostsWithCritical > 0) {
    const cvePart = t('dashboard.cveCritical', critical)
    list.push({
      key: 'cve',
      label: hostsWithCritical > 0
        ? t('dashboard.cveCriticalAcrossHosts', { cve: cvePart, hosts: t('dashboard.hostCount', hostsWithCritical) })
        : cvePart,
      to: '/apt',
      severity: 'danger',
      count: critical || undefined,
    })
  }

  const nodesDown = proxmoxSummary.value?.nodes_down ?? 0
  const storageNearFull = proxmoxSummary.value?.storage_near_full ?? 0
  const storageOffline = proxmoxSummary.value?.storage_offline ?? 0
  const failedTasks = proxmoxSummary.value?.recent_failed_tasks ?? 0
  if (nodesDown > 0 || storageNearFull > 0 || storageOffline > 0 || failedTasks > 0) {
    const parts: string[] = []
    if (nodesDown > 0) parts.push(t('dashboard.nodesDownCount', nodesDown))
    if (storageNearFull > 0) parts.push(t('dashboard.storageNearFullCount', storageNearFull))
    if (storageOffline > 0) parts.push(t('dashboard.storageOfflineCount', storageOffline))
    if (failedTasks > 0) parts.push(t('dashboard.failedTasksCount', failedTasks))
    list.push({
      key: 'proxmox-health',
      label: parts.join(' · '),
      to: '/proxmox',
      severity: 'warning',
    })
  }

  for (const item of attentionItems.value) {
    list.push({ key: item.key, label: item.label, to: item.to, severity: item.severity as BannerItem['severity'], count: item.count })
  }

  // Defensive sort: today CVE is always pushed first (the only "danger"
  // source) and attentionItems is info/warning-only, so insertion order
  // happens to already be severity-ordered — but nothing enforces that
  // invariant here. Sorting explicitly means a future "danger"-severity
  // attention item still surfaces above a "warning" one instead of silently
  // landing wherever it was pushed.
  const severityRank: Record<BannerItem['severity'], number> = { danger: 0, warning: 1, info: 2 }
  return list.sort((a, b) => severityRank[a.severity] - severityRank[b.severity])
})

function bannerBadgeClass(severity: BannerItem['severity']): string {
  if (severity === 'danger') return 'bg-danger-lt text-danger'
  if (severity === 'warning') return 'bg-warning-lt text-warning'
  return 'bg-primary-lt text-primary'
}

function bannerIconClass(severity: BannerItem['severity']): string {
  if (severity === 'danger') return 'text-danger'
  if (severity === 'warning') return 'text-warning'
  return 'text-primary'
}

const chartOpen = ref(false)

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

// Scoped to the current page, like Docker's equivalent header checkbox —
// selecting "all" shouldn't silently span every page of the current filter.
const allVisibleHostsSelected = computed(() =>
  paginatedHosts.value.length > 0 && paginatedHosts.value.every((h) => selectedHostIds.value.includes(h.id))
)

function toggleSelectAllVisibleHosts(checked: boolean): void {
  const next = new Set(selectedHostIds.value)
  for (const h of paginatedHosts.value) {
    if (checked) next.add(h.id)
    else next.delete(h.id)
  }
  selectedHostIds.value = Array.from(next)
}

function toggleSort(key: string): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }

  sortKey.value = key
  sortDir.value = 'asc'
}

watch([searchQuery, statusFilter, tagFilter, sortKey, sortDir], () => {
  currentHostPage.value = 1
})

watch(totalHostPages, (pages) => {
  if (currentHostPage.value > pages) {
    currentHostPage.value = pages
  }
})

// Lazy-mount the chart: defer loading vue3-apexcharts until the chart
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
.chart-chevron {
  flex-shrink: 0;
  transition: transform 0.2s;
}

.chart-chevron.is-open {
  transform: rotate(180deg);
}

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

/* text-decoration-none on these badge-as-link removes the only native hover
   hint a link has; restore it on hover instead of leaving zero feedback. */
.badge-link:hover {
  text-decoration: underline;
}

.last-activity-col {
  min-width: 13rem;
  white-space: nowrap;
}

.status-col {
  min-width: 110px;
}
</style>
