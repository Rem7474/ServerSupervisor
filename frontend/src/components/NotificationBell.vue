<template>
  <div
    ref="bellRef"
    class="dropdown"
  >
    <!-- Bell button -->
    <button
      type="button"
      class="btn btn-ghost-secondary d-flex align-items-center justify-content-center position-relative notification-bell-btn"
      :title="unreadCount > 0 ? t('alerts.notificationBellUnreadTitle', { count: unreadCount }, unreadCount) : t('alerts.notificationBellTitle')"
      :aria-label="unreadCount > 0 ? t('alerts.notificationBellUnreadTitle', { count: unreadCount }, unreadCount) : t('alerts.notificationBellTitle')"
      aria-haspopup="true"
      :aria-expanded="isOpen"
      @click.stop="toggleOpen"
    >
      <IconBell :size="16" />
      <span
        v-if="unreadCount > 0"
        class="badge bg-danger text-white position-absolute notification-bell-counter"
      >{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
    </button>

    <!-- Dropdown panel -->
    <div
      v-if="isOpen"
      class="dropdown-menu dropdown-menu-end show notification-dropdown"
    >
      <!-- Header -->
      <div class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom">
        <div class="fw-semibold">
          {{ t('alerts.notificationBellTitle') }}
          <span
            v-if="notifications.length"
            class="badge bg-secondary-lt text-secondary ms-1"
          >{{ notifications.length }}</span>
        </div>
        <button
          v-if="unreadCount > 0"
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          @click.stop="markAllRead"
        >
          {{ t('alerts.notificationBellMarkAllRead') }}
        </button>
      </div>

      <!-- List -->
      <div class="notification-list-scroll">
        <!-- Loading -->
        <LoadingSkeleton
          v-if="loading"
          variant="list"
        />

        <!-- Empty -->
        <EmptyState
          v-else-if="!notifications.length"
          :icon="IconBell"
          :title="t('alerts.notificationBellEmptyTitle')"
        />

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
              :class="notificationIconTone(item)"
            />
          </div>

          <!-- Content -->
          <div class="flex-fill notification-content">
            <div class="d-flex align-items-center justify-content-between gap-2 mb-1">
              <div
                class="fw-semibold text-truncate small notification-rule"
                :title="item.rule_name"
              >
                {{ notificationTitle(item) }}
              </div>
              <BadgePill
                v-if="notificationResolved(item)"
                :tone="notificationStateTone(item)"
                :text="notificationStateLabel(item)"
                compact
                class="flex-shrink-0"
              />
              <div
                v-else
                class="d-flex align-items-center gap-1 flex-shrink-0"
              >
                <BadgePill
                  :tone="notificationStateTone(item)"
                  :text="notificationStateLabel(item)"
                  compact
                />
                <button
                  v-if="auth.isAdmin && item.type === 'alert_incident'"
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-success py-0 px-1 notification-resolve-btn"
                  :title="t('alerts.notificationBellResolveAction')"
                  :disabled="resolvingId === item.id"
                  @click.stop="resolveIncident(item)"
                >
                  <span
                    v-if="resolvingId === item.id"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconCheck
                    v-else
                    :size="14"
                  />
                </button>
              </div>
            </div>
            <div class="d-flex align-items-center justify-content-between text-secondary notification-meta">
              <router-link
                :to="notificationRoute(item)"
                class="text-truncate text-secondary text-decoration-none notification-host-link notification-host"
                @click="isOpen = false"
              >
                <IconServer
                  :size="14"
                  class="me-1"
                />
                {{ item.host_name }}
              </router-link>
              <span class="flex-shrink-0 ms-2">
                <RelativeTime :date="item.triggered_at || ''" />
              </span>
            </div>
            <div
              v-if="item.type === 'release_tracker_detected' || item.type === 'release_tracker_execution'"
              class="text-secondary mt-1 notification-value-row"
            >
              {{ t('alerts.versionPrefixLabel') }} <code class="notification-value">{{ item.version || '-' }}</code>
              <span class="ms-1">{{ trackerStatusLabel(item.status) }}</span>
            </div>
            <div
              v-else
              class="text-secondary mt-1 notification-value-row"
            >
              {{ t('alerts.notificationBellValuePrefixLabel') }} <code class="notification-value">{{ item.value?.toFixed(2) }}</code>
              <span class="ms-1">{{ metricUnit(item.metric) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="px-3 py-2 text-center border-top">
        <router-link
          to="/alerts?tab=incidents"
          class="text-secondary small"
          @click="isOpen = false"
        >
          {{ t('alerts.notificationBellViewAllLink') }}
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconBell, IconCheck, IconServer } from '@tabler/icons-vue'
import BadgePill from './common/BadgePill.vue'
import EmptyState from './EmptyState.vue'
import LoadingSkeleton from './LoadingSkeleton.vue'
import RelativeTime from './RelativeTime.vue'
import { useAuthStore } from '../stores/auth'
import { useNotifications } from '../composables/useNotifications'
import {
  notificationIconTone,
  notificationStateLabel,
  notificationStateTone,
} from '../utils/notificationBadges'

const { t } = useI18n()
const auth = useAuthStore()
const bellRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)

const {
  notifications,
  loading,
  unreadCount,
  resolvingId,
  fetchNotifications,
  markAllRead,
  resolveIncident,
  isUnread,
  metricUnit,
  trackerStatusLabel,
  notificationResolved,
  notificationTitle,
  notificationRoute,
} = useNotifications()

function toggleOpen(): void {
  isOpen.value = !isOpen.value
  if (isOpen.value) fetchNotifications()
}

function onClickOutside(e: MouseEvent): void {
  if (bellRef.value && !bellRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', onClickOutside)
})
</script>

<style scoped>
/*
 * .dropdown-menu already supplies background/border/border-radius/box-shadow/
 * z-index via Tabler's tokens. Its top/right auto-positioning is gated behind
 * a [data-bs-popper] attribute that only Bootstrap's JS sets — this app never
 * loads that JS (see useModalChrome.ts's own hand-rolled positioning for the
 * same reason), so top/right/width still need to be set explicitly here.
 */
.notification-dropdown {
  top: calc(100% + 8px);
  right: 0;
  width: 380px;
  max-width: calc(100vw - 1rem);
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
