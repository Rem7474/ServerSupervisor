<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">Incidents recents</h3>
      <div class="d-flex align-items-center gap-2">
        <span v-if="activeIncidentCount > 0" class="badge bg-red-lt text-red">{{ activeIncidentCount }} actif{{ activeIncidentCount > 1 ? 's' : '' }}</span>
        <span class="text-secondary small">{{ incidents.length }} incident{{ incidents.length !== 1 ? 's' : '' }}</span>
        <button class="btn btn-sm btn-ghost-secondary" @click="$emit('refresh')">
          <svg class="icon" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
          Actualiser
        </button>
      </div>
    </div>
    <div v-if="loading" class="card-body text-center py-5">
      <div class="spinner-border text-primary" role="status"></div>
      <div class="mt-2 text-muted">Chargement...</div>
    </div>
    <div v-else-if="error" class="card-body text-center py-5 text-danger">{{ error }}</div>
    <div v-else-if="incidents.length === 0" class="card-body text-center py-5 text-muted">
      <svg class="icon icon-lg mb-3" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
      </svg>
      <div>Aucun incident enregistre</div>
      <div class="text-muted small mt-1">Les incidents apparaitront ici lorsqu'une regle d'alerte se declenchera</div>
    </div>
    <div v-else class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th style="width: 90px;">Etat</th>
            <th>Regle</th>
            <th>Hote</th>
            <th>Valeur</th>
            <th>Declenche</th>
            <th>Resolu</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="incident in incidents" :key="incident.id">
            <td>
              <span v-if="incident.resolved_at" class="badge bg-green-lt text-green">Resolu</span>
              <span v-else class="badge bg-red-lt text-red">Actif</span>
            </td>
            <td>
              <div class="fw-semibold text-truncate" style="max-width: 220px;" :title="incident.rule_name">{{ incident.rule_name }}</div>
              <div class="text-muted small">{{ incidentMetricLabel(incident.metric) }}</div>
            </td>
            <td>
              <router-link :to="`/hosts/${incident.host_id}`" class="text-decoration-none">{{ incident.host_name }}</router-link>
            </td>
            <td><code>{{ incidentFormatValue(incident.value, incident.metric) }}</code></td>
            <td class="text-muted small">{{ formatDate(incident.triggered_at) }}</td>
            <td class="text-muted small">
              <span v-if="incident.resolved_at">{{ formatDate(incident.resolved_at) }}</span>
              <span v-else class="text-secondary">-</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { getAlertMetricMeta } from '../../utils/alertMetrics'

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
  return getAlertMetricMeta(metric).label
}

function incidentFormatValue(value, metric) {
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