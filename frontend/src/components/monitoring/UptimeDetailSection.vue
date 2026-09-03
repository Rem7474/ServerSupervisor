<template>
  <div>
    <PageRefreshBar
      v-if="!hideRefreshBar"
      v-model="autoRefresh"
      :label="t('monitoring.probeTypeToggle')"
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
                {{ t('monitoring.statusColumn') }}
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
                {{ t('monitoring.uptimeChartTitle', { window: currentWindowLabel }) }}
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
                {{ t('monitoring.avgLatencyLabel') }}
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
                {{ t('monitoring.consecutiveFailuresLabel') }}
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
            {{ t('monitoring.availabilityTitle') }}
          </h3>
          <div class="d-flex align-items-center gap-2">
            <span
              v-if="statsLoading"
              class="spinner-border spinner-border-sm text-muted"
            />
            <div class="btn-group btn-group-sm">
              <button
                v-for="w in STATS_WINDOWS"
                :key="w.hours"
                type="button"
                class="btn"
                :class="statsWindow === w.hours ? 'btn-primary' : 'btn-outline-secondary'"
                @click="setStatsWindow(w.hours)"
              >
                {{ t(w.labelKey) }}
              </button>
            </div>
          </div>
        </div>
        <div class="card-body">
          <!--
            .tracking for the flex/gap layout, but items stay plain
            flex-fill + bg-* rather than .tracking-block: in Tabler's
            compiled CSS .tracking-block's own `background` rule is
            defined after the bg-* utilities, so it would win over
            bg-success/bg-danger at equal specificity and paint every
            block gray.
          -->
          <div
            v-if="heartbeatBar.length"
            class="tracking"
            style="height: 40px;"
          >
            <div
              v-for="b in heartbeatBar"
              :key="b.bucket_start"
              class="flex-fill rounded-1"
              :class="bucketColorClass(b)"
              style="height: 100%; min-width: 3px;"
              :title="bucketTitle(b)"
            />
          </div>
          <div
            v-else
            class="text-secondary small"
          >
            {{ t('monitoring.noChecksYet') }}
          </div>
          <div class="d-flex justify-content-between text-secondary small mt-1">
            <span>{{ heartbeatBar.length ? formatDateTime(heartbeatBar[0].bucket_start) : '' }}</span>
            <span>{{ t('monitoring.nowLabel') }}</span>
          </div>
        </div>
      </div>

      <div class="card mb-3">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title mb-0">
            {{ t('monitoring.latencyChartTitle', { window: currentWindowLabel }) }}
          </h3>
          <div class="d-flex align-items-center gap-2">
            <span
              v-if="statsLoading"
              class="spinner-border spinner-border-sm text-muted"
            />
            <div class="btn-group btn-group-sm">
              <button
                v-for="w in STATS_WINDOWS"
                :key="w.hours"
                type="button"
                class="btn"
                :class="statsWindow === w.hours ? 'btn-primary' : 'btn-outline-secondary'"
                @click="setStatsWindow(w.hours)"
              >
                {{ t(w.labelKey) }}
              </button>
            </div>
          </div>
        </div>
        <div
          class="card-body chart-body position-relative"
          style="height: 220px;"
        >
          <ApexChart
            v-if="chartData && chartOptions"
            ref="chartRef"
            type="area"
            height="100%"
            :options="chartOptions"
            :series="chartData.series"
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
            {{ t('monitoring.recentHistoryTitle') }}
          </h3>
          <small class="text-secondary">
            {{ t('monitoring.sequencesOfChecksCount', { seq: groupedResults.length, total: results.length }, groupedResults.length) }}
          </small>
        </div>
        <div class="table-responsive scroll-table">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>{{ t('monitoring.periodColumn') }}</th>
                <th>{{ t('monitoring.resultColumn') }}</th>
                <th>{{ t('monitoring.httpStatusColumn') }}</th>
                <th>{{ t('monitoring.latencyColumn') }}</th>
                <th>{{ t('monitoring.checksColumn') }}</th>
                <th>{{ t('monitoring.errorColumn') }}</th>
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
                  <span :class="['badge', g.success ? 'bg-success-lt text-success' : 'bg-danger-lt text-danger']">
                    {{ g.success ? t('monitoring.tickOkLabel') : t('monitoring.tickKoLabel') }}
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
                <td colspan="6">
                  <EmptyState :title="t('monitoring.noResultsYet', { sec: probe.interval_sec })" />
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
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApexOptions } from 'apexcharts'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import EmptyState from '../EmptyState.vue'
import { formatDateTime } from '../../utils/formatters'
import { getApexChartPalette, AsyncApexChart as ApexChart, type ApexChartInstance } from '../../utils/apexChartTheme'
import { useUptimeProbeDetail, STATS_WINDOWS } from '../../composables/useUptimeProbeDetail'
import type { UptimeProbe, UptimeHistoryBucket } from '../../types/generated'

// hideRefreshBar/autoRefresh: set together by MonitoringHostDetailView when
// both a probe and a cert are configured, so the two sections share one
// PageRefreshBar instead of each rendering its own "last updated" + dot.
// Neither is passed by the standalone /monitoring/probes/:id route, which
// keeps its own independent bar exactly as before.
const { t } = useI18n()

const props = defineProps<{
  probeId: string
  hideRefreshBar?: boolean
  autoRefresh?: boolean
}>()
const emit = defineEmits<{
  (e: 'loaded', probe: UptimeProbe | null): void
  (e: 'update:autoRefresh', value: boolean): void
}>()

// Gated on hideRefreshBar, not on `props.autoRefresh === undefined`: Vue
// casts an optional `boolean` prop that isn't bound at all (every caller
// except MonitoringHostDetailView's merged-bar case) to `false`, not
// `undefined` — checking the raw value here would permanently freeze
// autoRefresh off on every standalone route. hideRefreshBar and autoRefresh
// are always set together by the one caller that uses either.
const autoRefreshOverride = props.hideRefreshBar ? computed({
  get: () => props.autoRefresh as boolean,
  set: (v) => emit('update:autoRefresh', v),
}) : undefined

function bucketColorClass(b: UptimeHistoryBucket): string {
  if (b.total_checks === 0) return 'bg-secondary-lt'
  return b.down_checks > 0 ? 'bg-danger' : 'bg-success'
}

function bucketTitle(b: UptimeHistoryBucket): string {
  const when = formatDateTime(b.bucket_start)
  if (b.total_checks === 0) return `${when} — ${t('monitoring.noCheckYetTitle')}`
  const outcome = b.down_checks > 0
    ? `${b.down_checks} ${t('monitoring.tickKoLabel')} / ${b.total_checks}`
    : `${b.up_checks} ${t('monitoring.tickOkLabel')}`
  const latency = b.up_checks > 0 ? ` — ${Math.round(b.avg_latency_ms)} ms ${t('monitoring.avgSuffix')}` : ''
  return `${when} — ${outcome}${latency}`
}

const currentWindowLabel = computed(() => {
  const win = STATS_WINDOWS.find((w) => w.hours === statsWindow.value)
  return win ? t(win.labelKey) : ''
})

const chartRef = ref<ApexChartInstance | null>(null)

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
} = useUptimeProbeDetail(props.probeId, autoRefreshOverride)

watch(probe, (p) => emit('loaded', p), { immediate: true })

// Read by MonitoringHostDetailView's merged PageRefreshBar to compute the
// most-recent of this section's and SslDetailSection's own lastUpdatedAt.
defineExpose({ lastUpdatedAt })

// Built once (on first data arrival) rather than as a `computed` over
// `chartData`: vue3-apexcharts clones the whole `:options` prop via
// JSON.parse(JSON.stringify(...)) on every *reactive* update after mount —
// which silently drops every function (yaxis.labels.formatter,
// tooltip.custom) since JSON can't represent them. Keeping this object's
// identity stable after the initial build means later data refreshes (this
// probe polls on a timer, see PROBE_REFRESH_SEC) flow through the
// (function-free, always-safe) `:series` prop only; the category labels
// (the one other thing that legitimately changes per refresh, since this
// is a category- not datetime-axis chart) are pushed via the exposed
// updateOptions() method directly in the watcher below (bypasses the
// wrapper's buggy reactive watcher).
const chartOptions = shallowRef<ApexOptions | null>(null)

function buildChartOptions(categories: string[]): ApexOptions {
  const palette = getApexChartPalette()
  return {
    chart: { type: 'area', background: 'transparent', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
    theme: { mode: 'dark' },
    fill: { type: 'solid', opacity: 0.15 },
    stroke: { curve: 'smooth', width: 2 },
    markers: { size: 0, hover: { size: 4 } },
    dataLabels: { enabled: false },
    legend: { show: false },
    grid: { borderColor: palette.grid },
    xaxis: {
      type: 'category',
      categories,
      labels: { style: { colors: palette.tickText }, rotate: 0 },
      tickAmount: 8,
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: { min: 0, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${Math.round(v)} ms` } },
    tooltip: {
      y: { formatter: (v: number) => (v != null ? `${Number(v)} ms` : '—') },
    },
  }
}

watch(() => chartData.value?.categories, (categories) => {
  if (!categories) return
  if (!chartOptions.value) {
    chartOptions.value = buildChartOptions(categories)
  } else {
    chartRef.value?.updateOptions({ xaxis: { categories } }, false, false)
  }
}, { immediate: true })
</script>
