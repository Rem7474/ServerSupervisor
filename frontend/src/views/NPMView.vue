<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>Nginx Proxy Manager</span>
      </div>
      <h2 class="page-title">Proxy Hosts NPM</h2>
    </div>

    <div class="card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title mb-0">Tous les proxy hosts importés</h3>
        <button class="btn btn-sm btn-outline-secondary" :disabled="loading" @click="load">
          <svg class="icon icon-sm me-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="1 4 1 10 7 10" />
            <path d="M3.51 15a9 9 0 1 0 .49-3.86" />
          </svg>
          Actualiser
        </button>
      </div>

      <div v-if="loading" class="card-body text-center text-muted py-5">
        <div class="spinner-border spinner-border-sm me-2" />
        Chargement…
      </div>

      <div v-else-if="loadError" class="card-body">
        <div class="alert alert-danger mb-0">{{ loadError }}</div>
      </div>

      <div v-else-if="hosts.length === 0" class="card-body text-center text-muted py-5">
        Aucun proxy host importé. Configurez une connexion NPM dans les
        <router-link to="/settings?tab=integrations">Paramètres → Intégrations</router-link>.
      </div>

      <div v-else class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Connexion</th>
              <th>Domaine</th>
              <th>Forward</th>
              <th>NPM</th>
              <th class="text-center">Monitoring</th>
              <th class="text-center">Uptime</th>
              <th class="text-center">SSL</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="h in hosts" :key="h.id" :class="{ 'opacity-50': !h.npm_enabled }">
              <td class="text-muted small">{{ h.connection_name }}</td>
              <td>
                <div class="fw-medium">{{ h.domain_names[0] }}</div>
                <div v-if="h.domain_names.length > 1" class="d-flex flex-wrap gap-1 mt-1">
                  <span
                    v-for="d in h.domain_names.slice(1)"
                    :key="d"
                    class="badge bg-secondary-lt text-secondary"
                  >{{ d }}</span>
                </div>
              </td>
              <td class="text-muted small">{{ h.forward_host }}:{{ h.forward_port }}</td>
              <td>
                <span v-if="h.npm_enabled" class="badge bg-success-lt text-success">Actif</span>
                <span v-else class="badge bg-warning-lt text-warning">Désactivé</span>
              </td>

              <!-- Master monitoring toggle -->
              <td class="text-center">
                <label class="form-check form-switch mb-0 d-inline-flex justify-content-center">
                  <input
                    class="form-check-input"
                    type="checkbox"
                    :checked="h.monitoring_enabled"
                    :disabled="toggling[h.id]"
                    @change="toggle(h, 'monitoring_enabled', ($event.target as HTMLInputElement).checked)"
                  >
                </label>
              </td>

              <!-- Uptime sub-toggle + badge -->
              <td class="text-center">
                <div class="d-flex flex-column align-items-center gap-1">
                  <label class="form-check form-switch mb-0">
                    <input
                      class="form-check-input"
                      type="checkbox"
                      :checked="h.uptime_monitoring_enabled"
                      :disabled="toggling[h.id] || !h.uptime_probe_id"
                      @change="toggle(h, 'uptime_monitoring_enabled', ($event.target as HTMLInputElement).checked)"
                    >
                  </label>
                  <span
                    v-if="h.uptime_probe_id && h.uptime_status"
                    class="badge small"
                    :class="uptimeBadge(h.uptime_status)"
                  >
                    {{ h.uptime_status }}
                    <span v-if="h.uptime_last_latency_ms" class="ms-1 opacity-75">{{ h.uptime_last_latency_ms }}ms</span>
                  </span>
                  <span v-else-if="!h.uptime_probe_id" class="text-muted small">—</span>
                </div>
              </td>

              <!-- SSL sub-toggle + badge -->
              <td class="text-center">
                <div class="d-flex flex-column align-items-center gap-1">
                  <label class="form-check form-switch mb-0">
                    <input
                      class="form-check-input"
                      type="checkbox"
                      :checked="h.ssl_monitoring_enabled"
                      :disabled="toggling[h.id] || !h.ssl_certificate_id"
                      @change="toggle(h, 'ssl_monitoring_enabled', ($event.target as HTMLInputElement).checked)"
                    >
                  </label>
                  <span
                    v-if="h.ssl_certificate_id && h.ssl_days_remaining !== null && h.ssl_days_remaining !== undefined"
                    class="badge small"
                    :class="sslBadge(h.ssl_days_remaining)"
                  >{{ h.ssl_days_remaining }}j</span>
                  <span v-else-if="!h.ssl_certificate_id" class="text-muted small">—</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { npmApi } from '../api/npm'
import type { NPMProxyHostEnriched } from '../types/npm'

const hosts = ref<NPMProxyHostEnriched[]>([])
const loading = ref(true)
const loadError = ref('')
const toggling = ref<Record<string, boolean>>({})

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await npmApi.listAllProxyHosts()
    hosts.value = res.data.proxy_hosts ?? []
  } catch (e: any) {
    loadError.value = e?.response?.data?.error || 'Impossible de charger les proxy hosts.'
  } finally {
    loading.value = false
  }
}

async function toggle(
  host: NPMProxyHostEnriched,
  field: 'monitoring_enabled' | 'uptime_monitoring_enabled' | 'ssl_monitoring_enabled',
  value: boolean,
): Promise<void> {
  const prev = host[field]
  // Optimistic update
  host[field] = value
  if (field === 'monitoring_enabled' && !value) {
    host.uptime_monitoring_enabled = false
    host.ssl_monitoring_enabled = false
  }

  toggling.value[host.id] = true
  try {
    const res = await npmApi.updateProxyHost(host.id, { [field]: value })
    // Replace with server-confirmed state.
    const idx = hosts.value.findIndex(h => h.id === host.id)
    if (idx !== -1) hosts.value[idx] = res.data
  } catch {
    // Rollback on error.
    host[field] = prev
    if (field === 'monitoring_enabled' && !value) {
      host.uptime_monitoring_enabled = host.monitoring_enabled
      host.ssl_monitoring_enabled = host.monitoring_enabled
    }
  } finally {
    toggling.value[host.id] = false
  }
}

function uptimeBadge(status: string): string {
  if (status === 'up') return 'bg-success-lt text-success'
  if (status === 'down') return 'bg-danger-lt text-danger'
  return 'bg-secondary-lt text-secondary'
}

function sslBadge(days: number): string {
  if (days <= 7) return 'bg-danger-lt text-danger'
  if (days <= 30) return 'bg-warning-lt text-warning'
  return 'bg-success-lt text-success'
}

onMounted(load)
</script>
