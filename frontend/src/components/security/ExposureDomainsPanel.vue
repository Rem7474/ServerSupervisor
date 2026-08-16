<template>
  <div>
    <div
      v-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <LoadingSkeleton
      v-if="loading && !exposure"
      variant="kpi"
    />

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

      <EmptyState
        v-if="exposure.domains.length === 0"
        :title="`Aucun domaine NPM ne route vers ${subjectLabel}.`"
      />

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
              <th>Disponibilité</th>
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
              v-for="row in domainRows"
              :key="`${row.d.proxy_host_id}:${row.d.domain_name}`"
            >
              <td>
                <button
                  type="button"
                  class="btn btn-link btn-sm p-0 font-monospace small fw-medium text-decoration-none"
                  title="Voir le détail des requêtes/menaces pour ce domaine"
                  @click="openDomain(row.d.domain_name)"
                >
                  {{ row.d.domain_name }}
                </button>
              </td>
              <td>{{ row.d.connection_name }}</td>
              <td>
                <span class="badge bg-secondary-lt text-secondary me-1">:{{ row.d.forward_port }}</span>
                <span
                  v-if="row.d.ssl_enabled"
                  class="badge bg-success-lt text-success me-1"
                >SSL</span>
                <span
                  v-if="!row.d.npm_enabled"
                  class="badge bg-danger-lt text-danger"
                >Désactivé</span>
              </td>
              <td>
                <router-link
                  v-if="row.probe || row.cert"
                  :to="`/monitoring/host/${row.d.proxy_host_id}`"
                  class="d-inline-flex flex-column gap-1 text-decoration-none"
                  title="Voir le détail monitoring de ce domaine"
                >
                  <span class="d-flex align-items-center gap-1">
                    <span
                      v-if="row.probe"
                      :class="['badge', probeBadge(row.probe)]"
                    >{{ probeStatusLabel(row.probe) }}</span>
                    <span
                      v-if="row.cert"
                      :class="['badge', daysBadge(row.cert.days_remaining)]"
                    >SSL {{ daysLabel(row.cert.days_remaining) }}</span>
                  </span>
                  <div
                    v-if="row.probe && probeHistory[row.probe.id]?.length"
                    class="d-flex align-items-end gap-1"
                    style="height: 20px; min-width: 110px;"
                  >
                    <div
                      v-for="tick in probeHistory[row.probe.id]"
                      :key="tick.id"
                      class="flex-fill rounded-1"
                      :class="tick.success ? 'bg-success' : 'bg-danger'"
                      style="height: 100%; min-width: 2px;"
                      :title="`${formatDateTime(tick.checked_at)} — ${tick.success ? 'OK' : 'KO'}`"
                    />
                  </div>
                </router-link>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td class="text-end">
                {{ row.d.requests.toLocaleString('fr-FR') }}
              </td>
              <td class="text-end">
                {{ formatBytes(row.d.bytes) }}
              </td>
              <td class="text-end text-muted">
                {{ row.d.errors_4xx.toLocaleString('fr-FR') }} / {{ row.d.errors_5xx.toLocaleString('fr-FR') }}
              </td>
              <td class="text-end">
                <span :class="row.d.suspicious_requests > 0 ? 'text-warning fw-medium' : 'text-muted'">
                  {{ row.d.suspicious_requests.toLocaleString('fr-FR') }}
                </span>
              </td>
              <td class="text-end">
                <span :class="row.d.blocked_requests > 0 ? 'text-danger fw-medium' : 'text-muted'">
                  {{ row.d.blocked_requests.toLocaleString('fr-FR') }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <DomainDetailsModal
      :show="domainModal.show.value"
      :domain="domainModal.domain.value"
      :loading="domainModal.loading.value"
      :error="domainModal.error.value"
      :details="domainModal.details.value"
      :period="domainModal.period.value"
      :filters="domainModal.filters"
      :sort-key="domainModal.sortKey.value"
      :sort-dir="domainModal.sortDir.value"
      :page="domainModal.page.value"
      :total-pages="domainModal.totalPages.value"
      :has-active-filters="domainModal.hasActiveFilters.value"
      :block-state="domainModal.blockState"
      @close="domainModal.close"
      @update-filter="domainModal.setFilter($event.key, $event.value)"
      @clear-filters="domainModal.clearFilters"
      @toggle-sort="domainModal.toggleSort"
      @update:page="domainModal.setPage"
      @block-ip="domainModal.blockIP($event.ip, $event.hostId)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import DomainDetailsModal from './DomainDetailsModal.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import { useDomainDetails } from '../../composables/useDomainDetails'
import { useUptimeProbes } from '../../composables/useUptimeProbes'
import { useSslCertificates } from '../../composables/useSslCertificates'
import type { HostExposure, HostExposedDomain } from '../../types/host'
import type { UptimeProbe } from '../../types/uptime'
import type { SSLCertificate } from '../../types/ssl'
import { formatBytes, formatDateTime } from '../../utils/formatters'

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

const domainModal = useDomainDetails()

function openDomain(domain: string): void {
  if (!domain) return
  domainModal.open(domain, { period: props.period })
}

// Each exposed domain is one NPM proxy host — if that same proxy host's
// monitoring toggle also created an uptime probe and/or SSL cert
// (npm_proxy_host_id, see useMonitoringOverview.ts for the identical merge
// key), surface its live status here instead of leaving the exposure tab
// blind to whether the domain it's reporting traffic for is actually up.
// Domains with no monitoring configured just show "—" — this never creates
// anything, only reads what already exists.
const { probes, probeBadge, probeStatusLabel, probeHistory } = useUptimeProbes()
const { certs, daysLabel, daysBadge } = useSslCertificates()

const probeByProxyHost = computed(() => {
  const map = new Map<string, UptimeProbe>()
  for (const p of probes.value) {
    if (p.npm_proxy_host_id) map.set(p.npm_proxy_host_id, p)
  }
  return map
})
const certByProxyHost = computed(() => {
  const map = new Map<string, SSLCertificate>()
  for (const c of certs.value) {
    if (c.npm_proxy_host_id) map.set(c.npm_proxy_host_id, c)
  }
  return map
})

interface DomainRow {
  d: HostExposedDomain
  probe?: UptimeProbe
  cert?: SSLCertificate
}

const domainRows = computed<DomainRow[]>(() =>
  (props.exposure?.domains ?? []).map((d) => ({
    d,
    probe: probeByProxyHost.value.get(d.proxy_host_id),
    cert: certByProxyHost.value.get(d.proxy_host_id),
  }))
)
</script>
