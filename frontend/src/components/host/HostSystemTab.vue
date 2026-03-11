<template>
  <div>
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Logs systeme (journalctl)</h3>
      </div>
      <div class="card-body">
        <div class="d-flex gap-2">
          <input
            v-model="journalService"
            type="text"
            class="form-control"
            placeholder="Nom du service (ex: nginx, ssh, docker)"
            @keyup.enter="loadJournalLogs"
            style="max-width: 320px;"
          >
          <button
            class="btn btn-primary"
            @click="loadJournalLogs"
            :disabled="!journalService.trim() || journalLoading"
          >
            <span v-if="journalLoading" class="spinner-border spinner-border-sm me-1"></span>
            {{ journalLoading ? 'Chargement...' : 'Charger les logs' }}
          </button>
        </div>
        <div v-if="journalError" class="alert alert-danger mt-3 mb-0">{{ journalError }}</div>
        <div v-if="journalCmdId" class="text-secondary small mt-2">
          Stream -> commande #{{ journalCmdId }} - les logs apparaissent dans la Console Live ->
        </div>
      </div>
    </div>

    <HostSystemdPanel :hostId="hostId" :can-run="canRunApt" @open-console="$emit('open-command', $event)" @history-changed="$emit('history-changed')" />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import apiClient from '../../api'
import HostSystemdPanel from '../HostSystemdPanel.vue'

const emit = defineEmits(['open-command', 'history-changed'])

const props = defineProps({
  hostId: {
    type: [String, Number],
    required: true,
  },
  canRunApt: {
    type: Boolean,
    default: false,
  },
})

const journalService = ref('')
const journalLoading = ref(false)
const journalError = ref('')
const journalCmdId = ref(null)

async function loadJournalLogs() {
  const svc = journalService.value.trim()
  if (!svc) return
  journalLoading.value = true
  journalError.value = ''
  journalCmdId.value = null
  try {
    const res = await apiClient.sendJournalCommand(props.hostId, svc)
    const cmdId = res.data.command_id
    journalCmdId.value = cmdId
    emit('open-command', {
      id: cmdId,
      prefix: '',
      command: `journalctl -u ${svc}`,
      module: 'journal',
      action: 'read',
      target: svc,
      status: 'running',
      output: '',
    })
    emit('history-changed')
  } catch (e) {
    journalError.value = e.response?.data?.error || "Impossible d'envoyer la commande"
  } finally {
    journalLoading.value = false
  }
}
</script>
