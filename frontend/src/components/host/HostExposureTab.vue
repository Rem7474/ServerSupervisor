<template>
  <div>
    <div class="d-flex align-items-center gap-2 mb-3">
      <div class="d-flex gap-2 flex-wrap">
        <button
          v-for="p in PERIODS"
          :key="p.value"
          type="button"
          class="btn btn-sm"
          :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="period = p.value"
        >
          {{ p.label }}
        </button>
      </div>
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary ms-auto"
        :disabled="loading"
        @click="load"
      >
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm me-1"
        />
        Actualiser
      </button>
    </div>

    <div
      v-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <div
      v-if="loading && !exposure"
      class="text-center text-muted py-4"
    >
      <div class="spinner-border mb-2" />
      <div>Chargement de la corrélation…</div>
    </div>

    <template v-else-if="exposure">
      <div
        v-if="exposure.domains.length === 0"
        class="text-center text-muted py-4"
      >
        Aucun domaine NPM ne route vers cet hôte<span v-if="exposure.ip_address"> ({{ exposure.ip_address }})</span>.
      </div>

      <template v-else>
        <div class="row row-cards mb-3">
          <div class="col-sm-4">
            <div class="card card-sm">
              <div class="card-body">
                <div class="text-muted small">
                  Requêtes ({{ periodLabel }})
                </div>
                <div class="h2 mb-0">
                  {{ exposure.total_requests.toLocaleString('fr-FR') }}
                </div>
              </div>
            </div>
          </div>
          <div class="col-sm-4">
            <div class="card card-sm">
              <div class="card-body">
                <div class="text-muted small">
                  Suspectes
                </div>
                <div
                  class="h2 mb-0"
                  :class="exposure.total_suspicious_requests > 0 ? 'text-warning' : ''"
                >
                  {{ exposure.total_suspicious_requests.toLocaleString('fr-FR') }}
                </div>
              </div>
            </div>
          </div>
          <div class="col-sm-4">
            <div class="card card-sm">
              <div class="card-body">
                <div class="text-muted small">
                  Bloquées
                </div>
                <div
                  class="h2 mb-0"
                  :class="exposure.total_blocked_requests > 0 ? 'text-danger' : ''"
                >
                  {{ exposure.total_blocked_requests.toLocaleString('fr-FR') }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <p class="text-muted small">
          Domaines Nginx Proxy Manager dont la cible ({{ exposure.ip_address }}) correspond à cet hôte, avec le trafic web observé sur ces domaines — même si les journaux sont collectés par l'agent du proxy inverse, pas par celui de cet hôte.
        </p>

        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Domaine(s)</th>
                <th>Connexion NPM</th>
                <th>Cible</th>
                <th class="text-end">
                  Requêtes
                </th>
                <th class="text-end">
                  Volume
                </th>
                <th class="text-end">
                  Erreurs 4xx/5xx
                </th>
                <th class="text-end">
                  Suspectes
                </th>
                <th class="text-end">
                  Bloquées
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="d in exposure.domains"
                :key="d.proxy_host_id"
              >
                <td>
                  <div
                    v-for="name in d.domain_names"
                    :key="name"
                    class="font-monospace small"
                  >
                    {{ name }}
                  </div>
                </td>
                <td>{{ d.connection_name }}</td>
                <td>
                  <span class="badge bg-secondary-lt text-secondary me-1">:{{ d.forward_port }}</span>
                  <span
                    v-if="d.ssl_enabled"
                    class="badge bg-green-lt text-green me-1"
                  >SSL</span>
                  <span
                    v-if="!d.npm_enabled"
                    class="badge bg-red-lt text-red"
                  >Désactivé</span>
                </td>
                <td class="text-end">
                  {{ d.requests.toLocaleString('fr-FR') }}
                </td>
                <td class="text-end">
                  {{ formatBytes(d.bytes) }}
                </td>
                <td class="text-end text-muted">
                  {{ d.errors_4xx.toLocaleString('fr-FR') }} / {{ d.errors_5xx.toLocaleString('fr-FR') }}
                </td>
                <td class="text-end">
                  <span :class="d.suspicious_requests > 0 ? 'text-warning fw-medium' : 'text-muted'">
                    {{ d.suspicious_requests.toLocaleString('fr-FR') }}
                  </span>
                </td>
                <td class="text-end">
                  <span :class="d.blocked_requests > 0 ? 'text-danger fw-medium' : 'text-muted'">
                    {{ d.blocked_requests.toLocaleString('fr-FR') }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import api from '../../api'
import type { HostExposure } from '../../types/host'
import { getApiErrorMessage } from '../../api/client'
import { formatBytes } from '../../utils/formatters'

const props = defineProps<{ hostId: string }>()

const PERIODS = [
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]

const exposure = ref<HostExposure | null>(null)
const loading = ref(false)
const error = ref('')
const period = ref('24h')

const periodLabel = computed(() => PERIODS.find((p) => p.value === period.value)?.label ?? period.value)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getHostExposure(props.hostId, period.value)
    exposure.value = res.data
  } catch (err: unknown) {
    error.value = getApiErrorMessage(err, 'Erreur de chargement')
  } finally {
    loading.value = false
  }
}

watch(period, load)
onMounted(load)
</script>
