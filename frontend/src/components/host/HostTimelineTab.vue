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
                v-if="ev.type === 'audit'"
                d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
              />
              <path
                v-else-if="ev.type === 'command'"
                d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
              />
              <path
                v-else
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
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
