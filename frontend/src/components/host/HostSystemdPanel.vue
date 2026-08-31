<template>
  <div
    v-if="canRun"
    class="card mt-4"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        {{ t('host.systemdTitle') }}
      </h3>
      <div class="d-flex align-items-center gap-2">
        <div class="btn-group btn-group-sm">
          <button
            type="button"
            :class="filter === 'active' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
            @click="filter = 'active'"
          >
            {{ t('host.filterActive') }}
          </button>
          <button
            type="button"
            :class="filter === 'all' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
            @click="filter = 'all'"
          >
            {{ t('host.filterAllServices') }}
          </button>
        </div>
        <button
          type="button"
          class="btn btn-sm btn-outline-secondary"
          :disabled="loading"
          @click="loadServices"
        >
          <span
            v-if="loading"
            class="spinner-border spinner-border-sm me-1"
          />
          {{ loading ? t('host.loadingLabel') : t('host.loadServicesLabel') }}
        </button>
      </div>
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
        {{ t('host.clickToLoadServices') }}
      </div>
    </div>
    <div
      v-if="services.length && !loading"
      class="card-body"
    >
      <SystemdTable
        :services="filteredServices"
        :action-pending="actionPending"
        @action="runAction"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import SystemdTable from './SystemdTable.vue'
import type { SystemdService } from './SystemdTable.vue'
import apiClient, { getApiErrorMessage } from '../../api'
import { useCommandStream } from '../../composables/useCommandStream'
import { usePendingCommand } from '../../composables/usePendingCommand'
import { useLocalStorage } from '../../composables/useLocalStorage'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const props = withDefaults(defineProps<{
  hostId: string
  canRun?: boolean
}>(), {
  canRun: false,
})

const emit = defineEmits<{
  (e: 'open-console', payload: { commandId: string | number; prefix: string; command: string; module: string; target: string }): void
  (e: 'history-changed'): void
}>()

const { t } = useI18n()
const dialog = useConfirmDialog()
const services = ref<SystemdService[]>([])
const loading = ref(false)
const error = ref('')
const actionPending = ref<Record<string, string | null>>({})
const filter = useLocalStorage(`host-systemd-filter:${props.hostId}`, 'active')
const STREAM_TIMEOUT_MS = 60000
const { collectCommandOutput } = useCommandStream()
const pendingCommand = usePendingCommand()

const filteredServices = computed(() => {
  if (filter.value === 'all') return services.value
  return services.value.filter((s) => s.active_state === 'active')
})

async function loadServices(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await apiClient.sendSystemdCommand(props.hostId, '', 'list')
    const cmdId = res.data.command_id

    await collectCommandOutput(cmdId, { timeoutMs: STREAM_TIMEOUT_MS }).then((output: string) => {
      try {
        services.value = JSON.parse(output)
      } catch {
        error.value = t('host.parseServicesError')
      }
    }).catch((e: unknown) => {
      error.value = getApiErrorMessage(e, t('host.loadServicesError'))
    }).finally(() => { emit('history-changed') })
  } catch (e) {
    error.value = getApiErrorMessage(e, t('host.sendCommandError'))
  } finally {
    loading.value = false
  }
}

async function runAction(serviceName: string, action: string): Promise<void> {
  error.value = ''
  if (action === 'stop' || action === 'restart') {
    const ok = await dialog.confirm({
      title: action === 'stop' ? t('host.stopServiceAriaLabel') : t('host.restartServiceAriaLabel'),
      message: t('host.confirmSystemctlAction', { action, service: serviceName }),
      variant: action === 'stop' ? 'danger' : 'warning',
    })
    if (!ok) return
  }
  actionPending.value[serviceName] = action
  try {
    const res = await apiClient.sendSystemdCommand(props.hostId, serviceName, action)
    emit('open-console', {
      commandId: res.data.command_id,
      prefix: 'systemctl ',
      command: `${action} ${serviceName}`,
      module: 'systemd',
      target: serviceName,
    })
    await pendingCommand.track(res.data.command_id)
  } catch (e) {
    error.value = getApiErrorMessage(e, t('host.runSystemctlError', { action }))
  } finally {
    actionPending.value[serviceName] = null
  }
}
</script>
