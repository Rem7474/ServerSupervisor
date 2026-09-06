<template>
  <div>
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">
          {{ t('host.journalTitle') }}
        </h3>
      </div>
      <div class="card-body">
        <div class="d-flex gap-2">
          <input
            v-model="journalService"
            type="text"
            class="form-control"
            :placeholder="t('host.journalServicePlaceholder')"
            style="max-width: 320px;"
            @keyup.enter="loadJournalLogs"
          >
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!journalService.trim() || journalLoading"
            @click="loadJournalLogs"
          >
            <span
              v-if="journalLoading"
              class="spinner-border spinner-border-sm me-1"
            />
            {{ journalLoading ? t('host.loadingLabel') : t('host.journalLoadLabel') }}
          </button>
        </div>
        <div
          v-if="journalError"
          class="alert alert-danger mt-3 mb-0"
        >
          {{ journalError }}
        </div>
        <div
          v-if="journalCmdId"
          class="text-secondary small mt-2"
        >
          {{ t('host.journalStreamHint', { id: journalCmdId }) }}
        </div>
      </div>
    </div>

    <HostSystemdPanel
      :host-id="String(hostId)"
      :can-run="canRunApt"
      @open-console="$emit('open-command', $event)"
      @history-changed="$emit('history-changed')"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import apiClient from '../../api'
import HostSystemdPanel from './HostSystemdPanel.vue'
import { getApiErrorMessage } from '../../api/client'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'open-command', payload: Record<string, unknown>): void
  (e: 'history-changed'): void
}>()

const props = withDefaults(defineProps<{
  hostId: string | number
  canRunApt?: boolean
}>(), {
  canRunApt: false,
})

const journalService = ref('')
const journalLoading = ref(false)
const journalError = ref('')
const journalCmdId = ref<string | number | null>(null)

async function loadJournalLogs(): Promise<void> {
  const svc = journalService.value.trim()
  if (!svc) return
  journalLoading.value = true
  journalError.value = ''
  journalCmdId.value = null
  try {
    const res = await apiClient.sendJournalCommand(String(props.hostId), svc)
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
  } catch (e: unknown) {
    journalError.value = getApiErrorMessage(e, t('host.journalSendError'))
  } finally {
    journalLoading.value = false
  }
}
</script>
