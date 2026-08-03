<template>
  <div>
    <div class="card-header d-flex align-items-center gap-2 flex-wrap">
      <div class="btn-group btn-group-sm">
        <button
          type="button"
          :class="filter === 'active' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          @click="filter = 'active'"
        >
          Actifs
        </button>
        <button
          type="button"
          :class="filter === 'all' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          @click="filter = 'all'"
        >
          Tous
        </button>
      </div>
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary ms-2"
        :disabled="loading"
        @click="emit('refresh')"
      >
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm me-1"
        />
        <IconRefresh
          v-else
          :size="16"
          class="icon icon-sm me-1"
        />
        {{ loading ? 'Chargement...' : 'Actualiser' }}
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
      v-if="loading && !services.length"
      class="card-body"
    >
      <LoadingSkeleton
        variant="table"
        :lines="4"
      />
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
      class="table-responsive scroll-table"
    >
      <table class="table table-vcenter card-table mb-0">
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
              <div class="d-flex align-items-center gap-1">
                <button
                  v-if="svc['active-state'] !== 'active'"
                  type="button"
                  :disabled="!!actionLoading?.[svc.name]"
                  class="btn btn-icon btn-sm btn-ghost-success"
                  title="Démarrer"
                  aria-label="Démarrer le service"
                  @click="emit('action', { name: svc.name, action: 'start' })"
                >
                  <span
                    v-if="actionLoading?.[svc.name] === 'start'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconPlayerPlay
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  v-if="svc['active-state'] === 'active'"
                  type="button"
                  :disabled="!!actionLoading?.[svc.name]"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  title="Arrêter"
                  aria-label="Arrêter le service"
                  @click="emit('action', { name: svc.name, action: 'stop' })"
                >
                  <span
                    v-if="actionLoading?.[svc.name] === 'stop'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconPlayerStop
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  :disabled="!!actionLoading?.[svc.name]"
                  class="btn btn-icon btn-sm btn-ghost-warning"
                  title="Redémarrer"
                  aria-label="Redémarrer le service"
                  @click="emit('action', { name: svc.name, action: 'restart' })"
                >
                  <span
                    v-if="actionLoading?.[svc.name] === 'restart'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconRefresh
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  :disabled="!!actionLoading?.[svc.name]"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  title="Recharger"
                  aria-label="Recharger le service"
                  @click="emit('action', { name: svc.name, action: 'reload' })"
                >
                  <span
                    v-if="actionLoading?.[svc.name] === 'reload'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconReload
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
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
import { IconPlayerPlay, IconPlayerStop, IconRefresh, IconReload } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

interface Service {
  name: string
  state?: string
  'active-state'?: string
  desc?: string
  [key: string]: unknown
}

const props = defineProps<{
  services: Service[]
  loading?: boolean
  error?: string
  actionMsg?: string
  actionOk?: boolean
  actionLoading?: Record<string, string | null>
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

function svcStateClass(state?: string): string {
  if (state === 'active') return 'badge bg-success-lt text-success'
  if (state === 'failed') return 'badge bg-danger-lt text-danger'
  if (state === 'activating' || state === 'deactivating') return 'badge bg-warning-lt text-warning'
  return 'badge bg-secondary-lt text-secondary'
}
</script>

<style scoped>
.proxmox-service-desc {
  max-width: 240px;
}
</style>
