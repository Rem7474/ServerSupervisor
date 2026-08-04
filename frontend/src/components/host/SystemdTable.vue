<template>
  <div
    v-if="services.length"
    class="table-responsive scroll-table"
  >
    <table class="table table-vcenter card-table mb-0">
      <thead>
        <tr>
          <th>Service</th>
          <th>État</th>
          <th>Mode</th>
          <th>Description</th>
          <th v-if="!readonly">
            Actions
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="svc in services"
          :key="svc.name"
        >
          <td class="font-monospace small">
            {{ svc.name }}
          </td>
          <td>
            <span :class="stateClass(svc.active_state)">{{ svc.active_state }}</span>
          </td>
          <td class="text-secondary small">
            {{ svc.sub_state }}
          </td>
          <td
            class="text-secondary small text-truncate"
            style="max-width: 250px;"
            :title="svc.description"
          >
            {{ svc.description || '—' }}
          </td>
          <td
            v-if="!readonly"
            class="text-nowrap"
          >
            <div class="d-flex align-items-center gap-1">
              <button
                v-if="svc.active_state !== 'active'"
                type="button"
                :disabled="!!actionPending[svc.name]"
                class="btn btn-icon btn-sm btn-ghost-success"
                title="Démarrer"
                aria-label="Démarrer le service"
                @click="$emit('action', svc.name, 'start')"
              >
                <span
                  v-if="actionPending[svc.name] === 'start'"
                  class="spinner-border spinner-border-sm"
                />
                <IconPlayerPlay
                  v-else
                  :size="16"
                  class="icon icon-sm"
                />
              </button>
              <button
                v-if="svc.active_state === 'active'"
                type="button"
                :disabled="!!actionPending[svc.name]"
                class="btn btn-icon btn-sm btn-ghost-danger"
                title="Arrêter"
                aria-label="Arrêter le service"
                @click="$emit('action', svc.name, 'stop')"
              >
                <span
                  v-if="actionPending[svc.name] === 'stop'"
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
                :disabled="!!actionPending[svc.name]"
                class="btn btn-icon btn-sm btn-ghost-warning"
                title="Redémarrer"
                aria-label="Redémarrer le service"
                @click="$emit('action', svc.name, 'restart')"
              >
                <span
                  v-if="actionPending[svc.name] === 'restart'"
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
                :disabled="!!actionPending[svc.name]"
                class="btn btn-icon btn-sm btn-ghost-secondary"
                title="Statut"
                aria-label="Voir le statut du service"
                @click="$emit('action', svc.name, 'status')"
              >
                <span
                  v-if="actionPending[svc.name] === 'status'"
                  class="spinner-border spinner-border-sm"
                />
                <IconTerminal2
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
    v-else
    class="text-secondary small"
  >
    Aucun service.
  </div>
</template>

<script setup lang="ts">
import { IconPlayerPlay, IconPlayerStop, IconRefresh, IconTerminal2 } from '@tabler/icons-vue'

export interface SystemdService {
  name: string
  active_state?: string
  sub_state?: string
  description?: string
}

withDefaults(defineProps<{
  services: SystemdService[]
  // Hides the Actions column entirely — used when rendering a past command's
  // output (CommandLogPanel.vue) rather than the live host-detail panel,
  // where start/stop/restart wouldn't make sense against a historical log.
  readonly?: boolean
  actionPending?: Record<string, string | null>
}>(), {
  readonly: false,
  actionPending: () => ({}),
})

defineEmits<{
  (e: 'action', serviceName: string, action: 'start' | 'stop' | 'restart' | 'status'): void
}>()

function stateClass(state: string | undefined): string {
  if (state === 'active') return 'badge bg-success-lt text-success'
  if (state === 'failed') return 'badge bg-danger-lt text-danger'
  if (state === 'activating' || state === 'deactivating') return 'badge bg-warning-lt text-warning'
  return 'badge bg-secondary-lt text-secondary'
}
</script>
