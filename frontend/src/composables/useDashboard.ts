import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useDashboardStore } from '../stores/dashboard'
import { useWebSocket, wsEvents } from './useWebSocket'
import { useConfirmDialog } from './useConfirmDialog'
import { confirmBulkAction } from '../utils/bulkActionHelpers'
import { translateError } from '../utils/translateError'
import { formatRelativeTime } from './useDateFormatter'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

type AnyRecord = Record<string, unknown>
type SortDirection = 'asc' | 'desc'
type HostStatus = 'online' | 'warning' | 'offline'

interface DashboardHostRecord {
  id: string
  status?: string
  name?: string
  hostname?: string
  ip_address?: string
  os?: string
  last_seen?: string | number | Date | null
}

interface DashboardMetricPoint {
  timestamp?: string | number | Date
  cpu_avg?: number | string | null
  memory_avg?: number | string | null
}

interface DashboardAgentMetric {
  cpu_usage_percent?: number | null
  memory_percent?: number | null
}

interface DashboardProxmoxLinkRecord {
  host_id: string
  metrics_source?: 'proxmox' | 'auto' | string
  cpu_usage?: number | null
  mem_alloc?: number
  mem_usage?: number
}

interface DashboardWebSocketPayload {
  type?: string
  hosts?: DashboardHostRecord[]
  host_metrics?: Record<string, DashboardAgentMetric>
  version_comparisons?: Array<{
    is_up_to_date?: boolean
    running_version?: string
    update_confirmed?: boolean
  }>
  apt_pending?: number
  apt_pending_hosts?: Record<string, number>
  disk_usage?: Record<string, number>
  proxmox_nodes?: AnyRecord[]
  proxmox_links?: DashboardProxmoxLinkRecord[]
}

interface TooltipContext {
  dataset: { label?: string }
  parsed: { y: number }
}

interface SummaryDataset {
  label: string
  data: number[]
  borderColor: string
  backgroundColor: string
  fill: boolean
}

interface SummaryChartData {
  labels: string[]
  datasets: SummaryDataset[]
}

interface DashboardChartPalette {
  legendText: string
  tickText: string
  grid: string
  tooltipBackground: string
  tooltipText: string
  tooltipBorder: string
  cpuBorder: string
  cpuBackground: string
  ramBorder: string
  ramBackground: string
}

const FALLBACK_CHART_PALETTE: DashboardChartPalette = {
  legendText: '#1f2937',
  tickText: '#1f2937',
  grid: 'rgba(107,114,128,0.15)',
  tooltipBackground: 'rgba(17,24,39,0.90)',
  tooltipText: '#ffffff',
  tooltipBorder: '#4b5563',
  cpuBorder: '#206bc4',
  cpuBackground: 'rgba(32,107,196,0.12)',
  ramBorder: '#2fb344',
  ramBackground: 'rgba(47,179,68,0.12)',
}

function getThemeStyles(): { body: CSSStyleDeclaration | null; root: CSSStyleDeclaration | null } | null {
  if (typeof window === 'undefined' || typeof document === 'undefined') return null
  return {
    body: document.body ? window.getComputedStyle(document.body) : null,
    root: window.getComputedStyle(document.documentElement),
  }
}

function getCssVarValue(name: string, fallback: string): string {
  const styles = getThemeStyles()
  if (!styles) return fallback
  const value = styles.body?.getPropertyValue(name).trim() || styles.root?.getPropertyValue(name).trim() || ''
  return value || fallback
}

function isDarkRgbColor(color: string): boolean {
  const rgb = color.match(/^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/i)
  const rgba = color.match(/^rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9]*\.?[0-9]+)\s*\)$/i)
  const values = rgb || rgba
  if (!values) return false

  const r = Number(values[1])
  const g = Number(values[2])
  const b = Number(values[3])
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance < 0.5
}

function resolveCssColorForCanvas(color: string, fallback: string): string {
  if (!color) return fallback
  if (typeof window === 'undefined' || typeof document === 'undefined') return fallback

  const probe = document.createElement('span')
  probe.style.color = color
  probe.style.position = 'fixed'
  probe.style.left = '-9999px'
  probe.style.top = '-9999px'
  probe.style.visibility = 'hidden'

  document.body.appendChild(probe)
  const resolved = window.getComputedStyle(probe).color.trim()
  document.body.removeChild(probe)

  if (!resolved || resolved === 'rgba(0, 0, 0, 0)' || resolved === 'transparent') {
    return fallback
  }
  return resolved
}

function toRgba(color: string, alpha: number, fallback: string): string {
  const clamped = Math.max(0, Math.min(1, alpha))
  const hex = color.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i)
  if (hex) {
    const raw = hex[1]
    const normalized = raw.length === 3 ? raw.split('').map((c) => c + c).join('') : raw
    const int = Number.parseInt(normalized, 16)
    const r = (int >> 16) & 255
    const g = (int >> 8) & 255
    const b = int & 255
    return `rgba(${r},${g},${b},${clamped})`
  }

  const rgb = color.match(/^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/i)
  if (rgb) {
    return `rgba(${rgb[1]},${rgb[2]},${rgb[3]},${clamped})`
  }

  const rgba = color.match(/^rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9]*\.?[0-9]+)\s*\)$/i)
  if (rgba) {
    return `rgba(${rgba[1]},${rgba[2]},${rgba[3]},${clamped})`
  }

  return fallback
}

function getDashboardChartPalette(): DashboardChartPalette {
  const pageBackground = resolveCssColorForCanvas(
    getCssVarValue('--tblr-bg-surface', getCssVarValue('--tblr-body-bg', '#111827')),
    '#111827',
  )
  const lightText = resolveCssColorForCanvas(getCssVarValue('--tblr-light', '#f8fafc'), '#f8fafc')
  const darkText = resolveCssColorForCanvas(getCssVarValue('--tblr-dark', '#111827'), '#111827')
  const textOnPage = isDarkRgbColor(pageBackground) ? lightText : darkText
  const legendText = textOnPage
  const tickText = toRgba(textOnPage, 0.82, textOnPage)
  const gridBase = resolveCssColorForCanvas(
    getCssVarValue('--tblr-border-color', FALLBACK_CHART_PALETTE.tooltipBorder),
    FALLBACK_CHART_PALETTE.tooltipBorder,
  )
  const tooltipText = textOnPage
  const primary = resolveCssColorForCanvas(
    getCssVarValue('--tblr-primary', FALLBACK_CHART_PALETTE.cpuBorder),
    FALLBACK_CHART_PALETTE.cpuBorder,
  )
  const success = resolveCssColorForCanvas(
    getCssVarValue('--tblr-success', FALLBACK_CHART_PALETTE.ramBorder),
    FALLBACK_CHART_PALETTE.ramBorder,
  )

  return {
    legendText,
    tickText,
    grid: toRgba(gridBase, 0.35, FALLBACK_CHART_PALETTE.grid),
    tooltipBackground: pageBackground,
    tooltipText,
    tooltipBorder: resolveCssColorForCanvas(gridBase, FALLBACK_CHART_PALETTE.tooltipBorder),
    cpuBorder: primary,
    cpuBackground: toRgba(primary, 0.12, FALLBACK_CHART_PALETTE.cpuBackground),
    ramBorder: success,
    ramBackground: toRgba(success, 0.12, FALLBACK_CHART_PALETTE.ramBackground),
  }
}

export function useDashboard() {
  const dashboardStore = useDashboardStore()
  const {
    hosts,
    aptPending,
    versionComparisons,
    proxmoxSummary,
    hasProxmox,
    outdatedDockerImages,
  } = storeToRefs(dashboardStore)

  const latestAgentVersion = ref('')
  const cveSummary = ref<AnyRecord | null>(null)
  const cveLastUpdated = ref<Date | null>(null)
  const cveTimestampText = computed(() => formatRelativeTime(cveLastUpdated.value, 'Jamais mis à jour', true))
  const proxmoxNodes = ref<AnyRecord[]>([])
  const proxmoxLinks = ref<DashboardProxmoxLinkRecord[]>([])

  const hostMetrics = ref<Record<string, DashboardAgentMetric>>({})
  const aptPendingHosts = ref<Record<string, number>>({})
  const diskUsage = ref<Record<string, number>>({})
  const loading = ref(true)

  const searchQuery = ref('')
  const statusFilter = ref('all')
  const sortKey = ref(localStorage.getItem('dashboard.sortKey') || 'name')
  const sortDir = ref<SortDirection>((localStorage.getItem('dashboard.sortDir') as SortDirection) || 'asc')
  watch(sortKey, (v) => localStorage.setItem('dashboard.sortKey', v))
  watch(sortDir, (v) => localStorage.setItem('dashboard.sortDir', v))

  const selectedHostIds = ref<string[]>([])
  const aptLoading = ref('')
  const showDockerVersions = ref(false)

  const summaryHours = ref(24)
  const summaryChartData = ref<SummaryChartData | null>(null)
  const summaryLoading = ref(false)
  const chartSource = ref('agents')
  const chartSources = [
    { key: 'agents', label: 'Agents hôtes' },
    { key: 'proxmox', label: 'Nœuds Proxmox' },
  ]

  const auth = useAuthStore()
  const dialog = useConfirmDialog()

  const selectedCount = computed(() => selectedHostIds.value.length)
  const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

  const proxmoxLinkByHostId = computed(() => {
    const m: Record<string, DashboardProxmoxLinkRecord> = {}
    for (const link of proxmoxLinks.value) {
      m[link.host_id] = link
    }
    return m
  })

  function effectiveMetrics(hostId: string) {
    const link = proxmoxLinkByHostId.value[hostId]
    const agent = hostMetrics.value[hostId]

    if (link) {
      const src = link.metrics_source
      const useProxmox = src === 'proxmox' || (src === 'auto' && link.cpu_usage != null)
      if (useProxmox) {
        const cpu = link.cpu_usage != null ? link.cpu_usage * 100 : null
        const memAlloc = typeof link.mem_alloc === 'number' ? link.mem_alloc : 0
        const memUsage = typeof link.mem_usage === 'number' ? link.mem_usage : 0
        const memPct = memAlloc > 0 ? (memUsage / memAlloc) * 100 : null
        return { cpu, memPct, source: 'proxmox' }
      }
    }
    return {
      cpu: agent?.cpu_usage_percent ?? null,
      memPct: agent?.memory_percent ?? null,
      source: 'agent',
    }
  }

  const filteredHosts = computed(() => {
    const query = searchQuery.value.trim().toLowerCase()
    return hosts.value.filter((host: DashboardHostRecord) => {
      if (statusFilter.value !== 'all' && host.status !== statusFilter.value) return false
      if (!query) return true
      return [host.name, host.hostname, host.ip_address, host.os]
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

    list.sort((a: DashboardHostRecord, b: DashboardHostRecord) => {
      let aVal, bVal
      switch (sortKey.value) {
        case 'status':
          aVal = statusOrder[(a.status as HostStatus)] ?? 99
          bVal = statusOrder[(b.status as HostStatus)] ?? 99
          break
        case 'cpu':
          aVal = effectiveMetrics(a.id).cpu ?? -1
          bVal = effectiveMetrics(b.id).cpu ?? -1
          break
        case 'apt':
          aVal = aptPendingHosts.value[a.id] ?? 0
          bVal = aptPendingHosts.value[b.id] ?? 0
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

  const summaryChartOptions = computed(() => {
    const colors = getDashboardChartPalette()
    return {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: true, position: 'top', labels: { color: colors.legendText, boxWidth: 12, padding: 12 } },
        tooltip: {
          enabled: true,
          mode: 'index',
          intersect: false,
          backgroundColor: colors.tooltipBackground,
          titleColor: colors.tooltipText,
          bodyColor: colors.tooltipText,
          borderColor: colors.tooltipBorder,
          borderWidth: 1,
          padding: 10,
          callbacks: {
            label: (ctx: TooltipContext) => `${ctx.dataset.label}: ${ctx.parsed.y.toFixed(1)}%`,
          },
        },
      },
      scales: {
        x: { display: true, grid: { color: colors.grid }, ticks: { color: colors.tickText, maxTicksLimit: 10 } },
        y: { display: true, min: 0, max: 100, grid: { color: colors.grid }, ticks: { color: colors.tickText } },
      },
      elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 5 }, line: { tension: 0.3 } },
      interaction: { mode: 'nearest', axis: 'x', intersect: false },
    }
  })

  const proxmoxAutoSwitched = ref(false)

  const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket<DashboardWebSocketPayload>('/api/v1/ws/dashboard', (payload) => {
    if (payload.type !== 'dashboard') return
    dashboardStore.setHosts(payload.hosts || [])
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

  async function fetchSummary() {
    summaryLoading.value = true
    try {
      const colors = getDashboardChartPalette()
      const bucketMinutes = bucketMinutesFor(summaryHours.value)
      const isProxmox = chartSource.value === 'proxmox'
      const res = isProxmox
        ? await apiClient.getProxmoxNodeMetrics(summaryHours.value, bucketMinutes)
        : await apiClient.getMetricsSummary(summaryHours.value, bucketMinutes)

      const points: DashboardMetricPoint[] = Array.isArray(res.data) ? res.data : []
      if (!points.length) {
        summaryChartData.value = null
        return
      }

      const labels = points.map((p: DashboardMetricPoint) =>
        summaryHours.value >= 24 ? dayjs(p.timestamp).format('DD/MM HH:mm') : dayjs(p.timestamp).format('HH:mm')
      )
      summaryChartData.value = {
        labels,
        datasets: [
          {
            label: 'CPU %',
            data: points.map((p: DashboardMetricPoint) => Number(p.cpu_avg ?? 0)),
            borderColor: colors.cpuBorder,
            backgroundColor: colors.cpuBackground,
            fill: true,
          },
          {
            label: 'RAM %',
            data: points.map((p: DashboardMetricPoint) => Number(p.memory_avg ?? 0)),
            borderColor: colors.ramBorder,
            backgroundColor: colors.ramBackground,
            fill: true,
          },
        ],
      }
    } catch {
      summaryChartData.value = null
    } finally {
      summaryLoading.value = false
    }
  }

  function changeSummaryRange(hours: number) {
    summaryHours.value = hours
    fetchSummary()
  }

  function selectAllFiltered() {
    const ids = sortedHosts.value.map((h: DashboardHostRecord) => h.id)
    selectedHostIds.value = Array.from(new Set([...selectedHostIds.value, ...ids]))
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
    const confirmed = await confirmBulkAction(
      `apt ${command}`,
      selectedHostIds.value.length,
      hostnames
        ? `Exécuter sur ${selectedHostIds.value.length} hôte${selectedHostIds.value.length > 1 ? 's' : ''} :\n${hostnames}\n\nCela peut affecter la stabilité de plusieurs serveurs.`
        : 'Cette action peut affecter la stabilité de plusieurs serveurs.'
    )
    if (!confirmed) return
    aptLoading.value = command
    try {
      await apiClient.sendAptCommand(selectedHostIds.value, command)
    } catch (e: unknown) {
      await dialog.confirm({ title: 'Erreur', message: translateError(e), variant: 'danger' })
    } finally {
      aptLoading.value = ''
    }
  }

  function formatUptime(seconds: number) {
    if (!seconds) return 'N/A'
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    if (days > 0) return `${days}j ${hours}h`
    return `${hours}h ${Math.floor((seconds % 3600) / 60)}m`
  }

  function cpuColor(pct: number | null | undefined) {
    if (!pct) return 'text-secondary'
    if (pct > 90) return 'text-red'
    if (pct > 70) return 'text-yellow'
    return 'text-green'
  }

  function memColor(pct: number | null | undefined) {
    if (!pct) return 'text-secondary'
    if (pct > 90) return 'text-red'
    if (pct > 75) return 'text-yellow'
    return 'text-green'
  }

  function diskColor(pct: number | null | undefined) {
    if (pct == null) return 'text-secondary'
    if (pct > 90) return 'text-red'
    if (pct > 75) return 'text-yellow'
    return 'text-green'
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
    wsEvents.on('reconnected', refreshDashboardOnReconnect)
  })

  onUnmounted(() => {
    if (cveRefreshTimer) clearInterval(cveRefreshTimer)
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
    sortKey,
    sortDir,
    selectedHostIds,
    aptLoading,
    showDockerVersions,
    summaryHours,
    summaryChartData,
    summaryLoading,
    chartSource,
    chartSources,
    selectedCount,
    canRunApt,
    wsStatus,
    wsError,
    retryCount,
    dataStaleAlert,
    reconnect,
    effectiveMetrics,
    filteredHosts,
    sortedHosts,
    summaryChartOptions,
    fetchSummary,
    refreshCveSummary,
    changeSummaryRange,
    selectAllFiltered,
    clearSelection,
    sendBulkApt,
    formatUptime,
    cpuColor,
    memColor,
    diskColor,
    isAgentUpToDate,
  }
}
