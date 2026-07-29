<template>
  <div>
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
      <div class="row row-cards mb-3">
        <div class="col-6 col-sm-3">
          <div class="card card-sm">
            <div class="card-body">
              <div class="text-muted small">
                Adresse IP
              </div>
              <div class="h3 mb-0 font-monospace">
                {{ exposure.ip_address || '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-sm-3">
          <div class="card card-sm">
            <div class="card-body">
              <div class="text-muted small">
                Requêtes ({{ periodLabel }})
              </div>
              <div class="h3 mb-0">
                {{ exposure.total_requests.toLocaleString('fr-FR') }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-sm-3">
          <div class="card card-sm">
            <div class="card-body">
              <div class="text-muted small">
                Suspectes
              </div>
              <div
                class="h3 mb-0"
                :class="exposure.total_suspicious_requests > 0 ? 'text-warning' : ''"
              >
                {{ exposure.total_suspicious_requests.toLocaleString('fr-FR') }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-sm-3">
          <div class="card card-sm">
            <div class="card-body">
              <div class="text-muted small">
                Bloquées
              </div>
              <div
                class="h3 mb-0"
                :class="exposure.total_blocked_requests > 0 ? 'text-danger' : ''"
              >
                {{ exposure.total_blocked_requests.toLocaleString('fr-FR') }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="exposure.domains.length === 0"
        class="text-center text-muted py-4"
      >
        Aucun domaine NPM ne route vers {{ subjectLabel }}.
      </div>

      <div
        v-else
        class="table-responsive"
      >
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Domaine</th>
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
              :key="`${d.proxy_host_id}:${d.domain_name}`"
            >
              <td>
                <button
                  type="button"
                  class="btn btn-link btn-sm p-0 font-monospace small text-decoration-none"
                  title="Voir le détail des requêtes/menaces pour ce domaine"
                  @click="openDomain(d.domain_name)"
                >
                  {{ d.domain_name }}
                </button>
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
import { ref } from 'vue'
import api from '../../api'
import DomainDetailsModal from './DomainDetailsModal.vue'
import type { HostExposure } from '../../types/host'
import { formatBytes } from '../../utils/formatters'

const props = defineProps<{
  exposure: HostExposure | null
  loading: boolean
  error?: string
  period: string
  periodLabel: string
  // "cet hôte" / "ce guest" — the only sentence left that still needs to
  // name the subject, now that its IP has its own KPI card instead of being
  // buried in prose (see HostExposureTab.vue and GuestExposureCard.vue, the
  // two callers of this shared panel).
  subjectLabel: string
}>()

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
    const res = await api.getDomainDetails(domain, props.period)
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
</script>
