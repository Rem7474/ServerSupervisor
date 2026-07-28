<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Exposition"
      :interval-sec="EXPOSURE_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
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
                  >
                    <button
                      type="button"
                      class="btn btn-link btn-sm p-0 font-monospace small text-decoration-none"
                      title="Voir le détail des requêtes/menaces pour ce domaine"
                      @click="openDomain(name)"
                    >
                      {{ name }}
                    </button>
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

    <DomainDetailsModal
      :show="showDomainModal"
      :domain="selectedDomain"
      :loading="domainLoading"
      :details="domainDetails"
      :period="periodLabel"
      @close="closeDomainModal"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../../api'
import DomainDetailsModal from '../security/DomainDetailsModal.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import type { HostExposure } from '../../types/host'
import { getApiErrorMessage } from '../../api/client'
import { formatBytes } from '../../utils/formatters'

const props = defineProps<{ hostId: string }>()

const PERIODS = [
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]

const EXPOSURE_REFRESH_SEC = 60

const exposure = ref<HostExposure | null>(null)
const loading = ref(false)
const error = ref('')
const period = ref('24h')
const autoRefresh = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null

const periodLabel = computed(() => PERIODS.find((p) => p.value === period.value)?.label ?? period.value)

// The "Requêtes/Suspectes/Bloquées" KPIs and per-domain counts had no
// drill-down at all — reuses the same domain detail drawer Traffic/Threats
// already have (hits, top paths, top IPs, recent request log) instead of
// leaving these numbers unexplorable. Not host-scoped: exposure's own web
// traffic is domain-first (collected by the proxy's agent, not this host's),
// same reasoning as GetHostExposure itself — see root CLAUDE.md.
const showDomainModal = ref(false)
const selectedDomain = ref('')
const domainLoading = ref(false)
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- mirrors DomainDetailsModal's own ad-hoc details prop (no Go model for this aggregate)
const domainDetails = ref<Record<string, any>>({})

async function openDomain(domain: string): Promise<void> {
  if (!domain) return
  selectedDomain.value = domain
  showDomainModal.value = true
  domainLoading.value = true
  try {
    const res = await api.getDomainDetails(domain, period.value)
    domainDetails.value = res.data?.details || {}
  } catch {
    domainDetails.value = {}
  } finally {
    domainLoading.value = false
  }
}

function closeDomainModal(): void {
  showDomainModal.value = false
  selectedDomain.value = ''
  domainDetails.value = {}
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getHostExposure(props.hostId, period.value)
    exposure.value = res.data
    lastUpdatedAt.value = new Date()
  } catch (err: unknown) {
    error.value = getApiErrorMessage(err, 'Erreur de chargement')
  } finally {
    loading.value = false
  }
}

function startRefreshTimer(): void {
  stopRefreshTimer()
  refreshTimer = setInterval(() => {
    if (autoRefresh.value) load()
  }, EXPOSURE_REFRESH_SEC * 1000)
}

function stopRefreshTimer(): void {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}

watch(period, load)
onMounted(() => {
  load()
  startRefreshTimer()
})
onUnmounted(stopRefreshTimer)
</script>
