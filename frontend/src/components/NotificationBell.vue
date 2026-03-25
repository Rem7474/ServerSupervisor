<template>
  <div ref="bellRef" class="position-relative">
    <!-- Bell button -->
    <button
      class="btn btn-ghost-secondary d-flex align-items-center justify-content-center position-relative notification-bell-btn"
      @click.stop="toggleOpen"
      :title="unreadCount > 0 ? `${unreadCount} notification(s) non lue(s)` : 'Notifications'"
      :aria-label="unreadCount > 0 ? `${unreadCount} notification(s) non lue(s)` : 'Notifications'"
      aria-haspopup="true"
      :aria-expanded="isOpen"
    >
      <svg width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
      </svg>
      <span
        v-if="unreadCount > 0"
        class="badge bg-red text-white position-absolute notification-bell-counter"
      >{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
    </button>

    <!-- Dropdown panel -->
    <div
      v-if="isOpen"
      class="notification-dropdown"
    >
      <!-- Header -->
      <div class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom">
        <div class="fw-semibold">
          Notifications
          <span v-if="notifications.length" class="badge bg-secondary-lt text-secondary ms-1">{{ notifications.length }}</span>
        </div>
        <button
          v-if="unreadCount > 0"
          class="btn btn-sm btn-ghost-secondary"
          @click.stop="markAllRead"
        >
          Tout marquer comme lu
        </button>
      </div>

      <!-- List -->
      <div class="notification-list-scroll">
        <!-- Loading -->
        <div v-if="loading" class="text-center text-secondary py-4 small">Chargement…</div>

        <!-- Empty -->
        <div v-else-if="!notifications.length" class="text-center text-secondary py-4 small">
          <svg class="mb-2" width="32" height="32" fill="none" stroke="currentColor" viewBox="0 0 24 24" style="opacity:.4">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
              d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5"/>
          </svg>
          <div>Aucune notification</div>
        </div>

        <!-- Items -->
        <div
          v-for="item in notifications"
          :key="item.id"
          class="d-flex align-items-start px-3 py-2 border-bottom notification-item notification-item-layout"
          :class="isUnread(item) ? 'notification-unread' : ''"
        >
          <!-- Status dot -->
          <div class="flex-shrink-0 mt-1">
            <span
              class="badge notification-status-dot"
              :class="item.resolved_at ? 'bg-secondary-lt text-secondary' : 'bg-red-lt text-red'"
            ></span>
          </div>

          <!-- Content -->
          <div class="flex-fill notification-content">
            <div class="d-flex align-items-center justify-content-between gap-2 mb-1">
              <div class="fw-semibold text-truncate small notification-rule" :title="item.rule_name">
                {{ item.rule_name }}
              </div>
              <span v-if="item.resolved_at" class="badge bg-success-lt text-success flex-shrink-0 notification-state-badge">Résolu</span>
              <span v-else class="badge bg-red-lt text-red flex-shrink-0 notification-state-badge">Actif</span>
            </div>
            <div class="d-flex align-items-center justify-content-between text-secondary notification-meta">
              <router-link
                :to="`/hosts/${item.host_id}`"
                class="text-truncate text-secondary text-decoration-none notification-host-link notification-host"
                @click="isOpen = false"
              >
                <svg class="me-1" width="12" height="12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
                </svg>
                {{ item.host_name }}
              </router-link>
              <span class="flex-shrink-0 ms-2">
                <RelativeTime :date="item.triggered_at" />
              </span>
            </div>
            <div class="text-secondary mt-1 notification-value-row">
              Valeur : <code class="notification-value">{{ item.value?.toFixed(2) }}</code>
              <span class="ms-1">{{ metricUnit(item.metric) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="px-3 py-2 text-center border-top">
        <router-link to="/alerts?tab=incidents" class="text-secondary small" @click="isOpen = false">
          Voir l'historique des incidents →
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import RelativeTime from './RelativeTime.vue'
import apiClient from '../api'
import { useWebSocket } from '../composables/useWebSocket'

const bellRef = ref(null)
const isOpen = ref(false)
const loading = ref(false)
const notifications = ref([])

// readAtRef is now server-driven (cross-device sync).
// Populated from GET /api/v1/notifications (includes read_at field per user).
// Updated via POST /api/v1/notifications/mark-read and on every 30s poll.
const readAtRef = ref(null)

let pollTimer = null
// null = first fetch not done yet → avoid flooding on page load
let seenIdSet = null

const unreadCount = computed(() =>
  notifications.value.filter(n =>
    !readAtRef.value || new Date(n.triggered_at) > new Date(readAtRef.value)
  ).length
)

function isUnread(item) {
  return !readAtRef.value || new Date(item.triggered_at) > new Date(readAtRef.value)
}

function metricUnit(metric) {
  if (!metric) return ''
  if (['cpu', 'memory', 'disk'].includes(metric)) return '%'
  return ''
}

function toggleOpen() {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    fetchNotifications()
  }
}

async function markAllRead() {
  try {
    const { data } = await apiClient.markNotificationsRead()
    readAtRef.value = data.read_at ?? new Date().toISOString()
  } catch {
    // Fallback: mark locally if API is temporarily unavailable
    readAtRef.value = new Date().toISOString()
  }
}

function showBrowserNotification(item) {
  try {
    const n = new Notification(`Alerte : ${item.rule_name}`, {
      body: `${item.host_name} — Valeur : ${item.value?.toFixed(2)}${metricUnit(item.metric)}`,
      icon: '/favicon.ico',
      tag: `alert-${item.id}`,
      requireInteraction: false,
    })
    n.onclick = () => { window.focus(); n.close() }
  } catch {
    // API not supported or permission revoked mid-session
  }
}

// Convert URL-safe base64 to Uint8Array for PushManager.subscribe()
function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const rawData = atob(base64)
  return Uint8Array.from(rawData, (c) => c.charCodeAt(0))
}

// Register a Web Push subscription so the backend can push alerts to this device
// even when the app is closed (required for mobile PWA).
async function setupPushNotifications() {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) return
  if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return
  try {
    const reg = await navigator.serviceWorker.ready
    let sub = await reg.pushManager.getSubscription()
    if (!sub) {
      const { data } = await apiClient.getPushVapidPublicKey()
      if (!data?.public_key) return
      sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(data.public_key),
      })
    }
    // Always sync the current subscription to the server (covers new login / key rotation)
    await apiClient.subscribePush(sub.toJSON())
  } catch (err) {
    // Non-critical: push not supported or user declined — desktop Notification API still works
    console.debug('[Push] subscription setup failed:', err)
  }
}

// WebSocket push — receives new_alert events in real time from the alert engine
useWebSocket('/api/v1/ws/notifications', (payload) => {
  if (payload.type !== 'new_alert' || !payload.notification) return
  const item = payload.notification

  // Show in-app notification immediately (WS push is always new)
  if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
    showBrowserNotification(item)
  }

  // Mark as seen so the polling cycle doesn't re-trigger the browser notification
  if (seenIdSet !== null) seenIdSet.add(item.id)

  // Prepend to the in-app list (max 30)
  if (!notifications.value.some(n => n.id === item.id)) {
    notifications.value = [item, ...notifications.value].slice(0, 30)
  }
})

async function fetchNotifications() {
  if (loading.value) return
  loading.value = true
  try {
    const res = await apiClient.getNotifications()
    const incoming = res.data?.notifications || []

    // Sync server-side readAt (cross-device: if another device marked as read, update here)
    const serverReadAt = res.data?.read_at
    if (serverReadAt !== undefined) {
      readAtRef.value = serverReadAt  // null (never marked) or ISO timestamp
    }

    // Fallback browser notifications via polling — only for IDs not already seen via WS
    if (seenIdSet !== null && typeof Notification !== 'undefined' && Notification.permission === 'granted') {
      for (const item of incoming) {
        if (item.browser_notify && !seenIdSet.has(item.id)) {
          showBrowserNotification(item)
        }
      }
    }

    seenIdSet = new Set(incoming.map(n => n.id))
    notifications.value = incoming
  } catch {
    // Non-critical — silent fail
  } finally {
    loading.value = false
  }
}

function syncNotificationsIfVisible() {
  if (document.visibilityState !== 'visible') return
  fetchNotifications()
}

function onClickOutside(e) {
  if (bellRef.value && !bellRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

onMounted(async () => {
  // Request notification permission on first visit
  if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
    const perm = await Notification.requestPermission()
    if (perm === 'granted') {
      await setupPushNotifications()
    }
  } else if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
    // Already granted (e.g. page reload) — ensure subscription is registered with server
    await setupPushNotifications()
  }
  fetchNotifications()
  pollTimer = setInterval(fetchNotifications, 30_000)
  document.addEventListener('click', onClickOutside)
  document.addEventListener('visibilitychange', syncNotificationsIfVisible)
  window.addEventListener('focus', syncNotificationsIfVisible)
})

onUnmounted(() => {
  clearInterval(pollTimer)
  document.removeEventListener('click', onClickOutside)
  document.removeEventListener('visibilitychange', syncNotificationsIfVisible)
  window.removeEventListener('focus', syncNotificationsIfVisible)
})
</script>

<style scoped>
.notification-dropdown {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: 380px;
  max-width: calc(100vw - 1rem);
  z-index: 2100;
  background: var(--tblr-bg-surface);
  border: 1px solid var(--tblr-border-color);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.15);
}

.notification-bell-btn {
  width: 38px;
  height: 38px;
  padding: 0;
}

.notification-bell-counter {
  top: 2px;
  right: 2px;
  font-size: 0.6rem;
  min-width: 16px;
  height: 16px;
  padding: 0 3px;
  border-radius: 8px;
  line-height: 16px;
}

.notification-list-scroll {
  max-height: 340px;
  overflow-y: auto;
}

.notification-item-layout {
  cursor: default;
  gap: 10px;
}

.notification-status-dot {
  width: 8px;
  height: 8px;
  padding: 0;
  border-radius: 50%;
  display: inline-block;
}

.notification-content {
  min-width: 0;
}

.notification-rule {
  max-width: 220px;
}

.notification-state-badge {
  font-size: 0.65rem;
}

.notification-meta {
  font-size: 0.78rem;
}

.notification-host {
  max-width: 200px;
}

.notification-value {
  font-size: 0.75rem;
}

.notification-value-row {
  font-size: 0.75rem;
}

@media (max-width: 480px) {
  .notification-dropdown {
    position: fixed;
    top: 56px;
    right: 0.5rem;
    left: 0.5rem;
    width: auto;
    max-width: none;
    z-index: 2100;
  }
}

.notification-item:last-child {
  border-bottom: none !important;
}
.notification-unread {
  background: rgba(var(--tblr-azure-rgb), 0.04);
}
.notification-item:hover {
  background: var(--tblr-active-bg, rgba(0,0,0,0.04));
}
.notification-host-link:hover {
  color: var(--tblr-primary) !important;
}
</style>
