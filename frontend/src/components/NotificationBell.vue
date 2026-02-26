<template>
  <div ref="bellRef" class="position-relative">
    <!-- Bell button -->
    <button
      class="btn btn-ghost-secondary d-flex align-items-center justify-content-center position-relative"
      style="width: 38px; height: 38px; padding: 0;"
      @click.stop="toggleOpen"
      :title="unreadCount > 0 ? `${unreadCount} notification(s) non lue(s)` : 'Notifications'"
    >
      <svg width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
      </svg>
      <span
        v-if="unreadCount > 0"
        class="badge bg-red text-white position-absolute"
        style="top: 2px; right: 2px; font-size: 0.6rem; min-width: 16px; height: 16px; padding: 0 3px; border-radius: 8px; line-height: 16px;"
      >{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
    </button>

    <!-- Dropdown panel -->
    <div
      v-if="isOpen"
      class="notification-dropdown"
      style="position: absolute; top: calc(100% + 8px); right: 0; width: 380px; z-index: 1050; background: var(--tblr-bg-surface); border: 1px solid var(--tblr-border-color); border-radius: 8px; box-shadow: 0 8px 24px rgba(0,0,0,0.15);"
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
      <div style="max-height: 340px; overflow-y: auto;">
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
          class="d-flex align-items-start px-3 py-2 border-bottom notification-item"
          :class="isUnread(item) ? 'notification-unread' : ''"
          style="cursor: default; gap: 10px;"
        >
          <!-- Status dot -->
          <div class="flex-shrink-0 mt-1">
            <span
              class="badge"
              :class="item.resolved_at ? 'bg-secondary-lt text-secondary' : 'bg-red-lt text-red'"
              style="width: 8px; height: 8px; padding: 0; border-radius: 50%; display: inline-block;"
            ></span>
          </div>

          <!-- Content -->
          <div class="flex-fill" style="min-width: 0;">
            <div class="d-flex align-items-center justify-content-between gap-2 mb-1">
              <div class="fw-semibold text-truncate small" style="max-width: 220px;" :title="item.rule_name">
                {{ item.rule_name }}
              </div>
              <span v-if="item.resolved_at" class="badge bg-success-lt text-success flex-shrink-0" style="font-size: 0.65rem;">Résolu</span>
              <span v-else class="badge bg-red-lt text-red flex-shrink-0" style="font-size: 0.65rem;">Actif</span>
            </div>
            <div class="d-flex align-items-center justify-content-between text-secondary" style="font-size: 0.78rem;">
              <span class="text-truncate" style="max-width: 200px;">
                <svg class="me-1" width="12" height="12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
                </svg>
                {{ item.host_name }}
              </span>
              <span class="flex-shrink-0 ms-2">
                <RelativeTime :date="item.triggered_at" />
              </span>
            </div>
            <div class="text-secondary mt-1" style="font-size: 0.75rem;">
              Valeur : <code style="font-size: 0.75rem;">{{ item.value?.toFixed(2) }}</code>
              <span class="ms-1">{{ metricUnit(item.metric) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="px-3 py-2 text-center border-top">
        <router-link to="/alerts" class="text-secondary small" @click="isOpen = false">
          Voir toutes les alertes →
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import RelativeTime from './RelativeTime.vue'
import apiClient from '../api'

const STORAGE_KEY = 'notificationsReadAt'

const bellRef = ref(null)
const isOpen = ref(false)
const loading = ref(false)
const notifications = ref([])
const readAtRef = ref(localStorage.getItem(STORAGE_KEY))
let pollTimer = null

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
  if (['cpu', 'cpu_percent', 'memory', 'ram_percent', 'disk', 'disk_percent'].includes(metric)) return '%'
  return ''
}

function toggleOpen() {
  isOpen.value = !isOpen.value
}

function markAllRead() {
  const now = new Date().toISOString()
  localStorage.setItem(STORAGE_KEY, now)
  readAtRef.value = now
}

async function fetchNotifications() {
  if (loading.value) return
  loading.value = true
  try {
    const res = await apiClient.getNotifications()
    notifications.value = res.data?.notifications || []
  } catch {
    // non-critical — silent fail
  } finally {
    loading.value = false
  }
}

function onClickOutside(e) {
  if (bellRef.value && !bellRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

onMounted(() => {
  fetchNotifications()
  pollTimer = setInterval(fetchNotifications, 30_000)
  document.addEventListener('click', onClickOutside)
})

onUnmounted(() => {
  clearInterval(pollTimer)
  document.removeEventListener('click', onClickOutside)
})
</script>

<style scoped>
.notification-item:last-child {
  border-bottom: none !important;
}
.notification-unread {
  background: rgba(var(--tblr-azure-rgb), 0.04);
}
.notification-item:hover {
  background: var(--tblr-active-bg, rgba(0,0,0,0.04));
}
</style>
