<template>
  <div class="card-body">
    <!-- Apt update action bar -->
    <div class="d-flex align-items-center gap-2 mb-3 flex-wrap">
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary"
        :disabled="aptRefreshing"
        @click="emit('refresh-apt')"
      >
        <span
          v-if="aptRefreshing"
          class="spinner-border spinner-border-sm me-1"
        />
        apt update
      </button>
      <span
        v-if="aptRefreshMsg"
        :class="['small', aptRefreshOk ? 'text-success' : 'text-danger']"
      >{{ aptRefreshMsg }}</span>
    </div>

    <EmptyState
      v-if="pendingUpdates === 0"
      :title="t('proxmox.noPendingUpdatesTitle')"
      :subtitle="lastUpdateCheckAt
        ? t('proxmox.lastCheckSubtitle', { date: formatDate(lastUpdateCheckAt) })
        : t('proxmox.noUpdateDataYetSubtitle')"
    />
    <div v-else>
      <div class="d-flex align-items-center gap-3 mb-3">
        <div class="h2 mb-0">
          {{ pendingUpdates }}
        </div>
        <div>
          <div class="fw-medium">
            {{ t('proxmox.pendingPackagesLabel') }}
          </div>
          <div
            v-if="lastUpdateCheckAt"
            class="text-muted small"
          >
            {{ t('proxmox.detectedOnLabel', { date: formatDate(lastUpdateCheckAt) }) }}
          </div>
        </div>
      </div>
      <div class="alert alert-info mb-0">
        {{ t('proxmox.aptCacheReadOnlyHint') }}
        {{ t('proxmox.applyUpdatesHint') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'

const { t, locale } = useI18n()

defineProps<{
  pendingUpdates?: number
  lastUpdateCheckAt?: string | null
  aptRefreshing?: boolean
  aptRefreshMsg?: string
  aptRefreshOk?: boolean
}>()

const emit = defineEmits<{ (e: 'refresh-apt'): void }>()

function formatDate(iso?: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
}
</script>
