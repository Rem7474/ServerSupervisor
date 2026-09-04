<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      :label="t('proxmox.breadcrumbTitle')"
      :interval-sec="PROXMOX_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          {{ t('nav.sections.control.items.dashboard') }}
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ t('proxmox.breadcrumbTitle') }}</span>
      </div>
      <h2 class="page-title">
        {{ t('proxmox.breadcrumbTitle') }}
      </h2>
      <div class="text-secondary">
        {{ t('proxmox.overviewPageDescription') }}
      </div>
    </div>

    <!-- Summary cards -->
    <div class="row row-cards mb-4">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="d-flex align-items-center">
              <div class="subheader">
                {{ t('proxmox.connectionsLabel') }}
              </div>
            </div>
            <div class="h1 mt-2 mb-0">
              {{ summary.connection_count ?? '—' }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('proxmox.nodesLabel') }}
            </div>
            <div class="h1 mt-2 mb-0">
              {{ summary.node_count ?? '—' }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('proxmox.vmsLxcLabel') }}
            </div>
            <div class="h1 mt-2 mb-0">
              <span class="text-primary">{{ summary.vm_count ?? '—' }}</span>
              <span class="text-muted fs-5 ms-1">/ {{ summary.lxc_count ?? '—' }}</span>
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('proxmox.storageUsedLabel') }}
            </div>
            <div class="h1 mt-2 mb-0">
              {{ formatBytes(summary.storage_used) }}
            </div>
            <div class="text-muted small">
              {{ t('proxmox.ofTotalLabel', { total: formatBytes(summary.storage_total) }) }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('proxmox.cpuClusterAvgLabel') }}
            </div>
            <div
              class="h1 mt-2 mb-0"
              :class="cpuTextColor(clusterResources.avgCpu)"
            >
              {{ (clusterResources.avgCpu * 100).toFixed(1) }}%
            </div>
            <div class="text-muted small">
              {{ t('proxmox.onlineNodesSummary', { n: clusterResources.onlineCount }, clusterResources.onlineCount) }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('proxmox.ramClusterTitle') }}
            </div>
            <div
              class="h1 mt-2 mb-0"
              :class="ramTextColor(clusterResources.memUsed, clusterResources.memTotal)"
            >
              {{ formatBytes(clusterResources.memUsed) }}
            </div>
            <div class="text-muted small">
              {{ t('proxmox.ofTotalLabel', { total: formatBytes(clusterResources.memTotal) }) }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Cluster health signals (only shown when there are issues) — each card
         filters the node table below to the nodes behind that signal. -->
    <div
      v-if="hasHealthAlerts"
      class="row row-cards mb-4"
    >
      <div
        v-if="(summary.nodes_down ?? 0) > 0"
        class="col-6 col-lg-3"
      >
        <div
          class="card card-sm h-100 border-danger cursor-pointer"
          role="button"
          tabindex="0"
          :class="{ 'health-card-active': healthFilterLabel === t('proxmox.nodesDownLabel') }"
          @click="toggleHealthFilter(summary.nodes_down_ids, t('proxmox.nodesDownLabel'))"
          @keydown.enter.prevent="toggleHealthFilter(summary.nodes_down_ids, t('proxmox.nodesDownLabel'))"
        >
          <div class="card-body">
            <div class="subheader text-danger">
              {{ t('proxmox.nodesDownLabel') }}
            </div>
            <div class="h1 mt-2 mb-0 text-danger">
              {{ summary.nodes_down }}
            </div>
          </div>
        </div>
      </div>
      <div
        v-if="(summary.storage_near_full ?? 0) > 0"
        class="col-6 col-lg-3"
      >
        <div
          class="card card-sm h-100 border-warning cursor-pointer"
          role="button"
          tabindex="0"
          :class="{ 'health-card-active': healthFilterLabel === t('proxmox.storageOver80Label') }"
          @click="toggleHealthFilter(summary.storage_near_full_node_ids, t('proxmox.storageOver80Label'))"
          @keydown.enter.prevent="toggleHealthFilter(summary.storage_near_full_node_ids, t('proxmox.storageOver80Label'))"
        >
          <div class="card-body">
            <div class="subheader text-warning">
              {{ t('proxmox.storageOver80Label') }}
            </div>
            <div class="h1 mt-2 mb-0 text-warning">
              {{ summary.storage_near_full }}
            </div>
          </div>
        </div>
      </div>
      <div
        v-if="(summary.storage_offline ?? 0) > 0"
        class="col-6 col-lg-3"
      >
        <div
          class="card card-sm h-100 border-danger cursor-pointer"
          role="button"
          tabindex="0"
          :class="{ 'health-card-active': healthFilterLabel === t('proxmox.storageOfflineLabel') }"
          @click="toggleHealthFilter(summary.storage_offline_node_ids, t('proxmox.storageOfflineLabel'))"
          @keydown.enter.prevent="toggleHealthFilter(summary.storage_offline_node_ids, t('proxmox.storageOfflineLabel'))"
        >
          <div class="card-body">
            <div class="subheader text-danger">
              {{ t('proxmox.storageOfflineLabel') }}
            </div>
            <div class="h1 mt-2 mb-0 text-danger">
              {{ summary.storage_offline }}
            </div>
          </div>
        </div>
      </div>
      <div
        v-if="(summary.recent_failed_tasks ?? 0) > 0"
        class="col-6 col-lg-3"
      >
        <div
          class="card card-sm h-100 border-warning cursor-pointer"
          role="button"
          tabindex="0"
          :class="{ 'health-card-active': healthFilterLabel === t('proxmox.failedTasks24hLabel') }"
          @click="toggleHealthFilter(summary.failed_task_node_ids, t('proxmox.failedTasks24hLabel'))"
          @keydown.enter.prevent="toggleHealthFilter(summary.failed_task_node_ids, t('proxmox.failedTasks24hLabel'))"
        >
          <div class="card-body">
            <div class="subheader text-warning">
              {{ t('proxmox.failedTasks24hLabel') }}
            </div>
            <div class="h1 mt-2 mb-0 text-warning">
              {{ summary.recent_failed_tasks }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <!-- Nodes table (shown during loading with skeleton rows, then real data) -->
    <div
      v-else
      class="card"
    >
      <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
        <div class="d-flex align-items-center gap-2 flex-wrap">
          <h3 class="card-title mb-0">
            {{ t('proxmox.proxmoxNodesTitle') }}
          </h3>
          <span
            v-if="healthFilterLabel"
            class="badge bg-azure-lt text-azure d-flex align-items-center gap-1"
          >
            {{ t('proxmox.filteredByLabel', { label: healthFilterLabel }) }}
            <button
              type="button"
              class="btn-close ms-1"
              :aria-label="t('proxmox.removeFilterAriaLabel')"
              @click="clearHealthFilter"
            />
          </span>
        </div>
        <div class="d-flex gap-2 proxmox-toolbar-controls">
          <select
            v-model="filterConnection"
            class="form-select form-select-sm proxmox-filter-select"
            style="width:auto"
          >
            <option value="">
              {{ t('proxmox.allConnectionsOption') }}
            </option>
            <option
              v-for="inst in instances"
              :key="inst.id"
              :value="inst.id"
            >
              {{ inst.name }}
            </option>
          </select>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="loading"
            :title="t('proxmox.refreshNowTooltip')"
            @click="load"
          >
            <IconRefresh
              :size="16"
              class="icon icon-sm"
            />
          </button>
        </div>
      </div>
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>
                <SortableHeader
                  :label="t('proxmox.nodeColumn')"
                  :active="nodeSortKey === 'node_name'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('node_name')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('proxmox.instanceClusterColumn')"
                  :active="nodeSortKey === 'cluster_name'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('cluster_name')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('proxmox.vmsColumn')"
                  :active="nodeSortKey === 'vm_count'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('vm_count')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('proxmox.lxcColumn')"
                  :active="nodeSortKey === 'lxc_count'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('lxc_count')"
                />
              </th>
              <th style="width: 200px;">
                <SortableHeader
                  :label="t('proxmox.cpuColumn')"
                  :active="nodeSortKey === 'cpu_usage'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('cpu_usage')"
                />
              </th>
              <th style="width: 200px;">
                <SortableHeader
                  :label="t('proxmox.ramColumn')"
                  :active="nodeSortKey === 'mem_used'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('mem_used')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('proxmox.statusColumn')"
                  :active="nodeSortKey === 'status'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('status')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('proxmox.lastContactColumn')"
                  :active="nodeSortKey === 'last_seen_at'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('last_seen_at')"
                />
              </th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td
                colspan="9"
                class="py-2"
              >
                <LoadingSkeleton
                  variant="table"
                  :lines="3"
                />
              </td>
            </tr>
            <tr v-else-if="sortedNodes.length === 0 && healthFilterLabel">
              <td colspan="9">
                <EmptyState
                  :title="t('proxmox.noNodesMatchFilterTitle')"
                  :subtitle="t('proxmox.noNodesMatchFilterSubtitle', { label: healthFilterLabel })"
                  :cta-label="t('proxmox.removeFilterButton')"
                  @cta="clearHealthFilter"
                />
              </td>
            </tr>
            <tr v-else-if="sortedNodes.length === 0">
              <td colspan="9">
                <EmptyState
                  :title="t('proxmox.noNodesFoundTitle')"
                  :cta-label="auth.isAdmin ? t('proxmox.configureConnectionCta') : ''"
                  cta-to="/settings"
                />
              </td>
            </tr>
            <tr
              v-for="node in sortedNodes"
              :key="node.id"
            >
              <td>
                <div class="fw-medium">
                  {{ node.node_name }}
                </div>
                <div class="text-muted small text-break">
                  {{ node.ip_address }}
                </div>
              </td>
              <td>
                <span
                  v-if="node.cluster_name"
                  class="text-secondary"
                >{{ node.cluster_name }}</span>
                <span
                  v-else
                  class="text-muted"
                >—</span>
              </td>
              <td>
                <router-link
                  :to="`/proxmox/nodes/${node.id}?tab=vms`"
                  class="badge bg-azure-lt text-azure text-decoration-none"
                >
                  {{ node.vm_count }}
                </router-link>
              </td>
              <td>
                <router-link
                  :to="`/proxmox/nodes/${node.id}?tab=lxc`"
                  class="badge bg-azure-lt text-azure text-decoration-none"
                >
                  {{ node.lxc_count }}
                </router-link>
              </td>
              <td>
                <div class="d-flex align-items-center gap-2">
                  <div
                    class="progress progress-xs flex-grow-1"
                    style="min-width:60px"
                  >
                    <div
                      class="progress-bar"
                      :class="cpuColor(node.cpu_usage ?? 0)"
                      :style="`width:${((node.cpu_usage ?? 0) * 100).toFixed(1)}%`"
                    />
                  </div>
                  <span class="text-muted small">{{ ((node.cpu_usage ?? 0) * 100).toFixed(1) }}%</span>
                </div>
              </td>
              <td>
                <div class="d-flex align-items-center gap-2">
                  <div
                    class="progress progress-xs flex-grow-1"
                    style="min-width:60px"
                  >
                    <div
                      class="progress-bar"
                      :class="ramColor(node.mem_used ?? 0, node.mem_total ?? 0)"
                      :style="`width:${memPct(node)}%`"
                    />
                  </div>
                  <span class="text-muted small text-nowrap">{{ formatBytes(node.mem_used) }} / {{ formatBytes(node.mem_total) }}</span>
                </div>
              </td>
              <td>
                <span
                  v-if="node.status === 'online'"
                  class="status status-success"
                >
                  <span class="status-dot status-dot-animated" />
                  <span data-translation-id="online">{{ t('common.statusOnline') }}</span>
                </span>
                <span
                  v-else
                  class="status status-danger"
                >
                  <span class="status-dot status-dot-animated" />
                  <span data-translation-id="offline">{{ t('common.statusOffline') }}</span>
                </span>
              </td>
              <td class="text-muted small">
                {{ formatDate(node.last_seen_at) }}
              </td>
              <td>
                <router-link
                  :to="`/proxmox/nodes/${node.id}`"
                  class="btn btn-sm btn-outline-primary"
                >
                  {{ t('proxmox.detailButton') }}
                </router-link>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconRefresh } from '@tabler/icons-vue'
import { useAuthStore } from '../stores/auth'
import SortableHeader from '../components/common/SortableHeader.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import { useProxmox } from '../composables/useProxmox'
import { getMetricColorClass } from '../utils/metricColor'
import type { ProxmoxNode } from '../types/proxmox'

const auth = useAuthStore()
const { t, locale } = useI18n()

const {
  summary,
  instances,
  filterConnection,
  loading,
  error,
  autoRefresh,
  lastUpdatedAt,
  PROXMOX_REFRESH_SEC,
  nodeSortKey,
  nodeSortDir,
  sortedNodes,
  toggleNodeSort,
  hasHealthAlerts,
  clusterResources,
  healthFilterLabel,
  filterByHealthIds,
  clearHealthFilter,
  load,
} = useProxmox()

function toggleHealthFilter(ids: string[] | undefined, label: string): void {
  if (healthFilterLabel.value === label) {
    clearHealthFilter()
    return
  }
  filterByHealthIds(ids, label)
}

function memPct(node: ProxmoxNode): string | number {
  if (!node.mem_total) return 0
  return (((node.mem_used || 0) / node.mem_total) * 100).toFixed(1)
}

function cpuColor(usage: number): string {
  return getMetricColorClass(usage * 100, 'bg')
}

function ramColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  return getMetricColorClass((used / total) * 100, 'bg')
}

function cpuTextColor(usage: number): string {
  return getMetricColorClass(usage * 100)
}

function ramTextColor(used: number, total: number): string {
  if (!total) return 'text-secondary'
  return getMetricColorClass((used / total) * 100)
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const unitKeys = ['proxmox.byteUnitKilo', 'proxmox.byteUnitMega', 'proxmox.byteUnitGiga', 'proxmox.byteUnitTera']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < unitKeys.length) { v /= 1024; i++ }
  const unit = i === 0 ? 'B' : t(unitKeys[i - 1])
  return `${v.toFixed(i === 0 ? 0 : 1)} ${unit}`
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
}
</script>

<style scoped>
.cursor-pointer {
  cursor: pointer;
}

/* These KPI cards are real click targets (role="button" + @click + keydown)
   but had zero hover feedback beyond the cursor — same treatment as the
   shared .clickable-row convention (style.css) so hovering any of the 3
   inactive cards reads as interactive, not just the active one. */
.cursor-pointer:hover {
  background-color: var(--tblr-bg-surface-secondary);
}

.health-card-active {
  box-shadow: 0 0 0 2px var(--tblr-primary);
}

@media (max-width: 768px) {
  .proxmox-toolbar-controls {
    width: 100%;
  }

  .proxmox-filter-select {
    flex: 1 1 auto;
    min-width: 0;
    width: auto !important;
  }

  .proxmox-refresh-btn {
    flex: 0 0 auto;
  }
}
</style>
