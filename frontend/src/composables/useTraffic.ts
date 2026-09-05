import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import apiClient from '../api'
import { useHostsStore } from '../stores/hosts'
import { looksLikeIP } from '../utils/network'
import { useDomainDetails } from './useDomainDetails'
import type { WebLogIPTimelineRow } from '../types/security'
import type { TimeRangeModel } from '../types/timeRange'

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for aggregate web-logs data (no Go model)
type AnyRecord = Record<string, any>

interface TimeseriesPoint {
  timestamp: string
  human?: number | string
  bot?: number | string
  [key: string]: unknown
}

export function useTraffic() {
  const { locale } = useI18n()
  const hostsStore = useHostsStore()
  const route = useRoute()
  const router = useRouter()

  const periodOptions = [
    { value: '1h', label: '1h' },
    { value: '24h', label: '24h' },
    { value: '168h', label: '7j' },
    { value: '720h', label: '30j' },
  ]

  // The summary+timeseries call is an expensive multi-query aggregate over
  // the whole table (see GetWebLogsSummary server-side); polling it every few
  // seconds was compounding into the dashboard's request timeout on a large
  // table. The live tail (a simple indexed "last N requests" query) stays on
  // a fast cadence; the heavy aggregate refreshes on a much slower one.
  const REFRESH_INTERVAL_MS = 30000
  const LIVE_REFRESH_INTERVAL_MS = 8000

  const period = ref(typeof route.query.period === 'string' ? route.query.period : '24h')
  const source = ref(typeof route.query.source === 'string' ? route.query.source : '')
  const hostId = ref(typeof route.query.host_id === 'string' ? route.query.host_id : '')
  const from = ref<string | null>(typeof route.query.from === 'string' ? route.query.from : null)
  const to = ref<string | null>(typeof route.query.to === 'string' ? route.query.to : null)
  const timeRange = ref<TimeRangeModel>({
    mode: from.value && to.value ? 'custom' : 'preset',
    period: period.value,
    from: from.value,
    to: to.value,
  })
  const autoRefresh = ref(true)

  // Unlike useBot.ts (Menaces), this page never had URL sync before — added
  // alongside the custom range so a from/to (or a preset/host/source) picked
  // here survives a refresh or a shared link, same as Menaces already does.
  watch([period, source, hostId, from, to], ([p, s, h, f, toVal]) => {
    router.replace({ query: { ...route.query, period: p, source: s || undefined, host_id: h || undefined, from: f || undefined, to: toVal || undefined } })
  })

  const loading = ref(false)
  const summary = ref<AnyRecord>({ traffic: {}, threats: {} })
  const compare = ref<AnyRecord>({ delta_percent: {} })
  const timeseries = ref<TimeseriesPoint[]>([])
  const liveRequests = ref<AnyRecord[]>([])
  const lastUpdatedAt = ref<Date | null>(null)

  const domainModal = useDomainDetails()

  // Read-only IP view (no ban action — that stays a Threats-mode concern).
  const showIPModal = ref(false)
  const selectedIP = ref('')
  const ipTimelineLoading = ref(false)
  const ipTimeline = ref<WebLogIPTimelineRow[]>([])

  const searchTerm = ref('')

  let refreshTimer: number | null = null
  let liveRefreshTimer: number | null = null
  const chartReady = ref(false)

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
  const showInitialLoading = computed(() => loading.value && !chartReady.value)

  const sourceHasNoData = computed(() => {
    if (!source.value || loading.value || !chartReady.value) return false
    return (traffic.value.total_requests || 0) === 0
  })

  function numberFormat(v: number): string {
    return new Intl.NumberFormat(locale.value).format(Number(v) || 0)
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

  function formatDate(v: string): string {
    const d = new Date(v)
    if (Number.isNaN(d.getTime())) return v || '-'
    return d.toLocaleString()
  }

  function statusClass(status: number): string {
    if (status >= 200 && status < 300) return 'bg-success-lt text-success'
    if (status >= 300 && status < 400) return 'bg-warning-lt text-warning'
    if (status >= 400) return 'bg-danger-lt text-danger'
    return 'bg-secondary-lt text-secondary'
  }


  function hostWidth(hits: number): number {
    const max = Math.max(...(topProxyHosts.value.map((h: AnyRecord) => Number(h.hits) || 0)), 1)
    return Math.round(((Number(hits) || 0) / max) * 100)
  }

  function onRangeChange(): void {
    if (timeRange.value.mode === 'custom' && timeRange.value.from && timeRange.value.to) {
      from.value = timeRange.value.from
      to.value = timeRange.value.to
    } else {
      period.value = timeRange.value.period
      from.value = null
      to.value = null
    }
    void loadAll(true)
  }

  async function loadLive(): Promise<void> {
    try {
      const liveRes = await apiClient.getWebLogsLive(hostId.value || undefined, source.value || undefined, 120)
      liveRequests.value = liveRes.data?.requests || []
    } catch (err) {
      console.error('Failed to load live requests', err)
    }
  }

  async function loadSummary(showSpinner: boolean): Promise<void> {
    if (showSpinner) loading.value = true
    try {
      // bucket is left unset — the server now derives minute-vs-hour from the
      // actual since/until span (see parseTimeRange), the same threshold this
      // used to compute client-side from period==='1h', but that duplicated
      // logic didn't generalize to a custom range with no preset name.
      const range = from.value && to.value ? { from: from.value, to: to.value } : undefined
      const [summaryRes, timeseriesRes] = await Promise.all([
        apiClient.getWebLogsSummary(period.value, hostId.value || undefined, source.value || undefined, undefined, range),
        apiClient.getWebLogsTimeseries(period.value, undefined, hostId.value || undefined, source.value || undefined, range),
      ])
      summary.value = {
        traffic: summaryRes.data?.traffic || {},
        threats: summaryRes.data?.threats || {},
      }
      compare.value = summaryRes.data?.compare || { delta_percent: {} }
      timeseries.value = timeseriesRes.data?.points || []
      lastUpdatedAt.value = new Date()
    } catch (err) {
      console.error('Failed to load traffic dashboard', err)
    } finally {
      if (showSpinner) loading.value = false
      if (!chartReady.value) chartReady.value = true
    }
  }

  async function loadAll(showSpinner: boolean): Promise<void> {
    await Promise.all([loadSummary(showSpinner), loadLive()])
  }

  function resetAutoRefresh() {
    if (refreshTimer) {
      window.clearInterval(refreshTimer)
      refreshTimer = null
    }
    if (liveRefreshTimer) {
      window.clearInterval(liveRefreshTimer)
      liveRefreshTimer = null
    }
    if (!autoRefresh.value) return
    refreshTimer = window.setInterval(() => {
      void loadSummary(false)
    }, REFRESH_INTERVAL_MS)
    liveRefreshTimer = window.setInterval(() => {
      void loadLive()
    }, LIVE_REFRESH_INTERVAL_MS)
  }

  function openDomain(domain: string) {
    if (!domain) return
    domainModal.open(domain, {
      period: period.value,
      hostId: hostId.value || undefined,
      source: source.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
    })
  }

  async function openIP(ip: string) {
    if (!ip) return
    selectedIP.value = ip
    showIPModal.value = true
    ipTimelineLoading.value = true
    try {
      const range = from.value && to.value ? { from: from.value, to: to.value } : undefined
      const res = await apiClient.getIPTimeline(ip, hostId.value || undefined, period.value, 500, range)
      ipTimeline.value = res.data?.requests || []
    } catch (err) {
      console.error('Failed to load IP timeline', err)
      ipTimeline.value = []
    } finally {
      ipTimelineLoading.value = false
    }
  }

  function closeIPModal() {
    showIPModal.value = false
    selectedIP.value = ''
    ipTimeline.value = []
  }

  // Free-text search: routes to the domain or IP detail view depending on
  // what the input looks like, so a domain or IP that never made a "top N"
  // list is still directly reachable.
  function handleSearch() {
    const term = searchTerm.value.trim()
    if (!term) return
    if (looksLikeIP(term)) {
      void openIP(term)
    } else {
      void openDomain(term)
    }
    searchTerm.value = ''
  }

  watch(autoRefresh, resetAutoRefresh)

  onMounted(async () => {
    hostsStore.fetchHosts()
    await loadAll(true)
    resetAutoRefresh()
  })

  onBeforeUnmount(() => {
    if (refreshTimer) window.clearInterval(refreshTimer)
    if (liveRefreshTimer) window.clearInterval(liveRefreshTimer)
  })

  return {
    hostsStore,
    periodOptions,
    REFRESH_INTERVAL_MS,
    period,
    source,
    hostId,
    timeRange,
    autoRefresh,
    loading,
    summary,
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
    onRangeChange,
    loadAll,
    openDomain,
    openIP,
    closeIPModal,
    handleSearch,
  }
}
