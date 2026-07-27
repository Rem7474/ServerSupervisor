import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../api'
import type { UptimeProbe } from '../types/uptime'
import type { SSLCertificate } from '../types/ssl'
import { useConfirmDialog } from './useConfirmDialog'
import dayjs from '../utils/dayjs'
import { usePagination } from './usePagination'

type Probe = UptimeProbe
type SSLCert = SSLCertificate

interface ProbeForm {
  id: string
  name: string
  type: string
  target: string
  interval_sec: number
  timeout_sec: number
  expected_status: number
  expected_body_regex: string
  follow_redirects: boolean
  verify_tls: boolean
  enabled: boolean
}

interface CertForm {
  id: string
  name: string
  host: string
  port: number
  server_name: string
  enabled: boolean
}

const REFRESH_SEC = 30
const PAGE_SIZE = 25

export function useMonitoring() {
  const dialog = useConfirmDialog()

  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  // Kept separate (not a shared `error`) so a probe-fetch failure doesn't show
  // up while the user is looking at the SSL tab and vice versa.
  const probeError = ref('')
  const certError = ref('')

  // ── Uptime ────────────────────────────────────────────────────────────────────
  const probes = ref<Probe[]>([])
  const loadingProbes = ref(false)
  const probeStats = ref<Record<string, { uptime_percent: number }>>({})
  const checkingProbeId = ref('')

  const downCount = computed(() => probes.value.filter((p) => p.last_status === 'down').length)

  type ProbeCol = 'name' | 'status' | 'uptime' | 'latency' | 'last_checked'
  const probeSort = ref<{ col: ProbeCol; dir: 'asc' | 'desc' }>({ col: 'status', dir: 'asc' })

  function toggleProbeSort(col: ProbeCol): void {
    if (probeSort.value.col === col) {
      probeSort.value = { col, dir: probeSort.value.dir === 'asc' ? 'desc' : 'asc' }
    } else {
      probeSort.value = { col, dir: 'asc' }
    }
  }

  const sortedProbes = computed(() => {
    const arr = [...probes.value]
    const { col, dir } = probeSort.value
    const m = dir === 'asc' ? 1 : -1
    arr.sort((a, b) => {
      switch (col) {
        case 'name': return m * a.name.localeCompare(b.name)
        case 'status': {
          const rank = (p: Probe) => p.last_status === 'down' ? 0 : p.last_status === 'up' ? 1 : 2
          return m * (rank(a) - rank(b))
        }
        case 'uptime': {
          const ua = probeStats.value[a.id]?.uptime_percent ?? -1
          const ub = probeStats.value[b.id]?.uptime_percent ?? -1
          return m * (ua - ub)
        }
        case 'latency': {
          const la = a.last_status === 'up' && a.last_latency_ms != null ? a.last_latency_ms : Infinity
          const lb = b.last_status === 'up' && b.last_latency_ms != null ? b.last_latency_ms : Infinity
          return m * (la - lb)
        }
        case 'last_checked': {
          const ta = a.last_checked_at ? new Date(a.last_checked_at).getTime() : 0
          const tb = b.last_checked_at ? new Date(b.last_checked_at).getTime() : 0
          return m * (ta - tb)
        }
      }
      return 0
    })
    return arr
  })

  function probeBadge(p: Probe): string {
    if (!p.enabled) return 'bg-secondary-lt text-secondary'
    if (p.last_status === 'up') return 'bg-green-lt text-green'
    if (p.last_status === 'down') return 'bg-red-lt text-red'
    return 'bg-secondary-lt text-secondary'
  }

  function probeStatusLabel(p: Probe): string {
    if (p.last_status === 'up') return 'UP'
    if (p.last_status === 'down') return 'DOWN'
    return 'Inconnue'
  }

  function uptimeBadgeClass(pct: number): string {
    if (pct >= 99) return 'bg-green-lt text-green'
    if (pct >= 95) return 'bg-yellow-lt text-yellow'
    return 'bg-red-lt text-red'
  }

  async function fetchProbes(): Promise<void> {
    loadingProbes.value = true
    try {
      const { data } = await api.getUptimeProbes()
      probes.value = data?.probes || []
      lastUpdatedAt.value = new Date()
      probeError.value = ''
      fetchAllProbeStats()
    } catch (e: unknown) {
      probeError.value = (e as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error
        || (e as { message?: string })?.message || 'Impossible de charger les sondes'
    } finally {
      loadingProbes.value = false
    }
  }

  async function fetchAllProbeStats(): Promise<void> {
    const results = await Promise.allSettled(
      probes.value.map((p) => api.getUptimeStats(p.id, 24).then((r) => ({ id: p.id, data: r.data })))
    )
    const map: Record<string, { uptime_percent: number }> = {}
    for (const r of results) {
      if (r.status === 'fulfilled') {
        map[r.value.id] = { uptime_percent: r.value.data?.uptime_percent ?? 0 }
      }
    }
    probeStats.value = map
  }

  async function checkProbeNow(p: Probe): Promise<void> {
    checkingProbeId.value = p.id
    try {
      await api.checkUptimeProbeNow(p.id)
      await fetchProbes()
    } catch (e: unknown) {
      probeError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Échec de la vérification'
    } finally {
      checkingProbeId.value = ''
    }
  }

  // probe form
  const probeModalOpen = ref(false)
  const savingProbe = ref(false)
  const probeFormError = ref('')
  const probeForm = ref<ProbeForm>(emptyProbeForm())

  function emptyProbeForm(): ProbeForm {
    return { id: '', name: '', type: 'http', target: '', interval_sec: 60, timeout_sec: 10,
      expected_status: 200, expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true }
  }

  function openCreateProbe(): void {
    probeForm.value = emptyProbeForm()
    probeFormError.value = ''
    probeModalOpen.value = true
  }

  function openEditProbe(p: Probe): void {
    probeForm.value = {
      id: p.id, name: p.name, type: p.type, target: p.target,
      interval_sec: p.interval_sec, timeout_sec: p.timeout_sec,
      expected_status: p.expected_status, expected_body_regex: p.expected_body_regex || '',
      follow_redirects: p.follow_redirects, verify_tls: p.verify_tls, enabled: p.enabled,
    }
    probeFormError.value = ''
    probeModalOpen.value = true
  }

  function closeProbeModal(): void {
    probeModalOpen.value = false
    savingProbe.value = false
  }

  async function saveProbe(): Promise<void> {
    savingProbe.value = true
    probeFormError.value = ''
    try {
      const { id: _id, ...body } = probeForm.value
      if (probeForm.value.id) {
        await api.updateUptimeProbe(probeForm.value.id, body)
      } else {
        await api.createUptimeProbe(body)
      }
      closeProbeModal()
      await fetchProbes()
    } catch (e: unknown) {
      probeFormError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Erreur lors de l\'enregistrement'
    } finally {
      savingProbe.value = false
    }
  }

  async function confirmDeleteProbe(p: Probe): Promise<void> {
    const ok = await dialog.confirm({
      title: 'Supprimer la sonde ?',
      message: `Cette action supprimera "${p.name}" et tout son historique.`,
      okLabel: 'Supprimer',
      destructive: true,
    })
    if (!ok) return
    try {
      await api.deleteUptimeProbe(p.id)
      await fetchProbes()
    } catch (e: unknown) {
      probeError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Suppression impossible'
    }
  }

  // ── SSL ───────────────────────────────────────────────────────────────────────
  const certs = ref<SSLCert[]>([])
  const loadingCerts = ref(false)
  const checkingCertId = ref('')

  const expiringCount = computed(() => certs.value.filter((c) => {
    const d = c.days_remaining
    return d != null && d <= 30
  }).length)

  type CertCol = 'name' | 'expiration' | 'days' | 'last_checked'
  const certSort = ref<{ col: CertCol; dir: 'asc' | 'desc' }>({ col: 'days', dir: 'asc' })

  function toggleCertSort(col: CertCol): void {
    if (certSort.value.col === col) {
      certSort.value = { col, dir: certSort.value.dir === 'asc' ? 'desc' : 'asc' }
    } else {
      certSort.value = { col, dir: 'asc' }
    }
  }

  const sortedCerts = computed(() => {
    const arr = [...certs.value]
    const { col, dir } = certSort.value
    const m = dir === 'asc' ? 1 : -1
    arr.sort((a, b) => {
      switch (col) {
        case 'name': return m * a.name.localeCompare(b.name)
        case 'days': {
          const da = a.days_remaining ?? Infinity
          const db = b.days_remaining ?? Infinity
          return m * (da - db)
        }
        case 'expiration': {
          const ta = a.valid_to ? new Date(a.valid_to).getTime() : Infinity
          const tb = b.valid_to ? new Date(b.valid_to).getTime() : Infinity
          return m * (ta - tb)
        }
        case 'last_checked': {
          const ta = a.last_checked_at ? new Date(a.last_checked_at).getTime() : 0
          const tb = b.last_checked_at ? new Date(b.last_checked_at).getTime() : 0
          return m * (ta - tb)
        }
      }
      return 0
    })
    return arr
  })

  const {
    currentPage: probePage,
    totalPages: probeTotalPages,
    pagedItems: pagedProbes,
    resetPage: resetProbePage,
    setPage: setProbesPage,
  } = usePagination({ items: sortedProbes, pageSize: PAGE_SIZE })

  const {
    currentPage: certPage,
    totalPages: certTotalPages,
    pagedItems: pagedCerts,
    resetPage: resetCertPage,
    setPage: setCertPage,
  } = usePagination({ items: sortedCerts, pageSize: PAGE_SIZE })

  watch(probeSort, resetProbePage, { deep: true })
  watch(certSort, resetCertPage, { deep: true })

  function formatDate(ts: string | undefined | null): string {
    return ts ? dayjs(ts).format('YYYY-MM-DD') : '—'
  }

  function shortIssuer(s: string | undefined): string {
    if (!s) return ''
    const cn = /CN=([^,]+)/.exec(s)
    return cn ? cn[1] : s.split(',')[0]
  }

  function daysLabel(d: number | null | undefined): string {
    if (d == null) return 'Inconnu'
    if (d < 0) return `Expiré (${Math.abs(d)}j)`
    return `${d}j`
  }

  function daysBadge(d: number | null | undefined): string {
    if (d == null) return 'bg-secondary-lt text-secondary'
    if (d < 0) return 'bg-red text-white'
    if (d <= 7) return 'bg-red-lt text-red'
    if (d <= 30) return 'bg-yellow-lt text-yellow'
    return 'bg-green-lt text-green'
  }

  async function fetchCerts(): Promise<void> {
    loadingCerts.value = true
    try {
      const { data } = await api.getSSLCertificates()
      certs.value = data?.certificates || []
      lastUpdatedAt.value = new Date()
      certError.value = ''
    } catch (e: unknown) {
      certError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Impossible de charger les certificats'
    } finally {
      loadingCerts.value = false
    }
  }

  async function checkCertNow(c: SSLCert): Promise<void> {
    checkingCertId.value = c.id
    try {
      await api.checkSSLCertificateNow(c.id)
      await fetchCerts()
    } catch (e: unknown) {
      certError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Échec de la vérification'
    } finally {
      checkingCertId.value = ''
    }
  }

  // cert form
  const certModalOpen = ref(false)
  const savingCert = ref(false)
  const certFormError = ref('')
  const certForm = ref<CertForm>(emptyCertForm())

  function emptyCertForm(): CertForm {
    return { id: '', name: '', host: '', port: 443, server_name: '', enabled: true }
  }

  function openCreateCert(): void {
    certForm.value = emptyCertForm()
    certFormError.value = ''
    certModalOpen.value = true
  }

  function openEditCert(c: SSLCert): void {
    certForm.value = { id: c.id, name: c.name, host: c.host, port: c.port,
      server_name: c.server_name || '', enabled: c.enabled }
    certFormError.value = ''
    certModalOpen.value = true
  }

  function closeCertModal(): void {
    certModalOpen.value = false
    savingCert.value = false
  }

  async function saveCert(): Promise<void> {
    savingCert.value = true
    certFormError.value = ''
    try {
      const { id: _id, ...body } = certForm.value
      if (certForm.value.id) {
        await api.updateSSLCertificate(certForm.value.id, body)
      } else {
        await api.createSSLCertificate(body)
      }
      closeCertModal()
      await fetchCerts()
    } catch (e: unknown) {
      certFormError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Erreur lors de l\'enregistrement'
    } finally {
      savingCert.value = false
    }
  }

  async function confirmDeleteCert(c: SSLCert): Promise<void> {
    const ok = await dialog.confirm({
      title: 'Supprimer le certificat ?',
      message: `Cette action supprimera "${c.name}" du suivi.`,
      okLabel: 'Supprimer',
      destructive: true,
    })
    if (!ok) return
    try {
      await api.deleteSSLCertificate(c.id)
      await fetchCerts()
    } catch (e: unknown) {
      certError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Suppression impossible'
    }
  }

  // ── lifecycle ─────────────────────────────────────────────────────────────────
  let refreshTimer: ReturnType<typeof setInterval> | undefined

  function refreshAll() {
    fetchProbes()
    fetchCerts()
  }

  onMounted(() => {
    refreshAll()
    refreshTimer = setInterval(() => { if (autoRefresh.value) refreshAll() }, REFRESH_SEC * 1000)
  })
  onUnmounted(() => {
    if (refreshTimer) clearInterval(refreshTimer)
  })

  return {
    REFRESH_SEC,
    PAGE_SIZE,
    autoRefresh,
    lastUpdatedAt,
    probeError,
    certError,

    // uptime
    probes,
    loadingProbes,
    probeStats,
    checkingProbeId,
    downCount,
    probeSort,
    toggleProbeSort,
    pagedProbes,
    probeBadge,
    probeStatusLabel,
    uptimeBadgeClass,
    checkProbeNow,
    probeModalOpen,
    savingProbe,
    probeFormError,
    probeForm,
    openCreateProbe,
    openEditProbe,
    closeProbeModal,
    saveProbe,
    confirmDeleteProbe,
    probePage,
    probeTotalPages,
    setProbesPage,

    // ssl
    certs,
    loadingCerts,
    checkingCertId,
    expiringCount,
    certSort,
    toggleCertSort,
    pagedCerts,
    formatDate,
    shortIssuer,
    daysLabel,
    daysBadge,
    checkCertNow,
    certModalOpen,
    savingCert,
    certFormError,
    certForm,
    openCreateCert,
    openEditCert,
    closeCertModal,
    saveCert,
    confirmDeleteCert,
    certPage,
    certTotalPages,
    setCertPage,
  }
}
