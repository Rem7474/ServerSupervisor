<template>
  <div class="apt-page">
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>APT</span>
      </div>
      <h2 class="page-title">APT — Mises à jour système</h2>
      <div class="text-secondary">Gérer les mises à jour APT sur tous les hôtes</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <!-- Onglets -->
    <SubNavigation
      v-model="activeTab"
      :tabs="[
        { key: 'hosts', label: 'Hôtes' },
        { key: 'history', label: 'Historique', badge: allHistory.length || undefined }
      ]"
    />

    <!-- Layout partagé pour les deux onglets -->
    <div class="apt-layout">
      <!-- Contenu principal (bascule entre hôtes et historique) -->
      <div class="apt-hosts">
        <!-- === Vue Hôtes === -->
        <template v-if="activeTab === 'hosts'">
          <div class="card mb-3">
            <div class="card-body">
              <div class="row g-3 align-items-center mb-3">
                <div class="col-md-4">
                  <input
                    v-model="hostSearch"
                    type="text"
                    class="form-control"
                    placeholder="Rechercher un hôte..."
                  />
                </div>
                <div class="col-md-4">
                  <div class="btn-group w-100">
                    <button
                      v-for="f in hostFilterOptions"
                      :key="f.value"
                      class="btn btn-sm"
                      :class="hostQuickFilter === f.value ? 'btn-primary' : 'btn-outline-secondary'"
                      @click="hostQuickFilter = f.value"
                    >{{ f.label }}</button>
                  </div>
                </div>
                <div class="col-md-4 d-flex gap-2 ms-auto justify-content-md-end">
                  <select v-model="hostSortKey" class="form-select form-select-sm" style="width: auto;">
                    <option value="name">Trier par nom</option>
                    <option value="pending">Trier par paquets en attente</option>
                    <option value="security">Trier par mises à jour sécurité</option>
                    <option value="cve">Trier par CVE</option>
                  </select>
                  <button class="btn btn-sm btn-outline-secondary" @click="hostSortDir = hostSortDir === 'asc' ? 'desc' : 'asc'" :title="hostSortDir === 'asc' ? 'Croissant' : 'Décroissant'">
                    <svg v-if="hostSortDir === 'asc'" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 8l4-4 4 4M7 4v16M13 16l4 4 4-4M17 20V4"/></svg>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 16l4 4 4-4M7 20V4M13 8l4-4 4 4M17 4v16"/></svg>
                  </button>
                </div>
              </div>
              <div class="d-flex flex-wrap align-items-center gap-3">
                <label class="form-check">
                  <input type="checkbox" class="form-check-input" v-model="selectAll" @change="toggleSelectAll" />
                  <span class="form-check-label">Sélectionner tous les hôtes</span>
                </label>
                <div class="ms-auto d-flex flex-wrap gap-2">
                  <template v-if="canRunApt">
                    <button @click="bulkAptCmd('update')" class="btn btn-outline-secondary" :disabled="selectedHosts.length === 0">
                      apt update ({{ selectedHosts.length }})
                    </button>
                    <button @click="bulkAptCmd('upgrade')" class="btn btn-primary" :disabled="selectedHosts.length === 0">
                      apt upgrade ({{ selectedHosts.length }})
                    </button>
                    <button @click="bulkAptCmd('dist-upgrade')" class="btn btn-outline-danger" :disabled="selectedHosts.length === 0">
                      apt dist-upgrade ({{ selectedHosts.length }})
                    </button>
                  </template>
                  <div v-else class="text-secondary small">Mode lecture seule</div>
                </div>
              </div>
            </div>
          </div>

          <div class="row row-cards">
            <div v-if="filteredHosts.length === 0" class="col-12">
              <div class="card">
                <div class="card-body text-center text-secondary py-4">Aucun hôte ne correspond aux filtres.</div>
              </div>
            </div>
            <div v-for="host in filteredHosts" :key="host.id" class="col-12">
              <div class="card">
                <div class="card-body">
                  <div class="d-flex align-items-center gap-3 mb-3">
                    <label class="form-check">
                      <input type="checkbox" class="form-check-input" :value="host.id" v-model="selectedHosts" />
                      <span class="form-check-label"></span>
                    </label>
                    <div class="flex-fill">
                      <div class="fw-semibold">{{ host.hostname || host.name }}</div>
                      <div class="text-secondary small">{{ host.ip_address }}</div>
                    </div>
                    <span :class="host.status === 'online' ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
                      {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
                    </span>
                    <button v-if="canRunApt" class="btn btn-sm btn-outline-secondary" @click="openScheduleModal(host)" title="Planifier une commande APT">
                      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm me-1" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
                      </svg>
                      Planifier
                    </button>
                  </div>

                  <div v-if="aptStatuses[host.id]" class="row row-cards mb-3">
                    <div class="col-sm-6 col-md-3">
                      <div class="card card-sm h-100">
                        <div class="card-body text-center">
                          <div class="h2 mb-0" :class="aptStatuses[host.id].pending_packages > 0 ? 'text-yellow' : 'text-green'">
                            {{ aptStatuses[host.id].pending_packages }}
                          </div>
                          <div class="text-secondary small">En attente</div>
                        </div>
                      </div>
                    </div>
                    <div class="col-sm-6 col-md-3">
                      <div class="card card-sm h-100">
                        <div class="card-body text-center">
                          <div class="h2 mb-0 text-red">{{ aptStatuses[host.id].security_updates }}</div>
                          <div class="text-secondary small">Sécurité</div>
                        </div>
                      </div>
                    </div>
                    <div class="col-sm-6 col-md-3">
                      <div class="card card-sm h-100">
                        <div class="card-body text-center">
                          <div class="fw-semibold">{{ formatDate(aptStatuses[host.id].last_update) }}</div>
                          <div class="text-secondary small">Dernier update</div>
                        </div>
                      </div>
                    </div>
                    <div class="col-sm-6 col-md-3">
                      <div class="card card-sm h-100">
                        <div class="card-body text-center">
                          <div class="fw-semibold">{{ formatDate(aptStatuses[host.id].last_upgrade) }}</div>
                          <div class="text-secondary small">Dernière mise à jour</div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- CVE Information -->
                  <div v-if="aptStatuses[host.id]?.cve_list" class="mb-3">
                    <CVEList
                      :cveList="aptStatuses[host.id].cve_list"
                      :showMaxSeverity="true"
                      :alwaysExpanded="false"
                      :limit="5"
                    />
                  </div>

                  <!-- Package List -->
                  <div v-if="getPackages(aptStatuses[host.id]).length > 0" class="mb-3">
                    <div class="d-flex align-items-center mb-2">
                      <span class="fw-semibold me-2">Paquets en attente :</span>
                      <span class="badge bg-yellow-lt text-yellow">{{ getPackages(aptStatuses[host.id]).length }}</span>
                    </div>
                    <div v-if="packagesExpanded[host.id]" class="d-flex flex-wrap gap-1 mb-1">
                      <span
                        v-for="pkg in (packagesShowAll[host.id] ? getPackages(aptStatuses[host.id]) : getPackages(aptStatuses[host.id]).slice(0, 12))"
                        :key="pkg"
                        class="badge bg-blue-lt text-blue"
                        style="font-family: monospace; font-size: 0.72rem;"
                      >{{ pkg }}</span>
                      <button
                        v-if="getPackages(aptStatuses[host.id]).length > 12 && !packagesShowAll[host.id]"
                        @click="packagesShowAll[host.id] = true"
                        class="btn btn-sm btn-link p-0 ms-1"
                      >+{{ getPackages(aptStatuses[host.id]).length - 12 }} plus...</button>
                    </div>
                    <button
                      @click="packagesExpanded[host.id] = !packagesExpanded[host.id]"
                      class="btn btn-sm btn-link p-0"
                    >
                      {{ packagesExpanded[host.id]
                        ? 'Masquer'
                        : `Afficher ${getPackages(aptStatuses[host.id]).length} paquet${getPackages(aptStatuses[host.id]).length > 1 ? 's' : ''}` }}
                    </button>
                  </div>

                  <div v-if="aptHistories[host.id]?.length">
                    <button @click="toggleHistory(host.id)" class="btn btn-link p-0">
                      {{ expandedHistories[host.id] ? 'Masquer' : 'Voir' }} l'historique ({{ aptHistories[host.id].length }})
                    </button>
                    <div v-if="expandedHistories[host.id]" class="mt-3">
                      <div v-for="cmd in aptHistories[host.id]" :key="cmd.id" class="border rounded p-3 mb-2">
                        <div class="d-flex align-items-center justify-content-between">
                          <div class="fw-semibold">apt {{ cmd.action }}</div>
                          <div class="d-flex align-items-center gap-2">
                            <span :class="statusClass(cmd.status)">{{ cmd.status }}</span>
                            <span class="text-secondary small">{{ formatDuration(cmd.started_at, cmd.ended_at) }}</span>
                            <button @click="watchCommand(cmd, host)" class="btn btn-sm btn-ghost-secondary" title="Voir les logs">
                              <svg class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" xmlns="http://www.w3.org/2000/svg"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                            </button>
                          </div>
                        </div>
                        <div class="text-secondary small mt-1">
                          {{ formatDate(cmd.created_at) }}
                          <span v-if="cmd.triggered_by">• par {{ cmd.triggered_by }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- === Vue Historique global === -->
        <div v-else class="card">
          <div class="card-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
            <h3 class="card-title mb-0">Historique des mises à jour</h3>
            <div class="d-flex flex-wrap gap-2">
              <!-- Filtre hôte -->
              <select v-model="historyHostFilter" class="form-select form-select-sm" style="min-width: 160px;" @change="resetHistoryPage">
                <option value="all">Tous les hôtes</option>
                <option v-for="host in hosts" :key="host.id" :value="host.id">
                  {{ host.hostname || host.name }}
                </option>
              </select>
              <!-- Filtre période -->
              <div class="btn-group btn-group-sm">
                <button
                  v-for="p in periodOptions"
                  :key="p.value"
                  class="btn"
                  :class="historyPeriod === p.value ? 'btn-primary' : 'btn-outline-secondary'"
                  @click="historyPeriod = p.value; resetHistoryPage()"
                >
                  {{ p.label }}
                </button>
              </div>
            </div>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Hôte</th>
                  <th>Commande</th>
                  <th>Utilisateur</th>
                  <th>Statut</th>
                  <th>Durée</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="filteredHistory.length === 0">
                  <td colspan="7" class="text-center text-secondary py-4">Aucun historique pour cette période</td>
                </tr>
                <tr v-for="cmd in pagedHistory" :key="cmd.id">
                  <td class="text-secondary small">{{ formatDateExact(cmd.created_at) }}</td>
                  <td>
                    <div class="fw-semibold">{{ cmd.hostName }}</div>
                  </td>
                  <td><code>apt {{ cmd.action }}</code></td>
                  <td class="text-secondary">{{ cmd.triggered_by || '—' }}</td>
                  <td><span :class="statusClass(cmd.status)">{{ cmd.status }}</span></td>
                  <td class="text-secondary small">{{ formatDuration(cmd.started_at, cmd.ended_at) }}</td>
                  <td>
                    <button
                      @click="watchCommand(cmd, { hostname: cmd.hostName, id: cmd.hostId })"
                      class="btn btn-sm btn-outline-primary"
                    >
                      Logs
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="historyTotalPages > 1" class="card-footer d-flex align-items-center justify-content-between">
            <div class="text-secondary small">
              {{ filteredHistory.length }} entrée{{ filteredHistory.length > 1 ? 's' : '' }} —
              page {{ historyPage }} / {{ historyTotalPages }}
            </div>
            <PaginationNav
              :current-page="historyPage"
              :total-pages="historyTotalPages"
              @select="setHistoryPage"
            />
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="liveCommand"
        :show="showConsole"
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="apt-console"
        @open="showConsole = true"
        @close="closeLiveConsole"
      />
    </div>

    <!-- Schedule APT modal -->
    <div v-if="scheduleModal.open" class="modal modal-blur show d-block" tabindex="-1" style="background:rgba(0,0,0,.5);z-index:1050" @click.self="scheduleModal.open = false">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">Planifier une commande APT</h5>
              <div class="text-muted small mt-1">{{ scheduleModal.hostname }}</div>
            </div>
            <button type="button" class="btn-close" @click="scheduleModal.open = false"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">Nom de la tâche</label>
              <input v-model="scheduleModal.name" type="text" class="form-control" placeholder="ex: apt upgrade hebdo" />
            </div>
            <div class="mb-3">
              <label class="form-label">Commande</label>
              <select v-model="scheduleModal.action" class="form-select">
                <option value="update">apt update</option>
                <option value="upgrade">apt upgrade</option>
                <option value="dist-upgrade">apt dist-upgrade</option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-check form-switch">
                <input v-model="scheduleModal.manualOnly" type="checkbox" class="form-check-input" />
                <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
              </label>
            </div>
            <div v-if="!scheduleModal.manualOnly" class="mb-3">
              <CronBuilder v-model="scheduleModal.cron_expression" />
            </div>
            <div v-if="!scheduleModal.manualOnly" class="form-check form-switch mb-2">
              <input v-model="scheduleModal.enabled" type="checkbox" class="form-check-input" id="schedEnabled" />
              <label class="form-check-label" for="schedEnabled">Activée</label>
            </div>
            <div v-if="scheduleModal.error" class="alert alert-danger py-2">{{ scheduleModal.error }}</div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="scheduleModal.open = false">Annuler</button>
            <button class="btn btn-primary" :disabled="scheduleModal.saving" @click="saveSchedule">
              <span v-if="scheduleModal.saving" class="spinner-border spinner-border-sm me-1"></span>
              Créer la tâche
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="bulkActionFeedback" class="position-fixed bottom-0 end-0 p-3" style="z-index: 1100;">
      <div class="toast show align-items-center border-0" :class="bulkActionFeedback.variantClass">
        <div class="d-flex">
          <div class="toast-body">
            <strong>apt {{ bulkActionFeedback.command }}</strong> {{ bulkActionFeedback.message }}
            <div v-if="bulkActionFeedback.details" class="small mt-1">{{ bulkActionFeedback.details }}</div>
          </div>
          <button type="button" class="btn-close me-2 m-auto" :class="bulkActionFeedback.closeClass" @click="bulkActionFeedback = null"></button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onUnmounted, computed } from 'vue'
import CVEList from '../components/CVEList.vue'
import apiClient, { getApiErrorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from '../composables/useWebSocket'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useDateFormatter } from '../composables/useDateFormatter'
import { usePagination } from '../composables/usePagination'
import { useStatusBadge } from '../composables/useStatusBadge'
import { useToast } from '../composables/useToast'
import { useCommandStream } from '../composables/useCommandStream'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import PaginationNav from '../components/PaginationNav.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import SubNavigation from '../components/SubNavigation.vue'
import CronBuilder from '../components/CronBuilder.vue'

const { dayjs, formatRelativeDate, formatExactDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

// ── Tab ──────────────────────────────────────────────────────────────────────
const activeTab = ref('hosts')

// ── Hosts / APT state ────────────────────────────────────────────────────────
const hosts = ref([])
const selectedHosts = ref([])
const selectAll = ref(false)
const aptStatuses = ref({})
const aptHistories = ref({})
const expandedHistories = ref({})
const packagesExpanded = ref({})
const packagesShowAll = ref({})
const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

// ── Schedule modal ────────────────────────────────────────────────────────────
import { MANUAL_SENTINEL } from '../utils/cron'

const scheduleModal = ref({
  open: false, hostId: '', hostname: '', name: '',
  action: 'update', cron_expression: '0 3 * * 0',
  manualOnly: false, enabled: true, saving: false, error: '',
})

function openScheduleModal(host) {
  scheduleModal.value = {
    open: true,
    hostId: host.id,
    hostname: host.hostname || host.name,
    name: '',
    action: 'update',
    cron_expression: '0 3 * * 0',
    manualOnly: false,
    enabled: true,
    saving: false,
    error: '',
  }
}

async function saveSchedule() {
  scheduleModal.value.error = ''
  scheduleModal.value.saving = true
  const cronExpr = scheduleModal.value.manualOnly ? MANUAL_SENTINEL : scheduleModal.value.cron_expression
  try {
    await apiClient.createScheduledTask(scheduleModal.value.hostId, {
      name: scheduleModal.value.name || `apt ${scheduleModal.value.action}`,
      module: 'apt',
      action: scheduleModal.value.action,
      target: '',
      payload: '{}',
      cron_expression: cronExpr,
      enabled: scheduleModal.value.manualOnly ? false : scheduleModal.value.enabled,
    })
    scheduleModal.value.open = false
  } catch (e) {
    scheduleModal.value.error = e.response?.data?.error || 'Erreur lors de la création'
  } finally {
    scheduleModal.value.saving = false
  }
}

// ── Console ───────────────────────────────────────────────────────────────────
const showConsole = ref(false)
const liveCommand = ref(null)
const { value: bulkActionFeedback, showToast: showBulkActionFeedback } = useToast(null)
const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })

// ── Hosts filters / sort ─────────────────────────────────────────────────────
const hostSearch = ref('')
const hostQuickFilter = ref('all')
const hostSortKey = ref('name')
const hostSortDir = ref('asc')

const hostFilterOptions = [
  { value: 'all', label: 'Tous' },
  { value: 'critical', label: 'CVE critiques' },
  { value: 'security', label: 'Sécu > 0' },
]

const filteredHosts = computed(() => {
  let list = [...hosts.value]

  // Search by name
  const q = hostSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter(h => (h.hostname || h.name || '').toLowerCase().includes(q) || (h.ip_address || '').includes(q))
  }

  // Quick filter
  if (hostQuickFilter.value === 'critical') {
    list = list.filter(h => {
      const cves = aptStatuses.value[h.id]?.cve_list
      return Array.isArray(cves) && cves.some(c => c.severity === 'CRITICAL')
    })
  } else if (hostQuickFilter.value === 'security') {
    list = list.filter(h => (aptStatuses.value[h.id]?.security_updates || 0) > 0)
  }

  // Sort
  list.sort((a, b) => {
    let va, vb
    if (hostSortKey.value === 'pending') {
      va = aptStatuses.value[a.id]?.pending_packages || 0
      vb = aptStatuses.value[b.id]?.pending_packages || 0
    } else if (hostSortKey.value === 'security') {
      va = aptStatuses.value[a.id]?.security_updates || 0
      vb = aptStatuses.value[b.id]?.security_updates || 0
    } else if (hostSortKey.value === 'cve') {
      va = (aptStatuses.value[a.id]?.cve_list || []).length
      vb = (aptStatuses.value[b.id]?.cve_list || []).length
    } else {
      va = (a.hostname || a.name || '').toLowerCase()
      vb = (b.hostname || b.name || '').toLowerCase()
    }
    if (va < vb) return hostSortDir.value === 'asc' ? -1 : 1
    if (va > vb) return hostSortDir.value === 'asc' ? 1 : -1
    return 0
  })

  return list
})

// ── Historique filters ────────────────────────────────────────────────────────
const historyHostFilter = ref('all')
const historyPeriod = ref('7d')

const periodOptions = [
  { label: '7j',  value: '7d'  },
  { label: '30j', value: '30d' },
  { label: '90j', value: '90d' },
  { label: 'Tout', value: 'all' },
]

// Flatten all histories into a single array, enriched with host info
const allHistory = computed(() => {
  return Object.entries(aptHistories.value).flatMap(([hostId, cmds]) => {
    const host = hosts.value.find(h => h.id === hostId)
    const hostName = host?.hostname || host?.name || hostId
    return (cmds || []).map(cmd => ({ ...cmd, hostId, hostName }))
  }).sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
})

const filteredHistory = computed(() => {
  let list = allHistory.value

  // Filter by host
  if (historyHostFilter.value !== 'all') {
    list = list.filter(cmd => cmd.hostId === historyHostFilter.value)
  }

  // Filter by period
  if (historyPeriod.value !== 'all') {
    const days = parseInt(historyPeriod.value)
    const cutoff = dayjs().subtract(days, 'day')
    list = list.filter(cmd => dayjs(cmd.created_at).isAfter(cutoff))
  }

  return list
})

const HISTORY_PAGE_SIZE = 25
const {
  currentPage: historyPage,
  totalPages: historyTotalPages,
  pagedItems: pagedHistory,
  resetPage: resetHistoryPage,
  setPage: setHistoryPage,
} = usePagination({ items: filteredHistory, pageSize: HISTORY_PAGE_SIZE })

// Reset to page 1 when filters change
function prevHistoryPage() {
  setHistoryPage(historyPage.value - 1)
}

function nextHistoryPage() {
  setHistoryPage(historyPage.value + 1)
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function toggleSelectAll() {
  if (selectAll.value) {
    selectedHosts.value = hosts.value.map(h => h.id)
  } else {
    selectedHosts.value = []
  }
}

function toggleHistory(hostId) {
  expandedHistories.value[hostId] = !expandedHistories.value[hostId]
}

function getPackages(aptStatus) {
  if (!aptStatus?.package_list) return []
  try {
    const parsed = typeof aptStatus.package_list === 'string'
      ? JSON.parse(aptStatus.package_list)
      : aptStatus.package_list
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function watchCommand(cmd, host) {
  showConsole.value = true
  liveCommand.value = {
    id: cmd.id,
    hostId: host?.id || cmd.hostId || cmd.host_id || null,
    host_name: host?.hostname || host?.name || '—',
    module: 'apt',
    action: cmd.action || cmd.command || '—',
    target: '',
    status: cmd.status,
    output: cmd.output || '',
  }
  connectStreamWebSocket(cmd.id)
}

function closeStreamSocket() {
  closeStream()
}

function closeLiveConsole() {
  closeStreamSocket()
  liveCommand.value = null
  showConsole.value = false
}

function upsertAptHistory(hostId, nextCommand) {
  if (!hostId || !nextCommand?.id) return

  const currentHistory = Array.isArray(aptHistories.value[hostId]) ? [...aptHistories.value[hostId]] : []
  const currentIndex = currentHistory.findIndex(cmd => cmd.id === nextCommand.id)

  if (currentIndex >= 0) {
    currentHistory[currentIndex] = {
      ...currentHistory[currentIndex],
      ...nextCommand,
    }
  } else {
    currentHistory.unshift(nextCommand)
  }

  currentHistory.sort((left, right) => new Date(right.created_at || 0) - new Date(left.created_at || 0))
  aptHistories.value = {
    ...aptHistories.value,
    [hostId]: currentHistory,
  }
}

function syncLiveCommand(commandId, patch) {
  if (!liveCommand.value || liveCommand.value.id !== commandId) return
  liveCommand.value = {
    ...liveCommand.value,
    ...patch,
  }
}

function syncAptHistoryCommand(commandId, patch) {
  const hostId = liveCommand.value?.id === commandId ? liveCommand.value.hostId : null
  if (!hostId) return
  upsertAptHistory(hostId, {
    id: commandId,
    action: liveCommand.value?.action || patch.action,
    output: liveCommand.value?.output || '',
    ...patch,
  })
}

function buildBulkActionFeedback(command, launchedHosts, failedHosts) {
  const hasFailures = failedHosts.length > 0
  const launchedLabel = launchedHosts.length === 1
    ? `lancée sur ${launchedHosts[0]}`
    : `lancée sur ${launchedHosts.length} hôtes`
  const failedLabel = hasFailures
    ? `Échec d'envoi sur ${failedHosts.join(', ')}.`
    : 'Suivi disponible dans l’historique par hôte.'

  return {
    command,
    message: launchedHosts.length > 0 ? `commande ${launchedLabel}.` : 'aucune commande n’a été lancée.',
    details: failedLabel,
    variantClass: hasFailures ? 'text-bg-warning' : 'text-bg-success',
    closeClass: hasFailures ? '' : 'btn-close-white',
  }
}

function connectStreamWebSocket(commandId) {
  closeStreamSocket()
  openCommandStream(commandId, {
    closeOnTerminalStatus: true,
    onInit: (payload) => {
      syncLiveCommand(commandId, { status: payload.status, output: payload.output || '' })
      syncAptHistoryCommand(commandId, { status: payload.status })
    },
    onChunk: (payload) => {
      const nextOutput = `${liveCommand.value?.output || ''}${payload.chunk || ''}`
      syncLiveCommand(commandId, { output: nextOutput })
    },
    onStatus: (payload) => {
      const patch = { status: payload.status }
      if (typeof payload.output === 'string') patch.output = payload.output
      syncLiveCommand(commandId, patch)
      syncAptHistoryCommand(commandId, patch)
    },
  })
}

async function bulkAptCmd(command) {
  const hostnames = hosts.value.filter(h => selectedHosts.value.includes(h.id)).map(h => h.hostname).join(', ')

  const isDangerous = command === 'dist-upgrade'
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: isDangerous
      ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${hostnames} ?`
      : `Exécuter sur : ${hostnames} ?`,
    variant: isDangerous ? 'danger' : 'warning'
  })

  if (!confirmed) return

  try {
    const response = await apiClient.sendAptCommand(selectedHosts.value, command)
    const commandResults = Array.isArray(response.data?.commands) ? response.data.commands : []
    const hostNameById = new Map(hosts.value.map(host => [host.id, host.hostname || host.name || host.id]))
    const launchedCommands = commandResults.filter(item => item.command_id)
    const failedCommands = commandResults.filter(item => item.error)
    const createdAt = new Date().toISOString()

    launchedCommands.forEach((item) => {
      upsertAptHistory(item.host_id, {
        id: item.command_id,
        action: command,
        status: item.status || 'pending',
        output: '',
        created_at: createdAt,
        started_at: null,
        ended_at: null,
        triggered_by: auth.username || '',
      })
    })

    if (selectedHosts.value.length === 1 && launchedCommands.length > 0) {
      const launchedCommand = launchedCommands[0]
      const host = hosts.value.find(h => h.id === launchedCommand.host_id)
      if (host) {
        watchCommand({ id: launchedCommand.command_id, action: command, status: launchedCommand.status || 'pending', output: '' }, host)
      }
    }

    if (selectedHosts.value.length > 1 || failedCommands.length > 0) {
      showBulkActionFeedback(
        buildBulkActionFeedback(
          command,
          launchedCommands.map(item => hostNameById.get(item.host_id) || item.host_id),
          failedCommands.map(item => hostNameById.get(item.host_id) || item.host_id),
        ),
        7000,
      )
    }
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: getApiErrorMessage(e),
      variant: 'danger'
    })
  }
}

function formatDate(date) {
  return formatRelativeDate(date)
}

function formatDateExact(date) {
  return formatExactDate(date, '—')
}

function formatDuration(startedAt, endedAt) {
  if (!startedAt || !endedAt) return '—'
  const start = dayjs(startedAt)
  const end = dayjs(endedAt)
  if (!start.isValid() || !end.isValid()) return '—'
  const totalSeconds = end.diff(start, 'second')
  if (totalSeconds < 0) return '—'
  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`
}

function statusClass(status) {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/apt', (payload) => {
  if (payload.type !== 'apt') return
  hosts.value = payload.hosts || []
  aptStatuses.value = payload.apt_statuses || {}
  aptHistories.value = payload.apt_histories || {}
})

onUnmounted(() => {
  closeStreamSocket()
})
</script>

<style scoped>
.apt-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 120px);
}

.apt-layout {
  display: flex;
  flex: 1;
  gap: 1rem;
  overflow: hidden;
  min-height: 0;
}

.apt-hosts {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-width: 0;
}

:deep(.apt-console) {
  width: 40%;
  min-width: 380px;
  display: flex;
  flex-direction: column;
}

@media (max-width: 991px) {
  .apt-page {
    height: auto;
  }

  .apt-layout {
    flex-direction: column;
    overflow: visible;
    height: auto;
  }

  .apt-hosts {
    overflow-y: visible;
  }

  :deep(.apt-console) {
    width: 100%;
    min-width: 0;
    max-height: 60vh;
  }
}
</style>
