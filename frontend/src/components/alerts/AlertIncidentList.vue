<template>
  <div class="card">
    <div class="card-header flex-column flex-sm-row align-items-start align-items-sm-center gap-2">
      <h3 class="card-title mb-0">
        Historique de notifications
      </h3>
      <div class="d-flex align-items-center gap-2 flex-wrap ms-sm-auto">
        <div class="btn-group btn-group-sm">
          <button
            v-for="opt in TYPE_FILTERS"
            :key="opt.value"
            class="btn"
            :class="filterType === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
            @click="filterType = opt.value; currentPage = 1"
          >
            {{ opt.label }}
          </button>
        </div>
        <div class="btn-group btn-group-sm">
          <button
            v-for="opt in STATUS_FILTERS"
            :key="opt.value"
            class="btn"
            :class="filterStatus === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
            @click="filterStatus = opt.value; currentPage = 1"
          >
            {{ opt.label }}
          </button>
        </div>
        <span
          v-if="activeIncidentCount > 0"
          class="badge bg-red-lt text-red"
        >{{ activeIncidentCount }} actif{{ activeIncidentCount > 1 ? 's' : '' }}</span>
        <span class="text-secondary small text-nowrap">
          {{ filteredIncidents.length }}<template v-if="filteredIncidents.length !== incidents.length">/{{ incidents.length }}</template>
          notification{{ filteredIncidents.length !== 1 ? 's' : '' }}
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
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          Actualiser
        </button>
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
      <div>Aucune notification enregistree</div>
      <div class="text-muted small mt-1">
        Les alertes et notifications release tracker apparaitront ici
      </div>
    </div>
    <div
      v-else-if="filteredIncidents.length === 0"
      class="card-body text-center py-5 text-muted"
    >
      Aucune notification ne correspond aux filtres selectionnes.
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
            <th>Termine</th>
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
                >Termine</span>
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
                >Execution tracker</span>
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
import { computed, ref } from 'vue'
import apiClient from '../../api'
import { getAlertMetricMeta } from '../../utils/alertMetrics'
import { resolveIncidentHostRoute } from '../../utils/incidentRouting'

const PAGE_SIZE = 50
const AGE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000

const TYPE_FILTERS = [
  { value: 'all', label: 'Tous', activeClass: 'btn-secondary' },
  { value: 'crit', label: 'Critique', activeClass: 'btn-danger' },
  { value: 'warn', label: 'Avertissement', activeClass: 'btn-warning' },
  { value: 'tracker', label: 'Tracker', activeClass: 'btn-info' },
]

const STATUS_FILTERS = [
  { value: 'all', label: 'Tous états', activeClass: 'btn-secondary' },
  { value: 'active', label: 'Actifs', activeClass: 'btn-danger' },
  { value: 'resolved', label: 'Terminés', activeClass: 'btn-success' },
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
const currentPage = ref(1)
const markingRead = ref(false)

const filteredIncidents = computed(() => {
  const now = Date.now()
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

    return true
  })
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

async function markAllRead() {
  markingRead.value = true
  try {
    await apiClient.markNotificationsRead()
  } finally {
    markingRead.value = false
  }
}

function incidentMetricLabel(metric) {
  if (!metric) return ''
  if (metric === 'release_tracker') return 'Suivi de version'
  return getAlertMetricMeta(metric).label
}

function defaultNotificationTitle(incident) {
  if (incident?.type === 'release_tracker_detected') return 'Nouvelle release detectee'
  if (incident?.type === 'release_tracker_execution') return 'Execution release tracker'
  return 'Alerte'
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
