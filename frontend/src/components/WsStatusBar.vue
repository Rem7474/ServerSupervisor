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
      <svg
        class="flex-shrink-0"
        width="16"
        height="16"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
        />
      </svg>
      <span>
        <strong>Erreur WebSocket</strong>
        <span
          v-if="error"
          class="ms-1"
        >— {{ error }}</span>
      </span>
    </div>
    <button
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
      <svg
        class="flex-shrink-0 icon icon-sm"
        width="16"
        height="16"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M16 3.13a4 4 0 0 1 0 7.75M21 10a7 7 0 0 1-7 7m-3.5-6.5a3.5 3.5 0 1 1-5 3.12M12 3v18"
        />
      </svg>
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

<script setup>
defineProps({
  status: {
    type: String,
    required: true
  },
  error: {
    type: String,
    default: ''
  },
  retryCount: {
    type: Number,
    default: 0
  },
  dataStaleAlert: {
    type: Boolean,
    default: false
  }
})

defineEmits(['reconnect', 'dismiss-stale-alert'])
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