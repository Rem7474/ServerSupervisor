import { ref, computed, watch, onMounted } from 'vue'
import api from '../api'
import type { NotificationItem } from '../types/generated'
import { addToast } from './useGlobalToast'
import {
  isTrackerType,
  isUnread as sharedIsUnread,
  metricUnit,
  notificationResolved,
  notificationRoute,
  notificationTitle,
  resolvableIncidentId,
} from '../utils/incidentFormat'
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
    items.value.filter((n) => sharedIsUnread(n, readAt.value)).length
  )

  function isUnread(item: NotificationItem): boolean {
    return sharedIsUnread(item, readAt.value)
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

  async function resolveIncident(item: NotificationItem): Promise<void> {
    const id = resolvableIncidentId(item)
    if (!id) return
    resolvingId.value = item.id
    try {
      await api.resolveAlertIncident(id)
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
