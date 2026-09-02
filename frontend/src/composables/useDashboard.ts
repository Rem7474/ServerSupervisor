import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import type { ApexOptions } from 'apexcharts'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useDashboardStore } from '../stores/dashboard'
import { useHostsStore } from '../stores/hosts'
import { useWebSocket, wsEvents } from './useWebSocket'
import type { WSDashboardSnapshot } from '../types/ws'
import type { DashboardHostMetrics } from '../types/generated'
import type { Host } from '../types/host'
import { useConfirmDialog } from './useConfirmDialog'
import { confirmBulkAction } from '../utils/bulkActionHelpers'
import { translateError } from '../utils/translateError'
import { formatRelativeTime } from './useDateFormatter'
import dayjs from '../utils/dayjs'
import { useReactiveApexChartPalette } from '../utils/apexChartTheme'
import { clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp } from '../utils/chartTimeAxis'

interface DashboardCveSummary {
  critical_count?: number
  high_count?: number
  hosts_with_critical?: number
  hosts_with_high?: number
}

interface DashboardProxmoxNode {
  id: string
  node_name: string
  status: string
  cluster_name?: string
  cpu_usage?: number
  cpu_count?: number
  mem_used?: number
  mem_total?: number
  vm_count?: number
  lxc_count?: number
}
type SortDirection = 'asc' | 'desc'
type HostStatus = 'online' | 'warning' | 'offline'

interface DashboardHostRecord {
  id: string
  status?: string
  name?: string
  hostname?: string
  ip_address?: string
  os?: string
  agent_version?: string
  created_at?: string | number | Date | null
  last_seen?: string | number | Date | null
  tags?: string[]
}

interface DashboardMetricPoint {
  timestamp?: string | number | Date
  cpu_avg?: number | string | null
  memory_avg?: number | string | null
}

export interface DashboardProxmoxLinkRecord {
  host_id: string
  guest_id?: string
  status?: string
  metrics_source?: 'proxmox' | 'auto' | string
  cpu_usage?: number | null
  mem_alloc?: number
  mem_usage?: number
}

interface ChartPoint { x: number; y: number }
type SummaryChartSeries = { name: string; data: ChartPoint[]; color: string }[]

export function useDashboard() {
  const { t } = useI18n()
  const dashboardStore = useDashboardStore()
  const hostsStore = useHostsStore()
  const {
    hosts,
    aptPending,
    versionComparisons,
    proxmoxSummary,
    hasProxmox,
    outdatedDockerImages,
  } = storeToRefs(dashboardStore)

  const latestAgentVersion = ref('')
  const cveSummary = ref<DashboardCveSummary | null>(null)
  const cveLastUpdated = ref<Date | null>(null)
  const cveTimestampText = computed(() => formatRelativeTime(cveLastUpdated.value, t('dashboard.neverUpdated'), true))
  const proxmoxNodes = ref<DashboardProxmoxNode[]>([])
  const proxmoxLinks = ref<DashboardProxmoxLinkRecord[]>([])

  const hostMetrics = ref<Record<string, DashboardHostMetrics | undefined>>({})
  const aptPendingHosts = ref<Record<string, number>>({})
  const diskUsage = ref<Record<string, number>>({})
  const loading = ref(true)

  const searchQuery = ref('')
  const statusFilter = ref('all')
  const tagFilter = ref('all')
  const sortKey = ref(localStorage.getItem('dashboard.sortKey') || 'name')
  const sortDir = ref<SortDirection>((localStorage.getItem('dashboard.sortDir') as SortDirection) || 'asc')
  watch(sortKey, (v) => localStorage.setItem('dashboard.sortKey', v))
  watch(sortDir, (v) => localStorage.setItem('dashboard.sortDir', v))

  const selectedHostIds = ref<string[]>([])
  const aptLoading = ref('')
  const showDockerVersions = ref(false)

  const summaryHours = ref(24)
  const summaryChartSeries = ref<SummaryChartSeries | null>(null)
  const summaryLoading = ref(false)
  const chartSource = ref('agents')
  const chartSources = computed(() => [
    { key: 'agents', label: t('dashboard.sourceAgents') },
    { key: 'proxmox', label: t('dashboard.sourceProxmox') },
  ])

  const auth = useAuthStore()
  const dialog = useConfirmDialog()

  const selectedCount = computed(() => selectedHostIds.value.length)
  const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')
  const metricsReady = computed(() => {
    const keys = Object.keys(hostMetrics.value)
    if (keys.length === 0) return false
    return hosts.value.some((h: DashboardHostRecord) => keys.includes(h.id))
  })

  const proxmoxLinkByHostId = computed(() => {
    const m: Record<string, DashboardProxmoxLinkRecord> = {}
    for (const link of proxmoxLinks.value) {
      m[link.host_id] = link
    }
    return m
  })

  interface EffectiveMetric {
    cpu: number | null
    memPct: number | null
    source: 'agent' | 'proxmox'
  }
  const EMPTY_METRIC: EffectiveMetric = { cpu: null, memPct: null, source: 'agent' }

  // Pre-computed once per WS update; consumers do O(1) lookup instead of
  // re-running the link/source resolution on every row render.
  const effectiveMetricsByHost = computed<Record<string, EffectiveMetric>>(() => {
    const map: Record<string, EffectiveMetric> = {}
    const links = proxmoxLinkByHostId.value
    const metrics = hostMetrics.value
    for (const host of hosts.value as DashboardHostRecord[]) {
      const link = links[host.id]
      if (link) {
        const src = link.metrics_source
        const useProxmox = src === 'proxmox' || (src === 'auto' && link.cpu_usage != null)
        if (useProxmox) {
          const cpu = link.cpu_usage != null ? link.cpu_usage * 100 : null
          const memAlloc = typeof link.mem_alloc === 'number' ? link.mem_alloc : 0
          const memUsage = typeof link.mem_usage === 'number' ? link.mem_usage : 0
          const memPct = memAlloc > 0 ? (memUsage / memAlloc) * 100 : null
          map[host.id] = { cpu, memPct, source: 'proxmox' }
          continue
        }
      }
      const agent = metrics[host.id]
      map[host.id] = {
        cpu: agent?.cpu_usage_percent ?? null,
        memPct: agent?.memory_percent ?? null,
        source: 'agent',
      }
    }
    return map
  })

  function effectiveMetrics(hostId: string): EffectiveMetric {
    return effectiveMetricsByHost.value[hostId] ?? EMPTY_METRIC
  }

  const allTags = computed(() => {
    const set = new Set<string>()
    for (const host of hosts.value as DashboardHostRecord[]) {
      for (const tag of host.tags || []) set.add(tag)
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  })

  const filteredHosts = computed(() => {
    const query = searchQuery.value.trim().toLowerCase()
    return hosts.value.filter((host: DashboardHostRecord) => {
      if (statusFilter.value !== 'all' && host.status !== statusFilter.value) return false
      if (tagFilter.value !== 'all' && !(host.tags || []).includes(tagFilter.value)) return false
      if (!query) return true
      return [host.name, host.hostname, host.ip_address, host.os, ...(host.tags || [])]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query)
    })
  })

  const sortedHosts = computed(() => {
    const list = [...filteredHosts.value]
    const direction = sortDir.value === 'asc' ? 1 : -1
    const statusOrder: Record<HostStatus, number> = { online: 0, warning: 1, offline: 2 }
    const metricsMap = effectiveMetricsByHost.value

    list.sort((a: DashboardHostRecord, b: DashboardHostRecord) => {
      let aVal, bVal
      switch (sortKey.value) {
        case 'status':
          aVal = statusOrder[(a.status as HostStatus)] ?? 99
          bVal = statusOrder[(b.status as HostStatus)] ?? 99
          break
        case 'ip_os':
          aVal = `${a.ip_address || ''} ${a.os || ''}`.trim().toLowerCase()
          bVal = `${b.ip_address || ''} ${b.os || ''}`.trim().toLowerCase()
          break
        case 'agent':
          aVal = String(a.agent_version || '').toLowerCase()
          bVal = String(b.agent_version || '').toLowerCase()
          break
        case 'cpu':
          aVal = metricsMap[a.id]?.cpu ?? -1
          bVal = metricsMap[b.id]?.cpu ?? -1
          break
        case 'ram':
          aVal = metricsMap[a.id]?.memPct ?? -1
          bVal = metricsMap[b.id]?.memPct ?? -1
          break
        case 'disk':
          aVal = diskUsage.value[a.id] ?? -1
          bVal = diskUsage.value[b.id] ?? -1
          break
        case 'apt':
          aVal = aptPendingHosts.value[a.id] ?? 0
          bVal = aptPendingHosts.value[b.id] ?? 0
          break
        case 'uptime':
          aVal = hostMetrics.value[a.id]?.uptime ?? -1
          bVal = hostMetrics.value[b.id]?.uptime ?? -1
          break
        case 'last_seen':
          aVal = a.last_seen ? new Date(a.last_seen).getTime() : 0
          bVal = b.last_seen ? new Date(b.last_seen).getTime() : 0
          break
        default:
          aVal = (a.name || a.hostname || '').toLowerCase()
          bVal = (b.name || b.hostname || '').toLowerCase()
      }
      if (aVal < bVal) return -1 * direction
      if (aVal > bVal) return 1 * direction
      return 0
    })
    return list
  })

  const chartPalette = useReactiveApexChartPalette()

  const summaryChartOptions = computed((): ApexOptions => {
    const colors = chartPalette.value
    const allPoints = (summaryChartSeries.value ?? []).flatMap((s) => s.data)
    return {
      chart: { type: 'area', background: 'transparent', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
      theme: { mode: 'dark' },
      fill: { type: 'solid', opacity: 0.12 },
      stroke: { curve: 'smooth', width: 2 },
      markers: { size: 0, hover: { size: 5 } },
      dataLabels: { enabled: false },
      legend: { show: true, position: 'top', labels: { colors: colors.legendText }, markers: { size: 5 } },
      grid: { borderColor: colors.grid },
      xaxis: {
        type: 'datetime',
        min: getMinPointTimestamp(allPoints),
        max: getMaxPointTimestamp(allPoints),
        labels: { style: { colors: colors.tickText }, formatter: (v: string) => formatSummaryChartTime(Number(v)) },
        axisBorder: { show: false },
        axisTicks: { show: false },
        tooltip: { enabled: false },
      },
      yaxis: { min: 0, max: 100, labels: { style: { colors: colors.tickText }, formatter: (v: number) => `${v.toFixed(0)}%` } },
      tooltip: {
        shared: true,
        x: { formatter: (v: number) => formatSummaryChartTime(v) },
        y: { formatter: (v: number) => (v != null ? `${Number(v).toFixed(1)}%` : '—') },
      },
    }
  })

  const proxmoxAutoSwitched = ref(false)

  const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket<WSDashboardSnapshot>('/api/v1/ws/dashboard', (payload) => {
    if (payload.type !== 'dashboard') return
    hostsStore.setHosts((payload.hosts || []) as Host[])
    hostMetrics.value = payload.host_metrics || {}
    dashboardStore.setVersionComparisons(payload.version_comparisons || [])
    dashboardStore.setAptPending(payload.apt_pending ?? 0)
    aptPendingHosts.value = payload.apt_pending_hosts || {}
    diskUsage.value = payload.disk_usage || {}
    proxmoxNodes.value = payload.proxmox_nodes || []
    proxmoxLinks.value = payload.proxmox_links || []
    selectedHostIds.value = selectedHostIds.value.filter((id) => hosts.value.some((h: DashboardHostRecord) => h.id === id))
    loading.value = false

    if (!proxmoxAutoSwitched.value && proxmoxNodes.value.length > 0) {
      proxmoxAutoSwitched.value = true
      chartSource.value = 'proxmox'
      fetchSummary()
    }
  }, { debounceMs: 200 })

  let cveRefreshTimer: ReturnType<typeof setInterval> | null = null
  let summaryRefreshTimer: ReturnType<typeof setInterval> | null = null

  // Refresh the summary chart on a cadence matching its bucket granularity,
  // not on every WS tick (the chart spans 1h–30d so per-tick refreshes are wasted).
  function summaryRefreshIntervalMs(): number {
    return bucketMinutesFor(summaryHours.value) * 60_000
  }

  function startSummaryRefreshTimer() {
    if (summaryRefreshTimer) clearInterval(summaryRefreshTimer)
    summaryRefreshTimer = setInterval(() => {
      if (document.visibilityState !== 'visible') return
      if (summaryLoading.value) return
      fetchSummary()
    }, summaryRefreshIntervalMs())
  }

  async function refreshCveSummary() {
    try {
      const response = await apiClient.getAptCVESummary()
      cveSummary.value = response.data || null
      cveLastUpdated.value = new Date()
    } catch {
      // Keep last known CVE summary on error.
    }
  }

  async function refreshDashboardOnReconnect() {
    await Promise.allSettled([
      fetchSummary(),
      fetchProxmoxSummary(),
      refreshCveSummary(),
    ])
  }

  function bucketMinutesFor(hours: number) {
    if (hours <= 6) return 1
    if (hours <= 24) return 5
    if (hours <= 168) return 15
    return 60
  }

  function formatSummaryChartTime(timestamp?: number) {
    if (!timestamp) return ''
    const date = dayjs(timestamp)
    if (!date.isValid()) return ''
    if (summaryHours.value < 24) return date.format('HH:mm')
    if (summaryHours.value < 720) return date.format('DD/MM HH:mm')
    return date.format('DD/MM')
  }

  function toSummaryPoint(point: DashboardMetricPoint, key: 'cpu_avg' | 'memory_avg'): ChartPoint | null {
    const timestamp = clampTimestamp(dayjs(point.timestamp).valueOf())
    const value = Number(point[key] ?? 0)
    if (!Number.isFinite(timestamp)) return null
    return { x: timestamp, y: value }
  }

  async function fetchSummary() {
    summaryLoading.value = true
    try {
      const colors = chartPalette.value
      const bucketMinutes = bucketMinutesFor(summaryHours.value)
      const isProxmox = chartSource.value === 'proxmox'
      const res = isProxmox
        ? await apiClient.getProxmoxNodeMetrics(summaryHours.value, bucketMinutes)
        : await apiClient.getMetricsSummary(summaryHours.value, bucketMinutes)

      const points: DashboardMetricPoint[] = Array.isArray(res.data) ? res.data : []
      if (!points.length) {
        summaryChartSeries.value = null
        return
      }

      summaryChartSeries.value = [
        {
          name: 'CPU %',
          data: points.map((p: DashboardMetricPoint) => toSummaryPoint(p, 'cpu_avg')).filter((p): p is ChartPoint => p != null),
          color: colors.cpu,
        },
        {
          name: 'RAM %',
          data: points.map((p: DashboardMetricPoint) => toSummaryPoint(p, 'memory_avg')).filter((p): p is ChartPoint => p != null),
          color: colors.ram,
        },
      ]
    } catch {
      summaryChartSeries.value = null
    } finally {
      summaryLoading.value = false
    }
  }

  function changeSummaryRange(hours: number) {
    summaryHours.value = hours
    fetchSummary()
    startSummaryRefreshTimer()
  }

  function clearSelection() {
    selectedHostIds.value = []
  }

  async function sendBulkApt(command: string) {
    if (!selectedHostIds.value.length || aptLoading.value) return
    const hostnames = hosts.value
      .filter((h: DashboardHostRecord) => selectedHostIds.value.includes(h.id))
      .map((h: DashboardHostRecord) => h.hostname || h.name)
      .join(', ')
    // `apt update` only refreshes the package index — non-destructive, unlike
    // upgrade/dist-upgrade which stay confirmed.
    if (command !== 'update') {
      const confirmed = await confirmBulkAction(
        `apt ${command}`,
        selectedHostIds.value.length,
        hostnames
          ? t('dashboard.bulkAptWarningWithHosts', { hostnames }, selectedHostIds.value.length)
          : t('dashboard.bulkAptWarning')
      )
      if (!confirmed) return
    }
    aptLoading.value = command
    try {
      await apiClient.sendAptCommand(selectedHostIds.value, command)
    } catch (e: unknown) {
      await dialog.confirm({ title: t('common.error'), message: translateError(e), variant: 'danger' })
    } finally {
      aptLoading.value = ''
    }
  }

  function formatUptime(seconds: number | null | undefined) {
    if (seconds == null) return 'N/A'
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    if (days > 0) return `${days}j ${hours}h`
    return `${hours}h ${Math.floor((seconds % 3600) / 60)}m`
  }

  function cpuColor(pct: number | null | undefined) {
    if (!pct) return 'text-secondary'
    if (pct > 90) return 'text-danger'
    if (pct > 70) return 'text-warning'
    return 'text-success'
  }

  function memColor(pct: number | null | undefined) {
    if (!pct) return 'text-secondary'
    if (pct > 90) return 'text-danger'
    if (pct > 75) return 'text-warning'
    return 'text-success'
  }

  function diskColor(pct: number | null | undefined) {
    if (pct == null) return 'text-secondary'
    if (pct > 90) return 'text-danger'
    if (pct > 75) return 'text-warning'
    return 'text-success'
  }

  function isAgentUpToDate(version: string) {
    return version && latestAgentVersion.value && version === latestAgentVersion.value
  }

  async function fetchProxmoxSummary() {
    try {
      const res = await apiClient.getProxmoxSummary()
      dashboardStore.setProxmoxSummary(res.data)
    } catch {
      // non-critique
    }
  }

  onMounted(() => {
    loading.value = true
    fetchSummary()
    fetchProxmoxSummary()
    apiClient
      .getSettings()
      .then((r) => {
        latestAgentVersion.value = r.data?.settings?.latestAgentVersion || ''
      })
      .catch(() => {})

    refreshCveSummary()
    cveRefreshTimer = setInterval(refreshCveSummary, 5 * 60 * 1000)
    startSummaryRefreshTimer()
    wsEvents.on('reconnected', refreshDashboardOnReconnect)
  })

  onUnmounted(() => {
    if (cveRefreshTimer) clearInterval(cveRefreshTimer)
    if (summaryRefreshTimer) clearInterval(summaryRefreshTimer)
    wsEvents.off('reconnected', refreshDashboardOnReconnect)
  })

  return {
    hosts,
    aptPending,
    versionComparisons,
    proxmoxSummary,
    hasProxmox,
    outdatedDockerImages,
    latestAgentVersion,
    cveSummary,
    cveLastUpdated,
    cveTimestampText,
    proxmoxNodes,
    proxmoxLinks,
    hostMetrics,
    aptPendingHosts,
    diskUsage,
    loading,
    searchQuery,
    statusFilter,
    tagFilter,
    allTags,
    sortKey,
    sortDir,
    selectedHostIds,
    aptLoading,
    showDockerVersions,
    summaryHours,
    summaryChartSeries,
    summaryLoading,
    chartSource,
    chartSources,
    selectedCount,
    canRunApt,
    metricsReady,
    wsStatus,
    wsError,
    retryCount,
    dataStaleAlert,
    reconnect,
    effectiveMetrics,
    effectiveMetricsByHost,
    filteredHosts,
    sortedHosts,
    summaryChartOptions,
    fetchSummary,
    refreshCveSummary,
    changeSummaryRange,
    clearSelection,
    sendBulkApt,
    formatUptime,
    cpuColor,
    memColor,
    diskColor,
    isAgentUpToDate,
  }
}
