<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>APT</span>
      </div>
      <div class="d-flex align-items-center justify-content-between flex-wrap gap-2">
        <h2 class="page-title">
          APT — Mises à jour système
        </h2>
        <router-link
          to="/audit?module=apt"
          class="btn btn-sm btn-outline-secondary"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="icon icon-sm me-1"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          ><path
            stroke="none"
            d="M0 0h24v24H0z"
            fill="none"
          /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
          Historique des commandes
        </router-link>
      </div>
      <div class="text-secondary">
        Gérer les mises à jour APT sur tous les hôtes
      </div>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      :data-stale-alert="dataStaleAlert"
      @reconnect="reconnect"
      @dismiss-stale-alert="dataStaleAlert = false"
    />

    <DataToolbar
      searchable
      :search="hostSearch"
      search-placeholder="Rechercher un hôte..."
      @update:search="hostSearch = $event"
    >
      <template #right>
        <div class="btn-group">
          <button
            v-for="f in hostFilterOptions"
            :key="f.value"
            class="btn btn-sm"
            :class="hostQuickFilter === f.value ? 'btn-primary' : 'btn-outline-secondary'"
            @click="hostQuickFilter = f.value"
          >
            {{ f.label }}
          </button>
        </div>
        <select
          v-model="hostSortKey"
          class="form-select form-select-sm sort-select"
        >
          <option value="name">
            Trier par nom
          </option>
          <option value="pending">
            Trier par paquets en attente
          </option>
          <option value="security">
            Trier par mises à jour sécurité
          </option>
          <option value="cve">
            Trier par CVE
          </option>
        </select>
        <button
          class="btn btn-sm btn-outline-secondary"
          :title="hostSortDir === 'asc' ? 'Croissant' : 'Décroissant'"
          @click="hostSortDir = hostSortDir === 'asc' ? 'desc' : 'asc'"
        >
          <svg
            v-if="hostSortDir === 'asc'"
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          ><path d="M3 8l4-4 4 4M7 4v16M13 16l4 4 4-4M17 20V4" /></svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          ><path d="M3 16l4 4 4-4M7 20V4M13 8l4-4 4 4M17 4v16" /></svg>
        </button>
      </template>
      <template #bottom>
        <div class="d-flex flex-wrap align-items-center gap-3">
          <label class="form-check">
            <input
              v-model="selectAll"
              type="checkbox"
              class="form-check-input"
              @change="toggleSelectAll"
            >
            <span class="form-check-label">Sélectionner tous les hôtes</span>
          </label>
          <div class="ms-auto d-flex flex-wrap gap-2">
            <template v-if="canRunApt && selectedHosts.length > 0">
              <button
                class="btn btn-outline-secondary btn-sm"
                :disabled="!!aptBulkLoading"
                @click="bulkAptCmd('update')"
              >
                <span
                  v-if="aptBulkLoading === 'update'"
                  class="spinner-border spinner-border-sm me-1"
                  role="status"
                />
                apt update ({{ selectedHosts.length }})
              </button>
              <button
                class="btn btn-primary btn-sm"
                :disabled="!!aptBulkLoading"
                @click="bulkAptCmd('upgrade')"
              >
                <span
                  v-if="aptBulkLoading === 'upgrade'"
                  class="spinner-border spinner-border-sm me-1"
                  role="status"
                />
                apt upgrade ({{ selectedHosts.length }})
              </button>
              <button
                class="btn btn-outline-danger btn-sm"
                :disabled="!!aptBulkLoading"
                @click="bulkAptCmd('dist-upgrade')"
              >
                <span
                  v-if="aptBulkLoading === 'dist-upgrade'"
                  class="spinner-border spinner-border-sm me-1"
                  role="status"
                />
                apt dist-upgrade ({{ selectedHosts.length }})
              </button>
            </template>
            <span
              v-else-if="selectedHosts.length === 0"
              class="text-secondary small align-self-center"
            >Sélectionner des hôtes pour les actions groupées</span>
          </div>
        </div>
      </template>
    </DataToolbar>

    <div class="side-layout">
      <div class="side-main">
        <div class="row row-cards">
          <div
            v-if="filteredHosts.length === 0"
            class="col-12"
          >
            <div class="card">
              <div class="card-body text-center text-secondary py-4">
                <template v-if="wsStatus === 'connecting' || wsStatus === 'reconnecting'">
                  <span
                    class="spinner-border spinner-border-sm me-2"
                    role="status"
                    aria-hidden="true"
                  />
                  Chargement des hôtes...
                </template>
                <template v-else>
                  Aucun hôte ne correspond aux filtres.
                </template>
              </div>
            </div>
          </div>

          <div
            v-for="host in filteredHosts"
            :key="host.id"
            class="col-12"
          >
            <div class="card">
              <!-- Header : identité + statut + actions par hôte -->
              <div class="card-header">
                <div class="d-flex align-items-center gap-3 flex-wrap w-100">
                  <label class="form-check m-0">
                    <input
                      v-model="selectedHosts"
                      type="checkbox"
                      class="form-check-input"
                      :value="host.id"
                    >
                    <span class="form-check-label" />
                  </label>
                  <div class="flex-fill min-w-0">
                    <div class="d-flex align-items-center gap-2 flex-wrap">
                      <router-link
                        :to="`/hosts/${host.id}`"
                        class="fw-semibold text-reset text-decoration-none"
                      >
                        {{ host.name || host.hostname }}
                      </router-link>
                      <span
                        v-if="host.name && host.hostname && host.name !== host.hostname"
                        class="text-secondary small"
                      >
                        {{ host.hostname }}
                      </span>
                      <span class="text-muted small">{{ host.ip_address }}</span>
                    </div>
                  </div>
                  <span :class="host.status === 'online' ? 'status status-lime' : 'status status-red'">
                    <span :class="['status-dot', host.status === 'online' ? 'status-dot-animated' : '']" />
                    <span>{{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}</span>
                  </span>
                  <div
                    v-if="canRunApt"
                    class="d-flex gap-1 flex-shrink-0"
                  >
                    <div class="btn-group btn-group-sm">
                      <button
                        class="btn btn-outline-secondary"
                        :disabled="isHostCmdLoading(host.id)"
                        @click="runAptCmdForHost(host, 'update')"
                      >
                        <span
                          v-if="hostCmdLoading[host.id] === 'update'"
                          class="spinner-border spinner-border-sm me-1"
                          role="status"
                        />
                        update
                      </button>
                      <button
                        class="btn btn-primary"
                        :disabled="isHostCmdLoading(host.id)"
                        @click="runAptCmdForHost(host, 'upgrade')"
                      >
                        <span
                          v-if="hostCmdLoading[host.id] === 'upgrade'"
                          class="spinner-border spinner-border-sm me-1"
                          role="status"
                        />
                        upgrade
                      </button>
                      <button
                        class="btn btn-outline-danger"
                        :disabled="isHostCmdLoading(host.id)"
                        @click="runAptCmdForHost(host, 'dist-upgrade')"
                      >
                        <span
                          v-if="hostCmdLoading[host.id] === 'dist-upgrade'"
                          class="spinner-border spinner-border-sm me-1"
                          role="status"
                        />
                        dist-upgrade
                      </button>
                    </div>
                    <button
                      class="btn btn-sm btn-outline-secondary"
                      title="Planifier une commande APT"
                      @click="openScheduleModal(host)"
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="icon icon-sm"
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      >
                        <rect
                          x="3"
                          y="4"
                          width="18"
                          height="18"
                          rx="2"
                          ry="2"
                        /><line
                          x1="16"
                          y1="2"
                          x2="16"
                          y2="6"
                        /><line
                          x1="8"
                          y1="2"
                          x2="8"
                          y2="6"
                        /><line
                          x1="3"
                          y1="10"
                          x2="21"
                          y2="10"
                        />
                      </svg>
                    </button>
                  </div>
                  <span
                    v-else
                    class="text-secondary small flex-shrink-0"
                  >Mode lecture seule</span>
                </div>
              </div>

              <!-- Corps : KPI + CVE + paquets + historique -->
              <div class="card-body">
                <!-- Pas de données -->
                <div
                  v-if="!aptStatuses[host.id]"
                  class="text-secondary small"
                >
                  Données APT non disponibles — lancez <strong>apt update</strong> pour initialiser.
                </div>

                <template v-else>
                  <!-- KPI stats -->
                  <div class="row g-2 mb-3">
                    <div class="col-4">
                      <div
                        class="text-center p-2 rounded"
                        :class="aptStatuses[host.id].pending_packages > 0
                          ? 'bg-yellow-lt'
                          : 'bg-green-lt'"
                      >
                        <div
                          class="fs-3 fw-bold lh-1 mb-1"
                          :class="aptStatuses[host.id].pending_packages > 0 ? 'text-yellow' : 'text-green'"
                        >
                          {{ aptStatuses[host.id].pending_packages }}
                        </div>
                        <div class="text-secondary small">
                          en attente
                        </div>
                      </div>
                    </div>
                    <div class="col-4">
                      <div
                        class="text-center p-2 rounded"
                        :class="aptStatuses[host.id].security_updates > 0
                          ? 'bg-red-lt'
                          : 'bg-secondary-lt'"
                      >
                        <div
                          class="fs-3 fw-bold lh-1 mb-1"
                          :class="aptStatuses[host.id].security_updates > 0 ? 'text-red' : 'text-secondary'"
                        >
                          {{ aptStatuses[host.id].security_updates }}
                        </div>
                        <div class="text-secondary small">
                          sécurité
                        </div>
                      </div>
                    </div>
                    <div class="col-4">
                      <div class="text-center p-2 rounded bg-secondary-lt">
                        <div class="fw-semibold small lh-1 mb-1 text-truncate">
                          {{ aptStatuses[host.id].last_update
                            ? formatDate(aptStatuses[host.id].last_update)
                            : 'Jamais' }}
                        </div>
                        <div class="text-secondary small">
                          vérification
                        </div>
                        <div
                          v-if="aptStatuses[host.id].last_upgrade"
                          class="text-muted apt-date-hint"
                        >
                          upgrade : {{ formatDate(aptStatuses[host.id].last_upgrade) }}
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- CVE -->
                  <div
                    v-if="aptStatuses[host.id].cve_list?.length"
                    class="mb-3"
                  >
                    <CVEList
                      :cve-list="aptStatuses[host.id].cve_list"
                      :show-max-severity="true"
                      :always-expanded="false"
                      :initially-collapsed="false"
                      :limit="3"
                    />
                  </div>

                  <!-- Paquets en attente -->
                  <div
                    v-if="getPackages(aptStatuses[host.id]).length > 0"
                    class="mb-3"
                  >
                    <div class="d-flex align-items-center justify-content-between mb-2">
                      <span class="small fw-semibold text-secondary">
                        Paquets en attente
                        <span class="badge bg-yellow-lt text-yellow ms-1">
                          {{ getPackages(aptStatuses[host.id]).length }}
                        </span>
                      </span>
                      <button
                        v-if="getPackages(aptStatuses[host.id]).length > PKG_PREVIEW_COUNT"
                        class="btn btn-link btn-sm p-0 small text-secondary"
                        @click="pkgShowAll[host.id] = !pkgShowAll[host.id]"
                      >
                        {{ pkgShowAll[host.id]
                          ? 'Réduire'
                          : `Voir tout (${getPackages(aptStatuses[host.id]).length})` }}
                      </button>
                    </div>
                    <div class="row g-1">
                      <div
                        v-for="pkg in visiblePackages(host.id)"
                        :key="pkg"
                        class="col-12 col-sm-6 col-md-4 col-lg-3"
                      >
                        <code
                          class="small text-body d-block text-truncate"
                          :title="pkg"
                        >{{ pkg }}</code>
                      </div>
                    </div>
                  </div>

                  <!-- Historique (2 dernières commandes) -->
                  <div
                    v-if="aptHistories[host.id]?.length"
                    class="border-top pt-2"
                  >
                    <div class="d-flex align-items-center justify-content-between mb-1">
                      <span class="small fw-semibold text-secondary">Dernières commandes</span>
                      <router-link
                        to="/audit?module=apt"
                        class="small text-secondary text-decoration-none"
                      >
                        Historique complet →
                      </router-link>
                    </div>
                    <div
                      v-for="cmd in aptHistories[host.id].slice(0, 2)"
                      :key="cmd.id"
                      class="d-flex align-items-center gap-2 py-1 flex-wrap"
                    >
                      <code class="small">apt {{ cmd.action }}</code>
                      <span :class="statusClass(cmd.status)">{{ statusLabel(cmd.status) }}</span>
                      <span class="text-secondary small flex-shrink-0">{{ formatDate(cmd.created_at) }}</span>
                      <span
                        v-if="cmd.triggered_by"
                        class="text-muted small flex-shrink-0"
                      >· {{ cmd.triggered_by }}</span>
                      <button
                        class="btn btn-sm btn-ghost-secondary ms-auto flex-shrink-0"
                        title="Voir les logs"
                        @click="watchCommand(cmd, host)"
                      >
                        <svg
                          class="icon icon-sm"
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          stroke-width="2"
                          stroke="currentColor"
                          fill="none"
                          xmlns="http://www.w3.org/2000/svg"
                        ><path
                          stroke="none"
                          d="M0 0h24v24H0z"
                          fill="none"
                        /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                      </button>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="liveCommand"
        :show="showConsole"
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="side-panel"
        @open="showConsole = true"
        @close="closeLiveConsole"
      />
    </div>

    <!-- Modal planification -->
    <div
      v-if="scheduleModal.open"
      class="modal modal-blur show d-block modal-overlay"
      tabindex="-1"
      @click.self="scheduleModal.open = false"
    >
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">
                Planifier une commande APT
              </h5>
              <div class="text-muted small mt-1">
                {{ scheduleModal.hostname }}
              </div>
            </div>
            <button
              type="button"
              class="btn-close"
              @click="scheduleModal.open = false"
            />
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">Nom de la tâche</label>
              <input
                v-model="scheduleModal.name"
                type="text"
                class="form-control"
                placeholder="ex: apt upgrade hebdo"
              >
            </div>
            <div class="mb-3">
              <label class="form-label">Commande</label>
              <select
                v-model="scheduleModal.action"
                class="form-select"
              >
                <option value="update">
                  apt update
                </option>
                <option value="upgrade">
                  apt upgrade
                </option>
                <option value="dist-upgrade">
                  apt dist-upgrade
                </option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-check form-switch">
                <input
                  v-model="scheduleModal.manualOnly"
                  type="checkbox"
                  class="form-check-input"
                >
                <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
              </label>
            </div>
            <div
              v-if="!scheduleModal.manualOnly"
              class="mb-3"
            >
              <CronBuilder v-model="scheduleModal.cron_expression" />
            </div>
            <div
              v-if="!scheduleModal.manualOnly"
              class="form-check form-switch mb-2"
            >
              <input
                id="schedEnabled"
                v-model="scheduleModal.enabled"
                type="checkbox"
                class="form-check-input"
              >
              <label
                class="form-check-label"
                for="schedEnabled"
              >Activée</label>
            </div>
            <div
              v-if="scheduleModal.error"
              class="alert alert-danger py-2"
            >
              {{ scheduleModal.error }}
            </div>
          </div>
          <div class="modal-footer">
            <button
              class="btn btn-secondary"
              @click="scheduleModal.open = false"
            >
              Annuler
            </button>
            <button
              class="btn btn-primary"
              :disabled="scheduleModal.saving"
              @click="saveSchedule"
            >
              <span
                v-if="scheduleModal.saving"
                class="spinner-border spinner-border-sm me-1"
              />
              Créer la tâche
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Toast feedback actions groupées -->
    <div
      v-if="bulkActionFeedback"
      class="position-fixed bottom-0 end-0 p-3 toast-overlay"
    >
      <div
        class="toast show align-items-center border-0"
        :class="bulkActionFeedback.variantClass"
      >
        <div class="d-flex">
          <div class="toast-body">
            <strong>apt {{ bulkActionFeedback.command }}</strong> {{ bulkActionFeedback.message }}
            <div
              v-if="bulkActionFeedback.details"
              class="small mt-1"
            >
              {{ bulkActionFeedback.details }}
            </div>
          </div>
          <button
            type="button"
            class="btn-close me-2 m-auto"
            :class="bulkActionFeedback.closeClass"
            @click="bulkActionFeedback = null"
          />
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
import { confirmBulkAction } from '../utils/bulkActionHelpers'
import { useDateFormatter } from '../composables/useDateFormatter'
import { useStatusBadge } from '../composables/useStatusBadge'
import { useToast } from '../composables/useToast'
import { useCommandStream } from '../composables/useCommandStream'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import CronBuilder from '../components/CronBuilder.vue'
import DataToolbar from '../components/common/DataToolbar.vue'

const { dayjs, formatRelativeDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

const PKG_PREVIEW_COUNT = 15

// ── État hôtes / APT ─────────────────────────────────────────────────────────
const hosts = ref([])
const selectedHosts = ref([])
const selectAll = ref(false)
const aptStatuses = ref({})
const aptHistories = ref({})
const pkgShowAll = ref({})
const hostCmdLoading = ref({})
const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

// ── Modal planification ───────────────────────────────────────────────────────
import { MANUAL_SENTINEL } from '../utils/cron'

const scheduleModal = ref({
  open: false, hostId: '', hostname: '', name: '',
  action: 'update', cron_expression: '0 3 * * 0',
  manualOnly: false, enabled: true, saving: false, error: '',
})

function openScheduleModal(host) {
  const hostLabel = host.name && host.hostname && host.name !== host.hostname
    ? `${host.name} (${host.hostname})`
    : (host.name || host.hostname)

  scheduleModal.value = {
    open: true,
    hostId: host.id,
    hostname: hostLabel,
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
const aptBulkLoading = ref(null)

// ── Filtres / tri des hôtes ───────────────────────────────────────────────────
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

  const q = hostSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter((h) => {
      const primary = (h.name || h.hostname || '').toLowerCase()
      const secondary = (h.hostname || '').toLowerCase()
      return primary.includes(q) || secondary.includes(q) || (h.ip_address || '').includes(q)
    })
  }

  if (hostQuickFilter.value === 'critical') {
    list = list.filter(h => {
      const cves = aptStatuses.value[h.id]?.cve_list
      return Array.isArray(cves) && cves.some(c => c.severity === 'CRITICAL')
    })
  } else if (hostQuickFilter.value === 'security') {
    list = list.filter(h => (aptStatuses.value[h.id]?.security_updates || 0) > 0)
  }

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
      va = (a.name || a.hostname || '').toLowerCase()
      vb = (b.name || b.hostname || '').toLowerCase()
    }
    if (va < vb) return hostSortDir.value === 'asc' ? -1 : 1
    if (va > vb) return hostSortDir.value === 'asc' ? 1 : -1
    return 0
  })

  return list
})

// ── Helpers ───────────────────────────────────────────────────────────────────
function toggleSelectAll() {
  selectedHosts.value = selectAll.value ? hosts.value.map(h => h.id) : []
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

function visiblePackages(hostId) {
  const pkgs = getPackages(aptStatuses.value[hostId])
  return pkgShowAll.value[hostId] ? pkgs : pkgs.slice(0, PKG_PREVIEW_COUNT)
}

function isHostCmdLoading(hostId) {
  return !!hostCmdLoading.value[hostId]
}

function watchCommand(cmd, host) {
  showConsole.value = true
  liveCommand.value = {
    id: cmd.id,
    hostId: host?.id || cmd.hostId || cmd.host_id || null,
    host_name: host?.name || host?.hostname || '—',
    module: 'apt',
    action: cmd.action || cmd.command || '—',
    target: '',
    status: cmd.status,
    output: cmd.output || '',
  }
  connectStreamWebSocket(cmd.id)
}

function closeLiveConsole() {
  closeStream()
  liveCommand.value = null
  showConsole.value = false
}

function upsertAptHistory(hostId, nextCommand) {
  if (!hostId || !nextCommand?.id) return
  const currentHistory = Array.isArray(aptHistories.value[hostId]) ? [...aptHistories.value[hostId]] : []
  const currentIndex = currentHistory.findIndex(cmd => cmd.id === nextCommand.id)
  if (currentIndex >= 0) {
    currentHistory[currentIndex] = { ...currentHistory[currentIndex], ...nextCommand }
  } else {
    currentHistory.unshift(nextCommand)
  }
  currentHistory.sort((left, right) => new Date(right.created_at || 0) - new Date(left.created_at || 0))
  aptHistories.value = { ...aptHistories.value, [hostId]: currentHistory }
}

function syncLiveCommand(commandId, patch) {
  if (!liveCommand.value || liveCommand.value.id !== commandId) return
  liveCommand.value = { ...liveCommand.value, ...patch }
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

function connectStreamWebSocket(commandId) {
  closeStream()
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

function buildBulkActionFeedback(command, launchedHosts, failedHosts) {
  const hasFailures = failedHosts.length > 0
  const launchedLabel = launchedHosts.length === 1
    ? `lancée sur ${launchedHosts[0]}`
    : `lancée sur ${launchedHosts.length} hôtes`
  const failedLabel = hasFailures
    ? `Échec d'envoi sur ${failedHosts.join(', ')}.`
    : 'Suivi disponible via l\'historique APT.'

  return {
    command,
    message: launchedHosts.length > 0 ? `commande ${launchedLabel}.` : 'aucune commande n\'a été lancée.',
    details: failedLabel,
    variantClass: hasFailures ? 'text-bg-warning' : 'text-bg-success',
    closeClass: hasFailures ? '' : 'btn-close-white',
  }
}

// ── Commandes par hôte ────────────────────────────────────────────────────────
async function runAptCmdForHost(host, command) {
  if (!canRunApt.value) return

  const confirmed = await confirmBulkAction(
    `apt ${command}`,
    1,
    command === 'dist-upgrade'
      ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${host.name || host.hostname} ?`
      : `Exécuter sur : ${host.name || host.hostname} ?`
  )
  if (!confirmed) return

  hostCmdLoading.value = { ...hostCmdLoading.value, [host.id]: command }
  try {
    const response = await apiClient.sendAptCommand([host.id], command)
    const commandResults = Array.isArray(response.data?.commands) ? response.data.commands : []
    const launched = commandResults.filter(item => item.command_id)
    const failed = commandResults.filter(item => item.error)
    const createdAt = new Date().toISOString()

    launched.forEach(item => {
      upsertAptHistory(host.id, {
        id: item.command_id,
        action: command,
        status: item.status || 'pending',
        output: '',
        created_at: createdAt,
        triggered_by: auth.username || '',
      })
    })

    if (launched.length > 0) {
      watchCommand(
        { id: launched[0].command_id, action: command, status: launched[0].status || 'pending', output: '' },
        host
      )
    } else if (failed.length > 0) {
      await dialog.confirm({
        title: 'Erreur',
        message: failed[0].error || 'Erreur lors de l\'envoi de la commande',
        variant: 'danger',
      })
    }
  } catch (e) {
    await dialog.confirm({ title: 'Erreur', message: getApiErrorMessage(e), variant: 'danger' })
  } finally {
    const next = { ...hostCmdLoading.value }
    delete next[host.id]
    hostCmdLoading.value = next
  }
}

// ── Commandes groupées ────────────────────────────────────────────────────────
async function bulkAptCmd(command) {
  const hostnames = hosts.value
    .filter(h => selectedHosts.value.includes(h.id))
    .map(h => h.name || h.hostname)
    .join(', ')

  const confirmed = await confirmBulkAction(
    `apt ${command}`,
    selectedHosts.value.length,
    command === 'dist-upgrade'
      ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${hostnames || 'les hôtes sélectionnés'} ?`
      : `Exécuter sur : ${hostnames || 'les hôtes sélectionnés'} ?`
  )
  if (!confirmed) return

  aptBulkLoading.value = command
  try {
    const response = await apiClient.sendAptCommand(selectedHosts.value, command)
    const commandResults = Array.isArray(response.data?.commands) ? response.data.commands : []
    const hostNameById = new Map(hosts.value.map(host => [host.id, host.name || host.hostname || host.id]))
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
  } finally {
    aptBulkLoading.value = null
  }
}

// ── Formatage ─────────────────────────────────────────────────────────────────
function formatDate(date) {
  return formatRelativeDate(date)
}

const STATUS_LABELS = {
  pending:   'En attente',
  running:   'En cours',
  completed: 'Terminé',
  failed:    'Échoué',
}

function statusLabel(status) {
  return STATUS_LABELS[status] ?? status
}

function statusClass(status) {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}

// ── WebSocket ─────────────────────────────────────────────────────────────────
const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket('/api/v1/ws/apt', (payload) => {
  if (payload.type !== 'apt') return
  hosts.value = payload.hosts || []
  aptStatuses.value = payload.apt_statuses || {}
  aptHistories.value = payload.apt_histories || {}
})

onUnmounted(() => {
  closeStream()
})
</script>

<style scoped>
.sort-select { width: auto; }
.modal-overlay { background: rgba(0,0,0,.5); z-index: 1050; }
.toast-overlay { z-index: 1100; }
.apt-date-hint { font-size: 0.68rem; }
</style>
