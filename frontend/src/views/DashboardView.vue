<template>
  <div>
    <!-- ─── En-tête ─────────────────────────────────────────────────────────── -->
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center
                justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">ServerSupervisor</div>
        <h2 class="page-title">Dashboard</h2>
        <div class="text-secondary">Vue d'ensemble de l'infrastructure</div>
      </div>
      <div class="d-flex flex-wrap align-items-center gap-2">
        <template v-if="canRunApt">
          <div class="d-flex flex-wrap gap-2">
            <button class="btn btn-outline-secondary btn-sm" @click="selectAllFiltered">Tout sélectionner</button>
            <button class="btn btn-outline-secondary btn-sm" @click="clearSelection" :disabled="selectedCount === 0">Vider</button>
            <button class="btn btn-outline-secondary btn-sm" @click="sendBulkApt('update')" :disabled="selectedCount === 0 || aptLoading !== ''">
              <span v-if="aptLoading === 'update'" class="spinner-border spinner-border-sm me-1" role="status"></span>
              apt update ({{ selectedCount }})
            </button>
            <button class="btn btn-primary btn-sm" @click="sendBulkApt('upgrade')" :disabled="selectedCount === 0 || aptLoading !== ''">
              <span v-if="aptLoading === 'upgrade'" class="spinner-border spinner-border-sm me-1" role="status"></span>
              apt upgrade ({{ selectedCount }})
            </button>
          </div>
        </template>
        <div v-else class="text-secondary small">Mode lecture seule</div>
        <router-link to="/hosts/new" class="btn btn-primary btn-sm">
          <svg class="icon" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
          </svg>
          <span class="d-none d-sm-inline ms-1">Ajouter un hôte</span>
        </router-link>
      </div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <!-- ─── Bannières d'alerte ───────────────────────────────────────────────── -->
    <div v-if="cveSummary && (cveSummary.hosts_with_critical > 0 || cveSummary.hosts_with_high > 0)"
         class="alert alert-danger mb-3 d-flex align-items-center gap-3">
      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-lg flex-shrink-0" width="24" height="24" viewBox="0 0 24 24"
           fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
        <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
      </svg>
      <div class="flex-grow-1">
        <div class="fw-semibold">Vulnérabilités détectées sur vos hôtes</div>
        <div class="text-secondary small">
          <span v-if="cveSummary.hosts_with_critical > 0" class="me-3">
            <span class="badge bg-red-lt text-red me-1">CRITICAL</span>
            {{ cveSummary.critical_count }} CVE sur {{ cveSummary.hosts_with_critical }} hôte{{ cveSummary.hosts_with_critical > 1 ? 's' : '' }}
          </span>
          <span v-if="cveSummary.hosts_with_high > 0">
            <span class="badge bg-orange-lt text-orange me-1">HIGH</span>
            {{ cveSummary.high_count }} CVE sur {{ cveSummary.hosts_with_high }} hôte{{ cveSummary.hosts_with_high > 1 ? 's' : '' }}
          </span>
        </div>
      </div>
      <router-link to="/apt" class="btn btn-sm btn-danger">Voir les mises à jour</router-link>
    </div>

    <div v-if="proxmoxSummary && (proxmoxSummary.nodes_down > 0 || proxmoxSummary.recent_failed_tasks > 0 || proxmoxSummary.storage_near_full > 0 || proxmoxSummary.storage_offline > 0)"
         class="alert alert-warning mb-3 d-flex align-items-center gap-3">
      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-lg flex-shrink-0" width="24" height="24" viewBox="0 0 24 24"
           fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
        <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
      </svg>
      <div class="flex-grow-1">
        <div class="fw-semibold">Alertes Proxmox</div>
        <div class="text-secondary small d-flex flex-wrap gap-2">
          <span v-if="proxmoxSummary.nodes_down > 0">{{ proxmoxSummary.nodes_down }} nœud{{ proxmoxSummary.nodes_down > 1 ? 's' : '' }} hors ligne</span>
          <span v-if="proxmoxSummary.storage_near_full > 0">{{ proxmoxSummary.storage_near_full }} stockage{{ proxmoxSummary.storage_near_full > 1 ? 's' : '' }} presque plein{{ proxmoxSummary.storage_near_full > 1 ? 's' : '' }}</span>
          <span v-if="proxmoxSummary.storage_offline > 0">{{ proxmoxSummary.storage_offline }} stockage{{ proxmoxSummary.storage_offline > 1 ? 's' : '' }} hors ligne</span>
          <span v-if="proxmoxSummary.recent_failed_tasks > 0">{{ proxmoxSummary.recent_failed_tasks }} tâche{{ proxmoxSummary.recent_failed_tasks > 1 ? 's' : '' }} échouée{{ proxmoxSummary.recent_failed_tasks > 1 ? 's' : '' }} (24h)</span>
        </div>
      </div>
      <router-link to="/proxmox" class="btn btn-sm btn-warning">Voir Proxmox</router-link>
    </div>

    <!-- ─── KPIs ─────────────────────────────────────────────────────────────── -->
    <div class="row row-cards mb-4">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Hôtes</div>
            <div class="h1 mb-0">{{ hosts.length }}</div>
            <div class="text-secondary small mt-1">
              <span class="text-green me-2">{{ onlineCount }} en ligne</span>
              <span v-if="offlineCount > 0" class="text-red">{{ offlineCount }} hors ligne</span>
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Mises à jour</div>
            <div class="h1 mb-0" :class="outdatedVersions > 0 ? 'text-yellow' : 'text-green'">{{ outdatedVersions }}</div>
            <div class="text-secondary small mt-1">
              <span v-if="aptPending > 0" class="me-2">{{ aptPending }} paquet{{ aptPending > 1 ? 's' : '' }} APT</span>
              <span v-if="outdatedDockerImages > 0">{{ outdatedDockerImages }} image{{ outdatedDockerImages > 1 ? 's' : '' }} Docker</span>
              <span v-if="outdatedVersions === 0">Tout est à jour</span>
            </div>
          </div>
        </div>
      </div>
      <!-- Proxmox KPIs (masqués si non configuré) -->
      <template v-if="hasProxmox">
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">Proxmox — Nœuds</div>
              <div class="h1 mb-0" :class="proxmoxSummary?.nodes_down > 0 ? 'text-red' : 'text-green'">
                {{ (proxmoxSummary?.node_count ?? 0) - (proxmoxSummary?.nodes_down ?? 0) }}
                <span class="text-secondary fs-4">/ {{ proxmoxSummary?.node_count ?? 0 }}</span>
              </div>
              <div class="text-secondary small mt-1">{{ proxmoxSummary?.vm_count ?? 0 }} VM · {{ proxmoxSummary?.lxc_count ?? 0 }} LXC</div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">Proxmox — Stockage</div>
              <div class="h1 mb-0" :class="proxmoxStoragePct > 80 ? 'text-red' : proxmoxStoragePct > 60 ? 'text-yellow' : 'text-green'">
                {{ proxmoxStoragePct.toFixed(0) }}%
              </div>
              <div class="text-secondary small mt-1">
                {{ formatBytes(proxmoxSummary?.storage_used ?? 0) }} / {{ formatBytes(proxmoxSummary?.storage_total ?? 0) }}
              </div>
            </div>
          </div>
        </div>
      </template>
      <template v-else>
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">En ligne</div>
              <div class="h1 mb-0 text-green">{{ onlineCount }}</div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">Hors ligne</div>
              <div class="h1 mb-0 text-red">{{ offlineCount }}</div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- ─── Cluster Proxmox (conditionnel) ──────────────────────────────────── -->
    <ProxmoxClusterCard v-if="hasProxmox && proxmoxNodes.length" :nodes="proxmoxNodes" />

    <!-- ─── Graphiques de tendance ───────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="card-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <h3 class="card-title">Tendance CPU / RAM</h3>
          <div class="text-secondary small">
            <template v-if="hasProxmox">
              <!-- Tabs source -->
              <span
                v-for="src in chartSources"
                :key="src.key"
                class="me-3 cursor-pointer"
                :class="chartSource === src.key ? 'fw-semibold text-body' : 'text-secondary'"
                style="cursor:pointer"
                @click="chartSource = src.key; fetchSummary()"
              >{{ src.label }}</span>
            </template>
            <template v-else>Moyenne sur tous les hôtes</template>
          </div>
        </div>
        <div class="btn-group btn-group-sm">
          <button
            v-for="h in [1, 6, 24, 168, 720]"
            :key="h"
            @click="changeSummaryRange(h)"
            :class="summaryHours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          >
            {{ h >= 24 ? (h / 24) + 'j' : h + 'h' }}
          </button>
        </div>
      </div>
      <div class="card-body" style="height: 14rem;">
        <div v-if="summaryLoading" class="h-100 d-flex align-items-center justify-content-center">
          <div class="spinner-border text-secondary" role="status"></div>
        </div>
        <Line v-else-if="summaryChartData" :data="summaryChartData" :options="summaryChartOptions" class="h-100" />
        <div v-else class="h-100 d-flex align-items-center justify-content-center text-secondary">Aucune donnée</div>
      </div>
    </div>

    <!-- ─── Recherche / filtre ───────────────────────────────────────────────── -->
    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3 align-items-center">
          <div class="col-12 col-lg">
            <input v-model="searchQuery" type="text" class="form-control" placeholder="Rechercher un hôte..." />
          </div>
          <div class="col-12 col-md-4 col-lg-2">
            <select v-model="statusFilter" class="form-select">
              <option value="all">Tous</option>
              <option value="online">En ligne</option>
              <option value="offline">Hors ligne</option>
              <option value="warning">Warning</option>
            </select>
          </div>
          <div class="col-12 col-md-4 col-lg-3">
            <select v-model="sortKey" class="form-select">
              <option value="name">Trier par nom</option>
              <option value="status">Trier par statut</option>
              <option value="cpu">Trier par CPU</option>
              <option value="apt">Trier par APT en attente</option>
              <option value="last_seen">Trier par dernière activité</option>
            </select>
          </div>
          <div class="col-12 col-md-4 col-lg-2">
            <button class="btn btn-outline-secondary w-100" @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'">
              {{ sortDir === 'asc' ? 'Asc' : 'Desc' }}
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
              <th style="width: 1%"></th>
              <th>Nom</th>
              <th>État</th>
              <th>IP / OS</th>
              <th title="Version de l'agent">Agent</th>
              <th>CPU</th>
              <th>RAM</th>
              <th>Disque</th>
              <th title="Paquets APT en attente">APT</th>
              <th>Uptime</th>
              <th>Dernière activité</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="host in sortedHosts" :key="host.id">
              <td>
                <label class="form-check">
                  <input class="form-check-input" type="checkbox" :value="host.id" v-model="selectedHostIds" />
                  <span class="form-check-label"></span>
                </label>
              </td>
              <td>
                <router-link :to="`/hosts/${host.id}`" class="fw-semibold text-decoration-none">
                  {{ host.name || host.hostname || 'Sans nom' }}
                </router-link>
                <div class="text-secondary small">{{ host.hostname || 'Non connecté' }}</div>
              </td>
              <td>
                <span :class="hostStatusClass(host.status)">{{ formatHostStatus(host.status) }}</span>
              </td>
              <td>
                <div class="text-body">{{ host.ip_address }}</div>
                <div class="text-secondary small">{{ host.os || 'N/A' }}</div>
              </td>
              <td>
                <span v-if="host.agent_version" :class="isAgentUpToDate(host.agent_version) ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
                  v{{ host.agent_version }}
                </span>
                <span v-else class="text-secondary small">-</span>
              </td>
              <td>
                <span :class="cpuColor(effectiveMetrics(host.id).cpu)"
                      :title="effectiveMetrics(host.id).source === 'proxmox' ? 'Source : Proxmox' : ''">
                  {{ effectiveMetrics(host.id).cpu != null ? effectiveMetrics(host.id).cpu.toFixed(1) + '%' : '-' }}
                  <span v-if="effectiveMetrics(host.id).source === 'proxmox'" class="text-orange ms-1" style="font-size:.65em" title="Métriques Proxmox">⬡</span>
                </span>
              </td>
              <td>
                <span :class="memColor(effectiveMetrics(host.id).memPct)"
                      :title="effectiveMetrics(host.id).source === 'proxmox' ? 'Source : Proxmox' : ''">
                  {{ effectiveMetrics(host.id).memPct != null ? effectiveMetrics(host.id).memPct.toFixed(1) + '%' : '-' }}
                  <span v-if="effectiveMetrics(host.id).source === 'proxmox'" class="text-orange ms-1" style="font-size:.65em" title="Métriques Proxmox">⬡</span>
                </span>
              </td>
              <td>
                <span :class="diskColor(diskUsage[host.id])">
                  {{ diskUsage[host.id] != null ? diskUsage[host.id].toFixed(1) + '%' : '-' }}
                </span>
              </td>
              <td>
                <span v-if="aptPendingHosts[host.id]" class="badge bg-yellow-lt text-yellow">{{ aptPendingHosts[host.id] }}</span>
                <span v-else class="text-secondary">—</span>
              </td>
              <td>{{ hostMetrics[host.id] ? formatUptime(hostMetrics[host.id].uptime) : '-' }}</td>
              <td><RelativeTime :date="host.last_seen" /></td>
            </tr>
            <tr v-if="!loading && hosts.length > 0 && sortedHosts.length === 0">
              <td colspan="11" class="text-center text-secondary py-4">Aucun hôte ne correspond à votre recherche.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="loading" class="text-center py-4">
        <div class="spinner-border" role="status"></div>
      </div>

      <div v-if="!loading && hosts.length === 0" class="text-center py-5 text-secondary">
        <svg xmlns="http://www.w3.org/2000/svg" class="mb-3" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24" style="opacity:.35">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5 12h2m10 0h2M12 5v2m0 10v2M7.05 7.05l1.414 1.414m7.072 7.072 1.414 1.414M7.05 16.95l1.414-1.414m7.072-7.072 1.414-1.414"/>
          <circle cx="12" cy="12" r="4" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"/>
        </svg>
        <div class="fw-medium">Aucun hôte enregistré</div>
        <div class="small mt-1 mb-3 opacity-75">Ajoutez votre premier hôte pour commencer à surveiller votre infrastructure</div>
        <router-link to="/hosts/new" class="btn btn-primary btn-sm">Ajouter un hôte</router-link>
      </div>
    </div>

    <!-- ─── Versions Docker (collapsible) ───────────────────────────────────── -->
    <div class="card">
      <div
        class="card-header"
        style="cursor:pointer"
        @click="showDockerVersions = !showDockerVersions"
      >
        <h3 class="card-title d-flex align-items-center gap-2">
          Versions &amp; Mises à jour Docker
          <span v-if="outdatedDockerImages > 0" class="badge bg-yellow-lt text-yellow">{{ outdatedDockerImages }} en retard</span>
          <svg class="ms-auto" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"
               :style="showDockerVersions ? 'transform:rotate(180deg)' : ''" style="transition:transform .2s;flex-shrink:0">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </h3>
        <div class="card-options text-secondary small">Suivi via <router-link to="/git-webhooks" @click.stop>Git / Automatisation</router-link></div>
      </div>
      <div v-show="showDockerVersions" class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Image</th>
              <th>Hôte</th>
              <th>Conteneurs</th>
              <th>En cours</th>
              <th>Dernière version</th>
              <th>Statut</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in versionComparisons" :key="v.docker_image + v.host_id">
              <td class="fw-semibold">{{ v.docker_image }}</td>
              <td class="text-secondary">{{ v.hostname }}</td>
              <td>
                <span v-if="v.container_count > 0" class="badge bg-azure-lt text-azure" :title="`${v.container_count} conteneur${v.container_count > 1 ? 's' : ''} utilisent cette image`">{{ v.container_count }}</span>
                <span v-else class="text-secondary small">—</span>
              </td>
              <td><code v-if="v.running_version">{{ v.running_version }}</code><span v-else class="text-secondary small">inconnue</span></td>
              <td>
                <a v-if="v.release_url" :href="v.release_url" target="_blank" class="link-primary">{{ v.latest_version }}</a>
                <span v-else>{{ v.latest_version }}</span>
              </td>
              <td>
                <span v-if="v.is_up_to_date" class="badge bg-green-lt text-green">À jour</span>
                <span v-else-if="v.running_version || v.update_confirmed" class="badge bg-yellow-lt text-yellow">Mise à jour disponible</span>
                <span v-else class="badge bg-secondary-lt text-secondary">Version inconnue</span>
              </td>
            </tr>
            <tr v-if="versionComparisons.length === 0">
              <td colspan="6" class="text-center text-secondary py-4">
                Aucun suivi de version configuré. Ajoutez des release trackers dans
                <router-link to="/git-webhooks">Git / Automatisation</router-link>.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, defineAsyncComponent, onMounted } from 'vue'
import RelativeTime from '../components/RelativeTime.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import ProxmoxClusterCard from '../components/ProxmoxClusterCard.vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from '../composables/useWebSocket'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'
import { translateError } from '../utils/translateError'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend)
  return Line
})
dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

// ─── State ────────────────────────────────────────────────────────────────────
const latestAgentVersion = ref('')
const cveSummary = ref(null)
const proxmoxSummary = ref(null)
const proxmoxNodes = ref([])
const proxmoxLinks = ref([]) // confirmed links { host_id, metrics_source, cpu_usage, mem_alloc, mem_usage }

const hosts = ref([])
const hostMetrics = ref({})
const versionComparisons = ref([])
const aptPending = ref(0)
const aptPendingHosts = ref({})
const diskUsage = ref({})
const loading = ref(true)

const searchQuery = ref('')
const statusFilter = ref('all')
const sortKey = ref(localStorage.getItem('dashboard.sortKey') || 'name')
const sortDir = ref(localStorage.getItem('dashboard.sortDir') || 'asc')
watch(sortKey, v => localStorage.setItem('dashboard.sortKey', v))
watch(sortDir, v => localStorage.setItem('dashboard.sortDir', v))

const selectedHostIds = ref([])
const aptLoading = ref('')
const showDockerVersions = ref(false)

const summaryHours = ref(24)
const summaryChartData = ref(null)
const summaryLoading = ref(false)
// 'agents' | 'proxmox' — auto-switches to proxmox on first load if configured
const chartSource = ref('agents')
const chartSources = [
  { key: 'agents', label: 'Agents hôtes' },
  { key: 'proxmox', label: 'Nœuds Proxmox' },
]

const auth = useAuthStore()
const dialog = useConfirmDialog()

// ─── Computed ─────────────────────────────────────────────────────────────────
const hasProxmox = computed(() => (proxmoxSummary.value?.connection_count ?? 0) > 0)
const onlineCount = computed(() => hosts.value.filter(h => h.status === 'online').length)
const offlineCount = computed(() => hosts.value.filter(h => h.status !== 'online').length)
const outdatedDockerImages = computed(() => versionComparisons.value.filter(v => !v.is_up_to_date && (v.running_version || v.update_confirmed)).length)
const outdatedVersions = computed(() => outdatedDockerImages.value + aptPending.value)
const selectedCount = computed(() => selectedHostIds.value.length)
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

const proxmoxStoragePct = computed(() => {
  const s = proxmoxSummary.value
  if (!s || !s.storage_total) return 0
  return (s.storage_used / s.storage_total) * 100
})

// Map host_id → confirmed ProxmoxGuestLink (with live guest metrics)
const proxmoxLinkByHostId = computed(() => {
  const m = {}
  for (const link of proxmoxLinks.value) {
    m[link.host_id] = link
  }
  return m
})

// Returns { cpu: number|null, memPct: number|null } for a host.
// Uses Proxmox guest data when metrics_source is 'proxmox', or 'auto' with a confirmed link.
function effectiveMetrics(hostId) {
  const link = proxmoxLinkByHostId.value[hostId]
  const agent = hostMetrics.value[hostId]

  if (link) {
    const src = link.metrics_source
    const useProxmox = src === 'proxmox' || (src === 'auto' && link.cpu_usage != null)
    if (useProxmox) {
      const cpu = link.cpu_usage != null ? link.cpu_usage * 100 : null
      const memPct = link.mem_alloc > 0 ? (link.mem_usage / link.mem_alloc) * 100 : null
      return { cpu, memPct, source: 'proxmox' }
    }
  }
  return {
    cpu: agent?.cpu_usage_percent ?? null,
    memPct: agent?.memory_percent ?? null,
    source: 'agent',
  }
}

const filteredHosts = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return hosts.value.filter((host) => {
    if (statusFilter.value !== 'all' && host.status !== statusFilter.value) return false
    if (!query) return true
    return [host.name, host.hostname, host.ip_address, host.os]
      .filter(Boolean).join(' ').toLowerCase().includes(query)
  })
})

const sortedHosts = computed(() => {
  const list = [...filteredHosts.value]
  const direction = sortDir.value === 'asc' ? 1 : -1
  const statusOrder = { online: 0, warning: 1, offline: 2 }

  list.sort((a, b) => {
    let aVal, bVal
    switch (sortKey.value) {
      case 'status':
        aVal = statusOrder[a.status] ?? 99
        bVal = statusOrder[b.status] ?? 99
        break
      case 'cpu':
        aVal = effectiveMetrics(a.id).cpu ?? -1
        bVal = effectiveMetrics(b.id).cpu ?? -1
        break
      case 'apt':
        aVal = aptPendingHosts.value[a.id] ?? 0
        bVal = aptPendingHosts.value[b.id] ?? 0
        break
      case 'last_seen':
        aVal = a.last_seen ? new Date(a.last_seen).getTime() : 0
        bVal = b.last_seen ? new Date(b.last_seen).getTime() : 0
        break
      default:
        aVal = (a.name || a.hostname || '').toLowerCase()
        bVal = (b.name || b.hostname || '').toLowerCase()
    }
    if (aVal < bVal) return -1 * direction
    if (aVal > bVal) return 1 * direction
    return 0
  })
  return list
})

// ─── Chart options ────────────────────────────────────────────────────────────
const summaryChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top', labels: { color: '#6b7280', boxWidth: 12, padding: 12 } },
    tooltip: {
      enabled: true,
      mode: 'index',
      intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)',
      titleColor: '#fff',
      bodyColor: '#fff',
      borderColor: '#555',
      borderWidth: 1,
      padding: 10,
      callbacks: {
        label: (ctx) => `${ctx.dataset.label}: ${ctx.parsed.y.toFixed(1)}%`,
      },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 10 } },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280' } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 5 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}))

// ─── WebSocket ────────────────────────────────────────────────────────────────
const proxmoxAutoSwitched = ref(false)

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/dashboard', (payload) => {
  if (payload.type !== 'dashboard') return
  hosts.value = payload.hosts || []
  hostMetrics.value = payload.host_metrics || {}
  versionComparisons.value = payload.version_comparisons || []
  aptPending.value = payload.apt_pending ?? 0
  aptPendingHosts.value = payload.apt_pending_hosts || {}
  diskUsage.value = payload.disk_usage || {}
  proxmoxNodes.value = payload.proxmox_nodes || []
  proxmoxLinks.value = payload.proxmox_links || []
  selectedHostIds.value = selectedHostIds.value.filter(id => hosts.value.some(h => h.id === id))
  loading.value = false

  // Auto-switch chart to Proxmox nodes on first load when Proxmox is configured
  if (!proxmoxAutoSwitched.value && proxmoxNodes.value.length > 0) {
    proxmoxAutoSwitched.value = true
    chartSource.value = 'proxmox'
    fetchSummary()
  }
}, { debounceMs: 200 })

// ─── Chart fetch ──────────────────────────────────────────────────────────────
function bucketMinutesFor(hours) {
  if (hours <= 6) return 1
  if (hours <= 24) return 5
  if (hours <= 168) return 15
  return 60
}

async function fetchSummary() {
  summaryLoading.value = true
  try {
    const bucketMinutes = bucketMinutesFor(summaryHours.value)
    const isProxmox = chartSource.value === 'proxmox'
    const res = isProxmox
      ? await apiClient.getProxmoxNodeMetrics(summaryHours.value, bucketMinutes)
      : await apiClient.getMetricsSummary(summaryHours.value, bucketMinutes)

    const points = Array.isArray(res.data) ? res.data : []
    if (!points.length) { summaryChartData.value = null; return }

    const labels = points.map(p =>
      summaryHours.value >= 24 ? dayjs(p.timestamp).format('DD/MM HH:mm') : dayjs(p.timestamp).format('HH:mm')
    )
    summaryChartData.value = {
      labels,
      datasets: [
        {
          label: 'CPU %',
          data: points.map(p => p.cpu_avg),
          borderColor: '#3b82f6',
          backgroundColor: 'rgba(59,130,246,0.10)',
          fill: true,
        },
        {
          label: 'RAM %',
          data: points.map(p => p.memory_avg),
          borderColor: '#10b981',
          backgroundColor: 'rgba(16,185,129,0.10)',
          fill: true,
        },
      ],
    }
  } catch {
    summaryChartData.value = null
  } finally {
    summaryLoading.value = false
  }
}

function changeSummaryRange(hours) {
  summaryHours.value = hours
  fetchSummary()
}

// ─── Bulk APT ─────────────────────────────────────────────────────────────────
function selectAllFiltered() {
  const ids = sortedHosts.value.map(h => h.id)
  selectedHostIds.value = Array.from(new Set([...selectedHostIds.value, ...ids]))
}

function clearSelection() { selectedHostIds.value = [] }

async function sendBulkApt(command) {
  if (!selectedHostIds.value.length || aptLoading.value) return
  const hostnames = hosts.value
    .filter(h => selectedHostIds.value.includes(h.id))
    .map(h => h.hostname || h.name).join(', ')
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: `Exécuter sur ${selectedHostIds.value.length} hôte(s) :\n${hostnames}`,
    variant: 'warning',
  })
  if (!confirmed) return
  aptLoading.value = command
  try {
    await apiClient.sendAptCommand(selectedHostIds.value, command)
  } catch (e) {
    await dialog.confirm({ title: 'Erreur', message: translateError(e), variant: 'danger' })
  } finally {
    aptLoading.value = ''
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────
function formatUptime(seconds) {
  if (!seconds) return 'N/A'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days}j ${hours}h`
  return `${hours}h ${Math.floor((seconds % 3600) / 60)}m`
}

function cpuColor(pct) {
  if (!pct) return 'text-secondary'
  if (pct > 90) return 'text-red'
  if (pct > 70) return 'text-yellow'
  return 'text-green'
}

function memColor(pct) {
  if (!pct) return 'text-secondary'
  if (pct > 90) return 'text-red'
  if (pct > 75) return 'text-yellow'
  return 'text-green'
}

function diskColor(pct) {
  if (pct == null) return 'text-secondary'
  if (pct > 90) return 'text-red'
  if (pct > 75) return 'text-yellow'
  return 'text-green'
}

function isAgentUpToDate(version) {
  return version && latestAgentVersion.value && version === latestAgentVersion.value
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0, v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return v.toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}

// ─── Init ─────────────────────────────────────────────────────────────────────
async function fetchProxmoxSummary() {
  try {
    const res = await apiClient.getProxmoxSummary()
    proxmoxSummary.value = res.data
  } catch { /* non-critique */ }
}

onMounted(() => {
  loading.value = true
  fetchSummary()
  fetchProxmoxSummary()
  apiClient.getSettings().then(r => {
    latestAgentVersion.value = r.data?.settings?.latestAgentVersion || ''
  }).catch(() => {})
  apiClient.getAptCVESummary().then(r => {
    cveSummary.value = r.data
  }).catch(() => {})
})
</script>
