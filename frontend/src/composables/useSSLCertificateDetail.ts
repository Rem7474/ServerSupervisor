import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { SSLCertificate, SSLCertificateEvent } from '../types/ssl'
import dayjs from '../utils/dayjs'

type SSLCert = SSLCertificate

export function useSSLCertificateDetail() {
  const route = useRoute()
  const certId = route.params.id as string
  const signal = useAbortSignal()

  const cert = ref<SSLCert | null>(null)
  const events = ref<SSLCertificateEvent[]>([])
  const loading = ref(false)
  const loadingEvents = ref(false)
  const error = ref('')
  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  const REFRESH_SEC = 60

  function formatDate(ts: string | undefined | null): string {
    return ts ? dayjs(ts).format('YYYY-MM-DD') : '—'
  }

  function shortDN(s: string | undefined): string {
    if (!s) return ''
    const cn = /CN=([^,]+)/.exec(s)
    return cn ? cn[1] : s.split(',')[0]
  }

  function certDuration(from: string | undefined, to: string | undefined): string {
    if (!from || !to) return '—'
    const days = dayjs(to).diff(dayjs(from), 'day')
    if (days >= 365) return `${Math.round(days / 365 * 10) / 10} ans`
    return `${days}j`
  }

  const statusLabel = computed(() => {
    if (!cert.value) return ''
    const d = cert.value.days_remaining
    if (d == null) return 'Inconnu'
    if (d < 0) return 'Expiré'
    if (d <= 7) return 'Critique'
    if (d <= 30) return 'Attention'
    return 'Valide'
  })

  const statusColor = computed(() => {
    if (!cert.value) return ''
    const d = cert.value.days_remaining
    if (d == null) return 'text-secondary'
    if (d < 0) return 'text-danger'
    if (d <= 7) return 'text-danger'
    if (d <= 30) return 'text-warning'
    return 'text-success'
  })

  const daysColor = computed(() => statusColor.value)

  const daysLabel = computed(() => {
    if (!cert.value) return ''
    const d = cert.value.days_remaining
    if (d == null) return 'Inconnu'
    if (d < 0) return `Expiré (${Math.abs(d)}j)`
    return `${d} jour${d > 1 ? 's' : ''}`
  })

  async function fetchCert(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const { data } = await api.getSSLCertificate(certId, signal)
      cert.value = data
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Impossible de charger le certificat'
    } finally {
      loading.value = false
    }
  }

  async function fetchEvents(): Promise<void> {
    loadingEvents.value = true
    try {
      const { data } = await api.getSSLCertificateHistory(certId, signal)
      events.value = data?.events || []
    } catch {
      // non-fatal — history may be empty
    } finally {
      loadingEvents.value = false
    }
  }

  async function fetchAll(): Promise<void> {
    await Promise.all([fetchCert(), fetchEvents()])
  }

  let refreshTimer: ReturnType<typeof setInterval> | undefined
  onMounted(() => {
    fetchAll()
    refreshTimer = setInterval(() => { if (autoRefresh.value) fetchAll() }, REFRESH_SEC * 1000)
  })
  onUnmounted(() => {
    if (refreshTimer) clearInterval(refreshTimer)
  })

  return {
    cert,
    events,
    loading,
    loadingEvents,
    error,
    autoRefresh,
    lastUpdatedAt,
    REFRESH_SEC,
    formatDate,
    shortDN,
    certDuration,
    statusLabel,
    statusColor,
    daysColor,
    daysLabel,
  }
}
