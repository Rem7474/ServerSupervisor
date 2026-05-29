<template>
  <div class="card-body">
    <!-- Apt update action bar -->
    <div class="d-flex align-items-center gap-2 mb-3 flex-wrap">
      <button
        class="btn btn-outline-secondary"
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

    <div
      v-if="pendingUpdates === 0"
      class="text-center text-muted py-3"
    >
      <div class="mb-1">
        Aucune mise à jour en attente détectée.
      </div>
      <div
        v-if="lastUpdateCheckAt"
        class="small"
      >
        Dernière vérification : {{ formatDate(lastUpdateCheckAt) }}
      </div>
      <div
        v-else
        class="small"
      >
        Données non encore disponibles (prochain cycle de polling).
      </div>
    </div>
    <div v-else>
      <div class="d-flex align-items-center gap-3 mb-3">
        <div class="h2 mb-0">
          {{ pendingUpdates }}
        </div>
        <div>
          <div class="fw-medium">
            paquet(s) en attente de mise à jour
          </div>
          <div
            v-if="lastUpdateCheckAt"
            class="text-muted small"
          >
            Détecté le {{ formatDate(lastUpdateCheckAt) }}
          </div>
        </div>
      </div>
      <div class="alert alert-info mb-0">
        Ces informations proviennent du cache apt du nœud Proxmox (lecture seule).
        Pour appliquer les mises à jour, connectez-vous directement au nœud.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
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
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}
</script>
