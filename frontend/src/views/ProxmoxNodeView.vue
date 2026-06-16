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
      <!-- Header -->
      <div class="page-header mb-4">
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <router-link
            to="/proxmox"
            class="text-decoration-none"
          >
            Proxmox VE
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
        </div>
        <div class="text-secondary">
          {{ node.cluster_name || 'Nœud standalone' }} · PVE {{ node.pve_version || 'N/A' }} · {{ node.ip_address }}
        </div>
      </div>

      <!-- Shared sensor source mapping (CPU temp + fan RPM) -->
      <div class="card mb-3">
        <div class="card-body d-flex flex-wrap align-items-center gap-2">
          <div class="subheader mb-0 me-2">
            Source capteurs nœud (CPU + ventilateurs)
          </div>
          <select
            v-model="sensorSourceHostId"
            class="form-select form-select-sm proxmox-source-select"
          >
            <option value="">
              Aucune (capteurs locaux du nœud)
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
            Enregistrer
          </button>
          <span
            v-if="sensorSourceMsg"
            :class="['small', sensorSourceOk ? 'text-success' : 'text-danger']"
          >{{ sensorSourceMsg }}</span>
          <span
            v-else-if="sensorSourceHostName"
            class="small text-muted"
          >Actuel: {{ sensorSourceHostName }}</span>
        </div>
      </div>

      <!-- Compact node stats (static + live in one card) -->
      <div class="card mb-4">
        <div class="card-body position-relative">
          <div class="row g-4 align-items-start">
            <!-- CPU -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                CPU
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
                {{ node.cpu_count }} cœurs
              </div>
            </div>

            <!-- CPU Temp (from mapped source host) -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                CPU TEMP
              </div>
              <div
                class="h3 mb-1"
                :class="tempColor(nodeCpuTempCurrent)"
              >
                {{ nodeCpuTempCurrent > 0 ? `${nodeCpuTempCurrent.toFixed(1)}°C` : '—' }}
              </div>
              <div class="text-muted small">
                <span v-if="!sensorSourceHostName">Source non configurée</span>
              </div>
            </div>

            <!-- Fan RPM (from mapped source host) -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                FAN RPM
              </div>
              <div class="h3 mb-1 text-blue">
                {{ nodeFanRPMCurrent > 0 ? `${nodeFanRPMCurrent.toFixed(0)} RPM` : '—' }}
              </div>
              <div class="text-muted small">
                <span v-if="!sensorSourceHostName">Source non configurée</span>
              </div>
            </div>

            <!-- RAM -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                RAM
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
                Uptime
              </div>
              <div class="h3 mb-0">
                {{ formatUptime(node.uptime) }}
              </div>
            </div>

            <!-- Guests -->
            <div class="col-6 col-sm-4 col-lg">
              <div class="subheader mb-1">
                Guests
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
                  IO Wait
                </div>
                <div
                  class="h3 mb-0"
                  :class="liveStatus.wait > 0.2 ? 'text-danger' : liveStatus.wait > 0.05 ? 'text-warning' : 'text-success'"
                >
                  {{ (liveStatus.wait * 100).toFixed(2) }}%
                </div>
                <div class="text-muted small">
                  disque
                </div>
              </div>

              <!-- Swap -->
              <div class="col-6 col-sm-4 col-lg">
                <div class="subheader mb-1">
                  Swap
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
                  Rootfs
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
              <span class="spinner-border spinner-border-sm me-1" />Chargement…
            </div>
          </div>

          <!-- Live refresh timestamp + error (absolute, no added height) -->
          <div class="position-absolute bottom-0 end-0 pb-2 pe-3 d-flex align-items-center gap-2 node-live-meta">
            <span
              v-if="liveStatusError"
              class="text-danger node-live-meta-text"
            >{{ liveStatusError }}</span>
            <span
              v-if="liveStatus"
              class="text-muted node-live-meta-text"
            >
              <span
                v-if="liveStatusLoading"
                class="spinner-border me-1 node-live-meta-spinner"
              />
              Actualisé à {{ liveStatusTime }}
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
        :temp-empty-text="nodeTempLoading ? 'Chargement…' : (nodeTempError || (sensorSourceHostId ? 'Aucune donnée température disponible' : 'Configurez une source capteurs pour ce nœud'))"
        :fan-empty-text="nodeFanLoading ? 'Chargement…' : (nodeFanError || (sensorSourceHostId ? 'Aucune donnée ventilateur disponible' : 'Configurez une source capteurs pour ce nœud'))"
        @timeframe-changed="loadRRD"
      />

      <!-- Updates banner (only shown when pending updates exist) -->
      <div
        v-if="node.pending_updates > 0"
        class="alert alert-warning mb-4"
      >
        <div class="d-flex align-items-center gap-3">
          <div>
            <strong>Mises à jour disponibles sur ce nœud :</strong>
            {{ node.pending_updates }} paquet(s) en attente
          </div>
          <div
            v-if="node.last_update_check_at"
            class="ms-auto text-muted small"
          >
            Dernière vérification : {{ formatDate(node.last_update_check_at) }}
          </div>
        </div>
      </div>

      <!-- Tabs + side console -->
      <div class="side-layout">
        <div class="side-main">
          <div class="card">
            <div class="card-header">
              <ul class="nav nav-tabs card-header-tabs proxmox-node-tabs">
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'vms' }"
                    @click="tab = 'vms'; loadGuestNetworks()"
                  >
                    VMs <span class="badge bg-azure-lt text-azure ms-1">{{ vms.length }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'lxc' }"
                    @click="tab = 'lxc'; loadGuestNetworks()"
                  >
                    LXC <span class="badge bg-azure-lt text-azure ms-1">{{ lxcs.length }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'storage' }"
                    @click="tab = 'storage'"
                  >
                    Stockage <span class="badge bg-azure-lt text-azure ms-1">{{ node.storages?.length ?? 0 }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'disks' }"
                    @click="tab = 'disks'"
                  >
                    Disques <span class="badge bg-azure-lt text-azure ms-1">{{ node.disks?.length ?? 0 }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'tasks' }"
                    @click="tab = 'tasks'"
                  >
                    Tâches <span class="badge bg-azure-lt text-azure ms-1">{{ node.tasks?.length ?? 0 }}</span>
                    <span
                      v-if="failedTaskCount > 0"
                      class="badge bg-warning ms-1"
                    >{{ failedTaskCount }}</span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'updates' }"
                    @click="tab = 'updates'"
                  >
                    Mises à jour
                    <span
                      v-if="node.pending_updates > 0"
                      class="badge ms-1 bg-warning-lt text-warning"
                    >
                      {{ node.pending_updates }}
                    </span>
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'services' }"
                    @click="tab = 'services'; loadServices()"
                  >
                    Services
                  </button>
                </li>
                <li class="nav-item">
                  <button
                    type="button"
                    class="nav-link"
                    :class="{ active: tab === 'security' }"
                    @click="tab = 'security'"
                  >
                    Sécurité <span class="badge bg-azure-lt text-azure ms-1">{{ securityEventsCount }}</span>
                  </button>
                </li>
              </ul>
            </div>

            <!-- VMs tab -->
            <div
              v-if="isTabMounted('vms')"
              v-show="tab === 'vms'"
            >
              <ProxmoxNodeGuestsTab
                kind="vm"
                :guests="vms"
                :guest-networks="guestNetworks"
                :guest-networks-loading="guestNetworksLoading"
                :links="guestLinks"
                :peer-nodes="peerNodes"
                :node-id="String(route.params.id)"
                @confirm-link="confirmGuestLink"
                @ignore-link="ignoreGuestLink"
                @go-host="goToHost"
                @migrate="openMigrateModal($event, 'vm')"
              />
            </div>

            <!-- LXC tab -->
            <div
              v-if="isTabMounted('lxc')"
              v-show="tab === 'lxc'"
            >
              <ProxmoxNodeGuestsTab
                kind="lxc"
                :guests="lxcs"
                :guest-networks="guestNetworks"
                :guest-networks-loading="guestNetworksLoading"
                :links="guestLinks"
                :peer-nodes="peerNodes"
                :node-id="String(route.params.id)"
                @confirm-link="confirmGuestLink"
                @ignore-link="ignoreGuestLink"
                @go-host="goToHost"
                @migrate="openMigrateModal($event, 'lxc')"
              />
            </div>

            <!-- Link action feedback -->
            <div
              v-if="linkMsg"
              class="card-footer py-2"
            >
              <span :class="['small', linkMsgOk ? 'text-success' : 'text-danger']">{{ linkMsg }}</span>
            </div>

            <!-- Disks tab -->
            <div
              v-if="isTabMounted('disks')"
              v-show="tab === 'disks'"
            >
              <ProxmoxNodeDisksTab :disks="node.disks || []" />
            </div>

            <!-- Tasks tab -->
            <div
              v-if="isTabMounted('tasks')"
              v-show="tab === 'tasks'"
            >
              <ProxmoxNodeTasksTab
                :tasks="node.tasks || []"
                :active-upid="activeUpid"
                @view-logs="startPollingTask($event.upid, { action: $event.action, label: $event.label })"
              />
            </div>

            <!-- Updates tab -->
            <div
              v-if="isTabMounted('updates')"
              v-show="tab === 'updates'"
            >
              <ProxmoxNodeUpdatesTab
                :pending-updates="node.pending_updates"
                :last-update-check-at="node.last_update_check_at"
                :apt-refreshing="aptRefreshing"
                :apt-refresh-msg="aptRefreshMsg"
                :apt-refresh-ok="aptRefreshOk"
                @refresh-apt="triggerAptRefresh"
              />
            </div>

            <!-- Services tab -->
            <div
              v-if="isTabMounted('services')"
              v-show="tab === 'services'"
            >
              <ProxmoxNodeServicesTab
                :services="services"
                :loading="servicesLoading"
                :error="servicesError"
                :action-msg="svcActionMsg"
                :action-ok="svcActionOk"
                @refresh="loadServices"
                @action="svcAction($event.name, $event.action)"
              />
            </div>

            <!-- Security tab -->
            <div
              v-if="isTabMounted('security')"
              v-show="tab === 'security'"
            >
              <ProxmoxNodeSecurityTab
                :node-id="String(route.params.id)"
                :active="tab === 'security'"
                @count="securityEventsCount = $event"
              />
            </div>

            <!-- Storage tab -->
            <div
              v-if="isTabMounted('storage')"
              v-show="tab === 'storage'"
            >
              <ProxmoxNodeStorageTab :storages="node.storages || []" />
            </div>
          </div>
        </div> <!-- /side-main -->
        <CommandLogPanel
          :command="liveTask"
          :show="showConsole"
          title="Logs tâche PVE"
          empty-text="Cliquez sur 'Logs' dans une tâche pour suivre l'exécution"
          wrapper-class="side-panel"
          @open="showConsole = true"
          @close="closeConsole"
        />
      </div> <!-- /side-layout -->
    </div> <!-- /v-else-if node -->

    <!-- Migration modal -->
    <div
      v-if="migrateModal.open"
      class="modal modal-blur fade show d-block"
      tabindex="-1"
      style="background:rgba(0,0,0,.5)"
      @click.self="migrateModal.open = false"
    >
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              Migrer {{ migrateModal.guest?.name || `VMID ${migrateModal.guest?.vmid}` }}
            </h5>
            <button
              type="button"
              class="btn-close"
              @click="migrateModal.open = false"
            />
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">Nœud cible</label>
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
                <span class="form-check-label">Migration à chaud (sans arrêt)</span>
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
              Annuler
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
              Migrer
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, shallowRef, watch, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
const CommandLogPanel = defineAsyncComponent(() => import('../components/host/CommandLogPanel.vue'))
const ProxmoxNodeChartsPanel = defineAsyncComponent(() => import('../components/proxmox/ProxmoxNodeChartsPanel.vue'))
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import ProxmoxNodeDisksTab from '../components/proxmox/ProxmoxNodeDisksTab.vue'
import ProxmoxNodeStorageTab from '../components/proxmox/ProxmoxNodeStorageTab.vue'
import ProxmoxNodeTasksTab from '../components/proxmox/ProxmoxNodeTasksTab.vue'
import ProxmoxNodeUpdatesTab from '../components/proxmox/ProxmoxNodeUpdatesTab.vue'
import ProxmoxNodeServicesTab from '../components/proxmox/ProxmoxNodeServicesTab.vue'
import ProxmoxNodeSecurityTab from '../components/proxmox/ProxmoxNodeSecurityTab.vue'
import ProxmoxNodeGuestsTab from '../components/proxmox/ProxmoxNodeGuestsTab.vue'
import api from '../api'

const route = useRoute()
const router = useRouter()
const node = ref<any>(null)
const loading = ref(true)
const error = ref('')
const tab = ref('vms')
watch(tab, (t) => {
  router.replace({ query: { ...route.query, tab: t } })
  mountedTabs.value.add(t)
})

const mountedTabs = ref(new Set<string>(['vms']))

function isTabMounted(t: string): boolean {
  return mountedTabs.value.has(t)
}

const guestLinks = ref<Record<string, any>>({})
const linkMsg = ref('')
const linkMsgOk = ref(false)

const sensorSourceCandidates = ref<any[]>([])
const sensorSourceHostId = ref('')
const sensorSourceLoading = ref(false)
const sensorSourceSaving = ref(false)
const sensorSourceMsg = ref('')
const sensorSourceOk = ref(false)
const sensorSourceHostName = computed(() =>
  node.value?.cpu_temp_source_host_name || node.value?.fan_rpm_source_host_name || ''
)

const nodeTempLoading = ref(false)
const nodeTempError = ref('')
const nodeTempChart = shallowRef<any>(null)
const nodeCpuTempCurrent = ref(0)

const nodeFanLoading = ref(false)
const nodeFanError = ref('')
const nodeFanChart = shallowRef<any>(null)
const nodeFanRPMCurrent = ref(0)

// apt refresh
const aptRefreshing = ref(false)
const aptRefreshMsg = ref('')
const aptRefreshOk = ref(false)

// peer nodes for migration target list
const peerNodes = ref<any[]>([])

const migrateModal = ref<any>({
  open: false,
  guest: null,
  guestType: 'vm',
  target: '',
  online: false,
  loading: false,
  error: '',
})

const liveStatus = ref<any>(null)
const liveStatusLoading = ref(false)
const liveStatusTime = ref('')
const liveStatusError = ref('')

// RRD charts
const rrdTimeframe = ref('hour')
const rrdTimeframeToHours: Record<string, number> = {
  hour: 1,
  day: 24,
  week: 24 * 7,
  month: 24 * 30,
  year: 24 * 365,
}
const rrdCpuChart = shallowRef<any>(null)
const rrdRamChart = shallowRef<any>(null)
const rrdIowaitChart = shallowRef<any>(null)
const rrdNetChart = shallowRef<any>(null)
const rrdLoading = ref(false)
const rrdError = ref('')

const showConsole = ref(false)
const liveTask = ref<any>(null)
const activeUpid = ref<string | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let liveStatusTimer: ReturnType<typeof setInterval> | null = null

const guestNetworks = ref<Record<string, any[]>>({})
const guestNetworksLoading = ref(false)

async function loadGuestNetworks(): Promise<void> {
  if (guestNetworksLoading.value || Object.keys(guestNetworks.value).length > 0) return
  guestNetworksLoading.value = true
  try {
    const res = await api.getProxmoxNodeGuestNetworks(String(route.params.id))
    guestNetworks.value = res.data ?? {}
  } catch { /* non-bloquant */ }
  finally { guestNetworksLoading.value = false }
}

// services
const services = ref<any[]>([])
const servicesLoading = ref(false)
const servicesError = ref('')
const svcActionMsg = ref('')
const svcActionOk = ref(false)

const securityEventsCount = ref(0)

const vms = computed(() => node.value?.guests?.filter((g: any) => g.guest_type === 'vm') ?? [])
const lxcs = computed(() => node.value?.guests?.filter((g: any) => g.guest_type === 'lxc') ?? [])
const failedTaskCount = computed(() =>
  (node.value?.tasks ?? []).filter((t: any) => t.status === 'stopped' && t.exit_status && t.exit_status !== 'OK').length
)
async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const requestedTab = String(route.query.tab || '')
    const validTabs = ['vms', 'lxc', 'storage', 'disks', 'tasks', 'updates', 'services', 'security']
    if (validTabs.includes(requestedTab)) {
      tab.value = requestedTab
      mountedTabs.value.add(requestedTab)
    }
    const res = await api.getProxmoxNode(String(route.params.id))
    node.value = res.data
    sensorSourceHostId.value = node.value?.cpu_temp_source_host_id || node.value?.fan_rpm_source_host_id || ''
    await loadSensorSourceCandidates()
    await loadGuestLinks()
    // fire-and-forget: live status + RRD charts + peer nodes load in parallel
    loadLiveStatus()
    loadRRD('hour')
    loadPeerNodes()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur lors du chargement.'
  } finally {
    loading.value = false
  }
}

async function loadNodeCpuTempHistory(hours: number = rrdTimeframeToHours[rrdTimeframe.value] ?? 24): Promise<void> {
  nodeTempLoading.value = true
  nodeTempError.value = ''
  nodeTempChart.value = null
  nodeCpuTempCurrent.value = 0

  try {
    const sourceHostId = sensorSourceHostId.value || node.value?.cpu_temp_source_host_id || node.value?.fan_rpm_source_host_id
    if (!sourceHostId) {
      return
    }

    const res = await api.getProxmoxNodeCpuTempHistory(String(route.params.id), hours)
    const points = (Array.isArray(res.data) ? res.data : []).filter((p: any) => Number(p?.cpu_temperature) > 0)
    if (!points.length) {
      return
    }

    const labels = points.map((p: any) => {
      const d = new Date(p.timestamp)
      if (hours <= 24) {
        return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      }
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
    })
    const data = points.map((p: any) => Number(p.cpu_temperature))
    nodeCpuTempCurrent.value = data[data.length - 1] || 0
    nodeTempChart.value = {
      labels,
      datasets: [{
        data,
        borderColor: cssVar('--tblr-red'),
        backgroundColor: `rgba(${cssVar('--tblr-red-rgb')},0.12)`,
        fill: true,
        tension: 0.3,
        spanGaps: true,
      }],
    }
  } catch (e: any) {
    nodeTempError.value = e?.response?.data?.error || 'Erreur lors du chargement de la température CPU.'
  } finally {
    nodeTempLoading.value = false
  }
}

async function loadNodeFanRPMHistory(hours: number = rrdTimeframeToHours[rrdTimeframe.value] ?? 24): Promise<void> {
  nodeFanLoading.value = true
  nodeFanError.value = ''
  nodeFanChart.value = null
  nodeFanRPMCurrent.value = 0

  try {
    const sourceHostId = sensorSourceHostId.value || node.value?.fan_rpm_source_host_id || node.value?.cpu_temp_source_host_id
    if (!sourceHostId) {
      return
    }

    const res = await api.getProxmoxNodeFanRPMHistory(String(route.params.id), hours)
    const points = (Array.isArray(res.data) ? res.data : []).filter((p: any) => Number(p?.fan_rpm) > 0)
    if (!points.length) {
      return
    }

    const labels = points.map((p: any) => {
      const d = new Date(p.timestamp)
      if (hours <= 24) {
        return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      }
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
    })
    const data = points.map((p: any) => Number(p.fan_rpm))
    nodeFanRPMCurrent.value = data[data.length - 1] || 0
    nodeFanChart.value = {
      labels,
      datasets: [{
        data,
        borderColor: cssVar('--tblr-azure'),
        backgroundColor: `rgba(${cssVar('--tblr-azure-rgb')},0.12)`,
        fill: true,
        tension: 0.3,
        spanGaps: true,
      }],
    }
  } catch (e: any) {
    nodeFanError.value = e?.response?.data?.error || 'Erreur lors du chargement des RPM ventilateurs.'
  } finally {
    nodeFanLoading.value = false
  }
}

async function loadSensorSourceCandidates(): Promise<void> {
  sensorSourceLoading.value = true
  try {
    const res = await api.getProxmoxNodeSensorSourceCandidates(String(route.params.id))
    sensorSourceCandidates.value = Array.isArray(res.data) ? res.data : []
  } catch {
    sensorSourceCandidates.value = []
  } finally {
    sensorSourceLoading.value = false
  }
}

async function refreshNodeSensorSource() {
  try {
    const res = await api.getProxmoxNode(String(route.params.id))
    const n = res.data || {}
    if (node.value) {
      node.value.cpu_temp_source_host_id = n.cpu_temp_source_host_id || ''
      node.value.cpu_temp_source_host_name = n.cpu_temp_source_host_name || ''
      node.value.fan_rpm_source_host_id = n.fan_rpm_source_host_id || ''
      node.value.fan_rpm_source_host_name = n.fan_rpm_source_host_name || ''
    }
    sensorSourceHostId.value = n.cpu_temp_source_host_id || n.fan_rpm_source_host_id || ''
  } catch {
    // non-bloquant
  }
}

async function saveSensorSource() {
  sensorSourceSaving.value = true
  sensorSourceMsg.value = ''
  try {
    const target = sensorSourceHostId.value || null
    const res = await api.setProxmoxNodeSensorSource(String(route.params.id), target)
    if (node.value) {
      node.value.cpu_temp_source_host_id = res.data?.cpu_temp_source_host_id || ''
      node.value.cpu_temp_source_host_name = res.data?.cpu_temp_source_host_name || ''
      node.value.fan_rpm_source_host_id = res.data?.fan_rpm_source_host_id || ''
      node.value.fan_rpm_source_host_name = res.data?.fan_rpm_source_host_name || ''
    }
    sensorSourceHostId.value = res.data?.cpu_temp_source_host_id || res.data?.fan_rpm_source_host_id || ''
    await loadSensorSourceCandidates()
    await loadNodeCpuTempHistory(rrdTimeframeToHours[rrdTimeframe.value] ?? 24)
    await loadNodeFanRPMHistory(rrdTimeframeToHours[rrdTimeframe.value] ?? 24)
    sensorSourceMsg.value = 'Source capteurs mise à jour (CPU + ventilateurs).'
    sensorSourceOk.value = true
  } catch (e: any) {
    sensorSourceMsg.value = e?.response?.data?.error || 'Erreur lors de la mise à jour.'
    sensorSourceOk.value = false
  } finally {
    sensorSourceSaving.value = false
    setTimeout(() => { sensorSourceMsg.value = '' }, 4000)
  }
}

async function loadGuestLinks(): Promise<void> {
  const guests = node.value?.guests ?? []
  if (guests.length === 0) return
  try {
    const res = await api.getProxmoxLinks()
    const guestIds = new Set(guests.map((g: any) => g.id))
    const map: Record<string, any> = {}
    for (const link of res.data ?? []) {
      if (guestIds.has(link.guest_id)) {
        map[link.guest_id] = link
      }
    }
    guestLinks.value = map
  } catch {
    guestLinks.value = {}
  }
}

function linkForGuest(g: any): any {
  return guestLinks.value[g.id] ?? null
}

async function confirmGuestLink(g: any): Promise<void> {
  const link = linkForGuest(g)
  if (!link) return
  try {
    const res = await api.updateProxmoxLink(link.id, { status: 'confirmed' })
    guestLinks.value = { ...guestLinks.value, [g.id]: res.data }
    await loadSensorSourceCandidates()
    await refreshNodeSensorSource()
    showMsg(`[${g.name}] Lien confirmé.`, true)
  } catch (e: any) {
    showMsg(e?.response?.data?.error || 'Erreur.', false)
  }
}

async function ignoreGuestLink(g: any): Promise<void> {
  const link = linkForGuest(g)
  if (!link) return
  try {
    await api.deleteProxmoxLink(link.id)
    const m = { ...guestLinks.value }
    delete m[g.id]
    guestLinks.value = m
    showMsg(`[${g.name}] Suggestion ignorée.`, true)
  } catch (e: any) {
    showMsg(e?.response?.data?.error || 'Erreur.', false)
  }
}

function goToHost(link: any): void {
  if (link?.host_id) router.push(`/hosts/${link.host_id}`)
}

function showMsg(msg: string, ok: boolean): void {
  linkMsg.value = msg
  linkMsgOk.value = ok
  setTimeout(() => { linkMsg.value = '' }, 4000)
}

async function loadRRD(timeframe: string = rrdTimeframe.value): Promise<void> {
  rrdTimeframe.value = timeframe
  void loadNodeCpuTempHistory(rrdTimeframeToHours[timeframe] ?? 24)
  void loadNodeFanRPMHistory(rrdTimeframeToHours[timeframe] ?? 24)
  rrdLoading.value = true
  rrdError.value = ''
  try {
    const res = await api.getProxmoxNodeRRD(String(route.params.id), timeframe)
    buildRRDCharts(res.data ?? [], timeframe)
  } catch (e: any) {
    rrdError.value = e?.response?.data?.error || 'Erreur lors du chargement des métriques.'
    rrdCpuChart.value = null
    rrdRamChart.value = null
    rrdIowaitChart.value = null
    rrdNetChart.value = null
  } finally {
    rrdLoading.value = false
  }
}

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

function buildRRDCharts(points: any[], timeframe: string): void {
  const labels = points.map((p: any) => {
    const d = new Date(p.time * 1000)
    if (timeframe === 'hour' || timeframe === 'day')
      return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    if (timeframe === 'week')
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' }) + ' ' + d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
  })

  rrdCpuChart.value = {
    labels,
    datasets: [{
      data: points.map((p: any) => p.cpu != null ? p.cpu * 100 : null),
      borderColor: cssVar('--tblr-blue'), backgroundColor: `rgba(${cssVar('--tblr-blue-rgb')},0.1)`,
      fill: true, tension: 0.3, spanGaps: true,
    }],
  }

  // RAM: memused / memtotal are raw bytes from PVE RRD (JSON keys: memused, memtotal)
  const ramData = points.map((p: any) =>
    (p.memused != null && p.memtotal != null && p.memtotal > 0)
      ? (p.memused / p.memtotal) * 100
      : null
  )
  rrdRamChart.value = ramData.some((v: any) => v != null) ? {
    labels,
    datasets: [{
      data: ramData,
      borderColor: cssVar('--tblr-green'), backgroundColor: `rgba(${cssVar('--tblr-green-rgb')},0.1)`,
      fill: true, tension: 0.3, spanGaps: true,
    }],
  } : null

  const hasIowait = points.some((p: any) => p.iowait != null)
  rrdIowaitChart.value = hasIowait ? {
    labels,
    datasets: [{
      data: points.map((p: any) => p.iowait != null ? p.iowait * 100 : null),
      borderColor: cssVar('--tblr-yellow'), backgroundColor: `rgba(${cssVar('--tblr-yellow-rgb')},0.1)`,
      fill: true, tension: 0.3, spanGaps: true,
    }],
  } : null

  const hasNet = points.some((p: any) => p.netin != null || p.netout != null)
  rrdNetChart.value = hasNet ? {
    labels,
    datasets: [
      {
        label: 'Entrante',
        data: points.map((p: any) => p.netin ?? null),
        borderColor: cssVar('--tblr-indigo'), backgroundColor: `rgba(${cssVar('--tblr-indigo-rgb')},0.1)`,
        fill: true, tension: 0.3, spanGaps: true,
      },
      {
        label: 'Sortante',
        data: points.map((p: any) => p.netout ?? null),
        borderColor: cssVar('--tblr-pink'), backgroundColor: `rgba(${cssVar('--tblr-pink-rgb')},0.05)`,
        fill: false, tension: 0.3, spanGaps: true,
      },
    ],
  } : null
}

async function loadLiveStatus(): Promise<void> {
  liveStatusLoading.value = true
  liveStatusError.value = ''
  try {
    const res = await api.getProxmoxNodeStatus(String(route.params.id))
    liveStatus.value = res.data
    liveStatusTime.value = new Date().toLocaleTimeString('fr-FR')
  } catch (e: any) {
    liveStatusError.value = e?.response?.data?.error || `Erreur ${e?.response?.status ?? ''} — vérifiez la connectivité au nœud.`
  } finally {
    liveStatusLoading.value = false
  }
}


function stopPolling(): void {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = null
}

function closeConsole(): void {
  stopPolling()
  showConsole.value = false
  liveTask.value = null
  activeUpid.value = null
}

async function startPollingTask(upid: string, { action = '', label = '' }: { action?: string; label?: string } = {}): Promise<void> {
  stopPolling()
  activeUpid.value = upid
  liveTask.value = {
    host_name: node.value?.node_name ?? '',
    module: 'proxmox',
    action: action || upid,
    target: label || '',   // short display label, not the raw UPID
    status: 'running',
    output: '',
  }
  showConsole.value = true

  const poll = async (): Promise<void> => {
    try {
      const res = await api.getProxmoxTaskLog(String(route.params.id), upid)
      const lines = (res.data ?? []).map((l: any) => l.t).join('\n')
      const lastLine = res.data?.[res.data.length - 1]?.t ?? ''
      const done = lastLine.startsWith('TASK OK') || lastLine.startsWith('TASK ERROR')
      const status = done
        ? (lastLine.startsWith('TASK OK') ? 'completed' : 'failed')
        : 'running'
      liveTask.value = { ...liveTask.value, output: lines, status }
      if (!done) pollTimer = setTimeout(poll, 2000)
    } catch {
      pollTimer = setTimeout(poll, 3000)
    }
  }
  await poll()
}

async function triggerAptRefresh(): Promise<void> {
  aptRefreshing.value = true
  aptRefreshMsg.value = ''
  try {
    const res = await api.refreshProxmoxNodeApt(String(route.params.id))
    const upid = res.data?.upid
    aptRefreshMsg.value = upid ? 'Tâche lancée — logs en cours…' : (res.data?.message || 'Tâche lancée.')
    aptRefreshOk.value = true
    if (upid) startPollingTask(upid, { action: 'apt update' })
  } catch (e: any) {
    aptRefreshMsg.value = e?.response?.data?.error || 'Erreur lors du lancement de apt update.'
    aptRefreshOk.value = false
  } finally {
    aptRefreshing.value = false
    setTimeout(() => { aptRefreshMsg.value = '' }, 6000)
  }
}

function memPct(n: any): string | number {
  if (!n.mem_total) return 0
  return ((n.mem_used / n.mem_total) * 100).toFixed(1)
}


function cpuColor(usage: number): string {
  if (usage > 0.85) return 'bg-danger'
  if (usage > 0.6) return 'bg-warning'
  return 'bg-success'
}

function tempColor(temp: number | undefined): string {
  if (!temp) return 'text-secondary'
  if (temp >= 85) return 'text-danger'
  if (temp >= 70) return 'text-warning'
  return 'text-success'
}

function ramColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-success'
}

function storageColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-primary'
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0, v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatUptime(seconds: number | undefined): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}j ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

async function loadServices(): Promise<void> {
  if (servicesLoading.value || services.value.length > 0) return
  servicesLoading.value = true
  servicesError.value = ''
  try {
    const res = await api.getProxmoxNodeServices(String(route.params.id))
    services.value = res.data ?? []
  } catch (e: any) {
    servicesError.value = e?.response?.data?.error || 'Erreur lors du chargement des services.'
  } finally {
    servicesLoading.value = false
  }
}

async function svcAction(name: string, action: string): Promise<void> {
  svcActionMsg.value = ''
  try {
    const res = await api.proxmoxNodeServiceAction(String(route.params.id), name, action)
    const upid = res.data?.upid
    svcActionMsg.value = upid ? `${action} ${name} lancé — logs en cours…` : `${action} ${name} lancé.`
    svcActionOk.value = true
    if (upid) startPollingTask(upid, { action: `service ${action}`, label: name })
    else setTimeout(() => loadServices(), 2000)
  } catch (e: any) {
    svcActionMsg.value = e?.response?.data?.error || `Erreur lors de ${action} ${name}.`
    svcActionOk.value = false
  }
  setTimeout(() => { svcActionMsg.value = '' }, 6000)
}

async function loadPeerNodes(): Promise<void> {
  if (!node.value?.connection_id) return
  try {
    const res = await api.getProxmoxNodes(node.value.connection_id)
    peerNodes.value = (res.data ?? []).filter((n: any) => n.node_name !== node.value?.node_name && n.status === 'online')
  } catch {
    peerNodes.value = []
  }
}

function openMigrateModal(guest: any, guestType: string = 'vm'): void {
  migrateModal.value = {
    open: true,
    guest,
    guestType,
    target: peerNodes.value[0]?.node_name ?? '',
    online: false,
    loading: false,
    error: '',
  }
}

async function submitMigration(): Promise<void> {
  const m = migrateModal.value
  if (!m.target || !m.guest) return
  m.loading = true
  m.error = ''
  try {
    const res = await api.migrateProxmoxGuest(String(route.params.id), m.guest.vmid, {
      target: m.target,
      guest_type: m.guestType,
      online: m.online,
    })
    const upid = res.data?.upid
    migrateModal.value.open = false
    if (upid) {
      startPollingTask(upid, { action: 'migrate', label: `${m.guest.name || m.guest.vmid} → ${m.target}` })
    }
  } catch (e: any) {
    m.error = e?.response?.data?.error || 'Erreur lors du lancement de la migration.'
  } finally {
    m.loading = false
  }
}


onMounted(() => {
  load()
  liveStatusTimer = setInterval(loadLiveStatus, 60_000)
})
onUnmounted(() => {
  stopPolling()
  if (liveStatusTimer) clearInterval(liveStatusTimer)
})
</script>

<style scoped>
.proxmox-node-tabs {
  flex-wrap: nowrap;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
}

.proxmox-node-tabs .nav-item {
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
  .proxmox-node-tabs .nav-link {
    white-space: nowrap;
    padding-inline: 0.6rem;
  }
}
</style>
