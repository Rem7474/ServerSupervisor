import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../api'
import { npmApi } from '../api/npm'
import type { SSLCertificate } from '../types/ssl'
import { useConfirmDialog } from './useConfirmDialog'
import dayjs from '../utils/dayjs'
import { usePagination } from './usePagination'

type SSLCert = SSLCertificate

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

export function useSslCertificates() {
  const dialog = useConfirmDialog()

  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  const error = ref('')

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
      error.value = ''
    } catch (e: unknown) {
      error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Impossible de charger les certificats'
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
      error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Échec de la vérification'
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
    // See the identical guard+unification in useUptimeProbes.ts's
    // confirmDeleteProbe — same ON DELETE SET NULL desync risk against
    // npm_proxy_hosts, same fix: flip the NPM-side flag off as part of the
    // same action instead of leaving a second manual step in NPM.
    const ok = await dialog.confirm({
      title: 'Supprimer le certificat ?',
      message: c.npm_proxy_host_id
        ? `"${c.name}" est géré par le proxy host NPM "${c.npm_proxy_host_domain}". Le supprimer désactivera aussi le suivi SSL de ce proxy host dans NPM.`
        : `Cette action supprimera "${c.name}" du suivi.`,
      okLabel: 'Supprimer',
      destructive: true,
    })
    if (!ok) return
    try {
      if (c.npm_proxy_host_id) {
        await npmApi.updateProxyHost(c.npm_proxy_host_id, { ssl_monitoring_enabled: false })
      }
      await api.deleteSSLCertificate(c.id)
      await fetchCerts()
    } catch (e: unknown) {
      error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Suppression impossible'
    }
  }

  const {
    currentPage: certPage,
    totalPages: certTotalPages,
    pagedItems: pagedCerts,
    resetPage: resetCertPage,
    setPage: setCertPage,
  } = usePagination({ items: sortedCerts, pageSize: PAGE_SIZE })

  watch(certSort, resetCertPage, { deep: true })

  let refreshTimer: ReturnType<typeof setInterval> | undefined
  onMounted(() => {
    fetchCerts()
    refreshTimer = setInterval(() => { if (autoRefresh.value) fetchCerts() }, REFRESH_SEC * 1000)
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
