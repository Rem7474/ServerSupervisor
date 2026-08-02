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
      <div class="row row-cards mt-4">
        <div class="col-xl-8">
          <LoadingSkeleton variant="chart" />
        </div>
        <div class="col-xl-4">
          <LoadingSkeleton
            variant="table"
            :lines="6"
          />
        </div>
      </div>
      <div class="row row-cards mt-4">
        <div class="col-lg-6">
          <div class="card h-100">
            <div class="card-body">
              <LoadingSkeleton
                variant="table"
                :lines="5"
              />
            </div>
          </div>
        </div>
        <div class="col-lg-6">
          <div class="card h-100">
            <div class="card-body">
              <LoadingSkeleton
                variant="table"
                :lines="5"
              />
            </div>
          </div>
        </div>
      </div>
      <div class="row row-cards mt-4">
        <div class="col-12">
          <div class="card">
            <div class="card-body">
              <LoadingSkeleton
                variant="table"
                :lines="5"
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
                {{ (threats.suspicious_requests || 0).toLocaleString('fr-FR') }}
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
              <div class="h2 mb-0 text-orange">
                {{ (threats.suspicious_ips || 0).toLocaleString('fr-FR') }}
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
              <div class="h2 mb-0 text-orange">
                {{ (threats.targeted_hosts || 0).toLocaleString('fr-FR') }}
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
                {{ (threats.blocked_ips || 0).toLocaleString('fr-FR') }}
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
            <div class="table-responsive scroll-table">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>IP</th>
                    <th class="text-end">
                      <SortableHeader
                        label="Hits"
                        :active="ipSortKey === 'hits'"
                        :direction="ipSortDir"
                        @toggle="toggleIPSort('hits')"
                      />
                    </th>
                    <th class="text-end">
                      <SortableHeader
                        label="Chemins"
                        :active="ipSortKey === 'unique_paths'"
                        :direction="ipSortDir"
                        @toggle="toggleIPSort('unique_paths')"
                      />
                    </th>
                    <th class="text-end">
                      <SortableHeader
                        label="Domaines"
                        :active="ipSortKey === 'host_count'"
                        :direction="ipSortDir"
                        @toggle="toggleIPSort('host_count')"
                      />
                    </th>
                    <th>Niveau</th>
                    <th>Blocage</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!topIPs.length">
                    <td colspan="7">
                      <EmptyState title="Aucune IP suspecte sur la période." />
                    </td>
                  </tr>
                  <tr
                    v-for="ip in pagedTopIPs"
                    :key="ip.ip"
                  >
                    <td class="font-monospace small">
                      {{ ip.ip }}
                    </td>
                    <td class="text-end">
                      {{ (ip.hits || 0).toLocaleString('fr-FR') }}
                    </td>
                    <td class="text-end">
                      {{ (ip.unique_paths || 0).toLocaleString('fr-FR') }}
                    </td>
                    <td class="text-end">
                      {{ (ip.host_count || 0).toLocaleString('fr-FR') }}
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
            <div
              v-if="ipTotalPages > 1"
              class="card-footer d-flex align-items-center justify-content-between"
            >
              <div class="text-secondary small">
                Page {{ ipPage }} sur {{ ipTotalPages }} — {{ topIPs.length }} IPs
              </div>
              <PaginationNav
                :current-page="ipPage"
                :total-pages="ipTotalPages"
                @select="setIPPage"
              />
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
                v-for="p in pagedTopPaths"
                v-else
                :key="`${p.path}-${p.category}`"
                class="d-flex justify-content-between border-bottom px-3 py-2 top-path-row"
              >
                <div class="top-path-label">
                  <div class="font-monospace small">
                    {{ p.path }}
                  </div>
                  <div class="small text-secondary">
                    {{ p.category || 'Unknown' }}
                  </div>
                </div>
                <span class="badge bg-yellow-lt text-yellow flex-shrink-0">{{ (p.hits || 0).toLocaleString('fr-FR') }}</span>
              </div>
            </div>
            <div
              v-if="pathsTotalPages > 1"
              class="card-footer d-flex align-items-center justify-content-between"
            >
              <div class="text-secondary small">
                Page {{ pathsPage }} sur {{ pathsTotalPages }} — {{ topPaths.length }} chemins
              </div>
              <PaginationNav
                :current-page="pathsPage"
                :total-pages="pathsTotalPages"
                @select="setPathsPage"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mt-4 align-items-start">
        <div class="col-xl-8">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Carte mondiale des menaces
              </h3>
            </div>
            <div class="card-body">
              <TrafficWorldMap :country-distribution="countryDistribution" />
            </div>
          </div>
        </div>
        <div class="col-xl-4">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                Pays des IPs suspectes
              </h3>
              <span class="small text-secondary">{{ countryDistribution.length }} pays</span>
            </div>
            <div class="table-responsive scroll-table">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Pays</th>
                    <th>Code</th>
                    <th class="text-end">
                      Hits
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!countryDistribution.length">
                    <td colspan="3">
                      <EmptyState title="Aucune donnée pays." />
                    </td>
                  </tr>
                  <tr
                    v-for="item in countryDistribution"
                    :key="`country-${item.country}`"
                  >
                    <td class="small">
                      {{ item.country || 'Inconnu' }}
                    </td>
                    <td>
                      <span class="badge bg-azure-lt text-azure">{{ item.country_code || '--' }}</span>
                    </td>
                    <td class="text-end">
                      {{ (item.hits || 0).toLocaleString('fr-FR') }}
                    </td>
                  </tr>
                </tbody>
              </table>
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
            <div class="table-responsive scroll-table">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Domaine cible</th>
                    <th class="text-end">
                      <SortableHeader
                        label="Hits"
                        :active="hostSortKey === 'hits'"
                        :direction="hostSortDir"
                        @toggle="toggleHostSort('hits')"
                      />
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!mostTargetedHosts.length">
                    <td colspan="2">
                      <EmptyState title="Aucun domaine ciblé" />
                    </td>
                  </tr>
                  <tr
                    v-for="h in sortedMostTargetedHosts"
                    :key="h.host_id"
                    :class="{ 'clickable-row': h.host_id }"
                    :tabindex="h.host_id ? 0 : undefined"
                    :role="h.host_id ? 'button' : undefined"
                    @click="h.host_id && openDomain(h.host_id)"
                    @keydown.enter="h.host_id && openDomain(h.host_id)"
                    @keydown.space.prevent="h.host_id && openDomain(h.host_id)"
                  >
                    <td class="font-monospace small">
                      {{ h.host_name || h.host_id || '—' }}
                    </td>
                    <td class="text-end">
                      {{ (h.hits || 0).toLocaleString('fr-FR') }}
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
            <div class="table-responsive scroll-table">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>IP</th>
                    <th class="text-end">
                      <SortableHeader
                        label="Domaines"
                        :active="matrixSortKey === 'host_count'"
                        :direction="matrixSortDir"
                        @toggle="toggleMatrixSort('host_count')"
                      />
                    </th>
                    <th class="text-end">
                      <SortableHeader
                        label="Hits"
                        :active="matrixSortKey === 'hits'"
                        :direction="matrixSortDir"
                        @toggle="toggleMatrixSort('hits')"
                      />
                    </th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!ipHostMatrix.length">
                    <td colspan="4">
                      <EmptyState title="Pas de scan coordonné détecté" />
                    </td>
                  </tr>
                  <tr
                    v-for="m in sortedIpHostMatrix"
                    :key="m.ip"
                  >
                    <td class="font-monospace small">
                      {{ m.ip }}
                    </td>
                    <td class="text-end">
                      {{ (m.host_count || 0).toLocaleString('fr-FR') }}
                    </td>
                    <td class="text-end">
                      {{ (m.hits || 0).toLocaleString('fr-FR') }}
                    </td>
                    <td class="text-end">
                      <button
                        type="button"
                        class="btn btn-sm btn-outline-primary"
                        @click="openTimeline(m.ip)"
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
              <span class="badge bg-green-lt text-green fs-4">
                {{ crowdSecTotal.toLocaleString() }} décisions actives
              </span>
            </div>
            <div class="table-responsive scroll-table">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>
                      <SortableHeader
                        label="IP"
                        :active="crowdSecSortKey === 'ip'"
                        :direction="crowdSecSortDir"
                        @toggle="toggleCrowdSecSort('ip')"
                      />
                    </th>
                    <th>Décision</th>
                    <th>Scénario</th>
                    <th>Origine</th>
                    <th>
                      <SortableHeader
                        label="Pays"
                        :active="crowdSecSortKey === 'country'"
                        :direction="crowdSecSortDir"
                        @toggle="toggleCrowdSecSort('country')"
                      />
                    </th>
                    <th>AS / Opérateur</th>
                    <th>
                      <SortableHeader
                        label="Expiration"
                        :active="crowdSecSortKey === 'blocked_until'"
                        :direction="crowdSecSortDir"
                        @toggle="toggleCrowdSecSort('blocked_until')"
                      />
                    </th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="entry in sortedCrowdSecIPs"
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
import { computed, ref } from 'vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import EmptyState from '../EmptyState.vue'
import TrafficThreatsFilterBar from './TrafficThreatsFilterBar.vue'
import IPTimelineModal from './IPTimelineModal.vue'
import DomainDetailsModal from './DomainDetailsModal.vue'
import TrafficWorldMap from './TrafficWorldMap.vue'
import SortableHeader from '../common/SortableHeader.vue'
import PaginationNav from '../PaginationNav.vue'
import { useBot } from '../../composables/useBot'
import { usePagination } from '../../composables/usePagination'
import { compareValues } from '../../utils/sort'

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for aggregate web-logs data (no Go model)
type AnyRecord = Record<string, any>

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
  domainModal,
  searchTerm,
  threats,
  topPaths,
  mostTargetedHosts,
  ipHostMatrix,
  countryDistribution,
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
  handleSearch,
  handleBanFromModal,
  unblockCrowdSecEntry,
} = useBot()

// Client-side sort/pagination over each table's already-loaded data — every
// list here (topIPs, mostTargetedHosts, ipHostMatrix, crowdSecIPs) is server-
// capped (25/30/… rows) rather than genuinely paginated server-side, so this
// only reorders/chunks what's already in memory. crowdSecIPs specifically
// stays sort-only (no PaginationNav): its own "Affichage des N premières
// entrées sur M" footnote already says the rest isn't loaded, and a pager
// here would wrongly imply you could page through to it.
const ipSortKey = ref<'hits' | 'unique_paths' | 'host_count'>('hits')
const ipSortDir = ref<'asc' | 'desc'>('desc')
function toggleIPSort(key: typeof ipSortKey.value): void {
  if (ipSortKey.value === key) {
    ipSortDir.value = ipSortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    ipSortKey.value = key
    ipSortDir.value = 'desc'
  }
}
const sortedTopIPs = computed(() =>
  [...topIPs.value].sort((a: AnyRecord, b: AnyRecord) => compareValues(a[ipSortKey.value], b[ipSortKey.value], ipSortDir.value))
)
const { currentPage: ipPage, totalPages: ipTotalPages, pagedItems: pagedTopIPs, setPage: setIPPage } =
  usePagination({ items: sortedTopIPs, pageSize: 10 })

const topPathsArray = computed<AnyRecord[]>(() => [...topPaths.value])
const { currentPage: pathsPage, totalPages: pathsTotalPages, pagedItems: pagedTopPaths, setPage: setPathsPage } =
  usePagination({ items: topPathsArray, pageSize: 10 })

const hostSortKey = ref<'hits'>('hits')
const hostSortDir = ref<'asc' | 'desc'>('desc')
function toggleHostSort(key: typeof hostSortKey.value): void {
  hostSortDir.value = hostSortKey.value === key && hostSortDir.value === 'desc' ? 'asc' : 'desc'
  hostSortKey.value = key
}
const sortedMostTargetedHosts = computed(() =>
  [...mostTargetedHosts.value].sort((a: AnyRecord, b: AnyRecord) => compareValues(a[hostSortKey.value], b[hostSortKey.value], hostSortDir.value))
)

const matrixSortKey = ref<'host_count' | 'hits'>('hits')
const matrixSortDir = ref<'asc' | 'desc'>('desc')
function toggleMatrixSort(key: typeof matrixSortKey.value): void {
  if (matrixSortKey.value === key) {
    matrixSortDir.value = matrixSortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    matrixSortKey.value = key
    matrixSortDir.value = 'desc'
  }
}
const sortedIpHostMatrix = computed(() =>
  [...ipHostMatrix.value].sort((a: AnyRecord, b: AnyRecord) => compareValues(a[matrixSortKey.value], b[matrixSortKey.value], matrixSortDir.value))
)

const crowdSecSortKey = ref<'ip' | 'country' | 'blocked_until'>('blocked_until')
const crowdSecSortDir = ref<'asc' | 'desc'>('desc')
function toggleCrowdSecSort(key: typeof crowdSecSortKey.value): void {
  if (crowdSecSortKey.value === key) {
    crowdSecSortDir.value = crowdSecSortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    crowdSecSortKey.value = key
    crowdSecSortDir.value = 'asc'
  }
}
const sortedCrowdSecIPs = computed(() =>
  [...crowdSecIPs.value].sort((a: AnyRecord, b: AnyRecord) => compareValues(a[crowdSecSortKey.value], b[crowdSecSortKey.value], crowdSecSortDir.value))
)
</script>

<style scoped>
.top-path-row {
  gap: 0.5rem;
  align-items: flex-start;
}

.top-path-label {
  min-width: 0;
}

.top-path-row .font-monospace {
  overflow-wrap: anywhere;
}
</style>
