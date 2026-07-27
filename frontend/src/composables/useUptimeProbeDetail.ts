import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { UptimeProbe, UptimeStats } from '../types/generated'

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

const PROBE_REFRESH_SEC = 30

// Uptime Kuma itself pairs a fixed-length "last N pings" heartbeat bar with a
// separately selectable uptime-% window — the bar isn't re-bucketed per
// window (that would need server-side time-bucketing this API doesn't have),
// only the % figure is. HEARTBEAT_BAR_SIZE caps how many of the most recent
// `results` render as bars, independent of statsWindow.
const HEARTBEAT_BAR_SIZE = 50

export const STATS_WINDOWS = [
  { hours: 24, label: '24h' },
  { hours: 168, label: '7j' },
  { hours: 720, label: '30j' },
] as const

export function useUptimeProbeDetail() {
  const route = useRoute()
  const probeId = route.params.id as string
  const signal = useAbortSignal()

  const probe = ref<UptimeProbe | null>(null)
  const results = ref<ProbeResult[]>([])
  const stats = ref<UptimeStats | null>(null)
  const loading = ref(false)
  const error = ref('')
  const statsWindow = ref<number>(24)
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

  // Oldest-first (Uptime Kuma convention: reading left-to-right ends on "now")
  // slice of the most recent checks, for the heartbeat bar.
  const heartbeatBar = computed<ProbeResult[]>(() =>
    [...results.value].reverse().slice(-HEARTBEAT_BAR_SIZE)
  )

  async function fetchAll(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const [pr, hr, sr] = await Promise.all([
        api.getUptimeProbe(probeId, signal),
        api.getUptimeHistory(probeId, 200, signal),
        api.getUptimeStats(probeId, statsWindow.value, signal),
      ])
      probe.value = pr.data
      results.value = hr.data?.results || []
      stats.value = sr.data
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, 'Impossible de charger la sonde')
    } finally {
      loading.value = false
    }
  }

  // Only re-fetches Stats (the % uptime figure) — the heartbeat bar and
  // history table are independent of the selected window (see HEARTBEAT_BAR_SIZE).
  async function setStatsWindow(hours: number): Promise<void> {
    if (hours === statsWindow.value) return
    statsWindow.value = hours
    statsLoading.value = true
    try {
      const sr = await api.getUptimeStats(probeId, hours, signal)
      stats.value = sr.data
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
