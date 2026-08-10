import { computed, ComputedRef, Ref, ref } from 'vue'
import apiClient from '../api'
import { addToast } from './useGlobalToast'
import { getApiErrorMessage } from '../api/client'
import { resolvableIncidentId } from '../utils/incidentFormat'
import type { NotificationItem } from '../types/generated'
import type { WSNotificationMessage } from '../types/ws'

// Server caps `limit` at 200 (server/internal/handlers/notifications.go's
// clampQueryInt) — this is the single unfiltered fetch every filter/search/
// sort/group-by-host view in AlertIncidentList.vue then slices client-side.
// The previous incidents-slice of useAlertsPage.ts called getNotifications()
// with no params at all, silently defaulting to the server's limit=30 — that
// left AlertIncidentList's PaginationNav almost never rendering more than one
// page. Fetching the server's actual max here fixes that.
const FETCH_LIMIT = 200

interface UseNotificationHistoryApi {
  incidents: Ref<NotificationItem[]>
  loading: Ref<boolean>
  error: Ref<string>
  loaded: Ref<boolean>
  activeIncidentCount: ComputedRef<number>
  loadIncidents: () => Promise<void>
  markingRead: Ref<boolean>
  markAllRead: () => Promise<void>
  resolvingId: Ref<string | number | null>
  resolveIncident: (item: NotificationItem) => Promise<void>
  acknowledgingId: Ref<string | number | null>
  acknowledgeIncident: (item: NotificationItem) => Promise<void>
  onWebSocketAlert: (payload: WSNotificationMessage) => void
}

// Single state owner for the merged "Historique de notifications" page
// (AlertsView.vue's incidents tab), replacing both the retired
// useNotificationCenter.ts (the standalone /notifications page) and the
// incidents slice that used to live in useAlertsPage.ts. Deliberately thin:
// fetch + mutate + WS-refresh only — filtering/search/sort/pagination/
// grouping stay local to AlertIncidentList.vue, which already owns them.
export function useNotificationHistory(): UseNotificationHistoryApi {
  const incidents: Ref<NotificationItem[]> = ref([])
  const loading = ref(false)
  const error = ref('')
  const loaded = ref(false)
  const markingRead = ref(false)
  const resolvingId = ref<string | number | null>(null)
  const acknowledgingId = ref<string | number | null>(null)

  const activeIncidentCount = computed(
    () => incidents.value.filter((item) => (item.type === 'alert_incident' || !item.type) && !item.resolved_at).length
  )

  async function loadIncidents(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const response = await apiClient.getNotifications({ limit: FETCH_LIMIT })
      incidents.value = response.data?.notifications || []
      loaded.value = true
    } catch {
      error.value = "Impossible de charger l'historique des notifications"
    } finally {
      loading.value = false
    }
  }

  async function markAllRead(): Promise<void> {
    markingRead.value = true
    try {
      await apiClient.markNotificationsRead()
      addToast('Toutes les notifications marquées comme lues', 'success')
    } catch {
      addToast('Erreur lors du marquage', 'error')
    } finally {
      markingRead.value = false
    }
  }

  async function resolveIncident(item: NotificationItem): Promise<void> {
    const id = resolvableIncidentId(item)
    if (!id || resolvingId.value) return
    resolvingId.value = item.id
    try {
      await apiClient.resolveAlertIncident(id)
      incidents.value = incidents.value.map((n) =>
        n.id === item.id ? { ...n, resolved_at: new Date().toISOString() } : n
      )
      addToast('Incident résolu', 'success')
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, 'Impossible de résoudre'), 'error')
    } finally {
      resolvingId.value = null
    }
  }

  async function acknowledgeIncident(item: NotificationItem): Promise<void> {
    const id = resolvableIncidentId(item)
    if (!id || acknowledgingId.value) return
    acknowledgingId.value = item.id
    try {
      await apiClient.acknowledgeAlertIncident(id)
      incidents.value = incidents.value.map((n) =>
        n.id === item.id ? { ...n, acknowledged_at: new Date().toISOString() } : n
      )
      addToast('Incident pris en charge', 'success')
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, "Impossible d'accuser réception"), 'error')
    } finally {
      acknowledgingId.value = null
    }
  }

  function onWebSocketAlert(payload: WSNotificationMessage): void {
    if (payload.type === 'alert_incident_update') {
      loadIncidents()
      return
    }
    if (payload.type === 'release_tracker_detected' || payload.type === 'release_tracker_execution') {
      loadIncidents()
      return
    }

    if (payload.type !== 'new_alert' || !payload.notification) return

    const incoming = payload.notification
    const idx = incidents.value.findIndex((item) => item.id === incoming.id)

    if (idx >= 0) {
      incidents.value = [
        { ...incidents.value[idx], ...incoming },
        ...incidents.value.slice(0, idx),
        ...incidents.value.slice(idx + 1),
      ]
    } else {
      incidents.value = [incoming, ...incidents.value]
    }

    loaded.value = true
    loadIncidents()
  }

  return {
    incidents,
    loading,
    error,
    loaded,
    activeIncidentCount,
    loadIncidents,
    markingRead,
    markAllRead,
    resolvingId,
    resolveIncident,
    acknowledgingId,
    acknowledgeIncident,
    onWebSocketAlert,
  }
}
