<template>
  <div>
    <div
      v-if="showInitialLoading"
      class="traffic-page-skeleton"
    >
      <div class="traffic-topbar mb-3">
        <div class="traffic-topbar-skeleton-left">
          <span class="traffic-topbar-skeleton-dot" />
          <span class="traffic-topbar-skeleton-line traffic-topbar-skeleton-line-title" />
          <span class="traffic-topbar-skeleton-pill traffic-topbar-skeleton-pill-wide" />
          <span class="traffic-topbar-skeleton-line traffic-topbar-skeleton-line-meta" />
        </div>
        <div class="traffic-topbar-skeleton-right">
          <span
            v-for="n in 4"
            :key="n"
            class="traffic-topbar-skeleton-pill"
            :class="n === 1 ? 'traffic-topbar-skeleton-pill-active' : ''"
          />
        </div>
      </div>

      <div class="page-header mb-4">
        <LoadingSkeleton
          variant="card"
          :lines="3"
        />
      </div>

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

      <div class="page-header mb-4">
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>Stats web</span>
        </div>
        <h2 class="page-title">
          Stats web
        </h2>
        <div class="text-secondary">
          Trafic HTTP, erreurs, endpoints, géographie des clients et actualisation automatique
        </div>
      </div>

      <div class="card mb-4">
        <div class="card-body d-flex flex-wrap gap-2 align-items-end traffic-filters">
          <div class="traffic-filter-field">
            <label class="form-label mb-1">Source</label>
            <div class="input-group input-group-sm">
              <select
                v-model="source"
                class="form-select form-select-sm"
                :disabled="loading"
                style="min-width: 9rem;"
              >
                <option value="">
                  Toutes
                </option>
                <option value="npm">
                  npm
                </option>
                <option value="nginx">
                  nginx
                </option>
                <option value="apache">
                  apache
                </option>
                <option value="caddy">
                  caddy
                </option>
              </select>
              <span
                v-if="loading"
                class="input-group-text px-2"
              >
                <span
                  class="spinner-border"
                  style="width:.75rem;height:.75rem;border-width:2px"
                />
              </span>
              <span
                v-else-if="sourceHasNoData"
                class="input-group-text px-2 text-warning"
                title="Aucune donnée pour cette source sur la période sélectionnée"
              >
                <IconAlertTriangle :size="14" />
              </span>
            </div>
          </div>

          <div class="traffic-filter-field">
            <label class="form-label mb-1">Hôte</label>
            <select
              v-model="hostId"
              class="form-select form-select-sm"
              :disabled="loading"
              style="min-width: 12rem;"
            >
              <option value="">
                Tous les hôtes
              </option>
              <option
                v-for="h in hostsStore.hosts"
                :key="h.id"
                :value="h.id"
              >
                {{ h.name || h.hostname || h.ip_address }}
              </option>
            </select>
          </div>

          <div class="form-check form-switch mb-1 ms-1">
            <input
              id="auto-refresh"
              v-model="autoRefresh"
              class="form-check-input"
              type="checkbox"
            >
            <label
              class="form-check-label small"
              for="auto-refresh"
            >Rafraîchissement auto</label>
          </div>

          <button
            type="button"
            class="btn btn-primary btn-sm traffic-refresh-btn"
            :disabled="loading"
            @click="loadAll(true)"
          >
            <span
              v-if="loading"
              class="spinner-border spinner-border-sm me-1"
            />
            Rafraîchir
          </button>

          <div class="traffic-filter-field ms-auto">
            <label class="form-label mb-1">Rechercher un domaine ou une IP</label>
            <div class="input-group input-group-sm">
              <input
                v-model="searchTerm"
                type="text"
                class="form-control form-control-sm"
                placeholder="exemple.com ou 1.2.3.4"
                style="min-width: 16rem;"
                @keyup.enter="handleSearch"
              >
              <button
                type="button"
                class="btn btn-outline-secondary btn-sm"
                :disabled="!searchTerm.trim()"
                @click="handleSearch"
              >
                Voir les requêtes
              </button>
            </div>
          </div>
        </div>
      </div>

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
                    <td
                      colspan="4"
                      class="text-center text-secondary py-4"
                    >
                      Aucun endpoint sur la période.
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
              <div
                v-for="ip in topThreatIPs.slice(0, 10)"
                v-else
                :key="ip.ip"
                class="d-flex justify-content-between align-items-center px-3 py-2 border-bottom"
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
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-xl-8">
          <div class="card h-100">
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
          <div class="card h-100">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title mb-0">
                Pays les plus actifs
              </h3>
              <span class="small text-secondary">{{ numberFormat(topClientIPs.length) }} IPs</span>
            </div>
            <div class="table-responsive">
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
                    <td
                      colspan="3"
                      class="text-center text-secondary py-4"
                    >
                      Aucune donnée pays.
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
                Top domaines cibles (proxy)
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
                  class="mb-2"
                >
                  <div class="d-flex justify-content-between small mb-1">
                    <router-link
                      v-if="h.host_id"
                      :to="`/hosts/${h.host_id}`"
                      class="font-monospace text-decoration-none"
                    >
                      {{ h.vhost || h.host_name || h.host_id }}
                    </router-link>
                    <span
                      v-else
                      class="font-monospace"
                    >{{ h.vhost || h.host_name || '(unknown)' }}</span>
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
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!topDomains.length">
                    <td
                      colspan="5"
                      class="text-center text-secondary py-4"
                    >
                      Aucune donnée de trafic.
                    </td>
                  </tr>
                  <tr
                    v-for="item in topDomains.slice(0, 10)"
                    :key="item.domain"
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
                    <td class="text-end">
                      <button
                        type="button"
                        class="btn btn-sm btn-outline-primary"
                        @click="openDomain(item.domain)"
                      >
                        Détails
                      </button>
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
                <td
                  colspan="7"
                  class="text-center text-secondary py-4"
                >
                  Aucune requête récente.
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
                  {{ r.ip }}
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
        :show="showDomainModal"
        :domain="selectedDomain"
        :loading="domainLoading"
        :details="domainDetails"
        :period="period"
        @close="closeDomainModal"
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
import { IconAlertTriangle } from '@tabler/icons-vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import TrafficKpiCards from '../components/security/TrafficKpiCards.vue'
import TrafficWorldMap from '../components/security/TrafficWorldMap.vue'
import TrafficRequestsChart from '../components/security/TrafficRequestsChart.vue'
import TrafficStatusChart from '../components/security/TrafficStatusChart.vue'
import { useTraffic } from '../composables/useTraffic'
import DomainDetailsModal from '../components/security/DomainDetailsModal.vue'
import IPTimelineModal from '../components/security/IPTimelineModal.vue'

const {
  hostsStore,
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
  showDomainModal,
  selectedDomain,
  domainLoading,
  domainDetails,
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
  topClientIPs,
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
  closeDomainModal,
  closeIPModal,
  handleSearch,
} = useTraffic()
</script>

<style scoped>
.traffic-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.traffic-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1060;
  padding: 1rem;
}

.traffic-modal {
  width: min(1200px, 96vw);
  max-height: 92vh;
  overflow: auto;
}

@media (max-width: 992px) {
  .traffic-filters {
    align-items: stretch !important;
  }

  .traffic-filter-field {
    flex: 1 1 220px;
  }

  .traffic-filter-field .form-select,
  .traffic-filter-field .form-control {
    min-width: 0 !important;
    width: 100%;
  }

  .traffic-refresh-btn {
    width: 100%;
  }
}

@media (max-width: 768px) {
  .traffic-modal-backdrop {
    padding: 0;
  }

  .traffic-modal {
    width: 100vw;
    max-height: 100dvh;
    height: 100dvh;
    border-radius: 0;
  }

  .traffic-modal-body {
    padding: 0.75rem;
  }

  .endpoint-path,
  .live-path,
  .domain-path,
  .domain-ua {
    max-width: 12rem !important;
  }
}

/* Chart placeholder positioning */
.chart-body {
  position: relative;
}

.traffic-page-skeleton .traffic-topbar-skeleton-left,
.traffic-page-skeleton .traffic-topbar-skeleton-right {
  flex: 1 1 0;
  min-width: 0;
}

.traffic-page-skeleton .traffic-topbar-skeleton-left {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.traffic-page-skeleton .traffic-topbar-skeleton-right {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.traffic-topbar-skeleton-dot,
.traffic-topbar-skeleton-line,
.traffic-topbar-skeleton-pill {
  background: linear-gradient(90deg, rgba(203, 213, 225, 0.2) 25%, rgba(203, 213, 225, 0.5) 50%, rgba(203, 213, 225, 0.2) 75%);
  background-size: 220% 100%;
  animation: loading-skeleton-wave 1.4s ease infinite;
}

.traffic-topbar-skeleton-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  flex-shrink: 0;
}

.traffic-topbar-skeleton-line {
  height: 0.8rem;
  border-radius: 999px;
}

.traffic-topbar-skeleton-line-title {
  width: 110px;
}

.traffic-topbar-skeleton-line-meta {
  width: 120px;
}

.traffic-topbar-skeleton-pill {
  width: 88px;
  height: 2rem;
  border-radius: 999px;
  flex-shrink: 0;
}

.traffic-topbar-skeleton-pill-wide {
  width: 82px;
}

.traffic-topbar-skeleton-pill-active {
  width: 96px;
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

