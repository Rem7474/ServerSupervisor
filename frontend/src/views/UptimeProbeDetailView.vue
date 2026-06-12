<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Sonde uptime"
      :interval-sec="PROBE_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/monitoring"
          class="text-decoration-none"
        >
          Monitoring
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ probe?.name || 'Sonde' }}</span>
      </div>
      <h2 class="page-title">
        {{ probe?.name || '...' }}
      </h2>
      <div
        v-if="probe"
        class="text-secondary"
      >
        <code>{{ probe.target }}</code>
      </div>
    </div>

    <div
      v-if="loading"
      class="row row-cards"
    >
      <div class="col-12 col-md-3">
        <LoadingSkeleton
          variant="kpi"
          :lines="4"
        />
      </div>
    </div>

    <template v-else-if="probe">
      <div class="row row-cards mb-3">
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                Statut
              </div>
              <div
                class="h2 mb-0 mt-1"
                :class="statusColor"
              >
                {{ statusLabel }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                Uptime ({{ statsWindow }}h)
              </div>
              <div class="h2 mb-0 mt-1">
                {{ stats ? stats.uptime_percent.toFixed(2) + '%' : '—' }}
              </div>
              <div class="text-secondary small">
                {{ stats ? `${stats.successful_checks} OK / ${stats.total_checks} checks` : '' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                Latence moy.
              </div>
              <div class="h2 mb-0 mt-1">
                {{ stats ? Math.round(stats.avg_latency_ms) + ' ms' : '—' }}
              </div>
              <div class="text-secondary small">
                P95: {{ stats ? stats.p95_latency_ms + ' ms' : '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                Échecs consécutifs
              </div>
              <div
                class="h2 mb-0 mt-1"
                :class="(probe.consecutive_failures ?? 0) > 0 ? 'text-danger' : ''"
              >
                {{ probe.consecutive_failures }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card mb-3">
        <div class="card-header">
          <h3 class="card-title mb-0">
            Latence ({{ results.length }} derniers checks)
          </h3>
        </div>
        <div
          class="card-body chart-body position-relative"
          style="height: 220px;"
        >
          <Line
            v-if="chartData"
            :data="chartData"
            :options="chartOptions"
          />
          <LoadingSkeleton
            v-else
            variant="chart"
            class="position-absolute inset-0"
          />
        </div>
      </div>

      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title mb-0">
            Historique récent
          </h3>
          <small class="text-secondary">
            {{ groupedResults.length }} séquence(s) sur {{ results.length }} check(s)
          </small>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Période</th>
                <th>Résultat</th>
                <th>Statut HTTP</th>
                <th>Latence</th>
                <th>Checks</th>
                <th>Erreur</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="g in groupedResults"
                :key="g.key"
              >
                <td class="text-secondary small">
                  <div>{{ formatDateTime(g.from) }}</div>
                  <div
                    v-if="g.count > 1"
                    class="text-muted"
                  >
                    → {{ formatDateTime(g.to) }}
                  </div>
                </td>
                <td>
                  <span :class="['badge', g.success ? 'bg-green-lt text-green' : 'bg-red-lt text-red']">
                    {{ g.success ? 'OK' : 'KO' }}
                  </span>
                </td>
                <td>{{ g.statusCode ?? '—' }}</td>
                <td>
                  <template v-if="g.minLatency === g.maxLatency">
                    {{ g.minLatency }} ms
                  </template>
                  <template v-else>
                    <span :title="`min ${g.minLatency} / avg ${g.avgLatency} / max ${g.maxLatency}`">
                      {{ g.minLatency }}–{{ g.maxLatency }} ms
                    </span>
                  </template>
                </td>
                <td>
                  <span class="badge bg-secondary-lt text-secondary">×{{ g.count }}</span>
                </td>
                <td class="text-secondary small">
                  {{ g.error || '' }}
                </td>
              </tr>
              <tr v-if="!results.length">
                <td
                  colspan="6"
                  class="text-center text-secondary py-4"
                >
                  Aucun résultat encore. La première vérification arrive sous {{ probe.interval_sec }}s.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <div
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import { formatDateTime } from '../utils/formatters'
import { getChartPalette } from '../utils/chartTheme'

interface ProbeResult {
  id: string | number
  checked_at: string
  success: boolean
  status_code?: number | null
  error?: string
  latency_ms: number
}

interface Probe {
  last_status?: string
  consecutive_failures?: number
  [key: string]: any
}

const Line = defineAsyncComponent(async () => {
  const [{ Line: LineComponent }, chart] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  chart.Chart.register(
    chart.LineElement, chart.PointElement, chart.LineController,
    chart.CategoryScale, chart.LinearScale, chart.Tooltip, chart.Legend, chart.Filler,
  )
  return LineComponent
})

const route = useRoute()
const probeId = route.params.id as string

const probe = ref<Probe | null>(null)
const results = ref<ProbeResult[]>([])
const stats = ref<any>(null)
const loading = ref(false)
const error = ref('')
const statsWindow = 24

const palette = getChartPalette()
const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: palette.tooltipBackground,
      titleColor: palette.tooltipText,
      bodyColor: palette.tooltipText,
      borderColor: palette.tooltipBorder,
      borderWidth: 1,
      padding: 8,
    },
  },
  scales: {
    x: { grid: { color: palette.grid }, ticks: { color: palette.tickText, maxTicksLimit: 8 } },
    y: { min: 0, grid: { color: palette.grid }, ticks: { color: palette.tickText, callback: (v: number | string) => `${v} ms` } },
  },
  elements: { point: { radius: 0, hitRadius: 8 }, line: { tension: 0.3 } },
}

// Collapse consecutive results that share the same outcome (success +
// status_code + error) into a single run so the table shows transitions and
// failures clearly instead of hundreds of identical "OK 50ms" rows.
interface ResultGroup {
  key: string | number
  sigKey: string
  success: boolean
  statusCode?: number | null
  error: string
  count: number
  from: string
  to: string
  minLatency: number
  maxLatency: number
  latencySum: number
  avgLatency?: number
}

const groupedResults = computed<ResultGroup[]>(() => {
  if (!results.value.length) return []
  const groups: ResultGroup[] = []
  // results.value is ordered newest-first; iterate as-is so the table reads
  // most-recent at the top.
  for (const r of results.value) {
    const codeKey = r.status_code ?? 'null'
    const sigKey = `${r.success ? 'ok' : 'ko'}|${codeKey}|${r.error || ''}`
    const last = groups[groups.length - 1]
    if (last && last.sigKey === sigKey) {
      last.count += 1
      last.from = r.checked_at // older bound moves backwards
      if (r.latency_ms < last.minLatency) last.minLatency = r.latency_ms
      if (r.latency_ms > last.maxLatency) last.maxLatency = r.latency_ms
      last.latencySum += r.latency_ms
      continue
    }
    groups.push({
      key: r.id,
      sigKey,
      success: r.success,
      statusCode: r.status_code,
      error: r.error || '',
      count: 1,
      from: r.checked_at,
      to: r.checked_at,
      minLatency: r.latency_ms,
      maxLatency: r.latency_ms,
      latencySum: r.latency_ms,
    })
  }
  for (const g of groups) {
    g.avgLatency = Math.round(g.latencySum / g.count)
  }
  return groups
})

const chartData = computed(() => {
  if (!results.value.length) return null
  const ordered = [...results.value].reverse()
  return {
    labels: ordered.map((r) => new Date(r.checked_at).toLocaleTimeString()),
    datasets: [{
      label: 'Latence ms',
      data: ordered.map((r) => r.success ? r.latency_ms : null),
      borderColor: '#2fb344',
      backgroundColor: 'rgba(47,179,68,0.15)',
      fill: true,
      spanGaps: false,
    }],
  }
})

const statusLabel = computed(() => {
  if (!probe.value) return ''
  if (probe.value.last_status === 'up') return 'UP'
  if (probe.value.last_status === 'down') return 'DOWN'
  return 'Inconnue'
})

const statusColor = computed(() => {
  if (!probe.value) return ''
  if (probe.value.last_status === 'up') return 'text-success'
  if (probe.value.last_status === 'down') return 'text-danger'
  return 'text-secondary'
})

const autoRefresh = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
const PROBE_REFRESH_SEC = 30

async function fetchAll(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [pr, hr, sr] = await Promise.all([
      api.getUptimeProbe(probeId),
      api.getUptimeHistory(probeId, 200),
      api.getUptimeStats(probeId, statsWindow),
    ])
    probe.value = pr.data
    results.value = hr.data?.results || []
    stats.value = sr.data
    lastUpdatedAt.value = new Date()
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Impossible de charger la sonde'
  } finally {
    loading.value = false
  }
}

let refresh: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  fetchAll()
  refresh = setInterval(() => { if (autoRefresh.value) fetchAll() }, PROBE_REFRESH_SEC * 1000)
})
onUnmounted(() => {
  if (refresh) clearInterval(refresh)
})
</script>
