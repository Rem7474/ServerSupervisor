<template>
  <div>
    <div class="card-header d-flex align-items-center gap-2 flex-wrap">
      <div class="btn-group btn-group-sm">
        <button
          :class="filter === 'active' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          @click="filter = 'active'"
        >
          Actifs
        </button>
        <button
          :class="filter === 'all' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          @click="filter = 'all'"
        >
          Tous
        </button>
      </div>
      <button
        class="btn btn-sm btn-outline-secondary ms-2"
        :disabled="loading"
        @click="emit('refresh')"
      >
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm me-1"
        />
        {{ loading ? 'Chargement...' : '↻ Actualiser' }}
      </button>
      <span
        v-if="actionMsg"
        :class="['small ms-2', actionOk ? 'text-success' : 'text-danger']"
      >{{ actionMsg }}</span>
    </div>
    <div
      v-if="error"
      class="card-body pb-0"
    >
      <div class="alert alert-danger mb-0">
        {{ error }}
      </div>
    </div>
    <div
      v-if="!services.length && !loading && !error"
      class="card-body"
    >
      <div class="text-secondary small">
        Cliquez sur "Actualiser" pour charger les services du nœud Proxmox.
      </div>
    </div>
    <div
      v-if="filteredServices.length"
      class="table-responsive"
    >
      <table class="table table-vcenter table-hover card-table mb-0">
        <thead>
          <tr>
            <th>Service</th>
            <th>État</th>
            <th>Sous-état</th>
            <th>Démarrage</th>
            <th>Description</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="svc in filteredServices"
            :key="svc.name"
          >
            <td class="font-monospace small fw-medium">
              {{ svc.name }}
            </td>
            <td><span :class="svcStateClass(svc['active-state'])">{{ svc['active-state'] || svc.state }}</span></td>
            <td class="text-secondary small">
              {{ svc['sub-state'] || '—' }}
            </td>
            <td class="text-secondary small">
              {{ svc['unit-state'] || '—' }}
            </td>
            <td
              class="text-secondary small text-truncate proxmox-service-desc"
              :title="svc.desc"
            >
              {{ svc.desc || '—' }}
            </td>
            <td class="text-nowrap">
              <div class="btn-group btn-group-sm">
                <button
                  v-if="svc['active-state'] !== 'active'"
                  class="btn btn-outline-success"
                  title="Démarrer"
                  @click="emit('action', { name: svc.name, action: 'start' })"
                >
                  Start
                </button>
                <button
                  v-if="svc['active-state'] === 'active'"
                  class="btn btn-outline-danger"
                  title="Arrêter"
                  @click="emit('action', { name: svc.name, action: 'stop' })"
                >
                  Stop
                </button>
                <button
                  class="btn btn-outline-secondary"
                  title="Redémarrer"
                  @click="emit('action', { name: svc.name, action: 'restart' })"
                >
                  Restart
                </button>
                <button
                  class="btn btn-outline-secondary"
                  title="Recharger"
                  @click="emit('action', { name: svc.name, action: 'reload' })"
                >
                  Reload
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-if="error"
      class="card-footer text-muted small"
    >
      Lecture : Sys.Audit requis · Actions (start/stop/restart/reload) : Sys.Modify requis sur le token API.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

type Service = Record<string, any>

const props = defineProps<{
  services: Service[]
  loading?: boolean
  error?: string
  actionMsg?: string
  actionOk?: boolean
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
  (e: 'action', payload: { name: string; action: string }): void
}>()

const filter = ref<'active' | 'all'>('active')

const filteredServices = computed(() => {
  if (filter.value === 'all') return props.services
  return props.services.filter((s) => s['active-state'] === 'active' || s.state === 'running')
})

function svcStateClass(state: string): string {
  if (state === 'active') return 'badge bg-green-lt text-green'
  if (state === 'failed') return 'badge bg-red-lt text-red'
  if (state === 'activating' || state === 'deactivating') return 'badge bg-yellow-lt text-yellow'
  return 'badge bg-secondary-lt text-secondary'
}
</script>

<style scoped>
.proxmox-service-desc {
  max-width: 240px;
}
</style>
