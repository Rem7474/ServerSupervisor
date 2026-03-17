<template>
  <div v-if="canRun" class="card mt-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">Services système (systemd)</h3>
      <div class="d-flex align-items-center gap-2">
        <div class="btn-group btn-group-sm">
          <button
            :class="filter === 'active' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
            @click="filter = 'active'"
          >Actifs</button>
          <button
            :class="filter === 'all' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
            @click="filter = 'all'"
          >Tous</button>
        </div>
        <button class="btn btn-sm btn-outline-secondary" @click="loadServices" :disabled="loading">
          <span v-if="loading" class="spinner-border spinner-border-sm me-1"></span>
          {{ loading ? 'Chargement...' : 'Charger les services' }}
        </button>
      </div>
    </div>
    <div v-if="error" class="card-body pb-0">
      <div class="alert alert-danger mb-0">{{ error }}</div>
    </div>
    <div v-if="!services.length && !loading && !error" class="card-body">
      <div class="text-secondary small">Cliquez sur "Charger les services" pour afficher les services systemd de cet hôte.</div>
    </div>
    <div v-if="filteredServices.length" class="table-responsive">
      <table class="table table-vcenter table-hover card-table mb-0">
        <thead>
          <tr>
            <th>Service</th>
            <th>État</th>
            <th>Mode</th>
            <th>Description</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="svc in filteredServices" :key="svc.name">
            <td class="font-monospace small">{{ svc.name }}</td>
            <td>
              <span :class="stateClass(svc.active_state)">{{ svc.active_state }}</span>
            </td>
            <td class="text-secondary small">{{ svc.sub_state }}</td>
            <td class="text-secondary small text-truncate" style="max-width: 250px;" :title="svc.description">
              {{ svc.description || '—' }}
            </td>
            <td class="text-nowrap">
              <div class="btn-group btn-group-sm">
                <button
                  v-if="svc.active_state !== 'active'"
                  class="btn btn-outline-success"
                  @click="runAction(svc.name, 'start')"
                  title="Démarrer"
                >Start</button>
                <button
                  v-if="svc.active_state === 'active'"
                  class="btn btn-outline-danger"
                  @click="runAction(svc.name, 'stop')"
                  title="Arrêter"
                >Stop</button>
                <button
                  class="btn btn-outline-secondary"
                  @click="runAction(svc.name, 'restart')"
                  title="Redémarrer"
                >Restart</button>
                <button
                  class="btn btn-outline-secondary"
                  @click="runAction(svc.name, 'status')"
                  title="Statut"
                >Status</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient, { getApiErrorMessage } from '../api'
import { useCommandStream } from '../composables/useCommandStream'
import { useLocalStorage } from '../composables/useLocalStorage'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const props = defineProps({
  hostId: { type: String, required: true },
  canRun: { type: Boolean, default: false },
})

const emit = defineEmits(['open-console', 'history-changed'])

const auth = useAuthStore()
const dialog = useConfirmDialog()
const services = ref([])
const loading = ref(false)
const error = ref('')
const filter = useLocalStorage(`host-systemd-filter:${props.hostId}`, 'active')
const STREAM_TIMEOUT_MS = 20000
const { collectCommandOutput } = useCommandStream({ token: () => auth.token })

const filteredServices = computed(() => {
  if (filter.value === 'all') return services.value
  return services.value.filter(s => s.active_state === 'active')
})

function stateClass(state) {
  if (state === 'active') return 'badge bg-green-lt text-green'
  if (state === 'failed') return 'badge bg-red-lt text-red'
  if (state === 'activating' || state === 'deactivating') return 'badge bg-yellow-lt text-yellow'
  return 'badge bg-secondary-lt text-secondary'
}

async function loadServices() {
  loading.value = true
  error.value = ''
  try {
    const res = await apiClient.sendSystemdCommand(props.hostId, '', 'list')
    const cmdId = res.data.command_id

    await collectCommandOutput(cmdId, { timeoutMs: STREAM_TIMEOUT_MS }).then(output => {
      try {
        services.value = JSON.parse(output)
      } catch {
        error.value = 'Impossible de parser la liste des services'
      }
    }).catch(e => {
      error.value = e.message || 'Erreur lors du chargement des services'
    }).finally(() => { emit('history-changed') })
  } catch (e) {
    error.value = getApiErrorMessage(e, "Impossible d'envoyer la commande")
  } finally {
    loading.value = false
  }
}

async function runAction(serviceName, action) {
  error.value = ''
  if (action === 'stop' || action === 'restart') {
    const ok = await dialog.confirm({
      title: `${action === 'stop' ? 'Arrêter' : 'Redémarrer'} le service`,
      message: `Confirmer : systemctl ${action} ${serviceName}`,
      variant: action === 'stop' ? 'danger' : 'warning',
    })
    if (!ok) return
  }
  try {
    const res = await apiClient.sendSystemdCommand(props.hostId, serviceName, action)
    emit('open-console', {
      commandId: res.data.command_id,
      prefix: 'systemctl ',
      command: `${action} ${serviceName}`,
    })
  } catch (e) {
    error.value = getApiErrorMessage(e, `Impossible d'exécuter systemctl ${action}`)
  }
}
</script>
