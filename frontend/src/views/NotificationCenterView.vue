<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Notifications</span>
      </div>
      <div class="d-flex align-items-center justify-content-between">
        <h2 class="page-title mb-0">
          Centre de notifications
          <span
            v-if="unreadCount > 0"
            class="badge bg-red text-white ms-2"
          >{{ unreadCount }}</span>
        </h2>
        <button
          v-if="unreadCount > 0"
          type="button"
          class="btn btn-sm btn-outline-secondary"
          :disabled="markingRead"
          @click="handleMarkRead"
        >
          <span
            v-if="markingRead"
            class="spinner-border spinner-border-sm me-1"
          />
          Tout marquer comme lu
        </button>
      </div>
    </div>

    <!-- Filters -->
    <div class="d-flex flex-wrap gap-2 mb-3">
      <div class="d-flex gap-1">
        <button
          v-for="f in SEVERITY_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="severityFilter === f.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="severityFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <div class="d-flex gap-1">
        <button
          v-for="f in TYPE_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="typeFilter === f.value ? 'btn-secondary' : 'btn-outline-secondary'"
          @click="typeFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <div class="d-flex gap-1 ms-auto">
        <button
          v-for="f in STATUS_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="statusFilter === f.value ? 'btn-secondary' : 'btn-outline-secondary'"
          @click="statusFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div class="card">
      <div
        v-if="loading && items.length === 0"
        class="card-body text-center text-muted py-5"
      >
        <div class="spinner-border mb-2" />
        <div>Chargement…</div>
      </div>

      <div
        v-else-if="items.length === 0"
        class="card-body text-center text-muted py-5"
      >
        Aucune notification.
      </div>

      <div
        v-else
        class="list-group list-group-flush"
      >
        <div
          v-for="item in items"
          :key="item.id"
          class="list-group-item list-group-item-action px-3 py-3"
          :class="{ 'notification-unread': isUnread(item) }"
        >
          <div class="d-flex gap-3 align-items-start">
            <!-- Icon -->
            <div class="flex-shrink-0">
              <span
                class="avatar avatar-sm rounded"
                :class="iconBg(item)"
              >
                <IconCode
                  v-if="isTrackerType(item)"
                  :size="16"
                  class="icon"
                />
                <IconAlertTriangle
                  v-else
                  :size="16"
                  class="icon"
                />
              </span>
            </div>

            <!-- Content -->
            <div class="flex-grow-1 min-w-0">
              <div class="d-flex align-items-start justify-content-between gap-2">
                <div class="d-flex align-items-center gap-2 flex-wrap">
                  <span class="fw-medium">{{ notificationTitle(item) }}</span>
                  <span
                    v-if="item.severity"
                    class="badge"
                    :class="severityBadge(item.severity)"
                  >{{ item.severity }}</span>
                  <span
                    class="badge"
                    :class="resolvedBadge(item)"
                  >{{ notificationResolved(item) ? 'Résolu' : 'Actif' }}</span>
                </div>
                <div class="d-flex align-items-center gap-2 flex-shrink-0">
                  <button
                    v-if="auth.isAdmin && item.type === 'alert_incident' && !notificationResolved(item)"
                    type="button"
                    class="btn btn-sm btn-outline-success py-0 px-2"
                    :disabled="resolvingId === item.id"
                    @click.stop="resolveIncident(item)"
                  >
                    <span
                      v-if="resolvingId === item.id"
                      class="spinner-border spinner-border-sm me-1"
                    />
                    Résoudre
                  </button>
                  <span class="text-muted small">
                    <RelativeTime :date="item.triggered_at || ''" />
                  </span>
                </div>
              </div>

              <div class="text-muted small mt-1">
                <router-link
                  v-if="item.host_name"
                  :to="notificationRoute(item)"
                  class="text-secondary text-decoration-none"
                >
                  {{ item.host_name }}
                </router-link>
                <span v-else>—</span>
                <template v-if="isTrackerType(item) && item.version">
                  &nbsp;— version <code>{{ item.version }}</code>
                </template>
                <template v-else-if="item.value !== undefined">
                  &nbsp;— valeur : <code>{{ item.value?.toFixed(2) }}{{ metricUnit(item.metric) }}</code>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="total > items.length"
        class="card-footer text-center text-muted small"
      >
        Affichage de {{ items.length }} / {{ total }} notifications.
        <button
          type="button"
          class="btn btn-sm btn-link p-0 ms-1"
          @click="loadMore"
        >
          Charger plus
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconCode, IconAlertTriangle } from '@tabler/icons-vue'
import RelativeTime from '../components/RelativeTime.vue'
import { useAuthStore } from '../stores/auth'
import { useNotificationCenter } from '../composables/useNotificationCenter'

const auth = useAuthStore()

const {
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
} = useNotificationCenter()
</script>

<style scoped>
.notification-unread {
  background: rgba(var(--tblr-azure-rgb), 0.04);
}
</style>
