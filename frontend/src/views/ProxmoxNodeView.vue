<template>
  <div>
    <div v-if="loading">
      <LoadingSkeleton
        variant="kpi"
        :lines="4"
      />
      <LoadingSkeleton
        variant="card"
        :lines="4"
        class="mt-4"
      />
    </div>
    <div
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>
    <div v-else-if="node">
      <PageRefreshBar
        v-model="autoRefresh"
        :label="t('proxmox.nodeRefreshLabel')"
        :interval-sec="LIVE_STATUS_REFRESH_SEC"
        :last-updated-at="lastUpdatedAt"
      />

      <!-- Header -->
      <div class="page-header mb-4">
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            {{ t('nav.sections.control.items.dashboard') }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <router-link
            to="/proxmox"
            class="text-decoration-none"
          >
            {{ t('proxmox.breadcrumbTitle') }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ node.node_name }}</span>
        </div>
        <div class="d-flex align-items-center gap-3 flex-wrap">
          <h2 class="page-title mb-0">
            {{ node.node_name }}
          </h2>
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
        </div>
        <div class="text-secondary">
          {{ node.cluster_name || t('proxmox.standaloneNodeLabel') }} · PVE {{ node.pve_version || 'N/A' }} · {{ node.ip_address }}
        </div>
      </div>

      <!-- Shared sensor source mapping (CPU temp + fan RPM) -->
      <div class="card mb-3">
        <div class="card-body d-flex flex-wrap align-items-center gap-2">
          <div class="subheader mb-0 me-2">
            {{ t('proxmox.sensorSourceTitle') }}
          </div>
          <select
            v-model="sensorSourceHostId"
            class="form-select form-select-sm proxmox-source-select"
          >
            <option value="">
              {{ t('proxmox.noSensorSourceOption') }}
            </option>
            <option
              v-for="candidate in sensorSourceCandidates"
              :key="candidate.id"
              :value="candidate.id"
            >
              {{ candidate.hostname || candidate.name }} ({{ candidate.ip_address }})
            </option>
          </select>
          <button
            type="button"
            class="btn btn-sm btn-primary"
            :disabled="sensorSourceSaving || sensorSourceLoading"
            @click="saveSensorSource"
          >
            <span
              v-if="sensorSourceSaving"
              class="spinner-border spinner-border-sm me-1"
            />
            {{ t('common.save') }}
          </button>
          <span
            v-if="sensorSourceMsg"
            :class="['small', sensorSourceOk ? 'text-success' : 'text-danger']"
          >{{ sensorSourceMsg }}</span>
          <span
            v-else-if="sensorSourceHostName"
            class="small text-muted"
          >{{ t('proxmox.currentSensorSourceLabel', { name: sensorSourceHostName }) }}</span>
        </div>
      </div>

      <!-- Compact node stats (static + live in one card) -->
      <div class="card mb-4">
        <div class="card-body position-relative">
          <div class="row g-4 align-items-start">
            <!-- CPU -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                {{ t('proxmox.cpuColumn') }}
              </div>
              <div class="h3 mb-1">
                {{ (node.cpu_usage * 100).toFixed(1) }}%
              </div>
              <div class="progress progress-xs mb-1">
                <div
                  class="progress-bar"
                  :class="cpuColor(node.cpu_usage)"
                  :style="`width:${(node.cpu_usage*100).toFixed(1)}%`"
                />
              </div>
              <div class="text-muted small">
                {{ t('proxmox.coresLabel', { n: node.cpu_count }) }}
              </div>
            </div>

            <!-- CPU Temp (from mapped source host) -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                {{ t('proxmox.cpuTempLabel') }}
              </div>
              <div
                class="h3 mb-1"
                :class="tempColor(nodeCpuTempCurrent)"
              >
                {{ nodeCpuTempCurrent > 0 ? `${nodeCpuTempCurrent.toFixed(1)}°C` : '—' }}
              </div>
              <div class="text-muted small">
                <span v-if="!sensorSourceHostName">{{ t('proxmox.sensorSourceNotConfigured') }}</span>
              </div>
            </div>

            <!-- Fan RPM (from mapped source host) -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                {{ t('proxmox.fanRpmLabel') }}
              </div>
              <div class="h3 mb-1 text-blue">
                {{ nodeFanRPMCurrent > 0 ? `${nodeFanRPMCurrent.toFixed(0)} RPM` : '—' }}
              </div>
              <div class="text-muted small">
                <span v-if="!sensorSourceHostName">{{ t('proxmox.sensorSourceNotConfigured') }}</span>
              </div>
            </div>

            <!-- RAM -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                {{ t('proxmox.ramColumn') }}
              </div>
              <div class="h3 mb-1">
                {{ formatBytes(node.mem_used) }}
              </div>
              <div class="progress progress-xs mb-1">
                <div
                  class="progress-bar"
                  :class="ramColor(node.mem_used, node.mem_total)"
                  :style="`width:${memPct(node)}%`"
                />
              </div>
              <div class="text-muted small">
                / {{ formatBytes(node.mem_total) }}
              </div>
            </div>

            <!-- Uptime -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                {{ t('proxmox.uptimeLabel') }}
              </div>
              <div class="h3 mb-0">
                {{ formatUptime(node.uptime) }}
              </div>
            </div>

            <!-- Guests -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                {{ t('proxmox.guestsLabel') }}
              </div>
              <div class="h3 mb-0">
                <span class="text-primary">{{ node.vm_count }}</span><span class="text-muted fs-5 ms-1">VM</span>
                <span class="ms-2 text-info">{{ node.lxc_count }}</span><span class="text-muted fs-5 ms-1">LXC</span>
              </div>
            </div>

            <!-- Live data separator -->
            <template v-if="liveStatus">
              <div class="col-auto d-none d-lg-flex align-items-stretch py-1">
                <div class="vr" />
              </div>

              <!-- IO Wait -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  {{ t('proxmox.iowaitChartTitle') }}
                </div>
                <div
                  class="h3 mb-0"
                  :class="liveStatus.wait > 0.2 ? 'text-danger' : liveStatus.wait > 0.05 ? 'text-warning' : 'text-success'"
                >
                  {{ (liveStatus.wait * 100).toFixed(2) }}%
                </div>
                <div class="text-muted small">
                  {{ t('proxmox.diskSubLabel') }}
                </div>
              </div>

              <!-- Swap -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  {{ t('proxmox.swapLabel') }}
                </div>
                <div class="h3 mb-1">
                  {{ formatBytes(liveStatus.swap.used) }}
                </div>
                <div
                  v-if="liveStatus.swap.total"
                  class="progress progress-xs mb-1"
                >
                  <div
                    class="progress-bar"
                    :class="ramColor(liveStatus.swap.used, liveStatus.swap.total)"
                    :style="`width:${(liveStatus.swap.used/liveStatus.swap.total*100).toFixed(1)}%`"
                  />
                </div>
                <div class="text-muted small">
                  / {{ formatBytes(liveStatus.swap.total) }}
                </div>
              </div>

              <!-- Rootfs -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  {{ t('proxmox.rootfsLabel') }}
                </div>
                <div class="h3 mb-1">
                  {{ formatBytes(liveStatus.rootfs.used) }}
                </div>
                <div class="progress progress-xs mb-1">
                  <div
                    class="progress-bar"
                    :class="storageColor(liveStatus.rootfs.used, liveStatus.rootfs.total)"
                    :style="`width:${(liveStatus.rootfs.used/liveStatus.rootfs.total*100).toFixed(1)}%`"
                  />
                </div>
                <div class="text-muted small">
                  / {{ formatBytes(liveStatus.rootfs.total) }}
                </div>
              </div>
            </template>

            <!-- Live loading placeholder -->
            <div
              v-else-if="liveStatusLoading"
              class="col align-self-center text-muted small"
            >
              <span class="spinner-border spinner-border-sm me-1" />{{ t('proxmox.loadingLabel') }}
            </div>
          </div>

          <!-- Live refresh error + in-flight indicator (absolute, no added height) -->
          <!-- last-updated timestamp itself now lives in the page-level PageRefreshBar above -->
          <div class="position-absolute bottom-0 end-0 pb-2 pe-3 d-flex align-items-center gap-2 node-live-meta">
            <span
              v-if="liveStatusError"
              class="text-danger node-live-meta-text"
            >{{ liveStatusError }}</span>
            <span
              v-if="liveStatus && liveStatusLoading"
              class="text-muted node-live-meta-text"
            >
              <span class="spinner-border me-1 node-live-meta-spinner" />
              {{ t('proxmox.refreshingLabel') }}
            </span>
          </div>
        </div>
      </div>

      <!-- RRD Charts -->
      <ProxmoxNodeChartsPanel
        :cpu-chart="rrdCpuChart"
        :ram-chart="rrdRamChart"
        :iowait-chart="rrdIowaitChart"
        :net-chart="rrdNetChart"
        :temp-chart="nodeTempChart"
        :fan-chart="nodeFanChart"
        :timeframe="rrdTimeframe"
        :loading="rrdLoading"
        :error="rrdError"
        :temp-empty-text="nodeTempLoading ? t('proxmox.loadingLabel') : (nodeTempError || (sensorSourceHostId ? t('proxmox.noTempDataText') : t('proxmox.configureSensorSourceHint')))"
        :fan-empty-text="nodeFanLoading ? t('proxmox.loadingLabel') : (nodeFanError || (sensorSourceHostId ? t('proxmox.noFanDataText') : t('proxmox.configureSensorSourceHint')))"
        @timeframe-changed="loadRRD"
      />

      <!-- Updates banner (only shown when pending updates exist) -->
      <div
        v-if="node.pending_updates > 0"
        class="alert alert-warning mb-4"
      >
        <div class="d-flex align-items-center gap-3">
          <div>
            <strong>{{ t('proxmox.updatesAvailableTitle') }}</strong>
            {{ node.pending_updates }} {{ t('proxmox.pendingPackagesShortLabel') }}
          </div>
          <div
            v-if="node.last_update_check_at"
            class="ms-auto text-muted small"
          >
            {{ t('proxmox.lastCheckSubtitle', { date: formatDate(node.last_update_check_at) }) }}
          </div>
        </div>
      </div>

      <!-- Tabs + side console -->
      <div class="side-layout">
        <div class="side-main">
          <div class="card">
            <EntityTabShell
              :model-value="tab"
              :tabs="proxmoxTabs"
              nav-class="proxmox-node-tabs"
              card-header
              @update:model-value="onTabClick"
            >
              <template #vms>
                <ProxmoxNodeGuestsTab
                  kind="vm"
                  :guests="vms"
                  :guest-networks="guestNetworks"
                  :guest-networks-loading="guestNetworksLoading"
                  :guest-exposure="guestExposure"
                  :guest-exposure-loading="guestExposureLoading"
                  :peer-nodes="peerNodes"
                  :node-id="String(route.params.id)"
                  :action-loading="guestActionLoading"
                  @migrate="openMigrateModal($event, 'vm')"
                  @guest-action="handleGuestAction"
                />
              </template>

              <template #lxc>
                <ProxmoxNodeGuestsTab
                  kind="lxc"
                  :guests="lxcs"
                  :guest-networks="guestNetworks"
                  :guest-networks-loading="guestNetworksLoading"
                  :guest-exposure="guestExposure"
                  :guest-exposure-loading="guestExposureLoading"
                  :peer-nodes="peerNodes"
                  :node-id="String(route.params.id)"
                  :action-loading="guestActionLoading"
                  @migrate="openMigrateModal($event, 'lxc')"
                  @guest-action="handleGuestAction"
                />
              </template>

              <template #disks>
                <ProxmoxNodeDisksTab :disks="node.disks || []" />
              </template>

              <template #tasks>
                <ProxmoxNodeTasksTab
                  :tasks="node.tasks || []"
                  :active-upid="activeUpid"
                  @view-logs="startPollingTask($event.upid, { action: $event.action, label: $event.label })"
                />
              </template>

              <template #backups>
                <ProxmoxNodeBackupsTab
                  :jobs="backupJobs"
                  :runs="nodeBackupRuns"
                  :loading="backupsLoading"
                  :error="backupsError"
                  @view-logs="startPollingTask($event.upid, { action: $event.action, label: $event.label })"
                />
              </template>

              <template #updates>
                <ProxmoxNodeUpdatesTab
                  :pending-updates="node.pending_updates"
                  :last-update-check-at="node.last_update_check_at"
                  :apt-refreshing="aptRefreshing"
                  :apt-refresh-msg="aptRefreshMsg"
                  :apt-refresh-ok="aptRefreshOk"
                  @refresh-apt="triggerAptRefresh"
                />
              </template>

              <template #services>
                <ProxmoxNodeServicesTab
                  :services="services"
                  :loading="servicesLoading"
                  :error="servicesError"
                  :action-msg="svcActionMsg"
                  :action-ok="svcActionOk"
                  :action-loading="svcActionLoading"
                  @refresh="loadServices"
                  @action="svcAction($event.name, $event.action)"
                />
              </template>

              <template #security>
                <ProxmoxNodeSecurityTab
                  :node-id="String(route.params.id)"
                  :active="tab === 'security'"
                  @count="securityEventsCount = $event"
                />
              </template>

              <template #storage>
                <ProxmoxNodeStorageTab :storages="node.storages || []" />
              </template>
            </EntityTabShell>
          </div>
        </div> <!-- /side-main -->
        <CommandLogPanel
          :command="liveTask"
          :show="showConsole"
          :title="t('proxmox.taskLogsTitle')"
          :empty-text="t('proxmox.taskLogsEmptyText')"
          wrapper-class="side-panel"
          @open="showConsole = true"
          @close="closeConsole"
        />
      </div> <!-- /side-layout -->
    </div> <!-- /v-else-if node -->

    <!-- Migration modal -->
    <template v-if="migrateModal.open">
      <div
        ref="migrateModalRef"
        class="modal modal-blur fade show d-block"
        tabindex="-1"
        @click.self="migrateModal.open = false"
      >
        <div class="modal-dialog modal-sm modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                {{ t('proxmox.migrateModalTitle', { name: migrateModal.guest?.name || `VMID ${migrateModal.guest?.vmid}` }) }}
              </h5>
              <button
                type="button"
                class="btn-close"
                @click="migrateModal.open = false"
              />
            </div>
            <div class="modal-body">
              <div class="mb-3">
                <label class="form-label">{{ t('proxmox.targetNodeLabel') }}</label>
                <select
                  v-model="migrateModal.target"
                  class="form-select"
                >
                  <option
                    v-for="n in peerNodes"
                    :key="n.node_name"
                    :value="n.node_name"
                  >
                    {{ n.node_name }}
                  </option>
                </select>
              </div>
              <div class="mb-2">
                <label class="form-check">
                  <input
                    v-model="migrateModal.online"
                    type="checkbox"
                    class="form-check-input"
                  >
                  <span class="form-check-label">{{ t('proxmox.liveMigrationLabel') }}</span>
                </label>
              </div>
              <div
                v-if="migrateModal.error"
                class="alert alert-danger mb-0 mt-2 py-2 small"
              >
                {{ migrateModal.error }}
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn btn-secondary"
                @click="migrateModal.open = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="migrateModal.loading || !migrateModal.target"
                @click="submitMigration"
              >
                <span
                  v-if="migrateModal.loading"
                  class="spinner-border spinner-border-sm me-1"
                />
                {{ t('proxmox.migrateButton') }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
const CommandLogPanel = defineAsyncComponent(() => import('../components/host/CommandLogPanel.vue'))
const ProxmoxNodeChartsPanel = defineAsyncComponent(() => import('../components/proxmox/ProxmoxNodeChartsPanel.vue'))
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import EntityTabShell from '../components/EntityTabShell.vue'
import type { EntityTab } from '../components/EntityTabShell.vue'
import ProxmoxNodeDisksTab from '../components/proxmox/ProxmoxNodeDisksTab.vue'
import ProxmoxNodeStorageTab from '../components/proxmox/ProxmoxNodeStorageTab.vue'
import ProxmoxNodeTasksTab from '../components/proxmox/ProxmoxNodeTasksTab.vue'
import ProxmoxNodeUpdatesTab from '../components/proxmox/ProxmoxNodeUpdatesTab.vue'
import ProxmoxNodeServicesTab from '../components/proxmox/ProxmoxNodeServicesTab.vue'
import ProxmoxNodeBackupsTab from '../components/proxmox/ProxmoxNodeBackupsTab.vue'
import ProxmoxNodeSecurityTab from '../components/proxmox/ProxmoxNodeSecurityTab.vue'
import ProxmoxNodeGuestsTab from '../components/proxmox/ProxmoxNodeGuestsTab.vue'
import { useProxmoxNode } from '../composables/useProxmoxNode'
import { useModalChrome } from '../composables/useModalChrome'
import { getMetricColorClass } from '../utils/metricColor'

const route = useRoute()
const { t, locale } = useI18n()

const {
  node,
  loading,
  error,
  tab,
  sensorSourceCandidates,
  sensorSourceHostId,
  sensorSourceLoading,
  sensorSourceSaving,
  sensorSourceMsg,
  sensorSourceOk,
  sensorSourceHostName,
  saveSensorSource,
  nodeTempLoading,
  nodeTempError,
  nodeTempChart,
  nodeCpuTempCurrent,
  nodeFanLoading,
  nodeFanError,
  nodeFanChart,
  nodeFanRPMCurrent,
  aptRefreshing,
  aptRefreshMsg,
  aptRefreshOk,
  triggerAptRefresh,
  peerNodes,
  migrateModal,
  openMigrateModal,
  submitMigration,
  liveStatus,
  liveStatusLoading,
  liveStatusError,
  lastUpdatedAt,
  autoRefresh,
  LIVE_STATUS_REFRESH_SEC,
  rrdTimeframe,
  rrdCpuChart,
  rrdRamChart,
  rrdIowaitChart,
  rrdNetChart,
  rrdLoading,
  rrdError,
  loadRRD,
  showConsole,
  liveTask,
  activeUpid,
  closeConsole,
  startPollingTask,
  guestNetworks,
  guestNetworksLoading,
  loadGuestNetworks,
  guestExposure,
  guestExposureLoading,
  loadGuestExposure,
  services,
  servicesLoading,
  servicesError,
  svcActionMsg,
  svcActionOk,
  svcActionLoading,
  loadServices,
  svcAction,
  backupJobs,
  nodeBackupRuns,
  backupsLoading,
  backupsError,
  loadBackups,
  vms,
  lxcs,
  failedTaskCount,
  guestActionLoading,
  handleGuestAction,
} = useProxmoxNode()

const migrateModalRef = ref<HTMLElement | null>(null)
useModalChrome(migrateModalRef, () => migrateModal.value.open, { onClose: () => { migrateModal.value.open = false } })

// Trivial UI-only state fed by ProxmoxNodeSecurityTab's @count emit — no API/WS
// logic attached, so it stays here rather than in the composable.
const securityEventsCount = ref(0)

const azureBadge = 'badge bg-azure-lt text-azure ms-1'

const failedBackupCount = computed(() =>
  nodeBackupRuns.value.filter((r: { status?: string }) => r.status === 'error').length
)

const proxmoxTabs = computed<EntityTab[]>(() => [
  { key: 'vms', label: t('proxmox.vmsTabLabel'), badges: [{ value: vms.value.length, badgeClass: azureBadge }], lazy: true },
  { key: 'lxc', label: t('proxmox.lxcTabLabel'), badges: [{ value: lxcs.value.length, badgeClass: azureBadge }], lazy: true },
  { key: 'storage', label: t('proxmox.storageTabLabel'), badges: [{ value: node.value?.storages?.length ?? 0, badgeClass: azureBadge }], lazy: true },
  { key: 'disks', label: t('proxmox.disksTabLabel'), badges: [{ value: node.value?.disks?.length ?? 0, badgeClass: azureBadge }], lazy: true },
  {
    key: 'tasks',
    label: t('proxmox.tasksTabLabel'),
    badges: [
      { value: node.value?.tasks?.length ?? 0, badgeClass: azureBadge },
      ...(failedTaskCount.value > 0 ? [{ value: failedTaskCount.value, badgeClass: 'badge bg-danger text-white ms-1' }] : []),
    ],
    lazy: true,
  },
  {
    key: 'backups',
    label: t('proxmox.backupsTabLabel'),
    badges: failedBackupCount.value > 0 ? [{ value: failedBackupCount.value, badgeClass: 'badge bg-danger text-white ms-1' }] : [],
    lazy: true,
  },
  {
    key: 'updates',
    label: t('proxmox.updatesTabLabel'),
    badges: node.value?.pending_updates > 0 ? [{ value: node.value.pending_updates, badgeClass: 'badge ms-1 bg-warning-lt text-warning' }] : [],
    lazy: true,
  },
  { key: 'services', label: t('proxmox.servicesTabLabel'), lazy: true },
  // Labeled "Security logs" (not "Security") to avoid colliding with
  // HostDetailView's "Permissions" tab — same word previously used on both,
  // unrelated content (PVE syslog auth-failure search here vs. per-host RBAC there).
  { key: 'security', label: t('proxmox.securityLogsTabLabel'), badges: [{ value: securityEventsCount.value, badgeClass: azureBadge }], lazy: true },
])

// VMs/LXC and Services fetch their own supporting data lazily — on a real
// user click here, or (mirrored) once at mount in useProxmoxNode's load()
// for a tab restored from a ?tab= deep link. Each loader is itself
// load-once-per-mount guarded, so triggering it from both places is safe.
// Listening to the shell's click emit rather than watching `tab` itself
// avoids re-triggering on every tab.value change load() itself makes.
function onTabClick(key: string): void {
  tab.value = key
  if (key === 'vms' || key === 'lxc') { loadGuestNetworks(); loadGuestExposure(); loadBackups() }
  if (key === 'services') loadServices()
  if (key === 'backups') loadBackups()
}

function memPct(n: { mem_used?: number; mem_total?: number }): string | number {
  if (!n.mem_total) return 0
  return (((n.mem_used ?? 0) / n.mem_total) * 100).toFixed(1)
}

function cpuColor(usage: number): string {
  return getMetricColorClass(usage * 100, 'bg')
}

function tempColor(temp: number | undefined): string {
  if (!temp) return 'text-secondary'
  if (temp >= 85) return 'text-danger'
  if (temp >= 70) return 'text-warning'
  return 'text-success'
}

function ramColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  return getMetricColorClass((used / total) * 100, 'bg')
}

function storageColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  return getMetricColorClass((used / total) * 100, 'bg')
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const unitKeys = ['proxmox.byteUnitKilo', 'proxmox.byteUnitMega', 'proxmox.byteUnitGiga', 'proxmox.byteUnitTera']
  let i = 0, v = bytes
  while (v >= 1024 && i < unitKeys.length) { v /= 1024; i++ }
  const unit = i === 0 ? 'B' : t(unitKeys[i - 1])
  return `${v.toFixed(i === 0 ? 0 : 1)} ${unit}`
}

function formatUptime(seconds: number | undefined): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}${t('proxmox.daySuffix')} ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
}
</script>

<style scoped>
/* .proxmox-node-tabs is applied via a prop to EntityTabShell's own <ul>, so
   these selectors target markup owned by a child component's template —
   :deep() is required for scoped CSS to reach it. */
:deep(.proxmox-node-tabs) {
  flex-wrap: nowrap;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
}

:deep(.proxmox-node-tabs .nav-item) {
  flex: 0 0 auto;
}

.proxmox-source-select {
  max-width: 22.5rem;
}

.proxmox-chart-body {
  height: 11rem;
}

.node-live-meta-text {
  font-size: 0.7rem;
}

.node-live-meta-spinner {
  width: 0.65rem;
  height: 0.65rem;
  border-width: 0.1em;
}

@media (max-width: 992px) {
  .node-live-meta {
    position: static !important;
    margin-top: 0.75rem;
    padding: 0;
    justify-content: flex-end;
    width: 100%;
  }
}

@media (max-width: 768px) {
  :deep(.proxmox-node-tabs .nav-link) {
    white-space: nowrap;
    padding-inline: 0.6rem;
  }
}
</style>
