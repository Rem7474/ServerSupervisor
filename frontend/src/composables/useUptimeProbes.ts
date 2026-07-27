import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../api'
import type { UptimeProbe } from '../types/uptime'
import { useConfirmDialog } from './useConfirmDialog'
import { usePagination } from './usePagination'

type Probe = UptimeProbe

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

const REFRESH_SEC = 30
const PAGE_SIZE = 25

export function useUptimeProbes() {
  const dialog = useConfirmDialog()

  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  const error = ref('')

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
      error.value = ''
      fetchAllProbeStats()
    } catch (e: unknown) {
      error.value = (e as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error
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
      error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Échec de la vérification'
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
      error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Suppression impossible'
    }
  }

  const {
    currentPage: probePage,
    totalPages: probeTotalPages,
    pagedItems: pagedProbes,
    resetPage: resetProbePage,
    setPage: setProbesPage,
  } = usePagination({ items: sortedProbes, pageSize: PAGE_SIZE })

  watch(probeSort, resetProbePage, { deep: true })

  let refreshTimer: ReturnType<typeof setInterval> | undefined
  onMounted(() => {
    fetchProbes()
    refreshTimer = setInterval(() => { if (autoRefresh.value) fetchProbes() }, REFRESH_SEC * 1000)
  })
  onUnmounted(() => {
    if (refreshTimer) clearInterval(refreshTimer)
  })

  return {
    REFRESH_SEC,
    PAGE_SIZE,
    autoRefresh,
    lastUpdatedAt,
    error,
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
  }
}
