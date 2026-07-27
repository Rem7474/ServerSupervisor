import { ref, computed, watch, onMounted } from 'vue'
import api from '../api'
import type { NotificationItem } from '../types/generated'
import { addToast } from './useGlobalToast'
import {
  isUnread as sharedIsUnread,
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

  // ── Grouping by host ──────────────────────────────────────────────────────────
  // Default view: a flat chronological list makes it hard to see "is this host
  // having a bad day" at a glance once there are more than a handful of items.
  // Grouped-by-host stays opt-out (not opt-in) since it's strictly a display
  // reorganization of the same `items` — nothing is hidden by default, groups
  // just start expanded.
  const groupByHost = ref(true)
  const collapsedHosts = ref(new Set<string>())

  function toggleGroupByHost(): void {
    groupByHost.value = !groupByHost.value
  }

  function isHostCollapsed(key: string): boolean {
    return collapsedHosts.value.has(key)
  }

  function toggleHostGroup(key: string): void {
    const next = new Set(collapsedHosts.value)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    collapsedHosts.value = next
  }

  interface NotificationGroup {
    key: string
    hostName: string
    items: NotificationItem[]
    unreadCount: number
  }

  const groupedItems = computed<NotificationGroup[]>(() => {
    const order: string[] = []
    const map = new Map<string, NotificationItem[]>()
    for (const item of items.value) {
      const key = item.host_name || '__sans_hote__'
      if (!map.has(key)) {
        map.set(key, [])
        order.push(key)
      }
      map.get(key)!.push(item)
    }
    // `items` already arrives newest-first from the API, so `order` (first
    // occurrence per host) already reflects each group's most recent item —
    // no separate re-sort needed.
    return order.map((key) => {
      const list = map.get(key)!
      return {
        key,
        hostName: key === '__sans_hote__' ? 'Sans hôte' : key,
        items: list,
        unreadCount: list.filter((n) => sharedIsUnread(n, readAt.value)).length,
      }
    })
  })

  function isUnread(item: NotificationItem): boolean {
    return sharedIsUnread(item, readAt.value)
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
    groupByHost,
    toggleGroupByHost,
    groupedItems,
    isHostCollapsed,
    toggleHostGroup,
    isUnread,
    resolveIncident,
    loadMore,
    handleMarkRead,
  }
}
