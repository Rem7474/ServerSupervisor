<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Menaces web"
      :interval-sec="BOT_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    >
      <div class="d-flex align-items-center gap-2 flex-wrap">
        <span class="small text-secondary">Période :</span>
        <button
          v-for="p in periodOptions"
          :key="p.value"
          type="button"
          class="btn btn-sm"
          :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="setPeriod(p.value)"
        >
          {{ p.label }}
        </button>
      </div>
    </PageRefreshBar>

    <TrafficThreatsFilterBar
      v-model:source="source"
      v-model:host-id="hostId"
      v-model:search-term="searchTerm"
      :loading="loading"
      @refresh="loadThreats"
      @search="handleSearch"
    />

    <!-- Squelette chargement -->
    <template v-if="loading">
      <LoadingSkeleton
        variant="kpi"
        :lines="4"
        class="mb-4"
      />
      <div class="row row-cards">
        <div class="col-lg-7">
          <div class="card h-100">
            <div class="card-body">
              <LoadingSkeleton
                variant="table"
                :lines="6"
              />
            </div>
          </div>
        </div>
        <div class="col-lg-5">
          <div class="card h-100">
            <div class="card-body">
              <LoadingSkeleton
                variant="table"
                :lines="6"
              />
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Contenu réel -->
    <template v-else>
      <div class="row row-cards mb-4">
        <div class="col-12 col-sm-3">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                Requêtes suspectes
              </div>
              <div class="h2 mb-0 text-orange">
                {{ threats.suspicious_requests || 0 }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-sm-3">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                IPs suspectes
              </div>
              <div class="h2 mb-0">
                {{ threats.suspicious_ips || 0 }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-sm-3">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                Domaines ciblés
              </div>
              <div class="h2 mb-0">
                {{ threats.targeted_hosts || 0 }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-sm-3">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                IPs bloquées
              </div>
              <div class="h2 mb-0 text-success">
                {{ threats.blocked_ips || 0 }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards">
        <div class="col-lg-7">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                IPs suspectes
              </h3>
            </div>
            <div class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>IP</th>
                    <th class="text-end">
                      Hits
                    </th>
                    <th class="text-end">
                      Chemins
                    </th>
                    <th class="text-end">
                      Domaines
                    </th>
                    <th>Niveau</th>
                    <th>Blocage</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!topIPs.length">
                    <td
                      colspan="7"
                      class="text-center text-secondary py-4"
                    >
                      Aucune IP suspecte sur la période.
                    </td>
                  </tr>
                  <tr
                    v-for="ip in topIPs"
                    :key="ip.ip"
                  >
                    <td class="font-monospace small">
                      {{ ip.ip }}
                    </td>
                    <td class="text-end">
                      {{ ip.hits || 0 }}
                    </td>
                    <td class="text-end">
                      {{ ip.unique_paths || 0 }}
                    </td>
                    <td class="text-end">
                      {{ ip.host_count || 0 }}
                    </td>
                    <td>
                      <span
                        class="badge"
                        :class="levelClass(ip.level)"
                      >{{ ip.level || 'LOW' }}</span>
                    </td>
                    <td>
                      <template v-if="ip.blocked || ip.blocked_type || ip.blocked_source">
                        <span
                          class="badge"
                          :class="decisionBadgeClass(ip.blocked_type, ip.blocked_until)"
                          :title="formatBlockedUntil(ip.blocked_until)"
                        >
                          {{ decisionLabel(ip.blocked_type) }}
                        </span>
                      </template>
                      <span
                        v-else
                        class="text-secondary small"
                      >
                        —
                      </span>
                    </td>
                    <td class="text-end">
                      <button
                        type="button"
                        class="btn btn-sm btn-outline-primary"
                        @click="openTimeline(ip.ip)"
                      >
                        Timeline
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div class="col-lg-5">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Top chemins scannés
              </h3>
            </div>
            <div class="card-body p-0">
              <div
                v-if="!topPaths.length"
                class="text-center py-4 text-secondary small"
              >
                Aucun chemin suspect.
              </div>
              <div
                v-for="p in topPaths"
                v-else
                :key="`${p.path}-${p.category}`"
                class="d-flex justify-content-between border-bottom px-3 py-2 top-path-row"
              >
                <div>
                  <div class="font-monospace small">
                    {{ p.path }}
                  </div>
                  <div class="small text-secondary">
                    {{ p.category || 'Unknown' }}
                  </div>
                </div>
                <span class="badge bg-yellow-lt text-yellow">{{ p.hits }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mt-4">
        <div class="col-lg-6">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Domaines les plus ciblés
              </h3>
            </div>
            <div class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Domaine cible</th>
                    <th class="text-end">
                      Hits
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!mostTargetedHosts.length">
                    <td
                      colspan="2"
                      class="text-center text-secondary py-4"
                    >
                      Aucun domaine ciblé
                    </td>
                  </tr>
                  <tr
                    v-for="h in mostTargetedHosts"
                    :key="h.host_id"
                  >
                    <td>
                      <button
                        v-if="h.host_id"
                        type="button"
                        class="btn btn-link p-0 text-decoration-none"
                        @click="openDomain(h.host_id)"
                      >
                        {{ h.host_name || h.host_id }}
                      </button>
                      <span v-else>{{ h.host_name || '—' }}</span>
                    </td>
                    <td class="text-end">
                      {{ h.hits || 0 }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div class="col-lg-6">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                IP × Domaines (scan coordonné)
              </h3>
            </div>
            <div class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>IP</th>
                    <th class="text-end">
                      Domaines
                    </th>
                    <th class="text-end">
                      Hits
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!ipHostMatrix.length">
                    <td
                      colspan="3"
                      class="text-center text-secondary py-4"
                    >
                      Pas de scan coordonné détecté
                    </td>
                  </tr>
                  <tr
                    v-for="m in ipHostMatrix"
                    :key="m.ip"
                  >
                    <td class="font-monospace small">
                      {{ m.ip }}
                    </td>
                    <td class="text-end">
                      {{ m.host_count || 0 }}
                    </td>
                    <td class="text-end">
                      {{ m.hits || 0 }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="crowdSecIPs.length"
        class="row row-cards mt-4"
      >
        <div class="col-12">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
              <h3 class="card-title mb-0">
                IPs bloquées par CrowdSec
              </h3>
              <span class="badge bg-success-lt text-success fs-4">
                {{ crowdSecTotal.toLocaleString() }} décisions actives
              </span>
            </div>
            <div class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>IP</th>
                    <th>Action</th>
                    <th>Scénario</th>
                    <th>Origine</th>
                    <th>Pays</th>
                    <th>AS / Opérateur</th>
                    <th>Expiration</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="entry in crowdSecIPs"
                    :key="entry.ip"
                  >
                    <td class="font-monospace small">
                      {{ entry.ip }}
                    </td>
                    <td>
                      <span
                        class="badge"
                        :class="decisionBadgeClass(entry.type, entry.blocked_until)"
                      >{{ decisionLabel(entry.type) }}</span>
                    </td>
                    <td class="small text-secondary">
                      {{ entry.reason || '—' }}
                    </td>
                    <td>
                      <span
                        class="badge"
                        :class="entry.origin === 'CAPI' ? 'bg-azure-lt text-azure' : 'bg-purple-lt text-purple'"
                      >{{ entry.origin || '—' }}</span>
                    </td>
                    <td class="small">
                      {{ entry.country || '—' }}
                    </td>
                    <td
                      class="small text-secondary"
                      :title="entry.as_name"
                    >
                      {{ entry.as_name ? truncate(entry.as_name, 28) : '—' }}
                    </td>
                    <td class="small">
                      {{ formatBlockedUntil(entry.blocked_until) }}
                    </td>
                    <td class="text-end">
                      <div class="d-flex gap-1 justify-content-end">
                        <button
                          type="button"
                          class="btn btn-sm"
                          :class="rowState[entry.ip] === 'error' ? 'btn-danger' : 'btn-outline-success'"
                          :disabled="rowState[entry.ip] === 'loading'"
                          @click="unblockCrowdSecEntry(entry.ip)"
                        >
                          <span
                            v-if="rowState[entry.ip] === 'loading'"
                            class="spinner-border spinner-border-sm me-1"
                          />
                          <span v-if="rowState[entry.ip] === 'loading'">Déblocage…</span>
                          <span v-else-if="rowState[entry.ip] === 'error'">Erreur — Réessayer</span>
                          <span v-else>Débloquer</span>
                        </button>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline-primary"
                          @click="openTimeline(entry.ip)"
                        >
                          Timeline
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div
              v-if="crowdSecTotal > crowdSecIPs.length"
              class="card-footer text-secondary small"
            >
              Affichage des {{ crowdSecIPs.length }} premières entrées sur {{ crowdSecTotal.toLocaleString() }} IPs bloquées
            </div>
          </div>
        </div>
      </div>
    </template><!-- fin v-else contenu réel -->

    <IPTimelineModal
      :show="showTimeline"
      :ip="selectedIP"
      :timeline="timeline"
      :loading="timelineLoading"
      :blocked="isSelectedIPBlocked"
      :ban-loading="banState === 'loading'"
      :ban-error="banState === 'error'"
      :host-id="effectiveHostId"
      @close="closeTimeline"
      @ban="handleBanFromModal"
    />

    <DomainDetailsModal
      :show="showDomainModal"
      :domain="selectedDomain"
      :loading="domainLoading"
      :details="domainDetails"
      :period="period"
      @close="closeDomainModal"
    />
  </div>
</template>

<script setup lang="ts">
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import TrafficThreatsFilterBar from './TrafficThreatsFilterBar.vue'
import IPTimelineModal from './IPTimelineModal.vue'
import DomainDetailsModal from './DomainDetailsModal.vue'
import { useBot } from '../../composables/useBot'

const {
  period,
  periodOptions,
  source,
  hostId,
  loading,
  autoRefresh,
  lastUpdatedAt,
  BOT_REFRESH_SEC,
  showTimeline,
  timelineLoading,
  banState,
  selectedIP,
  timeline,
  showDomainModal,
  selectedDomain,
  domainLoading,
  domainDetails,
  searchTerm,
  threats,
  topPaths,
  mostTargetedHosts,
  ipHostMatrix,
  rowState,
  crowdSecIPs,
  crowdSecTotal,
  topIPs,
  isSelectedIPBlocked,
  effectiveHostId,
  levelClass,
  decisionLabel,
  decisionBadgeClass,
  truncate,
  formatBlockedUntil,
  loadThreats,
  setPeriod,
  openTimeline,
  closeTimeline,
  openDomain,
  closeDomainModal,
  handleSearch,
  handleBanFromModal,
  unblockCrowdSecEntry,
} = useBot()
</script>

<style scoped>
.top-path-row {
  gap: 0.5rem;
}

@media (max-width: 992px) {
  .top-path-row {
    align-items: flex-start;
  }

  .top-path-row .font-monospace {
    overflow-wrap: anywhere;
  }
}
</style>
