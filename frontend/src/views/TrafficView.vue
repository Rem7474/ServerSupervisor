<template>
  <div>
    <div class="traffic-topbar mb-3">
      <div class="d-flex align-items-center gap-2">
        <span
          class="live-dot"
          :class="{ paused: !autoRefresh }"
        />
        <span class="fw-semibold">Stats web</span>
        <span class="badge bg-green-lt text-green">{{ autoRefresh ? 'Live' : 'Pause' }}</span>
        <span class="text-secondary small">dernière MAJ {{ lastUpdatedLabel }}</span>
      </div>
      <div class="d-flex align-items-center gap-2 flex-wrap">
        <span class="small text-secondary">Période :</span>
        <button
          v-for="p in periodOptions"
          :key="p.value"
          class="btn btn-sm"
          :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="setPeriod(p.value)"
        >
          {{ p.label }}
        </button>
      </div>
    </div>

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
        Trafic HTTP, erreurs, endpoints, géographie des clients et suivi live
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-body d-flex flex-wrap gap-2 align-items-end traffic-filters">
        <div class="traffic-filter-field">
          <label class="form-label mb-1">Source</label>
          <select
            v-model="source"
            class="form-select form-select-sm"
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
        </div>

        <div class="traffic-filter-field">
          <label class="form-label mb-1">Hôte technique (ID)</label>
          <input
            v-model.trim="hostId"
            class="form-control form-control-sm"
            placeholder="(optionnel)"
            style="min-width: 14rem;"
          >
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
      </div>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Requêtes totales
            </div>
            <div class="h2 mb-0">
              {{ numberFormat(traffic.total_requests || 0) }}
            </div>
            <div
              class="small mt-1"
              :class="deltaClass('total_requests')"
            >
              {{ deltaLabel('total_requests') }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Bande passante
            </div>
            <div class="h2 mb-0">
              {{ formatBytes(traffic.total_bytes || 0) }}
            </div>
            <div
              class="small mt-1"
              :class="deltaClass('total_bytes')"
            >
              {{ deltaLabel('total_bytes') }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Taux 5xx
            </div>
            <div class="h2 mb-0 text-red">
              {{ percent(traffic.ratio_5xx) }}
            </div>
            <div
              class="small mt-1"
              :class="deltaClass('ratio_5xx')"
            >
              {{ deltaLabel('ratio_5xx') }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              IPs suspectes
            </div>
            <div class="h2 mb-0">
              {{ numberFormat(threats.suspicious_ips || 0) }}
            </div>
            <div
              class="small mt-1"
              :class="deltaClass('suspicious_ips')"
            >
              {{ deltaLabel('suspicious_ips') }}
            </div>
          </div>
        </div>
      </div>
    </div>

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
            class="card-body"
            style="height: 260px;"
          >
            <canvas ref="trafficCanvas" />
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
            class="card-body"
            style="height: 260px;"
          >
            <canvas ref="statusCanvas" />
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
            <svg
              ref="worldMapSvg"
              class="world-map"
              role="img"
              aria-label="Carte mondiale du trafic par pays"
            />
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
                  <span class="font-monospace">{{ h.vhost || h.host_name || h.host_id || '(unknown)' }}</span>
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
                {{ r.domain || r.host || r.host_name || r.host_id || '-' }}
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

    <div
      v-if="showDomainModal"
      class="traffic-modal-backdrop"
      @click.self="closeDomainModal"
    >
      <div class="traffic-modal card shadow-lg">
        <div class="card-header d-flex align-items-center justify-content-between">
          <div>
            <h3 class="card-title mb-0">
              Détails domaine: <span class="font-monospace">{{ selectedDomain }}</span>
            </h3>
            <div class="text-secondary small">
              Fenêtre de logs détaillée sur {{ period }}
            </div>
          </div>
          <button
            class="btn btn-sm btn-outline-secondary"
            @click="closeDomainModal"
          >
            Fermer
          </button>
        </div>

        <div class="card-body traffic-modal-body">
          <div
            v-if="domainLoading"
            class="text-center py-4 text-secondary"
          >
            <span class="spinner-border spinner-border-sm me-2" />
            Chargement des détails...
          </div>

          <template v-else>
            <div class="row row-cards mb-3">
              <div class="col-6 col-lg-3">
                <div class="border rounded p-2 text-center">
                  <div class="text-secondary small">
                    Hits
                  </div>
                  <div class="h3 mb-0">
                    {{ domainDetails.hits || 0 }}
                  </div>
                </div>
              </div>
              <div class="col-6 col-lg-3">
                <div class="border rounded p-2 text-center">
                  <div class="text-secondary small">
                    Bytes
                  </div>
                  <div class="h3 mb-0">
                    {{ formatBytes(domainDetails.bytes || 0) }}
                  </div>
                </div>
              </div>
              <div class="col-6 col-lg-3">
                <div class="border rounded p-2 text-center">
                  <div class="text-secondary small">
                    4xx
                  </div>
                  <div class="h3 mb-0 text-yellow">
                    {{ domainDetails.status_4xx || 0 }}
                  </div>
                </div>
              </div>
              <div class="col-6 col-lg-3">
                <div class="border rounded p-2 text-center">
                  <div class="text-secondary small">
                    5xx
                  </div>
                  <div class="h3 mb-0 text-red">
                    {{ domainDetails.status_5xx || 0 }}
                  </div>
                </div>
              </div>
            </div>

            <div class="row row-cards mb-3">
              <div class="col-lg-6">
                <div class="card h-100">
                  <div class="card-header">
                    <h4 class="card-title mb-0">
                      Top chemins
                    </h4>
                  </div>
                  <div class="card-body p-0">
                    <div
                      v-if="!(domainDetails.top_paths || []).length"
                      class="text-center py-3 text-secondary small"
                    >
                      Aucun chemin
                    </div>
                    <div
                      v-for="p in domainDetails.top_paths"
                      v-else
                      :key="p.path"
                      class="d-flex justify-content-between border-bottom px-3 py-2"
                    >
                      <span
                        class="font-monospace small text-truncate me-2"
                        style="max-width: 75%;"
                      >{{ p.path }}</span>
                      <span class="badge bg-azure-lt text-azure">{{ p.hits }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="col-lg-6">
                <div class="card h-100">
                  <div class="card-header">
                    <h4 class="card-title mb-0">
                      Top IPs clientes
                    </h4>
                  </div>
                  <div class="card-body p-0">
                    <div
                      v-if="!(domainDetails.top_clients || []).length"
                      class="text-center py-3 text-secondary small"
                    >
                      Aucune IP
                    </div>
                    <div
                      v-for="ip in domainDetails.top_clients"
                      v-else
                      :key="ip.ip"
                      class="d-flex justify-content-between border-bottom px-3 py-2"
                    >
                      <span class="font-monospace small">{{ ip.ip }}</span>
                      <span class="badge bg-purple-lt text-purple">{{ ip.hits }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="card">
              <div class="card-header">
                <h4 class="card-title mb-0">
                  Logs récents
                </h4>
              </div>
              <div
                class="table-responsive"
                style="max-height: 360px;"
              >
                <table class="table table-sm table-vcenter mb-0">
                  <thead>
                    <tr>
                      <th>Heure</th>
                      <th>IP</th>
                      <th>Méthode</th>
                      <th>Chemin</th>
                      <th>Status</th>
                      <th>Bytes</th>
                      <th>UA</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-if="!(domainDetails.requests || []).length">
                      <td
                        colspan="7"
                        class="text-center text-secondary py-3"
                      >
                        Aucune requête disponible
                      </td>
                    </tr>
                    <tr
                      v-for="(r, idx) in domainDetails.requests || []"
                      :key="`${r.timestamp}-${idx}`"
                    >
                      <td class="small">
                        {{ formatDate(r.timestamp) }}
                      </td>
                      <td class="font-monospace small">
                        {{ r.ip }}
                      </td>
                      <td><span class="badge bg-blue-lt text-blue">{{ r.method }}</span></td>
                      <td
                        class="font-monospace small text-truncate domain-path"
                        :title="r.path"
                        style="max-width: 18rem;"
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
                      <td
                        class="small text-truncate domain-ua"
                        :title="r.user_agent || '-'"
                        style="max-width: 20rem;"
                      >
                        {{ r.user_agent || '-' }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { max as d3Max } from 'd3-array'
import { geoNaturalEarth1, geoPath } from 'd3-geo'
import { scaleSequential } from 'd3-scale'
import { interpolateYlOrRd } from 'd3-scale-chromatic'
import { select } from 'd3-selection'
import { feature } from 'topojson-client'
import apiClient from '../api'

type AnyRecord = Record<string, any>

const periodOptions = [
  { value: '1h', label: '1h' },
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]

const period = ref('24h')
const source = ref('')
const hostId = ref('')
const autoRefresh = ref(true)

const loading = ref(false)
const summary = ref<AnyRecord>({ traffic: {}, threats: {} })
const compare = ref<AnyRecord>({ delta_percent: {} })
const timeseries = ref<AnyRecord[]>([])
const liveRequests = ref<AnyRecord[]>([])
const lastUpdatedAt = ref<Date | null>(null)

const showDomainModal = ref(false)
const selectedDomain = ref('')
const domainLoading = ref(false)
const domainDetails = ref<AnyRecord>({})

const trafficCanvas = ref<HTMLCanvasElement | null>(null)
const statusCanvas = ref<HTMLCanvasElement | null>(null)
const worldMapSvg = ref<SVGSVGElement | null>(null)
let trafficChart: any = null
let statusChart: any = null
let refreshTimer: number | null = null
let chartLib: any = null
let worldMapResizeHandler: (() => void) | null = null

const traffic = computed(() => summary.value.traffic || {})
const threats = computed(() => summary.value.threats || {})
const topDomains = computed(() => traffic.value.top_domains || [])
const topProxyHosts = computed(() => {
  const fromApi = traffic.value.top_proxy_hosts || []
  if (fromApi.length) return fromApi
  return topDomains.value.map((d: AnyRecord) => ({
    vhost: d.domain || '(unknown)',
    hits: d.hits || 0,
  }))
})
const topEndpoints = computed(() => traffic.value.top_endpoints || [])
const topThreatIPs = computed(() => threats.value.top_ips || [])
const topClientIPs = computed(() => traffic.value.top_client_ips || [])
const countryDistribution = computed(() => {
  const rows = traffic.value.country_distribution || []
  return [...rows].sort((a: AnyRecord, b: AnyRecord) => (Number(b?.hits) || 0) - (Number(a?.hits) || 0))
})
const statusDistribution = computed(() => traffic.value.status_distribution || { '2xx': 0, '3xx': 0, '4xx': 0, '5xx': 0 })

const lastUpdatedLabel = computed(() => {
  if (!lastUpdatedAt.value) return 'jamais'
  return lastUpdatedAt.value.toLocaleTimeString()
})

function numberFormat(v: number): string {
  return new Intl.NumberFormat('fr-FR').format(Number(v) || 0)
}

function formatBytes(bytes: number): string {
  const value = Number(bytes) || 0
  if (value < 1024) return `${value} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  return `${size.toFixed(1)} ${units[unit]}`
}

function percent(v: number): string {
  const n = Number(v) || 0
  return `${(n * 100).toFixed(2)}%`
}

function formatDate(v: string): string {
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v || '-'
  return d.toLocaleString()
}

function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'bg-green-lt text-green'
  if (status >= 300 && status < 400) return 'bg-yellow-lt text-yellow'
  if (status >= 400) return 'bg-red-lt text-red'
  return 'bg-secondary-lt text-secondary'
}

function normalizeCountryName(name: string): string {
  return (name || '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]/g, '')
}

function mapCountryKey(name: string): string {
  const key = normalizeCountryName(name)
  const aliases: Record<string, string> = {
    usa: 'unitedstatesofamerica',
    unitedstates: 'unitedstatesofamerica',
    uk: 'unitedkingdom',
    greatbritain: 'unitedkingdom',
    russia: 'russianfederation',
    southkorea: 'southkorea',
    northkorea: 'northkorea',
    czechia: 'czechrepublic',
    ivorycoast: 'cotedivoire',
    uae: 'unitedarabemirates',
  }
  return aliases[key] || key
}

async function renderWorldMap() {
  if (!worldMapSvg.value) return

  const worldMod = await import('world-atlas/countries-110m.json')
  const worldAtlas = (worldMod as any).default || worldMod
  const world = feature(worldAtlas, worldAtlas.objects.countries) as AnyRecord
  const features = world?.features || []
  if (!Array.isArray(features) || !features.length) return

  const width = Math.max(320, worldMapSvg.value.clientWidth || 320)
  const height = width < 540 ? 240 : 340
  const svg = select(worldMapSvg.value)
  svg.attr('viewBox', `0 0 ${width} ${height}`)

  const countryHits = new Map<string, number>()
  for (const row of countryDistribution.value) {
    const key = mapCountryKey(String(row?.country || ''))
    if (!key) continue
    countryHits.set(key, Number(row?.hits) || 0)
  }

  const maxHits = Math.max(1, d3Max(countryDistribution.value, (d: AnyRecord) => Number(d?.hits) || 0) || 1)
  const color = scaleSequential(interpolateYlOrRd).domain([0, maxHits])

  const projection = geoNaturalEarth1().fitSize([width, height], world as any)
  const path = geoPath(projection as any)

  const root = svg.selectAll<SVGGElement, null>('g.world-root').data([null]).join('g').attr('class', 'world-root')

  const countries = root
    .selectAll<SVGPathElement, any>('path.country')
    .data(features)
    .join('path')
    .attr('class', 'country')
    .attr('d', path as any)
    .attr('fill', (d: AnyRecord) => {
      const key = mapCountryKey(String(d?.properties?.name || ''))
      const hits = countryHits.get(key) || 0
      return hits > 0 ? color(hits) : '#e9edf2'
    })
    .attr('stroke', '#ffffff')
    .attr('stroke-width', 0.6)

  countries
    .selectAll('title')
    .data((d: any) => [d])
    .join('title')
    .text((d: AnyRecord) => {
      const country = String(d?.properties?.name || 'Unknown')
      const key = mapCountryKey(country)
      const hits = countryHits.get(key) || 0
      return `${country}: ${numberFormat(hits)} hits`
    })
}

function hostWidth(hits: number): number {
  const max = Math.max(...(topProxyHosts.value.map((h: AnyRecord) => Number(h.hits) || 0)), 1)
  return Math.round(((Number(hits) || 0) / max) * 100)
}

function kpiDelta(metric: string): number | null {
  const raw = compare.value?.delta_percent?.[metric]
  if (raw === null || raw === undefined) return null
  const n = Number(raw)
  return Number.isFinite(n) ? n : null
}

function deltaClass(metric: string): string {
  const value = kpiDelta(metric)
  if (value === null) return 'text-secondary'

  const increaseIsBad = metric === 'ratio_5xx' || metric === 'suspicious_ips'
  if (!increaseIsBad) {
    if (value > 0) return 'text-green'
    if (value < 0) return 'text-red'
  } else {
    if (value > 0) return 'text-red'
    if (value < 0) return 'text-green'
  }
  return 'text-secondary'
}

function deltaLabel(metric: string): string {
  const v = kpiDelta(metric)
  if (v === null) return 'N/A vs période précédente'
  const sign = v > 0 ? '+' : ''
  return `${sign}${v.toFixed(1)}% vs période précédente`
}

function setPeriod(value: string) {
  if (period.value === value) return
  period.value = value
  void loadAll(true)
}

async function ensureChartLib() {
  if (chartLib) return chartLib
  const mod = await import('chart.js')
  mod.Chart.register(...mod.registerables)
  chartLib = mod.Chart
  return chartLib
}

function bucketLabel(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return period.value === '1h'
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleString([], { day: '2-digit', month: '2-digit', hour: '2-digit' })
}

async function renderCharts() {
  const Chart = await ensureChartLib()

  if (trafficChart) {
    trafficChart.destroy()
    trafficChart = null
  }
  if (statusChart) {
    statusChart.destroy()
    statusChart = null
  }

  if (trafficCanvas.value) {
    const labels = timeseries.value.map((p) => bucketLabel(p.timestamp))
    const human = timeseries.value.map((p) => Number(p.human) || 0)
    const bot = timeseries.value.map((p) => Number(p.bot) || 0)
    trafficChart = new Chart(trafficCanvas.value.getContext('2d'), {
      type: 'bar',
      data: {
        labels,
        datasets: [
          { label: 'Humain', data: human, backgroundColor: '#378ADD', stack: 'traffic' },
          { label: 'Bot/scan', data: bot, backgroundColor: '#E24B4A', stack: 'traffic' },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: { legend: { position: 'bottom' } },
        scales: { x: { stacked: true, grid: { display: false } }, y: { stacked: true } },
      },
    })
  }

  if (statusCanvas.value) {
    statusChart = new Chart(statusCanvas.value.getContext('2d'), {
      type: 'doughnut',
      data: {
        labels: ['2xx', '3xx', '4xx', '5xx'],
        datasets: [{
          data: [
            Number(statusDistribution.value['2xx']) || 0,
            Number(statusDistribution.value['3xx']) || 0,
            Number(statusDistribution.value['4xx']) || 0,
            Number(statusDistribution.value['5xx']) || 0,
          ],
          backgroundColor: ['#639922', '#185FA5', '#BA7517', '#E24B4A'],
          borderWidth: 0,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        cutout: '70%',
        plugins: { legend: { position: 'bottom' } },
      },
    })
  }
}

async function loadAll(showSpinner: boolean) {
  if (showSpinner) loading.value = true
  try {
    const bucket = period.value === '1h' ? 'minute' : 'hour'
    const [summaryRes, timeseriesRes, liveRes] = await Promise.all([
      apiClient.getWebLogsSummary(period.value, hostId.value || undefined, source.value || undefined),
      apiClient.getWebLogsTimeseries(period.value, bucket, hostId.value || undefined, source.value || undefined),
      apiClient.getWebLogsLive(hostId.value || undefined, source.value || undefined, 120),
    ])
    summary.value = {
      traffic: summaryRes.data?.traffic || {},
      threats: summaryRes.data?.threats || {},
    }
    compare.value = summaryRes.data?.compare || { delta_percent: {} }
    timeseries.value = timeseriesRes.data?.points || []
    liveRequests.value = liveRes.data?.requests || []
    lastUpdatedAt.value = new Date()
    await nextTick()
    await renderCharts()
    await renderWorldMap()
  } catch (err) {
    console.error('Failed to load traffic dashboard', err)
  } finally {
    if (showSpinner) loading.value = false
  }
}

function resetAutoRefresh() {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
  if (!autoRefresh.value) return
  refreshTimer = window.setInterval(() => {
    void loadAll(false)
  }, 8000)
}

async function openDomain(domain: string) {
  if (!domain) return
  selectedDomain.value = domain
  showDomainModal.value = true
  domainLoading.value = true
  try {
    const res = await apiClient.getDomainDetails(domain, period.value, hostId.value || undefined, source.value || undefined, 300)
    domainDetails.value = res.data?.details || {}
  } catch (err) {
    console.error('Failed to load domain details', err)
    domainDetails.value = {}
  } finally {
    domainLoading.value = false
  }
}

function closeDomainModal() {
  showDomainModal.value = false
  selectedDomain.value = ''
  domainDetails.value = {}
}

watch(autoRefresh, resetAutoRefresh)

watch(countryDistribution, () => {
  void nextTick().then(renderWorldMap)
})

onMounted(async () => {
  await loadAll(true)
  resetAutoRefresh()
  worldMapResizeHandler = () => {
    void renderWorldMap()
  }
  window.addEventListener('resize', worldMapResizeHandler)
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  if (trafficChart) trafficChart.destroy()
  if (statusChart) statusChart.destroy()
  if (worldMapResizeHandler) window.removeEventListener('resize', worldMapResizeHandler)
})
</script>

<style scoped>
.traffic-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.live-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #639922;
  animation: pulse 1.6s infinite;
}

.live-dot.paused {
  animation: none;
  background: #9ca3af;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
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

.world-map {
  width: 100%;
  height: 340px;
  display: block;
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

  .world-map {
    height: 260px;
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

  .world-map {
    height: 220px;
  }
}
</style>

