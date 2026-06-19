<template>
  <div class="card mt-0">
    <div class="card-header">
      <h3 class="card-title">
        Historique de commandes
      </h3>
      <div class="card-options">
        <span class="badge bg-secondary-lt text-secondary">{{ showFullHistory ? total : displayedCommands.length }}</span>
      </div>
    </div>
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Date</th>
            <th>Type</th>
            <th>Commande</th>
            <th>Statut</th>
            <th>Utilisateur</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="cmd in displayedCommands"
            :key="cmd.id"
          >
            <td class="text-secondary small">
              {{ formatDate(cmd.created_at) }}
            </td>
            <td>
              <span :class="cmdModuleClass(cmd.module || '')">{{ cmdModuleLabel(cmd.module || '') }}</span>
            </td>
            <td>
              <code class="small">{{ cmdLabel(cmd) }}</code>
            </td>
            <td><span :class="statusClass(cmd.status)">{{ cmd.status }}</span></td>
            <td class="text-secondary small">
              {{ cmd.triggered_by || '-' }}
            </td>
            <td>
              <button
                type="button"
                class="btn btn-sm btn-ghost-secondary"
                title="Voir les logs"
                @click="$emit('watch-command', cmd)"
              >
                <IconList
                  :size="16"
                  class="icon icon-sm"
                />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-if="total > 5 && !showFullHistory"
      class="card-footer text-center"
    >
      <button
        type="button"
        class="btn btn-outline-secondary btn-sm"
        @click="showFullHistory = true"
      >
        Afficher tout ({{ total - 5 }} autres)
      </button>
    </div>
  </div>
  <div
    v-if="!total"
    class="card"
  >
    <div class="card-body text-secondary">
      Aucune commande exécutée sur cet hôte.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IconList } from '@tabler/icons-vue'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { usePagination } from '../../composables/usePagination'
import { useStatusBadge } from '../../composables/useStatusBadge'
import { moduleLabel as cmdModuleLabel, moduleClass as cmdModuleClass } from '../../utils/moduleMeta'

interface Command {
  id: string | number
  created_at?: string
  module?: string
  action?: string
  target?: string
  status?: string
  triggered_by?: string
}

defineEmits<{
  (e: 'watch-command', cmd: Command): void
}>()

const props = withDefaults(defineProps<{
  commands?: Command[]
}>(), {
  commands: () => [],
})

const { formatRelativeDate: formatDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

const showFullHistory = ref(false)
const total = computed(() => props.commands.length)
const { pagedItems: firstPageCommands } = usePagination({
  items: computed(() => props.commands),
  pageSize: 5,
})
const displayedCommands = computed(() => (showFullHistory.value ? props.commands : firstPageCommands.value))

function cmdLabel(cmd: Command): string {
  const parts = [cmd.action]
  if (cmd.target) parts.push(cmd.target)
  return parts.filter(Boolean).join(' ')
}

function statusClass(status: string | undefined): string {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}
</script>

