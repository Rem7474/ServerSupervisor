<template>
  <div
    class="list-group-item list-group-item-action px-3 py-3"
    :class="{ 'notification-unread': unread }"
  >
    <div class="d-flex gap-3 align-items-start">
      <!-- Icon -->
      <div class="flex-shrink-0">
        <span
          class="avatar avatar-sm rounded"
          :class="iconBg"
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
              :class="severityBadge"
            >{{ item.severity }}</span>
            <span
              class="badge"
              :class="resolvedBadge"
            >{{ notificationResolved(item) ? 'Résolu' : 'Actif' }}</span>
          </div>
          <div class="d-flex align-items-center gap-2 flex-shrink-0">
            <button
              v-if="isAdmin && item.type === 'alert_incident' && !notificationResolved(item)"
              type="button"
              class="btn btn-sm btn-outline-success py-0 px-2"
              :disabled="resolving"
              @click.stop="$emit('resolve', item)"
            >
              <span
                v-if="resolving"
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
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IconCode, IconAlertTriangle } from '@tabler/icons-vue'
import RelativeTime from './RelativeTime.vue'
import type { NotificationItem } from '../types/generated'
import {
  isTrackerType,
  metricUnit,
  notificationResolved,
  notificationRoute,
  notificationTitle,
} from '../utils/incidentFormat'

const props = defineProps<{
  item: NotificationItem
  unread: boolean
  isAdmin: boolean
  resolving: boolean
}>()

defineEmits<{
  (e: 'resolve', item: NotificationItem): void
}>()

const iconBg = computed(() => {
  if (isTrackerType(props.item)) return 'bg-blue text-white'
  if (props.item.severity === 'crit') return 'bg-red text-white'
  if (props.item.severity === 'warn') return 'bg-yellow text-white'
  return 'bg-secondary text-white'
})

const severityBadge = computed(() => (props.item.severity === 'crit' ? 'bg-red-lt text-red' : 'bg-yellow-lt text-yellow'))

const resolvedBadge = computed(() => (notificationResolved(props.item) ? 'bg-green-lt text-green' : 'bg-red-lt text-red'))
</script>

<style scoped>
.notification-unread {
  background: rgba(var(--tblr-azure-rgb), 0.04);
}
</style>
