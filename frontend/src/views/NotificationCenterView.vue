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
                <svg
                  class="icon"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    v-if="isTrackerType(item)"
                    d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"
                  />
                  <path
                    v-else
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
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
import { ref, computed, watch, onMounted } from 'vue'
import api from '../api'
import type { NotificationItem } from '../types/generated'
import { addToast } from '../composables/useGlobalToast'
import RelativeTime from '../components/RelativeTime.vue'
import { resolveIncidentHostRoute } from '../utils/incidentRouting'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

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
  } catch (err: any) {
    addToast(err?.response?.data?.error || 'Impossible de résoudre', 'error')
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
    })
    items.value = res.data.notifications || []
    total.value = res.data.total || 0
    if (res.data.read_at !== undefined) readAt.value = res.data.read_at
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || 'Erreur de chargement'
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
</script>

<style scoped>
.notification-unread {
  background: rgba(var(--tblr-azure-rgb), 0.04);
}
</style>
