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
              <svg
                class="icon"
                width="16"
                height="16"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <circle
                  cx="11"
                  cy="11"
                  r="8"
                />
                <path d="M21 21l-4.35-4.35" />
              </svg>
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
              <svg
                class="icon"
                width="16"
                height="16"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M18 6L6 18M6 6l12 12"
                />
              </svg>
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
                class="btn btn-sm rounded-pill"
                :class="filterType === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
                @click="setTypeFilter(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
            <div class="vr opacity-50" style="height: 24px;" />
            <div class="d-flex flex-wrap align-items-center gap-2">
              <span class="text-muted small me-1 fw-semibold">État :</span>
              <button
                v-for="opt in STATUS_FILTERS"
                :key="opt.value"
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
            class="btn btn-sm btn-ghost-secondary"
            :disabled="markingRead"
            @click="markAllRead"
          >
            <svg
              class="icon me-1"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M5 13l4 4L19 7"
              />
            </svg>
            Tout marquer lu
          </button>
          <button
            class="btn btn-sm btn-ghost-secondary"
            @click="$emit('refresh')"
          >
            <svg
              class="icon me-1"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            Actualiser
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
      <svg
        class="icon icon-lg mb-3"
        width="48"
        height="48"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
        />
      </svg>
      <div>Aucune notification enregistrée</div>
      <div class="text-muted small mt-1">
        Les alertes et les notifications du release tracker apparaîtront ici
      </div>
    </div>
    <div
      v-else-if="filteredIncidents.length === 0"
      class="card-body text-center py-5 text-muted"
    >
      <svg
        class="icon icon-lg mb-3"
        width="48"
        height="48"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
        />
      </svg>
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
      class="table-responsive"
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
          </tr>
        </thead>
        <tbody>
          <template
            v-for="item in paginatedIncidents"
            :key="item.id"
          >
            <tr
              v-if="item._showSeparator"
              class="table-light"
            >
              <td
                colspan="7"
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
                  {{ item.rule_name || defaultNotificationTitle(item) }}
                </div>
                <div
                  v-if="item.metric"
                  class="text-muted small"
                >
                  {{ incidentMetricLabel(item.metric) }}
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
                  <code>{{ incidentFormatValue(item.value, item.metric) }}</code>
                </template>
              </td>
              <td class="text-muted small">
                {{ formatDate(item.triggered_at) }}
              </td>
              <td class="text-muted small">
                <span v-if="item.resolved_at">{{ formatDate(item.resolved_at) }}</span>
                <span
                  v-else
                  class="text-secondary"
                >-</span>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div
      v-if="totalPages > 1"
      class="card-footer d-flex align-items-center"
    >
      <p class="m-0 text-muted">
        Page {{ currentPage }} / {{ totalPages }}
      </p>
      <ul class="pagination m-0 ms-auto">
        <li
          class="page-item"
          :class="{ disabled: currentPage === 1 }"
        >
          <button
            class="page-link"
            @click="currentPage--"
          >
            <svg
              class="icon"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15 19l-7-7 7-7"
              />
            </svg>
          </button>
        </li>
        <li
          v-for="page in visiblePages"
          :key="page"
          class="page-item"
          :class="{ active: page === currentPage, disabled: page === '…' }"
        >
          <button
            class="page-link"
            @click="typeof page === 'number' && (currentPage = page)"
          >
            {{ page }}
          </button>
        </li>
        <li
          class="page-item"
          :class="{ disabled: currentPage === totalPages }"
        >
          <button
            class="page-link"
            @click="currentPage++"
          >
            <svg
              class="icon"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 5l7 7-7 7"
              />
            </svg>
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import apiClient from '../../api'
import BadgePill from '../common/BadgePill.vue'
import { getAlertMetricMeta } from '../../utils/alertMetrics'
import { resolveIncidentHostRoute } from '../../utils/incidentRouting'

const PAGE_SIZE = 50
const AGE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000

const TYPE_FILTERS = [
  { value: 'all', label: 'Tous', activeClass: 'btn-primary shadow-sm' },
  { value: 'crit', label: 'Critique', activeClass: 'btn-danger shadow-sm' },
  { value: 'warn', label: 'Avertissement', activeClass: 'btn-warning shadow-sm' },
  { value: 'tracker', label: 'Tracker', activeClass: 'btn-info shadow-sm' },
]

const STATUS_FILTERS = [
  { value: 'all', label: 'Tous états', activeClass: 'btn-primary shadow-sm' },
  { value: 'active', label: 'Actifs', activeClass: 'btn-danger shadow-sm' },
  { value: 'resolved', label: 'Terminés', activeClass: 'btn-success shadow-sm' },
]

const props = defineProps({
  incidents: {
    type: Array,
    default: () => [],
  },
  loading: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
  activeIncidentCount: {
    type: Number,
    default: 0,
  },
})

defineEmits(['refresh'])

const filterType = ref('all')
const filterStatus = ref('all')
const searchQuery = ref('')
const currentPage = ref(1)
const markingRead = ref(false)

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

const annotatedIncidents = computed(() => {
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

const visiblePages = computed(() => {
  const total = totalPages.value
  const cur = currentPage.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages = []
  if (cur <= 4) {
    pages.push(1, 2, 3, 4, 5, '…', total)
  } else if (cur >= total - 3) {
    pages.push(1, '…', total - 4, total - 3, total - 2, total - 1, total)
  } else {
    pages.push(1, '…', cur - 1, cur, cur + 1, '…', total)
  }
  return pages
})

function setTypeFilter(value) {
  filterType.value = value
  currentPage.value = 1
}

function setStatusFilter(value) {
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

function notificationRoute(incident) {
  if (incident?.type === 'release_tracker_detected' || incident?.type === 'release_tracker_execution') {
    if (incident?.tracker_id) return `/release-trackers/${encodeURIComponent(incident.tracker_id)}`
    return '/git-webhooks?tab=trackers'
  }
  return resolveIncidentHostRoute(incident?.host_id, incident?.metric)
}

function trackerStatusLabel(status) {
  if (status === 'pending' || status === 'running') return 'Détection en cours'
  if (status === 'completed' || status === 'success') return 'Exécution réussie'
  if (status === 'failed' || status === 'error') return 'Exécution échouée'
  return status || 'État inconnu'
}

function isCompleted(incident) {
  if (incident?.type === 'release_tracker_detected' || incident?.type === 'release_tracker_execution') {
    return !!incident?.resolved_at || ['completed', 'success', 'failed', 'error'].includes((incident?.status || '').toLowerCase())
  }
  return !!incident?.resolved_at
}

function incidentMetricLabel(metric) {
  const meta = getAlertMetricMeta(metric)
  return meta?.label || metric || '-'
}

function defaultNotificationTitle(incident) {
  if (incident?.type === 'release_tracker_detected') return 'Nouvelle version détectée'
  if (incident?.type === 'release_tracker_execution') return 'Exécution de tracker'
  return incident?.metric ? incidentMetricLabel(incident.metric) : 'Notification'
}

function incidentFormatValue(value, metric) {
  if (metric === 'release_tracker') return '-'
  if (metric === 'status_offline') return value === 1 ? 'offline' : 'online'
  if (metric === 'disk_smart_status') return Number(value) >= 1 ? 'FAILED' : 'OK'
  const unit = getAlertMetricMeta(metric).unit
  return `${Number(value).toFixed(2)}${unit}`
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>
