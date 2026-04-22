<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        Historique de notifications
      </h3>
      <div class="d-flex align-items-center gap-2">
        <span
          v-if="activeIncidentCount > 0"
          class="badge bg-red-lt text-red"
        >{{ activeIncidentCount }} actif{{ activeIncidentCount > 1 ? 's' : '' }}</span>
        <span class="text-secondary small">{{ incidents.length }} notification{{ incidents.length !== 1 ? 's' : '' }}</span>
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
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th style="width: 90px;">
              Etat
            </th>
            <th>Type</th>
            <th>Element</th>
            <th>Source</th>
            <th>Details</th>
            <th>Declenche</th>
            <th>Termine</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="incident in incidents"
            :key="incident.id"
          >
            <td>
              <span
                v-if="isCompleted(incident)"
                class="badge bg-green-lt text-green"
              >Termine</span>
              <span
                v-else
                class="badge bg-red-lt text-red"
              >Actif</span>
            </td>
            <td>
              <span
                v-if="incident.type === 'release_tracker_detected'"
                class="badge bg-blue-lt text-blue"
              >Release tracker</span>
              <span
                v-else-if="incident.type === 'release_tracker_execution'"
                class="badge bg-indigo-lt text-indigo"
              >Execution tracker</span>
              <span
                v-else-if="(incident.severity || '').toLowerCase() === 'crit'"
                class="badge bg-red-lt text-red"
              >Alerte critique</span>
              <span
                v-else-if="(incident.severity || '').toLowerCase() === 'warn'"
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
                :title="incident.rule_name"
              >
                {{ incident.rule_name || defaultNotificationTitle(incident) }}
              </div>
              <div
                v-if="incident.metric"
                class="text-muted small"
              >
                {{ incidentMetricLabel(incident.metric) }}
              </div>
            </td>
            <td>
              <router-link
                v-if="notificationRoute(incident)"
                :to="notificationRoute(incident)"
                class="text-decoration-none"
              >
                {{ incident.host_name || 'Source inconnue' }}
              </router-link>
              <span v-else>{{ incident.host_name || 'Source inconnue' }}</span>
              <div
                v-if="incident.source_label && incident.source_label !== incident.host_name"
                class="text-muted small text-truncate"
                :title="incident.source_label"
                style="max-width: 260px;"
              >
                {{ incident.source_label }}
              </div>
            </td>
            <td>
              <template v-if="incident.type === 'release_tracker_detected' || incident.type === 'release_tracker_execution'">
                <div>
                  Version : <code>{{ incident.version || '-' }}</code>
                </div>
                <div class="text-muted small">
                  {{ trackerStatusLabel(incident.status) }}
                </div>
              </template>
              <template v-else>
                <code>{{ incidentFormatValue(incident.value, incident.metric) }}</code>
              </template>
            </td>
            <td class="text-muted small">
              {{ formatDate(incident.triggered_at) }}
            </td>
            <td class="text-muted small">
              <span v-if="incident.resolved_at">{{ formatDate(incident.resolved_at) }}</span>
              <span
                v-else
                class="text-secondary"
              >-</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { getAlertMetricMeta } from '../../utils/alertMetrics'
import { resolveIncidentHostRoute } from '../../utils/incidentRouting'

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
  if (status === 'pending' || status === 'running') return 'Detection en cours'
  if (status === 'completed' || status === 'success') return 'Execution reussie'
  if (status === 'failed' || status === 'error') return 'Execution echouee'
  return status || 'Etat inconnu'
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