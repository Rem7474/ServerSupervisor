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
      Reconnexion en cours
      <span
        v-if="retryCount > 1"
        class="text-secondary ms-1"
      >(tentative {{ retryCount }})</span>
      …
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
    <span>Connexion au serveur…</span>
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
        <strong>Erreur WebSocket</strong>
        <span
          v-if="error"
          class="ms-1"
        >— {{ error }}</span>
      </span>
    </div>
    <button
      type="button"
      class="btn btn-sm btn-danger"
      @click="$emit('reconnect')"
    >
      Réessayer
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
        <strong>Données actualisées</strong>
        <span class="ms-1">après reconnexion</span>
      </span>
      <button
        type="button"
        class="btn-close"
        aria-label="Fermer l'alerte de fraîcheur"
        @click="$emit('dismiss-stale-alert')"
      />
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { IconAlertTriangle, IconBroadcast } from '@tabler/icons-vue'

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