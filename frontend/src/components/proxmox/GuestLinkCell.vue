<template>
  <span
    v-if="!link"
    class="text-muted small"
  >—</span>
  <div
    v-else-if="link.status === 'suggested'"
    class="d-flex align-items-center gap-1"
  >
    <span class="badge bg-warning-lt text-warning">{{ t('proxmox.suggestedBadge') }}</span>
    <span class="text-muted small">{{ link.host_hostname || link.host_name }}</span>
    <button
      type="button"
      class="btn btn-sm btn-success ms-1"
      @click="emit('confirm')"
    >
      ✓
    </button>
    <button
      type="button"
      class="btn btn-sm btn-outline-secondary"
      @click="emit('ignore')"
    >
      ✗
    </button>
  </div>
  <div
    v-else-if="link.status === 'confirmed'"
    class="d-flex align-items-center gap-1"
  >
    <span class="badge bg-success-lt text-success">{{ t('proxmox.linkedBadge') }}</span>
    <button
      type="button"
      class="btn btn-sm btn-outline-primary ms-1"
      :title="t('proxmox.viewHostTooltip')"
      @click="emit('go')"
    >
      {{ link.host_hostname || link.host_name }}
    </button>
  </div>
  <span
    v-else
    class="text-muted small"
  >—</span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

interface GuestLink {
  status?: string
  host_hostname?: string
  host_name?: string
}

const { t } = useI18n()

defineProps<{ link?: GuestLink | null }>()

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'ignore'): void
  (e: 'go'): void
}>()
</script>
