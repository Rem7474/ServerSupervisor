import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import apiClient from '../api'

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for aggregate web-logs data (no Go model)
type AnyRecord = Record<string, any>

interface TimeseriesPoint {
  timestamp: string
  human?: number | string
  bot?: number | string
  [key: string]: unknown
}

export function useTraffic() {
  const periodOptions = [
    { value: '1h', label: '1h' },
    { value: '24h', label: '24h' },
    { value: '168h', label: '7j' },
    { value: '720h', label: '30j' },
  ]

  const REFRESH_INTERVAL_MS = 8000

  const period = ref('24h')
  const source = ref('')
  const hostId = ref('')
  const autoRefresh = ref(true)

  const loading = ref(false)
  const summary = ref<AnyRecord>({ traffic: {}, threats: {} })
  const compare = ref<AnyRecord>({ delta_percent: {} })
  const timeseries = ref<TimeseriesPoint[]>([])
  const liveRequests = ref<AnyRecord[]>([])
  const lastUpdatedAt = ref<Date | null>(null)

  const showDomainModal = ref(false)
  const selectedDomain = ref('')
  const domainLoading = ref(false)
  const domainDetails = ref<AnyRecord>({})

  let refreshTimer: number | null = null
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

  const lastUpdatedLabel = computed(() => {
    if (!lastUpdatedAt.value) return 'jamais'
    return lastUpdatedAt.value.toLocaleTimeString()
  })

  const sourceHasNoData = computed(() => {
    if (!source.value || loading.value || !chartReady.value) return false
    return (traffic.value.total_requests || 0) === 0
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


  function hostWidth(hits: number): number {
    const max = Math.max(...(topProxyHosts.value.map((h: AnyRecord) => Number(h.hits) || 0)), 1)
    return Math.round(((Number(hits) || 0) / max) * 100)
  }

  function setPeriod(value: string) {
    if (period.value === value) return
    period.value = value
    void loadAll(true)
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
    } catch (err) {
      console.error('Failed to load traffic dashboard', err)
    } finally {
      if (showSpinner) loading.value = false
      if (!chartReady.value) chartReady.value = true
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
    }, REFRESH_INTERVAL_MS)
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

  onMounted(async () => {
    await loadAll(true)
    resetAutoRefresh()
  })

  onBeforeUnmount(() => {
    if (refreshTimer) window.clearInterval(refreshTimer)
  })

  return {
    periodOptions,
    REFRESH_INTERVAL_MS,
    period,
    source,
    hostId,
    autoRefresh,
    loading,
    summary,
    compare,
    timeseries,
    liveRequests,
    lastUpdatedAt,
    showDomainModal,
    selectedDomain,
    domainLoading,
    domainDetails,
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
    lastUpdatedLabel,
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
  }
}
