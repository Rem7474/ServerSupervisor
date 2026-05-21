<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/uptime"
          class="text-decoration-none"
        >
          Uptime
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
                :class="probe.consecutive_failures > 0 ? 'text-danger' : ''"
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
        <div class="card-header">
          <h3 class="card-title mb-0">
            Historique récent
          </h3>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Horodatage</th>
                <th>Résultat</th>
                <th>Statut HTTP</th>
                <th>Latence</th>
                <th>Erreur</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="r in results"
                :key="r.id"
              >
                <td class="text-secondary small">
                  {{ formatDateTime(r.checked_at) }}
                </td>
                <td>
                  <span :class="['badge', r.success ? 'bg-green-lt text-green' : 'bg-red-lt text-red']">
                    {{ r.success ? 'OK' : 'KO' }}
                  </span>
                </td>
                <td>{{ r.status_code ?? '—' }}</td>
                <td>{{ r.latency_ms }} ms</td>
                <td class="text-secondary small">
                  {{ r.error || '' }}
                </td>
              </tr>
              <tr v-if="!results.length">
                <td
                  colspan="5"
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

<script setup>
import { ref, computed, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import { formatDateTime } from '../utils/formatters'
import { getChartPalette } from '../utils/chartTheme'

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
const probeId = route.params.id

const probe = ref(null)
const results = ref([])
const stats = ref(null)
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
    y: { min: 0, grid: { color: palette.grid }, ticks: { color: palette.tickText, callback: (v) => `${v} ms` } },
  },
  elements: { point: { radius: 0, hitRadius: 8 }, line: { tension: 0.3 } },
}

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

async function fetchAll () {
  loading.value = true
  error.value = ''
  try {
    const [pr, hr, sr] = await Promise.all([
      api.get(`/v1/uptime/probes/${probeId}`),
      api.get(`/v1/uptime/probes/${probeId}/history?limit=200`),
      api.get(`/v1/uptime/probes/${probeId}/stats?hours=${statsWindow}`),
    ])
    probe.value = pr.data
    results.value = hr.data?.results || []
    stats.value = sr.data
  } catch (e) {
    error.value = e?.response?.data?.error || 'Impossible de charger la sonde'
  } finally {
    loading.value = false
  }
}

let refresh
onMounted(() => {
  fetchAll()
  refresh = setInterval(fetchAll, 30000)
})
onUnmounted(() => {
  if (refresh) clearInterval(refresh)
})
</script>
