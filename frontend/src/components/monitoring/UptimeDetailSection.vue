<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Sonde uptime"
      :interval-sec="PROBE_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

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
                Uptime ({{ STATS_WINDOWS.find((w) => w.hours === statsWindow)?.label }})
              </div>
              <div class="h2 mb-0 mt-1">
                <span
                  v-if="statsLoading"
                  class="spinner-border spinner-border-sm"
                />
                <template v-else>
                  {{ stats ? stats.uptime_percent.toFixed(2) + '%' : '—' }}
                </template>
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
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title mb-0">
            Disponibilité
          </h3>
          <div class="btn-group btn-group-sm">
            <button
              v-for="w in STATS_WINDOWS"
              :key="w.hours"
              type="button"
              class="btn"
              :class="statsWindow === w.hours ? 'btn-primary' : 'btn-outline-secondary'"
              @click="setStatsWindow(w.hours)"
            >
              {{ w.label }}
            </button>
          </div>
        </div>
        <div class="card-body">
          <div
            v-if="heartbeatBar.length"
            class="d-flex align-items-end gap-1"
            style="height: 40px;"
          >
            <div
              v-for="r in heartbeatBar"
              :key="r.id"
              class="flex-fill rounded-1"
              :class="r.success ? 'bg-green' : 'bg-red'"
              style="height: 100%; min-width: 3px;"
              :title="`${formatDateTime(r.checked_at)} — ${r.success ? 'OK' : 'KO'}${r.success ? ` (${r.latency_ms} ms)` : (r.error ? ` — ${r.error}` : '')}`"
            />
          </div>
          <div
            v-else
            class="text-secondary small"
          >
            Aucun check encore enregistré.
          </div>
          <div class="d-flex justify-content-between text-secondary small mt-1">
            <span>{{ heartbeatBar.length ? formatDateTime(heartbeatBar[0].checked_at) : '' }}</span>
            <span>maintenant</span>
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
        <div class="table-responsive scroll-table">
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
import { defineAsyncComponent, watch } from 'vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import { formatDateTime } from '../../utils/formatters'
import { getChartPalette } from '../../utils/chartTheme'
import { useUptimeProbeDetail, STATS_WINDOWS } from '../../composables/useUptimeProbeDetail'
import type { UptimeProbe } from '../../types/generated'

const props = defineProps<{ probeId: string }>()
const emit = defineEmits<{
  (e: 'loaded', probe: UptimeProbe | null): void
}>()

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

const {
  probe,
  results,
  stats,
  loading,
  error,
  statsWindow,
  statsLoading,
  setStatsWindow,
  heartbeatBar,
  groupedResults,
  chartData,
  statusLabel,
  statusColor,
  autoRefresh,
  lastUpdatedAt,
  PROBE_REFRESH_SEC,
} = useUptimeProbeDetail(props.probeId)

watch(probe, (p) => emit('loaded', p), { immediate: true })

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
</script>
