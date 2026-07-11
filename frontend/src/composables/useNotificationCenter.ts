import { ref, computed, watch, onMounted } from 'vue'
import api from '../api'
import type { NotificationItem } from '../types/generated'
import { addToast } from './useGlobalToast'
import { resolveIncidentHostRoute } from '../utils/incidentRouting'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'

const SEVERITY_FILTERS = [
  { value: '', label: 'Toute sévérité' },
  { value: 'warn', label: 'Warn' },
  { value: 'crit', label: 'Critique' },
] as const

const TYPE_FILTERS = [
  { value: '', label: 'Tous types' },
  { value: 'alert_incident', label: 'Alertes' },
  { value: 'release_tracker', label: 'Trackers' },
] as const

const STATUS_FILTERS = [
  { value: '', label: 'Tous' },
  { value: 'active', label: 'Actif' },
  { value: 'resolved', label: 'Résolu' },
] as const

export function useNotificationCenter() {
  const signal = useAbortSignal()

  const items = ref<NotificationItem[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref('')
  const markingRead = ref(false)
  const readAt = ref<string | null>(null)
  const currentLimit = ref(50)
  const resolvingId = ref<string | null>(null)

  const severityFilter = ref<'warn' | 'crit' | ''>('')
  const typeFilter = ref<'alert_incident' | 'release_tracker' | ''>('')
  const statusFilter = ref<'active' | 'resolved' | ''>('')

  const unreadCount = computed(() =>
    items.value.filter((n) => !readAt.value || new Date(n.triggered_at) > new Date(readAt.value)).length
  )

  function isUnread(item: NotificationItem): boolean {
    return !readAt.value || new Date(item.triggered_at) > new Date(readAt.value)
  }

  function isTrackerType(item: NotificationItem): boolean {
    return item.type === 'release_tracker_detected' || item.type === 'release_tracker_execution'
  }

  function notificationTitle(item: NotificationItem): string {
    if (isTrackerType(item)) return item.rule_name || 'Release tracker'
    return item.rule_name || 'Alerte'
  }

  function notificationResolved(item: NotificationItem): boolean {
    if (isTrackerType(item)) {
      return !!item.resolved_at || ['completed', 'success', 'failed', 'error'].includes((item.status || '').toLowerCase())
    }
    return !!item.resolved_at
  }

  function notificationRoute(item: NotificationItem): string {
    if (isTrackerType(item)) {
      if (item.tracker_id) return `/release-trackers/${encodeURIComponent(String(item.tracker_id))}`
      return '/git-webhooks?tab=trackers'
    }
    return resolveIncidentHostRoute(item.host_id, item.metric, item.link_host_id)
  }

  function metricUnit(metric?: string): string {
    if (!metric) return ''
    if (['cpu', 'memory', 'disk'].some((k) => metric.includes(k))) return '%'
    return ''
  }

  function iconBg(item: NotificationItem): string {
    if (isTrackerType(item)) return 'bg-blue text-white'
    if (item.severity === 'crit') return 'bg-red text-white'
    if (item.severity === 'warn') return 'bg-yellow text-white'
    return 'bg-secondary text-white'
  }

  function severityBadge(severity: string): string {
    return severity === 'crit' ? 'bg-red-lt text-red' : 'bg-yellow-lt text-yellow'
  }

  function resolvedBadge(item: NotificationItem): string {
    return notificationResolved(item) ? 'bg-green-lt text-green' : 'bg-red-lt text-red'
  }

  function incidentNumericId(item: NotificationItem): string {
    // item.id is formatted as "alert:123" for alert incidents
    return item.id.replace(/^alert:/, '')
  }

  async function resolveIncident(item: NotificationItem): Promise<void> {
    resolvingId.value = item.id
    try {
      await api.resolveAlertIncident(incidentNumericId(item))
      items.value = items.value.map((n) =>
        n.id === item.id ? { ...n, resolved_at: new Date().toISOString() } : n
      )
      addToast('Incident résolu', 'success')
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, 'Impossible de résoudre'), 'error')
    } finally {
      resolvingId.value = null
    }
  }

  async function load(limit = currentLimit.value): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await api.getNotifications({
        limit,
        severity: severityFilter.value || undefined,
        type: typeFilter.value || undefined,
        status: statusFilter.value || undefined,
      }, signal)
      items.value = res.data.notifications || []
      total.value = res.data.total || 0
      if (res.data.read_at !== undefined) readAt.value = res.data.read_at
    } catch (err: unknown) {
      if (isApiAbort(err)) return
      error.value = getApiErrorMessage(err, 'Erreur de chargement')
    } finally {
      loading.value = false
    }
  }

  async function loadMore(): Promise<void> {
    currentLimit.value += 50
    await load(currentLimit.value)
  }

  async function handleMarkRead(): Promise<void> {
    markingRead.value = true
    try {
      const res = await api.markNotificationsRead()
      readAt.value = res.data?.read_at ?? new Date().toISOString()
      addToast('Toutes les notifications marquées comme lues', 'success')
    } catch {
      addToast('Erreur lors du marquage', 'error')
    } finally {
      markingRead.value = false
    }
  }

  watch([severityFilter, typeFilter, statusFilter], () => {
    currentLimit.value = 50
    load()
  })

  onMounted(load)

  return {
    SEVERITY_FILTERS,
    TYPE_FILTERS,
    STATUS_FILTERS,
    items,
    total,
    loading,
    error,
    markingRead,
    resolvingId,
    severityFilter,
    typeFilter,
    statusFilter,
    unreadCount,
    isUnread,
    isTrackerType,
    notificationTitle,
    notificationResolved,
    notificationRoute,
    metricUnit,
    iconBg,
    severityBadge,
    resolvedBadge,
    resolveIncident,
    loadMore,
    handleMarkRead,
  }
}
