import { ref, computed, onMounted, onUnmounted } from 'vue'
import apiClient from '../api'
import { useWebSocket } from './useWebSocket'
import { resolveIncidentHostRoute } from '../utils/incidentRouting'
import type { WSNotificationMessage } from '../types/ws'

export interface NotificationItem {
  id: string | number
  type?: string
  triggered_at?: string
  status?: string
  resolved_at?: string | null
  rule_name?: string
  tracker_id?: string | number
  tracker_type?: string
  tracker_name?: string
  version?: string
  release_name?: string
  host_id?: string
  host_name?: string
  link_host_id?: string
  value_label?: string
  metric?: string
  value?: number
  browser_notify?: boolean
  webhook_id?: string | number
  webhook_name?: string
}


export function useNotifications() {
  const notifications = ref<NotificationItem[]>([])
  const loading = ref(false)
  const readAtRef = ref<string | null>(null)

  let pollTimer: ReturnType<typeof setInterval> | null = null
  let seenIdSet: Set<string | number> | null = null

  const unreadCount = computed(() =>
    notifications.value.filter((n) =>
      !readAtRef.value || new Date(n.triggered_at ?? 0) > new Date(readAtRef.value)
    ).length
  )

  function isUnread(item: NotificationItem): boolean {
    return !readAtRef.value || new Date(item.triggered_at ?? 0) > new Date(readAtRef.value)
  }

  function metricUnit(metric?: string): string {
    if (!metric) return ''
    if (['cpu', 'memory', 'disk'].includes(metric)) return '%'
    return ''
  }

  function trackerStatusLabel(status?: string): string {
    if (status === 'pending' || status === 'running') return 'Détection en cours'
    if (status === 'completed' || status === 'success') return 'Exécution réussie'
    if (status === 'failed' || status === 'error') return 'Exécution échouée'
    return status || 'État inconnu'
  }

  function notificationResolved(item?: NotificationItem): boolean {
    if (item?.type === 'release_tracker_detected' || item?.type === 'release_tracker_execution') {
      return !!item?.resolved_at || ['completed', 'success', 'failed', 'error'].includes((item?.status || '').toLowerCase())
    }
    return !!item?.resolved_at
  }

  function notificationTitle(item?: NotificationItem): string {
    if (!item) return 'Notification'
    if (item.type === 'release_tracker_detected') return item.rule_name || 'Nouvelle release detectee'
    if (item.type === 'release_tracker_execution') return item.rule_name || 'Execution release tracker'
    return item.rule_name || 'Alerte'
  }

  function notificationRoute(item?: NotificationItem): string {
    if (item?.type === 'release_tracker_detected' || item?.type === 'release_tracker_execution') {
      if (item?.tracker_id) return `/release-trackers/${encodeURIComponent(String(item.tracker_id))}`
      return '/git-webhooks?tab=trackers'
    }
    return resolveIncidentHostRoute(item?.host_id, item?.metric, item?.link_host_id)
  }

  async function markAllRead(): Promise<void> {
    try {
      const { data } = await apiClient.markNotificationsRead()
      readAtRef.value = data.read_at ?? new Date().toISOString()
    } catch {
      readAtRef.value = new Date().toISOString()
    }
  }

  function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
    const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
    const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
    const rawData = atob(base64)
    const bytes = new Uint8Array(rawData.length)
    for (let i = 0; i < rawData.length; i += 1) {
      bytes[i] = rawData.charCodeAt(i)
    }
    return bytes
  }

  async function cleanupPushSubscription(): Promise<void> {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) return
    try {
      const reg = await navigator.serviceWorker.ready
      const sub = await reg.pushManager.getSubscription()
      if (sub) {
        await apiClient.unsubscribePush(sub.endpoint).catch(() => {})
        await sub.unsubscribe()
      }
      localStorage.removeItem('ss_vapid_public_key')
    } catch {
      // Non-critical
    }
  }

  async function setupPushNotifications(): Promise<void> {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) return
    if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return
    try {
      const reg = await navigator.serviceWorker.ready
      const { data } = await apiClient.getPushVapidPublicKey()
      if (!data?.public_key) return

      let sub = await reg.pushManager.getSubscription()

      if (sub) {
        const cachedKey = localStorage.getItem('ss_vapid_public_key')
        const expired = sub.expirationTime != null && Date.now() > sub.expirationTime
        const keyRotated = cachedKey != null && cachedKey !== data.public_key
        if (expired || keyRotated) {
          await sub.unsubscribe()
          sub = null
        }
      }

      if (!sub) {
        sub = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(data.public_key),
        })
      }

      localStorage.setItem('ss_vapid_public_key', data.public_key)
      await apiClient.subscribePush(sub.toJSON())
    } catch (err) {
      console.debug('[Push] subscription setup failed:', err)
    }
  }

  function watchPermissionChange(): void {
    if (!navigator.permissions) return
    navigator.permissions.query({ name: 'notifications' as PermissionName }).then((status) => {
      status.onchange = () => {
        if (status.state === 'denied') {
          cleanupPushSubscription()
        } else if (status.state === 'granted') {
          setupPushNotifications()
        }
      }
    }).catch(() => {})
  }

  async function fetchNotifications(): Promise<void> {
    if (loading.value) return
    loading.value = true
    try {
      const res = await apiClient.getNotifications()
      const incoming: NotificationItem[] = res.data?.notifications || []

      const serverReadAt = res.data?.read_at
      if (serverReadAt !== undefined) {
        readAtRef.value = serverReadAt
      }

      seenIdSet = new Set(incoming.map((n) => n.id))
      notifications.value = incoming
    } catch {
      // Non-critical — silent fail
    } finally {
      loading.value = false
    }
  }

  // Native OS notifications are shown exclusively by the service worker's Web Push
  // handler (see setupPushNotifications below) — it fires regardless of whether this
  // tab is open, so calling the Notification API here too would double-pop every
  // event while the app is in the foreground. This WS handler only drives the in-app
  // notification list/bell.
  useWebSocket<WSNotificationMessage>('/api/v1/ws/notifications', (payload) => {
    if (!payload?.type) return

    if (payload.type === 'new_alert') {
      const item = payload.notification
      if (seenIdSet !== null) seenIdSet.add(item.id)
      if (!notifications.value.some((n) => n.id === item.id)) {
        notifications.value = [item, ...notifications.value].slice(0, 30)
      }
      return
    }

    if (
      payload.type === 'release_tracker_detected' ||
      payload.type === 'release_tracker_execution' ||
      payload.type === 'webhook_execution'
    ) {
      fetchNotifications()
    }
  })

  function syncNotificationsIfVisible(): void {
    if (document.visibilityState !== 'visible') return
    fetchNotifications()
  }

  onMounted(async () => {
    if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
      const perm = await Notification.requestPermission()
      if (perm === 'granted') {
        await setupPushNotifications()
      }
    } else if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
      await setupPushNotifications()
    }
    watchPermissionChange()
    fetchNotifications()
    pollTimer = setInterval(fetchNotifications, 30_000)
    window.addEventListener('ss:app-resume', syncNotificationsIfVisible)
  })

  onUnmounted(() => {
    if (pollTimer) clearInterval(pollTimer)
    window.removeEventListener('ss:app-resume', syncNotificationsIfVisible)
  })

  return {
    notifications,
    loading,
    readAtRef,
    unreadCount,
    fetchNotifications,
    markAllRead,
    isUnread,
    metricUnit,
    trackerStatusLabel,
    notificationResolved,
    notificationTitle,
    notificationRoute,
  }
}
