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

    <div class="card">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Type</th>
              <th>Événement</th>
              <th>Détail</th>
              <th>Statut</th>
              <th>Utilisateur</th>
              <th class="text-end" />
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && events.length === 0">
              <td
                colspan="7"
                class="py-2"
              >
                <LoadingSkeleton
                  variant="table"
                  :lines="5"
                />
              </td>
            </tr>
            <tr v-else-if="filteredEvents.length === 0">
              <td colspan="7">
                <EmptyState title="Aucun événement." />
              </td>
            </tr>
            <tr
              v-for="ev in filteredEvents"
              :key="ev.type + ev.id"
            >
              <td class="text-secondary small">
                <RelativeTime :date="ev.timestamp" />
              </td>
              <td>
                <span
                  class="avatar avatar-xs rounded me-1"
                  :class="iconBg(ev)"
                >
                  <IconClipboard
                    v-if="ev.type === 'audit'"
                    :size="12"
                    class="icon"
                  />
                  <IconTerminal2
                    v-else-if="ev.type === 'command'"
                    :size="12"
                    class="icon"
                  />
                  <IconAlertTriangle
                    v-else
                    :size="12"
                    class="icon"
                  />
                </span>
                <span
                  v-if="ev.type === 'command' && ev.module"
                  :class="moduleClass(ev.module)"
                >{{ moduleLabel(ev.module) }}</span>
                <span
                  v-else
                  class="badge bg-secondary-lt text-secondary"
                >{{ TYPE_LABELS[ev.type] || ev.type }}</span>
              </td>
              <td class="fw-medium">
                {{ ev.title }}
              </td>
              <td
                class="text-secondary small text-truncate"
                style="max-width: 320px"
              >
                {{ ev.detail || '—' }}
              </td>
              <td>
                <span
                  v-if="ev.severity"
                  class="badge me-1"
                  :class="severityBadge(ev.severity)"
                >{{ ev.severity }}</span>
                <span
                  v-if="ev.status"
                  class="badge"
                  :class="statusBadge(ev.status)"
                >{{ ev.status }}</span>
                <span v-if="!ev.severity && !ev.status">—</span>
              </td>
              <td class="text-secondary small">
                {{ ev.user || '—' }}
              </td>
              <td class="text-end">
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
              </td>
            </tr>
          </tbody>
        </table>
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
import { moduleClass, moduleLabel } from '../../utils/moduleMeta'

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

const TYPE_LABELS: Record<string, string> = {
  audit: 'Audit',
  command: 'Commande',
  incident: 'Incident',
}

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
