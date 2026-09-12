<template>
  <div>
    <div
      v-if="loading"
      class="card"
    >
      <div class="card-body">
        <LoadingSkeleton variant="list" />
      </div>
    </div>
    <div
      v-else-if="error"
      class="card"
    >
      <div class="card-body text-center py-5 text-danger">
        {{ error }}
      </div>
    </div>
    <div
      v-else-if="critIncidents.length === 0 && warnIncidents.length === 0"
      class="card"
    >
      <div class="card-body">
        <EmptyState
          :icon="IconShieldCheck"
          :title="t('alerts.warRoomNoActiveIncidentsTitle')"
          :subtitle="t('alerts.warRoomNoActiveIncidentsSubtitle')"
        />
      </div>
    </div>
    <div
      v-else
      class="row g-3"
    >
      <div class="col-12 col-xl-6">
        <WarRoomSeverityColumn
          :title="t('alerts.warRoomCriticalColumn')"
          tone="danger"
          :items="critIncidents"
          :correlated-counts="correlatedCounts"
          :is-admin="isAdmin"
          :resolving-id="resolvingId"
          :acknowledging-id="acknowledgingId"
          @resolve="$emit('resolve', $event)"
          @acknowledge="$emit('acknowledge', $event)"
        />
      </div>
      <div class="col-12 col-xl-6">
        <WarRoomSeverityColumn
          :title="t('alerts.warRoomWarningColumn')"
          tone="warning"
          :items="warnIncidents"
          :correlated-counts="correlatedCounts"
          :is-admin="isAdmin"
          :resolving-id="resolvingId"
          :acknowledging-id="acknowledgingId"
          @resolve="$emit('resolve', $event)"
          @acknowledge="$emit('acknowledge', $event)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconShieldCheck } from '@tabler/icons-vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import WarRoomSeverityColumn from './WarRoomSeverityColumn.vue'
import { isTrackerType, notificationResolved } from '../../utils/incidentFormat'
import type { NotificationItem } from '../../types/generated'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  incidents?: NotificationItem[]
  loading?: boolean
  error?: string
  isAdmin?: boolean
  resolvingId?: string | number | null
  acknowledgingId?: string | number | null
}>(), {
  incidents: () => [],
  loading: false,
  error: '',
  isAdmin: false,
  resolvingId: null,
  acknowledgingId: null,
})

defineEmits<{
  (e: 'resolve', item: NotificationItem): void
  (e: 'acknowledge', item: NotificationItem): void
}>()

// Active alert incidents only — release-tracker entries and anything already
// resolved don't belong on a "what's on fire right now" board.
const activeAlertIncidents = computed(() =>
  props.incidents.filter((item) => !isTrackerType(item) && !notificationResolved(item))
)

// A correlated child (host-down cascade, see server's alert_incidents.correlated_with)
// doesn't get its own row — it's not independently actionable, its parent
// host-down incident is. It's still counted so the parent shows a "+N
// correlated" badge instead of silently hiding the cascade.
const correlatedCounts = computed(() => {
  const counts = new Map<number, number>()
  for (const item of activeAlertIncidents.value) {
    if (item.correlated_with != null) {
      counts.set(item.correlated_with, (counts.get(item.correlated_with) || 0) + 1)
    }
  }
  return counts
})

const rootIncidents = computed(() => activeAlertIncidents.value.filter((item) => item.correlated_with == null))

// Oldest-first within each severity: an incident nobody has looked at for
// the longest deserves attention before one that just fired.
function bySeverity(severity: string): NotificationItem[] {
  return rootIncidents.value
    .filter((item) => (item.severity || '').toLowerCase() === severity)
    .slice()
    .sort((a, b) => new Date(a.triggered_at || 0).getTime() - new Date(b.triggered_at || 0).getTime())
}

const critIncidents = computed(() => bySeverity('crit'))
const warnIncidents = computed(() => bySeverity('warn'))
</script>
