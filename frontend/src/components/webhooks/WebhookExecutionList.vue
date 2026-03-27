<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">{{ title }}</h3>
      <button v-if="showRefresh" class="btn btn-sm btn-ghost-secondary" @click="$emit('refresh')">
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
        </svg>
        Actualiser
      </button>
    </div>

    <div v-if="loading" class="card-body text-center py-5">
      <div class="spinner-border text-primary" role="status"></div>
    </div>
    <div v-else-if="executions.length === 0" class="card-body text-center text-muted py-5">{{ emptyText }}</div>
    <div v-else class="table-responsive">
      <table class="table table-vcenter card-table table-sm">
        <thead>
          <tr v-if="kind === 'tracker'">
            <th>Date</th>
            <th>Release</th>
            <th>Statut</th>
            <th>Logs</th>
          </tr>
          <tr v-else>
            <th>Date</th>
            <th>Repo / Branche</th>
            <th>Commit</th>
            <th>Statut</th>
            <th>Logs</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="execution in executions" :key="execution.id || execution.sourceId">
            <template v-if="kind === 'tracker'">
              <td class="text-muted small text-nowrap"><RelativeTime :date="execution.triggered_at" /></td>
              <td class="small">
                <div>
                  <a v-if="execution.release_url" :href="execution.release_url" target="_blank" class="link-primary fw-semibold">{{ execution.tag_name || '—' }}</a>
                  <span v-else class="fw-semibold">{{ execution.tag_name || '—' }}</span>
                </div>
                <div v-if="execution.release_name && execution.release_name !== execution.tag_name" class="text-muted text-truncate" style="max-width: 180px">{{ execution.release_name }}</div>
              </td>
              <td><span class="badge" :class="execStatusBadge(execution.status)">{{ execution.status }}</span></td>
              <td>
                <router-link v-if="execution.command_id" :to="`/audit?command=${execution.command_id}`" class="btn btn-sm btn-ghost-secondary" title="Voir les logs">
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/>
                    <line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>
                  </svg>
                </router-link>
                <span v-else class="text-muted">—</span>
              </td>
            </template>
            <template v-else>
              <td class="text-muted small text-nowrap"><RelativeTime :date="execution.triggered_at" /></td>
              <td class="small">
                <div class="text-truncate" style="max-width: 160px" :title="execution.repo_name">{{ execution.repo_name || execution.sourceName || '—' }}</div>
                <div class="text-muted">{{ execution.branch || '—' }}</div>
              </td>
              <td class="small">
                <div v-if="execution.commit_sha" class="font-monospace text-muted">{{ execution.commit_sha.slice(0, 7) }}</div>
                <div class="text-truncate" style="max-width: 140px" :title="execution.commit_message">{{ execution.commit_message || '—' }}</div>
              </td>
              <td><span class="badge" :class="execStatusBadge(execution.status)">{{ execution.status }}</span></td>
              <td>
                <router-link v-if="execution.command_id" :to="`/audit?command=${execution.command_id}`" class="btn btn-sm btn-ghost-secondary" title="Voir les logs">
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/>
                    <line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>
                  </svg>
                </router-link>
                <span v-else class="text-muted">—</span>
              </td>
            </template>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import RelativeTime from '../RelativeTime.vue'

defineProps({
  executions: {
    type: Array,
    default: () => [],
  },
  loading: {
    type: Boolean,
    default: false,
  },
  kind: {
    type: String,
    default: 'webhook',
  },
  title: {
    type: String,
    default: 'Historique des exécutions',
  },
  emptyText: {
    type: String,
    default: 'Aucune exécution enregistrée.',
  },
  showRefresh: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['refresh'])

function execStatusBadge(status) {
  const map = {
    pending: 'bg-yellow-lt text-yellow',
    running: 'bg-blue-lt text-blue',
    completed: 'bg-success-lt text-success',
    failed: 'bg-danger-lt text-danger',
    skipped: 'bg-secondary-lt text-secondary',
  }
  return map[status] || 'bg-secondary-lt text-secondary'
}
</script>
