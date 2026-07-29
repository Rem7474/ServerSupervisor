<template>
  <div class="card">
    <div class="card-header d-flex flex-column flex-lg-row align-items-start align-items-lg-center justify-content-between gap-3">
      <div>
        <h3 class="card-title mb-1">
          Historique de notifications
        </h3>
        <div class="text-muted small">
          Recherche, filtre par type ou par état, puis ouvre le détail en un clic.
        </div>
      </div>
      <div class="d-flex flex-wrap align-items-center gap-2">
        <BadgePill
          v-if="activeIncidentCount > 0"
          :text="`${activeIncidentCount} actif${activeIncidentCount > 1 ? 's' : ''}`"
          tone="danger"
          compact
        />
        <BadgePill
          :text="incidentCountLabel"
          tone="secondary"
          compact
        />
      </div>
    </div>

    <div class="card-body border-bottom py-3">
      <div class="row g-3 align-items-end">
        <div class="col-12 col-xl-4">
          <label class="form-label text-muted small mb-2">
            Recherche
          </label>
          <div class="input-icon">
            <span class="input-icon-addon">
              <IconSearch
                :size="16"
                class="icon"
              />
            </span>
            <input
              v-model="searchQuery"
              type="text"
              class="form-control"
              placeholder="Rechercher une règle, un hôte, une source..."
            >
            <button
              v-if="searchQuery"
              class="btn btn-icon btn-outline-secondary"
              type="button"
              aria-label="Effacer la recherche"
              @click="clearSearch"
            >
              <IconX
                :size="16"
                class="icon"
              />
            </button>
          </div>
        </div>
        <div class="col-12 col-xl-8">
          <div class="d-flex flex-wrap align-items-center justify-content-xl-end gap-3">
            <div class="d-flex flex-wrap align-items-center gap-2">
              <span class="text-muted small me-1 fw-semibold">Type :</span>
              <button
                v-for="opt in TYPE_FILTERS"
                :key="opt.value"
                type="button"
                class="btn btn-sm rounded-pill"
                :class="filterType === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
                @click="setTypeFilter(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
            <div
              class="vr opacity-50"
              style="height: 24px;"
            />
            <div class="d-flex flex-wrap align-items-center gap-2">
              <span class="text-muted small me-1 fw-semibold">État :</span>
              <button
                v-for="opt in STATUS_FILTERS"
                :key="opt.value"
                type="button"
                class="btn btn-sm rounded-pill"
                :class="filterStatus === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
                @click="setStatusFilter(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
          </div>
        </div>
        <div class="col-12 d-flex flex-wrap align-items-center gap-2">
          <span class="text-muted small me-1">
            Actions rapides
          </span>
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            :disabled="markingRead"
            @click="markAllRead"
          >
            <IconCheck
              :size="16"
              class="icon me-1"
            />
            Tout marquer lu
          </button>
          <button
            v-if="hasActiveFilters"
            class="btn btn-sm btn-outline-secondary"
            type="button"
            @click="resetFilters"
          >
            Réinitialiser
          </button>
          <span class="ms-auto text-secondary small text-nowrap">
            {{ incidentCountLabel }}
          </span>
        </div>
      </div>
    </div>

    <div
      v-if="loading"
      class="card-body text-center py-5"
    >
      <div
        class="spinner-border text-primary"
        role="status"
      />
      <div class="mt-2 text-muted">
        Chargement...
      </div>
    </div>
    <div
      v-else-if="error"
      class="card-body text-center py-5 text-danger"
    >
      {{ error }}
    </div>
    <div
      v-else-if="incidents.length === 0"
      class="card-body text-center py-5 text-muted"
    >
      <IconBell
        :size="48"
        class="icon icon-lg mb-3"
        :stroke-width="1.5"
      />
      <div>Aucune notification enregistrée</div>
      <div class="text-muted small mt-1">
        Les alertes et les notifications du release tracker apparaîtront ici
      </div>
    </div>
    <div
      v-else-if="filteredIncidents.length === 0"
      class="card-body text-center py-5 text-muted"
    >
      <IconBell
        :size="48"
        class="icon icon-lg mb-3"
        :stroke-width="1.5"
      />
      <div class="fw-semibold text-body">
        Aucune notification ne correspond à cette recherche.
      </div>
      <div class="text-muted small mt-1">
        Essayez un autre mot-clé ou réinitialisez les filtres.
      </div>
      <button
        v-if="hasActiveFilters"
        class="btn btn-sm btn-outline-secondary mt-3"
        type="button"
        @click="resetFilters"
      >
        Réinitialiser
      </button>
    </div>
    <div
      v-else
      class="table-responsive scroll-table"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th style="width: 90px;">
              État
            </th>
            <th>Type</th>
            <th>Élément</th>
            <th>Source</th>
            <th>Détails</th>
            <th>Déclenché</th>
            <th>Terminé</th>
            <th style="width: 60px;" />
          </tr>
        </thead>
        <tbody>
          <template
            v-for="item in paginatedIncidents"
            :key="item.id"
          >
            <tr
              v-if="item._showSeparator"
            >
              <td
                colspan="8"
                class="text-center text-muted small py-1 border-top"
              >
                — Plus de 7 jours —
              </td>
            </tr>
            <tr :class="{ 'text-muted': item._isOld }">
              <td>
                <span
                  v-if="isCompleted(item)"
                  class="badge bg-green-lt text-green"
                >Terminé</span>
                <span
                  v-else
                  class="badge bg-red-lt text-red"
                >Actif</span>
              </td>
              <td>
                <span
                  v-if="item.type === 'release_tracker_detected'"
                  class="badge bg-blue-lt text-blue"
                >Release tracker</span>
                <span
                  v-else-if="item.type === 'release_tracker_execution'"
                  class="badge bg-indigo-lt text-indigo"
                >Exécution tracker</span>
                <span
                  v-else-if="(item.severity || '').toLowerCase() === 'crit'"
                  class="badge bg-red-lt text-red"
                >Alerte critique</span>
                <span
                  v-else-if="(item.severity || '').toLowerCase() === 'warn'"
                  class="badge bg-yellow-lt text-yellow"
                >Alerte avertissement</span>
                <span
                  v-else
                  class="badge bg-secondary-lt text-secondary"
                >-</span>
              </td>
              <td>
                <div
                  class="fw-semibold text-truncate"
                  style="max-width: 220px;"
                  :title="item.rule_name"
                >
                  {{ notificationTitle(item) }}
                </div>
                <div
                  v-if="item.metric"
                  class="text-muted small"
                >
                  {{ metricLabel(item.metric) }}
                </div>
              </td>
              <td>
                <router-link
                  v-if="notificationRoute(item)"
                  :to="notificationRoute(item)"
                  class="text-decoration-none"
                >
                  {{ item.host_name || 'Source inconnue' }}
                </router-link>
                <span v-else>{{ item.host_name || 'Source inconnue' }}</span>
                <div
                  v-if="item.source_label && item.source_label !== item.host_name"
                  class="text-muted small text-truncate"
                  :title="item.source_label"
                  style="max-width: 260px;"
                >
                  {{ item.source_label }}
                </div>
              </td>
              <td>
                <template v-if="item.type === 'release_tracker_detected' || item.type === 'release_tracker_execution'">
                  <div>
                    Version : <code>{{ item.version || '-' }}</code>
                  </div>
                  <div class="text-muted small">
                    {{ trackerStatusLabel(item.status) }}
                  </div>
                </template>
                <template v-else>
                  <code>{{ formatIncidentValue({ value: item.value, metric: item.metric, value_label: item.value_label }) }}</code>
                  <div
                    v-if="!isCompleted(item) && item.current_value != null"
                    class="text-muted small mt-1"
                  >
                    Actuel :
                    <span class="fw-medium">{{ formatIncidentValue({ value: item.current_value, metric: item.metric, value_label: item.value_label }) }}</span>
                    <span
                      v-if="resolveHint(item)"
                      class="ms-1"
                    >· {{ resolveHint(item) }}</span>
                  </div>
                  <div
                    v-if="item.command_status"
                    class="text-muted small mt-1"
                  >
                    Remédiation :
                    <span :class="getExecutionStateClass(item.command_status)">{{ commandStatusLabel(item.command_status) }}</span>
                  </div>
                </template>
              </td>
              <td class="text-muted small">
                {{ formatDate(item.triggered_at) }}
              </td>
              <td class="text-muted small">
                <template v-if="item.resolved_at">
                  {{ formatDate(item.resolved_at) }}
                  <span v-if="!isTrackerType(item)">({{ incidentDuration(item) }})</span>
                </template>
                <span
                  v-else
                  class="text-secondary"
                >-</span>
              </td>
              <td>
                <button
                  v-if="isAdmin && !isCompleted(item) && item.id"
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :disabled="resolvingId === item.id"
                  title="Clôturer manuellement"
                  @click="manualResolve(item)"
                >
                  <span
                    v-if="resolvingId === item.id"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconCheck v-else
                    :size="14"
                    class="icon"
                  />
                </button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div
      v-if="totalPages > 1"
      class="card-footer d-flex align-items-center justify-content-between"
    >
      <p class="m-0 text-muted">
        Page {{ currentPage }} / {{ totalPages }}
      </p>
      <PaginationNav
        :current-page="currentPage"
        :total-pages="totalPages"
        @select="currentPage = $event"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconBell, IconCheck, IconSearch, IconX } from '@tabler/icons-vue'
import apiClient from '../../api'
import BadgePill from '../common/BadgePill.vue'
import PaginationNav from '../PaginationNav.vue'
import { addToast } from '../../composables/useGlobalToast'
import { getApiErrorMessage } from '../../api/client'
import { getExecutionStateClass } from '../../utils/statusClasses'
import {
  formatIncidentValue,
  incidentDuration,
  isTrackerType,
  metricLabel,
  notificationResolved as isCompleted,
  notificationRoute,
  notificationTitle,
  resolvableIncidentId,
  resolveHint,
  trackerStatusLabel,
} from '../../utils/incidentFormat'
import type { NotificationItem } from '../../types/generated'

type Incident = NotificationItem

interface AnnotatedIncident extends Incident {
  _isOld: boolean
  _showSeparator: boolean
}

const PAGE_SIZE = 50
const AGE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000

const TYPE_FILTERS = [
  { value: 'all', label: 'Tous', activeClass: 'btn-primary shadow-sm' },
  { value: 'crit', label: 'Critique', activeClass: 'btn-danger shadow-sm' },
  { value: 'warn', label: 'Avertissement', activeClass: 'btn-warning shadow-sm' },
  { value: 'tracker', label: 'Tracker', activeClass: 'btn-info shadow-sm' },
] as const

const STATUS_FILTERS = [
  { value: 'all', label: 'Tous états', activeClass: 'btn-primary shadow-sm' },
  { value: 'active', label: 'Actifs', activeClass: 'btn-danger shadow-sm' },
  { value: 'resolved', label: 'Terminés', activeClass: 'btn-success shadow-sm' },
] as const

const props = withDefaults(defineProps<{
  incidents?: Incident[]
  loading?: boolean
  error?: string
  activeIncidentCount?: number
  isAdmin?: boolean
  initialSearch?: string
}>(), {
  incidents: () => [],
  loading: false,
  error: '',
  activeIncidentCount: 0,
  isAdmin: false,
  initialSearch: '',
})

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

const filterType = ref('all')
const filterStatus = ref('all')
// Seeded from the caller (e.g. HostDetailView's "?host=" deep link) so
// arriving from a specific host's incidents lands pre-filtered instead of on
// the full undifferentiated list.
const searchQuery = ref(props.initialSearch)
const currentPage = ref(1)
const markingRead = ref(false)
const resolvingId = ref<string | number | null>(null)

const filteredIncidents = computed(() => {
  const search = searchQuery.value.trim().toLowerCase()
  return props.incidents.filter((incident) => {
    if (filterType.value === 'crit') {
      if (incident.type === 'release_tracker_detected' || incident.type === 'release_tracker_execution') return false
      if ((incident.severity || '').toLowerCase() !== 'crit') return false
    } else if (filterType.value === 'warn') {
      if (incident.type === 'release_tracker_detected' || incident.type === 'release_tracker_execution') return false
      if ((incident.severity || '').toLowerCase() !== 'warn') return false
    } else if (filterType.value === 'tracker') {
      if (incident.type !== 'release_tracker_detected' && incident.type !== 'release_tracker_execution') return false
    }

    if (filterStatus.value === 'active' && isCompleted(incident)) return false
    if (filterStatus.value === 'resolved' && !isCompleted(incident)) return false

    if (search) {
      const haystack = [
        incident.rule_name,
        incident.host_name,
        incident.source_label,
        incident.metric,
        incident.type,
        incident.status,
        incident.version,
        incident.value,
      ]
        .filter(Boolean)
        .map((value) => String(value).toLowerCase())
        .join(' ')
      if (!haystack.includes(search)) return false
    }

    return true
  })
})

const hasActiveFilters = computed(() => filterType.value !== 'all' || filterStatus.value !== 'all' || searchQuery.value.trim().length > 0)

const incidentCountLabel = computed(() => {
  const visible = filteredIncidents.value.length
  const total = props.incidents.length
  return `${visible}${visible !== total ? `/${total}` : ''} notification${visible !== 1 ? 's' : ''}`
})

watch([filterType, filterStatus, searchQuery], () => {
  currentPage.value = 1
})

const annotatedIncidents = computed<AnnotatedIncident[]>(() => {
  const now = Date.now()
  let separatorShown = false
  return filteredIncidents.value.map((incident) => {
    const isOld = incident.triggered_at
      ? now - new Date(incident.triggered_at).getTime() > AGE_THRESHOLD_MS
      : false
    const showSeparator = isOld && !separatorShown
    if (isOld) separatorShown = true
    return { ...incident, _isOld: isOld, _showSeparator: showSeparator }
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(annotatedIncidents.value.length / PAGE_SIZE)))

const paginatedIncidents = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return annotatedIncidents.value.slice(start, start + PAGE_SIZE)
})

function setTypeFilter(value: string): void {
  filterType.value = value
  currentPage.value = 1
}

function setStatusFilter(value: string): void {
  filterStatus.value = value
  currentPage.value = 1
}

function clearSearch() {
  searchQuery.value = ''
}

function resetFilters() {
  filterType.value = 'all'
  filterStatus.value = 'all'
  searchQuery.value = ''
  currentPage.value = 1
}

async function markAllRead() {
  markingRead.value = true
  try {
    await apiClient.markNotificationsRead()
  } finally {
    markingRead.value = false
  }
}

async function manualResolve(incident: Incident) {
  const id = resolvableIncidentId(incident)
  if (!id || resolvingId.value) return
  resolvingId.value = incident.id
  try {
    await apiClient.resolveAlertIncident(id)
    addToast('Incident résolu', 'success')
    emit('refresh')
  } catch (err: unknown) {
    addToast(getApiErrorMessage(err, 'Impossible de résoudre'), 'error')
  } finally {
    resolvingId.value = null
  }
}

// Describes the remote_commands row a rule's command_trigger dispatched when
// this incident fired (item.command_status, joined server-side from
// remote_commands.status) — adjectival wording ("réussie"/"échouée") to
// agree with "Remédiation :" above it, not the standalone noun forms
// commandStatusLabel in utils/commandStatus.ts uses elsewhere.
function commandStatusLabel(status: string | undefined): string {
  if (status === 'pending') return 'en attente'
  if (status === 'running') return 'en cours'
  if (status === 'completed') return 'réussie'
  if (status === 'failed') return 'échouée'
  return status || 'inconnue'
}

function formatDate(dateStr: string | undefined | null): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>
