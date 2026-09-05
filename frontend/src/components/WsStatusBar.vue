<template>
  <!-- Connected: nothing shown -->
  <div
    v-if="status === 'reconnecting'"
    class="alert alert-warning d-flex align-items-center gap-2 py-2 px-3 mb-3 ws-status-bar"
    role="alert"
  >
    <div
      class="spinner-border spinner-border-sm text-warning flex-shrink-0"
      role="status"
    />
    <span>
      {{ t('common.wsReconnectingLabel', { attempt: retryCount > 1 ? t('common.wsRetryAttemptLabel', { n: retryCount }) : '' }) }}
    </span>
  </div>

  <div
    v-else-if="status === 'connecting'"
    class="alert alert-secondary d-flex align-items-center gap-2 py-2 px-3 mb-3 ws-status-bar"
    role="alert"
  >
    <div
      class="spinner-border spinner-border-sm flex-shrink-0"
      role="status"
    />
    <span>{{ t('common.wsConnectingToServerLabel') }}</span>
  </div>

  <div
    v-else-if="status === 'error'"
    class="alert alert-danger d-flex align-items-center justify-content-between gap-2 py-2 px-3 mb-3 ws-status-bar"
    role="alert"
  >
    <div class="d-flex align-items-center gap-2">
      <!-- X icon -->
      <IconAlertTriangle
        :size="16"
        class="flex-shrink-0"
      />
      <span>
        <strong>{{ t('common.wsErrorTitle') }}</strong>
        <span
          v-if="error"
          class="ms-1"
        >{{ t('common.wsErrorDetailLabel', { error }) }}</span>
      </span>
    </div>
    <button
      type="button"
      class="btn btn-sm btn-danger"
      @click="$emit('reconnect')"
    >
      {{ t('common.retry') }}
    </button>
  </div>

  <Transition name="fade">
    <div
      v-if="dataStaleAlert"
      class="alert alert-info alert-dismissible d-flex align-items-center gap-2 py-2 px-3 mb-3 ws-status-bar ws-status-bar-info"
      role="status"
    >
      <IconBroadcast
        :size="16"
        class="flex-shrink-0 icon icon-sm"
      />
      <span class="flex-grow-1">
        <strong>{{ t('common.wsDataRefreshedTitle') }}</strong>
        <span class="ms-1">{{ t('common.wsAfterReconnectLabel') }}</span>
      </span>
      <button
        type="button"
        class="btn-close"
        :aria-label="t('common.wsCloseStaleAlertAriaLabel')"
        @click="$emit('dismiss-stale-alert')"
      />
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconAlertTriangle, IconBroadcast } from '@tabler/icons-vue'

const { t } = useI18n()

withDefaults(defineProps<{
  status: string
  error?: string
  retryCount?: number
  dataStaleAlert?: boolean
}>(), {
  error: '',
  retryCount: 0,
  dataStaleAlert: false,
})

defineEmits<{
  (e: 'reconnect'): void
  (e: 'dismiss-stale-alert'): void
}>()
</script>

<style scoped>
.ws-status-bar {
  position: sticky;
  top: 0;
  z-index: var(--z-index-sticky);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 300ms ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>