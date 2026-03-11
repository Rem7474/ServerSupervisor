<template>
  <div class="card mt-0">
    <div class="card-header">
      <h3 class="card-title">Historique de commandes</h3>
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
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="cmd in displayedCommands" :key="cmd.id">
            <td class="text-secondary small">{{ formatDate(cmd.created_at) }}</td>
            <td>
              <span :class="cmdModuleClass(cmd.module)">{{ cmdModuleLabel(cmd.module) }}</span>
            </td>
            <td>
              <code class="small">{{ cmdLabel(cmd) }}</code>
            </td>
            <td><span :class="statusClass(cmd.status)">{{ cmd.status }}</span></td>
            <td class="text-secondary small">{{ cmd.triggered_by || '-' }}</td>
            <td>
              <button
                @click="$emit('watch-command', cmd)"
                class="btn btn-sm btn-ghost-secondary"
                title="Voir les logs"
              >
                <svg class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" xmlns="http://www.w3.org/2000/svg"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="total > 5 && !showFullHistory" class="card-footer text-center">
      <button @click="showFullHistory = true" class="btn btn-outline-secondary btn-sm">
        Afficher tout ({{ total - 5 }} autres)
      </button>
    </div>
  </div>
  <div v-if="!total" class="card"><div class="card-body text-secondary">Aucune commande executee sur cet hote.</div></div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { usePagination } from '../../composables/usePagination'
import { useStatusBadge } from '../../composables/useStatusBadge'

defineEmits(['watch-command'])

const props = defineProps({
  commands: {
    type: Array,
    default: () => [],
  },
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

const MODULE_META = {
  apt: { label: 'APT', cls: 'badge bg-azure-lt text-azure' },
  docker: { label: 'Docker', cls: 'badge bg-blue-lt text-blue' },
  systemd: { label: 'Systemd', cls: 'badge bg-green-lt text-green' },
  journal: { label: 'Journal', cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus', cls: 'badge bg-orange-lt text-orange' },
  custom: { label: 'Custom', cls: 'badge bg-teal-lt text-teal' },
}

function cmdModuleLabel(module) {
  return MODULE_META[module]?.label ?? module
}

function cmdModuleClass(module) {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary'
}

function cmdLabel(cmd) {
  const parts = [cmd.action]
  if (cmd.target) parts.push(cmd.target)
  return parts.join(' ')
}

function statusClass(status) {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}
</script>
