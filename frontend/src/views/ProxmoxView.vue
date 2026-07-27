<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Proxmox VE"
      :interval-sec="PROXMOX_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Proxmox VE</span>
      </div>
      <h2 class="page-title">
        Proxmox VE
      </h2>
      <div class="text-secondary">
        Supervision de l'infrastructure de virtualisation
      </div>
    </div>

    <!-- Summary cards -->
    <div class="row row-cards mb-4">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="d-flex align-items-center">
              <div class="subheader">
                Connexions
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
              Nœuds
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
              VMs / LXC
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
              Stockage utilisé
            </div>
            <div class="h1 mt-2 mb-0">
              {{ formatBytes(summary.storage_used) }}
            </div>
            <div class="text-muted small">
              sur {{ formatBytes(summary.storage_total) }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              CPU cluster (moy.)
            </div>
            <div
              class="h1 mt-2 mb-0"
              :class="cpuTextColor(clusterResources.avgCpu)"
            >
              {{ (clusterResources.avgCpu * 100).toFixed(1) }}%
            </div>
            <div class="text-muted small">
              sur {{ clusterResources.onlineCount }} nœud{{ clusterResources.onlineCount > 1 ? 's' : '' }} en ligne
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              RAM cluster
            </div>
            <div
              class="h1 mt-2 mb-0"
              :class="ramTextColor(clusterResources.memUsed, clusterResources.memTotal)"
            >
              {{ formatBytes(clusterResources.memUsed) }}
            </div>
            <div class="text-muted small">
              sur {{ formatBytes(clusterResources.memTotal) }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Cluster health signals (only shown when there are issues) -->
    <div
      v-if="hasHealthAlerts"
      class="row row-cards mb-4"
    >
      <div
        v-if="(summary.nodes_down ?? 0) > 0"
        class="col-6 col-lg-3"
      >
        <div class="card card-sm h-100 border-danger">
          <div class="card-body">
            <div class="subheader text-danger">
              Nœuds hors ligne
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
        <div class="card card-sm h-100 border-warning">
          <div class="card-body">
            <div class="subheader text-warning">
              Stockages &gt; 80 %
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
        <div class="card card-sm h-100 border-danger">
          <div class="card-body">
            <div class="subheader text-danger">
              Stockages inactifs
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
        <div class="card card-sm h-100 border-warning">
          <div class="card-body">
            <div class="subheader text-warning">
              Tâches échouées (24 h)
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
        <h3 class="card-title mb-0">
          Nœuds Proxmox
        </h3>
        <div class="d-flex gap-2 proxmox-toolbar-controls">
          <select
            v-model="filterConnection"
            class="form-select form-select-sm proxmox-filter-select"
            style="width:auto"
          >
            <option value="">
              Toutes les connexions
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
            title="Rafraîchir maintenant"
            @click="load"
          >
            <IconRefresh
              :size="2"
              class="icon icon-sm"
            />
          </button>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>
                <SortableHeader
                  label="Nœud"
                  :active="nodeSortKey === 'node_name'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('node_name')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Instance / Cluster"
                  :active="nodeSortKey === 'cluster_name'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('cluster_name')"
                />
              </th>
              <th>
                <SortableHeader
                  label="VMs"
                  :active="nodeSortKey === 'vm_count'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('vm_count')"
                />
              </th>
              <th>
                <SortableHeader
                  label="LXC"
                  :active="nodeSortKey === 'lxc_count'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('lxc_count')"
                />
              </th>
              <th>
                <SortableHeader
                  label="CPU"
                  :active="nodeSortKey === 'cpu_usage'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('cpu_usage')"
                />
              </th>
              <th>
                <SortableHeader
                  label="RAM"
                  :active="nodeSortKey === 'mem_used'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('mem_used')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Statut"
                  :active="nodeSortKey === 'status'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('status')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Dernier contact"
                  :active="nodeSortKey === 'last_seen_at'"
                  :direction="nodeSortDir"
                  @toggle="toggleNodeSort('last_seen_at')"
                />
              </th>
              <th />
            </tr>
          </thead>
          <tbody>
            <!-- Skeleton rows while loading -->
            <template v-if="loading">
              <tr
                v-for="i in 3"
                :key="`sk-${i}`"
              >
                <td><div class="skeleton-text w-75" /></td>
                <td><div class="skeleton-text w-50" /></td>
                <td><div class="skeleton-text w-25" /></td>
                <td><div class="skeleton-text w-25" /></td>
                <td><div class="skeleton-text" /></td>
                <td><div class="skeleton-text" /></td>
                <td><div class="skeleton-text w-50" /></td>
                <td><div class="skeleton-text w-75" /></td>
                <td />
              </tr>
            </template>
            <tr v-else-if="sortedNodes.length === 0">
              <td
                colspan="9"
                class="text-center text-muted py-4"
              >
                Aucun nœud Proxmox trouvé.
                <router-link
                  v-if="auth.isAdmin"
                  to="/settings"
                  class="ms-1"
                >
                  Configurer une connexion
                </router-link>
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
                  class="status status-lime"
                >
                  <span class="status-dot status-dot-animated" />
                  <span data-translation-id="online">En ligne</span>
                </span>
                <span
                  v-else
                  class="status status-red"
                >
                  <span class="status-dot status-dot-animated" />
                  <span data-translation-id="offline">Hors ligne</span>
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
                  Détail
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
import { IconRefresh } from '@tabler/icons-vue'
import { useAuthStore } from '../stores/auth'
import SortableHeader from '../components/common/SortableHeader.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import { useProxmox } from '../composables/useProxmox'
import type { ProxmoxNode } from '../types/proxmox'

const auth = useAuthStore()

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
  load,
} = useProxmox()

function memPct(node: ProxmoxNode): string | number {
  if (!node.mem_total) return 0
  return (((node.mem_used || 0) / node.mem_total) * 100).toFixed(1)
}

function cpuColor(usage: number): string {
  if (usage > 0.85) return 'bg-danger'
  if (usage > 0.6) return 'bg-warning'
  return 'bg-success'
}

function ramColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-success'
}

function cpuTextColor(usage: number): string {
  if (usage > 0.85) return 'text-danger'
  if (usage > 0.6) return 'text-warning'
  return 'text-success'
}

function ramTextColor(used: number, total: number): string {
  if (!total) return 'text-secondary'
  const pct = used / total
  if (pct > 0.85) return 'text-danger'
  if (pct > 0.6) return 'text-warning'
  return 'text-success'
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}
</script>

<style scoped>
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
