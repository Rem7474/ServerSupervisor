<template>
  <div class="card h-100">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0 d-flex align-items-center gap-2">
        <span :class="`status status-${tone}`">
          <span class="status-dot status-dot-animated" />
        </span>
        {{ title }}
      </h3>
      <BadgePill
        :text="String(items.length)"
        :tone="tone"
        compact
      />
    </div>
    <div
      v-if="items.length === 0"
      class="card-body py-4"
    >
      <p class="text-muted text-center mb-0 small">
        Aucun incident {{ title.toLowerCase() }} actif.
      </p>
    </div>
    <div
      v-else
      class="list-group list-group-flush"
    >
      <div
        v-for="item in items"
        :key="item.id"
        class="list-group-item"
      >
        <div class="d-flex align-items-start justify-content-between gap-2">
          <div class="flex-fill min-w-0">
            <div class="d-flex align-items-center gap-2">
              <router-link
                v-if="notificationRoute(item)"
                :to="notificationRoute(item)"
                class="fw-semibold text-decoration-none text-truncate"
              >
                {{ notificationTitle(item) }}
              </router-link>
              <span
                v-else
                class="fw-semibold text-truncate"
              >{{ notificationTitle(item) }}</span>
              <BadgePill
                v-if="notificationAcknowledged(item)"
                text="En cours"
                tone="warning"
                compact
              />
              <BadgePill
                v-if="correlatedCount(item) > 0"
                :text="`+${correlatedCount(item)} corrélée${correlatedCount(item) > 1 ? 's' : ''}`"
                tone="secondary"
                compact
                :title="`${correlatedCount(item)} incident(s) sur des containers/VM de cet hôte, corrélés à celui-ci — pas de notification séparée`"
              />
            </div>
            <div class="text-muted small text-truncate">
              {{ item.host_name || 'Source inconnue' }} · {{ formatIncidentValue({ value: item.value, metric: item.metric, value_label: item.value_label }) }}
            </div>
            <div class="text-muted small">
              Depuis {{ incidentDuration(item) }}
              <span v-if="resolveHint(item)">· {{ resolveHint(item) }}</span>
            </div>
          </div>
          <div
            v-if="isAdmin"
            class="d-flex gap-1 flex-shrink-0"
          >
            <button
              v-if="!notificationAcknowledged(item)"
              type="button"
              class="btn btn-icon btn-sm btn-ghost-warning"
              :disabled="acknowledgingId === item.id"
              title="Accuser réception — je m'en occupe"
              aria-label="Accuser réception de l'incident"
              @click="$emit('acknowledge', item)"
            >
              <span
                v-if="acknowledgingId === item.id"
                class="spinner-border spinner-border-sm"
              />
              <IconEye
                v-else
                :size="14"
              />
            </button>
            <button
              type="button"
              class="btn btn-icon btn-sm btn-ghost-success"
              :disabled="resolvingId === item.id"
              title="Clôturer manuellement"
              aria-label="Clôturer l'incident"
              @click="$emit('resolve', item)"
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
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconCheck, IconEye } from '@tabler/icons-vue'
import BadgePill from '../common/BadgePill.vue'
import {
  formatIncidentValue,
  incidentDuration,
  notificationAcknowledged,
  notificationRoute,
  notificationTitle,
  resolveHint,
  resolvableIncidentId,
} from '../../utils/incidentFormat'
import type { NotificationItem } from '../../types/generated'

const props = withDefaults(defineProps<{
  title: string
  tone: 'danger' | 'warning'
  items?: NotificationItem[]
  correlatedCounts?: Map<number, number>
  isAdmin?: boolean
  resolvingId?: string | number | null
  acknowledgingId?: string | number | null
}>(), {
  items: () => [],
  correlatedCounts: () => new Map(),
  isAdmin: false,
  resolvingId: null,
  acknowledgingId: null,
})

defineEmits<{
  (e: 'resolve', item: NotificationItem): void
  (e: 'acknowledge', item: NotificationItem): void
}>()

function correlatedCount(item: NotificationItem): number {
  const id = resolvableIncidentId(item)
  if (!id) return 0
  return props.correlatedCounts.get(Number(id)) || 0
}
</script>
