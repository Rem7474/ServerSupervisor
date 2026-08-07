import { computed, ref, watch } from 'vue'
import { useUptimeProbes } from './useUptimeProbes'
import { useSslCertificates } from './useSslCertificates'
import { usePagination } from './usePagination'
import type { UptimeProbe } from '../types/uptime'
import type { SSLCertificate } from '../types/ssl'

export interface MonitoringRow {
  id: string
  name: string
  npmProxyHostId?: string
  npmProxyHostDomain?: string
  probe?: UptimeProbe
  cert?: SSLCertificate
}

const PAGE_SIZE = 25

// Merges the Monitoring page's two previously-separate tabs (uptime probes,
// SSL certificates) into one table. A probe and a cert only have a reliable
// shared identity when the same NPM proxy host's monitoring toggle created
// both (npm_proxy_host_id) — anything manually created stays its own row,
// since guessing a pairing by hostname text could match the wrong two
// entities. This composable owns only the merging/sorting/pagination layer;
// all fetching/CRUD/modal state stays in useUptimeProbes/useSslCertificates
// so nothing is duplicated.
export function useMonitoringOverview() {
  const uptime = useUptimeProbes()
  const ssl = useSslCertificates()

  const mergedRows = computed<MonitoringRow[]>(() => {
    const rows: MonitoringRow[] = []
    const certByHost = new Map<string, SSLCertificate>()
    for (const c of ssl.certs.value) {
      if (c.npm_proxy_host_id) certByHost.set(c.npm_proxy_host_id, c)
    }
    const usedCertIds = new Set<string>()

    for (const p of uptime.probes.value) {
      const pairedCert = p.npm_proxy_host_id ? certByHost.get(p.npm_proxy_host_id) : undefined
      if (pairedCert) usedCertIds.add(pairedCert.id)
      rows.push({
        id: p.id,
        name: p.npm_proxy_host_domain || p.name,
        npmProxyHostId: p.npm_proxy_host_id,
        npmProxyHostDomain: p.npm_proxy_host_domain,
        probe: p,
        cert: pairedCert,
      })
    }
    for (const c of ssl.certs.value) {
      if (usedCertIds.has(c.id)) continue
      rows.push({
        id: c.id,
        name: c.npm_proxy_host_domain || c.name,
        npmProxyHostId: c.npm_proxy_host_id,
        npmProxyHostDomain: c.npm_proxy_host_domain,
        cert: c,
      })
    }
    return rows
  })

  const search = ref('')
  const filteredRows = computed(() => {
    const q = search.value.trim().toLowerCase()
    if (!q) return mergedRows.value
    return mergedRows.value.filter((r) =>
      r.name.toLowerCase().includes(q) ||
      r.npmProxyHostDomain?.toLowerCase().includes(q) ||
      r.probe?.target.toLowerCase().includes(q) ||
      r.cert?.host.toLowerCase().includes(q)
    )
  })

  type RowCol = 'name' | 'status' | 'uptime' | 'ssl_days' | 'last_checked'
  const rowSort = ref<{ col: RowCol; dir: 'asc' | 'desc' }>({ col: 'status', dir: 'asc' })

  function toggleRowSort(col: RowCol): void {
    if (rowSort.value.col === col) {
      rowSort.value = { col, dir: rowSort.value.dir === 'asc' ? 'desc' : 'asc' }
    } else {
      rowSort.value = { col, dir: 'asc' }
    }
  }

  const sortedRows = computed(() => {
    const arr = [...filteredRows.value]
    const { col, dir } = rowSort.value
    const m = dir === 'asc' ? 1 : -1
    arr.sort((a, b) => {
      switch (col) {
        case 'name':
          return m * a.name.localeCompare(b.name, 'fr')
        case 'status': {
          // Rows without a probe (SSL-only) have no uptime status at all —
          // sorted alongside disabled probes, since neither is "actionable"
          // the way a real, currently-checked outage is.
          const rank = (r: MonitoringRow) => {
            if (!r.probe) return 4
            if (!r.probe.enabled) return 3
            if (r.probe.last_status === 'down') return 0
            if (r.probe.last_status === 'up') return 1
            return 2
          }
          return m * (rank(a) - rank(b))
        }
        case 'uptime': {
          const ua = a.probe ? (uptime.probeStats.value[a.probe.id]?.uptime_percent ?? -1) : -1
          const ub = b.probe ? (uptime.probeStats.value[b.probe.id]?.uptime_percent ?? -1) : -1
          return m * (ua - ub)
        }
        case 'ssl_days': {
          const da = a.cert?.days_remaining ?? Infinity
          const db = b.cert?.days_remaining ?? Infinity
          return m * (da - db)
        }
        case 'last_checked': {
          const latest = (r: MonitoringRow) => Math.max(
            r.probe?.last_checked_at ? new Date(r.probe.last_checked_at).getTime() : 0,
            r.cert?.last_checked_at ? new Date(r.cert.last_checked_at).getTime() : 0,
          )
          return m * (latest(a) - latest(b))
        }
      }
      return 0
    })
    return arr
  })

  const {
    currentPage: rowPage,
    totalPages: rowTotalPages,
    pagedItems: pagedRows,
    setPage: setRowPage,
    resetPage: resetRowPage,
  } = usePagination({ items: sortedRows, pageSize: PAGE_SIZE })

  watch(search, () => resetRowPage())

  const totalMonitored = computed(() => mergedRows.value.length)
  const filteredCount = computed(() => filteredRows.value.length)

  async function checkRowNow(row: MonitoringRow): Promise<void> {
    await Promise.all([
      row.probe ? uptime.checkProbeNow(row.probe) : Promise.resolve(),
      row.cert ? ssl.checkCertNow(row.cert) : Promise.resolve(),
    ])
  }

  // Uptime (30s) and SSL (60s) each auto-refresh on their own cadence
  // independently — combined into one visible toggle/timestamp for a single
  // shared PageRefreshBar instead of showing two redundant ones.
  const autoRefresh = computed<boolean>({
    get: () => uptime.autoRefresh.value,
    set: (v: boolean) => {
      uptime.autoRefresh.value = v
      ssl.autoRefresh.value = v
    },
  })
  const lastUpdatedAt = computed<Date | null>(() => {
    const u = uptime.lastUpdatedAt.value
    const s = ssl.lastUpdatedAt.value
    if (!u) return s
    if (!s) return u
    return u > s ? u : s
  })

  return {
    PAGE_SIZE,
    loading: computed(() => uptime.loadingProbes.value || ssl.loadingCerts.value),
    error: computed(() => uptime.error.value || ssl.error.value),
    downCount: uptime.downCount,
    expiringCount: ssl.expiringCount,
    totalMonitored,
    filteredCount,
    search,
    rowSort,
    toggleRowSort,
    pagedRows,
    rowPage,
    rowTotalPages,
    setRowPage,
    checkingProbeId: uptime.checkingProbeId,
    checkingCertId: ssl.checkingCertId,
    checkRowNow,
    probeStats: uptime.probeStats,
    probeHistory: uptime.probeHistory,
    probeBadge: uptime.probeBadge,
    probeStatusLabel: uptime.probeStatusLabel,
    uptimeBadgeClass: uptime.uptimeBadgeClass,
    daysLabel: ssl.daysLabel,
    daysBadge: ssl.daysBadge,
    autoRefresh,
    lastUpdatedAt,
    REFRESH_SEC: uptime.REFRESH_SEC,
    // probe CRUD (modal shared with the merged table's "Nouvelle sonde" action)
    probeModalOpen: uptime.probeModalOpen,
    savingProbe: uptime.savingProbe,
    probeFormError: uptime.probeFormError,
    probeForm: uptime.probeForm,
    openCreateProbe: uptime.openCreateProbe,
    openEditProbe: uptime.openEditProbe,
    closeProbeModal: uptime.closeProbeModal,
    saveProbe: uptime.saveProbe,
    confirmDeleteProbe: uptime.confirmDeleteProbe,
    // cert CRUD (modal shared with the merged table's "Ajouter un certificat" action)
    certModalOpen: ssl.certModalOpen,
    savingCert: ssl.savingCert,
    certFormError: ssl.certFormError,
    certForm: ssl.certForm,
    openCreateCert: ssl.openCreateCert,
    openEditCert: ssl.openEditCert,
    closeCertModal: ssl.closeCertModal,
    saveCert: ssl.saveCert,
    confirmDeleteCert: ssl.confirmDeleteCert,
  }
}
