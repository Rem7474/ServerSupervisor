<template>
  <div>
    <div v-if="showInitialLoading">
      <LoadingSkeleton
        variant="card"
        :lines="2"
        class="mb-4"
      />

      <LoadingSkeleton
        variant="kpi"
        :lines="4"
      />

      <div class="row row-cards mb-4">
        <div class="col-xl-7">
          <LoadingSkeleton variant="chart" />
        </div>
        <div class="col-xl-5">
          <LoadingSkeleton variant="donut" />
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-xl-7">
          <LoadingSkeleton
            variant="table"
            :lines="7"
          />
        </div>
        <div class="col-xl-5">
          <LoadingSkeleton
            variant="table"
            :lines="7"
          />
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-xl-8">
          <LoadingSkeleton
            variant="card"
            :lines="4"
          />
        </div>
        <div class="col-xl-4">
          <LoadingSkeleton
            variant="table"
            :lines="8"
          />
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-xl-6">
          <LoadingSkeleton
            variant="card"
            :lines="4"
          />
        </div>
        <div class="col-xl-6">
          <LoadingSkeleton
            variant="table"
            :lines="8"
          />
        </div>
      </div>

      <LoadingSkeleton
        variant="table"
        :lines="8"
      />
    </div>

    <div v-else>
      <PageRefreshBar
        v-model="autoRefresh"
        label="Stats web"
        :interval-sec="REFRESH_INTERVAL_MS / 1000"
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
        :source-has-no-data="sourceHasNoData"
        @refresh="loadAll(true)"
        @search="handleSearch"
      />

      <TrafficKpiCards
        :traffic="traffic"
        :threats="threats"
        :compare="compare"
      />

      <div class="row row-cards mb-4">
        <div class="col-xl-7">
          <div class="card h-100">
            <div class="card-header d-flex justify-content-between align-items-center">
              <h3 class="card-title mb-0">
                Trafic - requêtes par tranche
              </h3>
              <span class="small text-secondary">Humain vs Bot</span>
            </div>
            <div
              class="card-body chart-body"
              style="height: 260px;"
            >
              <TrafficRequestsChart
                :timeseries="timeseries"
                :period="period"
                :chart-ready="chartReady"
                :loading="loading"
              />
            </div>
          </div>
        </div>
        <div class="col-xl-5">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Distribution HTTP
              </h3>
            </div>
            <div
              class="card-body chart-body"
              style="height: 260px;"
            >
              <TrafficStatusChart
                :status-distribution="statusDistribution"
                :chart-ready="chartReady"
                :loading="loading"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-xl-7">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Top endpoints
              </h3>
            </div>
            <div class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Méthode</th>
                    <th>Chemin</th>
                    <th class="text-end">
                      Req.
                    </th>
                    <th class="text-end">
                      Status
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!topEndpoints.length">
                    <td colspan="4">
                      <EmptyState title="Aucun endpoint sur la période." />
                    </td>
                  </tr>
                  <tr
                    v-for="(row, idx) in topEndpoints.slice(0, 12)"
                    :key="`${row.method}-${row.path}-${idx}`"
                  >
                    <td><span class="badge bg-blue-lt text-blue">{{ row.method }}</span></td>
                    <td
                      class="font-monospace small text-truncate endpoint-path"
                      :title="row.path"
                      style="max-width: 24rem;"
                    >
                      {{ row.path }}
                    </td>
                    <td class="text-end">
                      {{ numberFormat(row.hits || 0) }}
                    </td>
                    <td class="text-end">
                      <span
                        class="badge"
                        :class="statusClass(row.status)"
                      >{{ row.status }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div class="col-xl-5">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Top IPs suspectes
              </h3>
            </div>
            <div class="card-body p-0">
              <div
                v-if="!topThreatIPs.length"
                class="text-center text-secondary py-4"
              >
                Aucune IP suspecte.
              </div>
              <button
                v-for="ip in topThreatIPs.slice(0, 10)"
                v-else
                :key="ip.ip"
                type="button"
                class="btn clickable-row d-flex justify-content-between align-items-center px-3 py-2 border-bottom w-100 rounded-0 text-start"
                @click="openIP(ip.ip)"
              >
                <div>
                  <div class="font-monospace small">
                    {{ ip.ip }}
                  </div>
                  <div class="small text-secondary">
                    {{ ip.level || 'LOW' }} · chemins {{ ip.unique_paths || 0 }}
                  </div>
                </div>
                <span class="badge bg-red-lt text-red">{{ numberFormat(ip.hits || 0) }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mb-4 align-items-start">
        <div class="col-xl-8">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                Carte mondiale des IP clientes
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
                Pays les plus actifs
              </h3>
              <span class="small text-secondary">{{ numberFormat(countryDistribution.length) }} pays</span>
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
                    v-for="item in countryDistribution.slice(0, 20)"
                    :key="`country-${item.country}`"
                  >
                    <td>
                      <span class="small">{{ item.country || 'Inconnu' }}</span>
                    </td>
                    <td>
                      <span class="badge bg-azure-lt text-azure">{{ item.country_code || '--' }}</span>
                    </td>
                    <td class="text-end">
                      {{ numberFormat(Number(item.hits) || 0) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-xl-6">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                Répartition du trafic par domaine
              </h3>
            </div>
            <div class="card-body">
              <div
                v-if="!topProxyHosts.length"
                class="text-center text-secondary py-4"
              >
                Aucune donnée domaine.
              </div>
              <div v-else>
                <div
                  v-for="h in topProxyHosts.slice(0, 8)"
                  :key="h.vhost || h.host_id || h.host_name"
                  class="mb-2 rounded p-1"
                  :class="{ 'clickable-row': h.vhost }"
                  :tabindex="h.vhost ? 0 : undefined"
                  :role="h.vhost ? 'button' : undefined"
                  @click="h.vhost && openDomain(h.vhost)"
                  @keydown.enter="h.vhost && openDomain(h.vhost)"
                  @keydown.space.prevent="h.vhost && openDomain(h.vhost)"
                >
                  <div class="d-flex justify-content-between small mb-1">
                    <span class="font-monospace">{{ h.vhost || h.host_name || '(unknown)' }}</span>
                    <span>{{ numberFormat(h.hits || 0) }}</span>
                  </div>
                  <div
                    class="progress"
                    style="height: 6px;"
                  >
                    <div
                      class="progress-bar bg-blue"
                      :style="{ width: hostWidth(h.hits) + '%' }"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="col-xl-6">
          <div class="card h-100">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                Top domaines cibles
              </h3>
            </div>
            <div class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Domaine</th>
                    <th class="text-end">
                      Hits
                    </th>
                    <th class="text-end">
                      4xx
                    </th>
                    <th class="text-end">
                      5xx
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!topDomains.length">
                    <td colspan="4">
                      <EmptyState title="Aucune donnée de trafic." />
                    </td>
                  </tr>
                  <tr
                    v-for="item in topDomains.slice(0, 10)"
                    :key="item.domain"
                    :class="{ 'clickable-row': item.domain }"
                    :tabindex="item.domain ? 0 : undefined"
                    :role="item.domain ? 'button' : undefined"
                    @click="item.domain && openDomain(item.domain)"
                    @keydown.enter="item.domain && openDomain(item.domain)"
                    @keydown.space.prevent="item.domain && openDomain(item.domain)"
                  >
                    <td class="font-monospace small">
                      {{ item.domain || '(unknown)' }}
                    </td>
                    <td class="text-end">
                      {{ numberFormat(item.hits || 0) }}
                    </td>
                    <td class="text-end text-yellow">
                      {{ numberFormat(item.errors_4xx || 0) }}
                    </td>
                    <td class="text-end text-red">
                      {{ numberFormat(item.errors_5xx || 0) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title mb-0">
            Flux temps réel - dernières requêtes
          </h3>
          <span class="small text-secondary">auto-refresh {{ autoRefresh ? 'ON' : 'OFF' }}</span>
        </div>
        <div class="table-responsive">
          <table class="table table-sm table-vcenter card-table">
            <thead>
              <tr>
                <th>Heure</th>
                <th>IP</th>
                <th>Domaine cible</th>
                <th>Méthode</th>
                <th>Chemin</th>
                <th>Status</th>
                <th>Bytes</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!liveRequests.length">
                <td colspan="7">
                  <EmptyState title="Aucune requête récente." />
                </td>
              </tr>
              <tr
                v-for="(r, idx) in liveRequests.slice(0, 16)"
                :key="`${r.timestamp}-${idx}`"
              >
                <td class="small">
                  {{ formatDate(r.timestamp) }}
                </td>
                <td class="font-monospace small">
                  <button
                    type="button"
                    class="btn btn-link btn-sm p-0 font-monospace text-decoration-none"
                    @click="openIP(r.ip)"
                  >
                    {{ r.ip }}
                  </button>
                </td>
                <td class="small">
                  <router-link
                    v-if="r.host_id"
                    :to="`/hosts/${r.host_id}`"
                    class="text-decoration-none"
                  >
                    {{ r.domain || r.host || r.host_name || r.host_id }}
                  </router-link>
                  <span v-else>{{ r.domain || r.host || r.host_name || '-' }}</span>
                </td>
                <td><span class="badge bg-blue-lt text-blue">{{ r.method }}</span></td>
                <td
                  class="font-monospace small text-truncate live-path"
                  :title="r.path"
                  style="max-width: 28rem;"
                >
                  {{ r.path }}
                </td>
                <td>
                  <span
                    class="badge"
                    :class="statusClass(r.status)"
                  >{{ r.status }}</span>
                </td>
                <td class="small">
                  {{ formatBytes(r.bytes || 0) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

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

      <IPTimelineModal
        :show="showIPModal"
        :ip="selectedIP"
        :timeline="ipTimeline"
        :loading="ipTimelineLoading"
        read-only
        @close="closeIPModal"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import EmptyState from '../EmptyState.vue'
import TrafficThreatsFilterBar from './TrafficThreatsFilterBar.vue'
import TrafficKpiCards from './TrafficKpiCards.vue'
import TrafficWorldMap from './TrafficWorldMap.vue'
import TrafficRequestsChart from './TrafficRequestsChart.vue'
import TrafficStatusChart from './TrafficStatusChart.vue'
import { useTraffic } from '../../composables/useTraffic'
import DomainDetailsModal from './DomainDetailsModal.vue'
import IPTimelineModal from './IPTimelineModal.vue'

const {
  periodOptions,
  REFRESH_INTERVAL_MS,
  period,
  source,
  hostId,
  autoRefresh,
  loading,
  compare,
  timeseries,
  liveRequests,
  lastUpdatedAt,
  domainModal,
  showIPModal,
  selectedIP,
  ipTimelineLoading,
  ipTimeline,
  searchTerm,
  chartReady,
  traffic,
  threats,
  topDomains,
  topProxyHosts,
  topEndpoints,
  topThreatIPs,
  countryDistribution,
  statusDistribution,
  showInitialLoading,
  sourceHasNoData,
  numberFormat,
  formatBytes,
  formatDate,
  statusClass,
  hostWidth,
  setPeriod,
  loadAll,
  openDomain,
  openIP,
  closeIPModal,
  handleSearch,
} = useTraffic()
</script>

<style scoped>
@media (max-width: 768px) {
  .endpoint-path,
  .live-path {
    max-width: 12rem !important;
  }
}

/* Chart placeholder positioning */
.chart-body {
  position: relative;
}

.skeleton-fade-enter-active,
.skeleton-fade-leave-active {
  transition: opacity 0.2s ease;
}

.skeleton-fade-enter-from,
.skeleton-fade-leave-to {
  opacity: 0;
}
</style>
