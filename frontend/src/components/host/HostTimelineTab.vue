<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Timeline"
      :interval-sec="TIMELINE_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="d-flex align-items-center gap-2 mb-3">
      <div class="d-flex gap-2 flex-wrap">
        <button
          v-for="f in TYPE_FILTERS"
          :key="f.value"
          type="button"
          class="btn btn-sm"
          :class="typeFilter === f.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="typeFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <LoadingSkeleton
      v-if="loading && events.length === 0"
      variant="list"
    />

    <EmptyState
      v-else-if="filteredEvents.length === 0"
      title="Aucun événement."
    />

    <div
      v-else
      class="timeline-list"
    >
      <div
        v-for="ev in filteredEvents"
        :key="ev.type + ev.id"
        class="timeline-event d-flex gap-3 mb-3"
      >
        <div class="timeline-icon flex-shrink-0">
          <span
            class="avatar avatar-sm rounded"
            :class="iconBg(ev)"
          >
            <IconClipboard
              v-if="ev.type === 'audit'"
              :size="16"
              class="icon"
            />
            <IconTerminal2
              v-else-if="ev.type === 'command'"
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
        <div class="flex-grow-1 min-w-0">
          <div class="d-flex align-items-start justify-content-between gap-2">
            <div>
              <span class="fw-medium">{{ ev.title }}</span>
              <span
                v-if="ev.module"
                class="badge bg-secondary-lt text-secondary ms-1 small"
              >{{ ev.module }}</span>
            </div>
            <div class="d-flex gap-1 flex-shrink-0 align-items-center">
              <span
                v-if="ev.severity"
                class="badge"
                :class="severityBadge(ev.severity)"
              >{{ ev.severity }}</span>
              <span
                v-if="ev.status"
                class="badge"
                :class="statusBadge(ev.status)"
              >{{ ev.status }}</span>
              <button
                v-if="ev.type === 'command'"
                type="button"
                class="btn btn-icon btn-sm btn-ghost-secondary"
                title="Voir les logs"
                aria-label="Voir les logs"
                @click="emit('watch-command', ev)"
              >
                <IconList
                  :size="16"
                  class="icon icon-sm"
                />
              </button>
            </div>
          </div>
          <div
            v-if="ev.detail"
            class="text-muted small mt-1 text-truncate"
            style="max-width: 600px"
          >
            {{ ev.detail }}
          </div>
          <div class="text-muted small mt-1">
            <RelativeTime :date="ev.timestamp" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { IconClipboard, IconTerminal2, IconAlertTriangle, IconList } from '@tabler/icons-vue'
import api from '../../api'
import type { HostTimelineEvent } from '../../types/audit'
import RelativeTime from '../RelativeTime.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import { getApiErrorMessage } from '../../api/client'

const props = defineProps<{ hostId: string }>()

const emit = defineEmits<{
  (e: 'watch-command', ev: HostTimelineEvent): void
}>()

const TYPE_FILTERS = [
  { value: '', label: 'Tout' },
  { value: 'audit', label: 'Audit' },
  { value: 'command', label: 'Commandes' },
  { value: 'incident', label: 'Incidents' },
]

const TIMELINE_REFRESH_SEC = 60

const events = ref<HostTimelineEvent[]>([])
const loading = ref(false)
const error = ref('')
const typeFilter = ref('')
const autoRefresh = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null

const filteredEvents = computed(() =>
  typeFilter.value ? events.value.filter((e) => e.type === typeFilter.value) : events.value
)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getHostTimeline(props.hostId, 100)
    events.value = res.data.events || []
    lastUpdatedAt.value = new Date()
  } catch (err: unknown) {
    error.value = getApiErrorMessage(err, 'Erreur de chargement')
  } finally {
    loading.value = false
  }
}

function startRefreshTimer(): void {
  stopRefreshTimer()
  refreshTimer = setInterval(() => {
    if (autoRefresh.value) load()
  }, TIMELINE_REFRESH_SEC * 1000)
}

function stopRefreshTimer(): void {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}

function iconBg(ev: HostTimelineEvent): string {
  if (ev.type === 'incident') {
    return ev.severity === 'crit' ? 'bg-danger text-white' : 'bg-warning text-white'
  }
  if (ev.type === 'command') return 'bg-primary text-white'
  return 'bg-secondary text-white'
}

function severityBadge(severity: string): string {
  return severity === 'crit' ? 'bg-danger-lt text-danger' : 'bg-warning-lt text-warning'
}

function statusBadge(status: string): string {
  const map: Record<string, string> = {
    completed: 'bg-success-lt text-success',
    failed: 'bg-danger-lt text-danger',
    running: 'bg-primary-lt text-primary',
    pending: 'bg-secondary-lt text-secondary',
    cancelled: 'bg-secondary-lt text-secondary',
    active: 'bg-danger-lt text-danger',
    resolved: 'bg-success-lt text-success',
  }
  return map[status] || 'bg-secondary-lt text-secondary'
}

onMounted(() => {
  load()
  startRefreshTimer()
})
onUnmounted(stopRefreshTimer)

// Lets a caller (the "Commandes récentes" KPI card, since this tab absorbed
// the standalone Commandes tab) jump straight to the command-type filter.
function filterCommands(): void {
  typeFilter.value = 'command'
}

defineExpose({ filterCommands })
</script>

<style scoped>
.timeline-event:not(:last-child) {
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--tblr-border-color);
}
</style>
