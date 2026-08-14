import { ref, computed, onMounted, onUnmounted, type Ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import dayjs from '../utils/dayjs'
import type { UptimeProbe, UptimeStats, UptimeHistoryBucket } from '../types/generated'

// 1h/24h windows are dense enough that only the time-of-day matters; wider
// windows (7j/30j) need the date too or every bucket label looks identical.
function formatBucketLabel(bucketStart: string, windowHours: number): string {
  return dayjs(bucketStart).format(windowHours > 24 ? 'DD/MM HH:mm' : 'HH:mm:ss')
}

interface ProbeResult {
  id: string | number
  checked_at: string
  success: boolean
  status_code?: number | null
  error?: string
  latency_ms: number
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

export const PROBE_REFRESH_SEC = 30

export const STATS_WINDOWS = [
  { hours: 1, label: '1h' },
  { hours: 24, label: '24h' },
  { hours: 168, label: '7j' },
  { hours: 720, label: '30j' },
] as const

// probeIdOverride lets MonitoringHostDetailView's unified per-host page reuse
// this composable keyed by an id it already resolved (the NPM proxy host's
// linked probe) instead of route.params.id, which only exists on the
// standalone /monitoring/probes/:id route. autoRefreshOverride lets that same
// page drive the pause/resume toggle from one shared control (see
// MonitoringHostDetailView.vue's merged PageRefreshBar) instead of this
// composable's own independent ref — standalone routes never pass it, so
// their behavior is unchanged.
export function useUptimeProbeDetail(probeIdOverride?: string, autoRefreshOverride?: Ref<boolean>) {
  const route = useRoute()
  const probeId = probeIdOverride ?? (route.params.id as string)
  const signal = useAbortSignal()

  const probe = ref<UptimeProbe | null>(null)
  const results = ref<ProbeResult[]>([])
  const stats = ref<UptimeStats | null>(null)
  const buckets = ref<UptimeHistoryBucket[]>([])
  const loading = ref(false)
  const error = ref('')
  const statsWindow = ref<number>(1)
  const statsLoading = ref(false)

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
    if (!buckets.value.length) return null
    return {
      labels: buckets.value.map((b) => formatBucketLabel(b.bucket_start, statsWindow.value)),
      datasets: [{
        label: 'Latence moy. ms',
        data: buckets.value.map((b) => b.up_checks > 0 ? Math.round(b.avg_latency_ms) : null),
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

  const autoRefresh = autoRefreshOverride ?? ref(true)
  const lastUpdatedAt = ref<Date | null>(null)

  // Bucket-based availability bar, oldest-first (reading left-to-right ends on
  // "now") — scales with statsWindow instead of always showing the last few
  // minutes' worth of raw checks.
  const heartbeatBar = computed<UptimeHistoryBucket[]>(() => buckets.value)

  async function fetchAll(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const [pr, hr, sr, br] = await Promise.all([
        api.getUptimeProbe(probeId, signal),
        api.getUptimeHistory(probeId, 200, signal),
        api.getUptimeStats(probeId, statsWindow.value, signal),
        api.getUptimeHistoryBuckets(probeId, statsWindow.value, signal),
      ])
      probe.value = pr.data
      results.value = hr.data?.results || []
      stats.value = sr.data
      buckets.value = br.data?.buckets || []
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, 'Impossible de charger la sonde')
    } finally {
      loading.value = false
    }
  }

  // Re-fetches Stats (the % uptime figure) and the bucketed availability/latency
  // data together — both are scoped to the same selected window.
  async function setStatsWindow(hours: number): Promise<void> {
    if (hours === statsWindow.value) return
    statsWindow.value = hours
    statsLoading.value = true
    try {
      const [sr, br] = await Promise.all([
        api.getUptimeStats(probeId, hours, signal),
        api.getUptimeHistoryBuckets(probeId, hours, signal),
      ])
      stats.value = sr.data
      buckets.value = br.data?.buckets || []
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, 'Impossible de charger les statistiques')
    } finally {
      statsLoading.value = false
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

  return {
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
  }
}
