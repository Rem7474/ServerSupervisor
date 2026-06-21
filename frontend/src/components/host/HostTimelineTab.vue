<template>
  <div>
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
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary ms-auto"
        :disabled="loading"
        @click="load"
      >
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm me-1"
        />
        Actualiser
      </button>
    </div>

    <div
      v-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <div
      v-if="loading && events.length === 0"
      class="text-center text-muted py-4"
    >
      <div class="spinner-border mb-2" />
      <div>Chargement de la timeline…</div>
    </div>

    <div
      v-else-if="filteredEvents.length === 0"
      class="text-center text-muted py-4"
    >
      Aucun événement.
    </div>

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
            <div class="d-flex gap-1 flex-shrink-0">
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
import { ref, computed, onMounted } from 'vue'
import { IconClipboard, IconTerminal2, IconAlertTriangle } from '@tabler/icons-vue'
import api from '../../api'
import type { HostTimelineEvent } from '../../types/audit'
import RelativeTime from '../RelativeTime.vue'
import { getApiErrorMessage } from '../../api/client'

const props = defineProps<{ hostId: string }>()

const TYPE_FILTERS = [
  { value: '', label: 'Tout' },
  { value: 'audit', label: 'Audit' },
  { value: 'command', label: 'Commandes' },
  { value: 'incident', label: 'Incidents' },
]

const events = ref<HostTimelineEvent[]>([])
const loading = ref(false)
const error = ref('')
const typeFilter = ref('')

const filteredEvents = computed(() =>
  typeFilter.value ? events.value.filter((e) => e.type === typeFilter.value) : events.value
)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getHostTimeline(props.hostId, 100)
    events.value = res.data.events || []
  } catch (err: unknown) {
    error.value = getApiErrorMessage(err, 'Erreur de chargement')
  } finally {
    loading.value = false
  }
}

function iconBg(ev: HostTimelineEvent): string {
  if (ev.type === 'incident') {
    return ev.severity === 'crit' ? 'bg-red text-white' : 'bg-yellow text-white'
  }
  if (ev.type === 'command') return 'bg-blue text-white'
  return 'bg-secondary text-white'
}

function severityBadge(severity: string): string {
  return severity === 'crit' ? 'bg-red-lt text-red' : 'bg-yellow-lt text-yellow'
}

function statusBadge(status: string): string {
  const map: Record<string, string> = {
    completed: 'bg-green-lt text-green',
    failed: 'bg-red-lt text-red',
    running: 'bg-blue-lt text-blue',
    pending: 'bg-secondary-lt text-secondary',
    cancelled: 'bg-orange-lt text-orange',
    active: 'bg-red-lt text-red',
    resolved: 'bg-green-lt text-green',
  }
  return map[status] || 'bg-secondary-lt text-secondary'
}

onMounted(load)
</script>

<style scoped>
.timeline-event:not(:last-child) {
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--tblr-border-color);
}
</style>
