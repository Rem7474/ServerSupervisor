import { onUnmounted, ref } from 'vue'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import { useHostsStore } from '../stores/hosts'
import { useCommandStream } from './useCommandStream'
import { i18n } from '../i18n'
import type { CommandStreamInitMsg, CommandStreamChunkMsg, CommandStatusUpdateMsg } from '../types/ws'
import type { Runbook, RunbookCreate, RunbookStepCreate, RunbookExecution, RunbookExecutionStep } from '../types/generated'
import type { DispatchOption } from '../utils/dispatchStep'

const EXECUTION_POLL_MS = 3_000

function isRunbookTerminal(status: string | undefined): boolean {
  return status === 'completed' || status === 'failed'
}

// A function (not a static Record) so labels re-resolve on every call — a
// locale switch would otherwise leave these frozen in whichever language was
// active when this module first loaded, same reasoning as moduleMeta.ts.
function actionLabels(): Record<string, string> {
  const { t } = i18n.global
  return {
    logs: t('runbooks.logsActionLabel'), restart: t('runbooks.restartActionLabel'),
    start: t('runbooks.startActionLabel'), stop: t('runbooks.stopActionLabel'),
    compose_up: 'Compose up', compose_down: 'Compose down', compose_pull: t('runbooks.composePullActionLabel'),
    compose_logs: t('runbooks.composeLogsActionLabel'), compose_restart: t('runbooks.composeRestartActionLabel'),
    update: 'apt update', upgrade: 'apt upgrade', 'full-upgrade': 'apt full-upgrade', autoremove: 'apt autoremove',
    status: t('runbooks.statusActionLabel'), list: t('runbooks.listActionLabel'),
    read: t('runbooks.readActionLabel'), run: t('runbooks.runActionLabel'),
  }
}

const MODULE_ACTIONS: Record<string, string[]> = {
  docker: ['logs', 'restart', 'start', 'stop', 'compose_up', 'compose_down', 'compose_pull', 'compose_logs', 'compose_restart'],
  journal: ['read'],
  apt: ['update', 'upgrade', 'full-upgrade', 'autoremove'],
  systemd: ['status', 'start', 'stop', 'restart', 'list'],
  processes: ['list'],
  custom: ['run'],
}

const MODULES_REQUIRING_TARGET = new Set(['journal', 'systemd', 'custom'])

export function actionsForModule(module: string): DispatchOption[] {
  const actions = MODULE_ACTIONS[module] || []
  const labels = actionLabels()
  return actions.map((value) => ({ value, label: labels[value] || value }))
}

export function moduleRequiresTarget(module: string): boolean {
  return MODULES_REQUIRING_TARGET.has(module)
}

export function emptyStep(): RunbookStepCreate {
  return { host_id: '', module: 'docker', action: 'restart', target: '', payload: '', continue_on_failure: false }
}

export function useRunbooks() {
  const hostsStore = useHostsStore()

  const runbooks = ref<Runbook[]>([])
  const loading = ref(false)
  const fetched = ref(false)
  const error = ref('')

  const showModal = ref(false)
  const editingRunbook = ref<Runbook | null>(null)
  const saving = ref(false)
  const saveError = ref('')

  const historyRunbook = ref<Runbook | null>(null)
  const executions = ref<RunbookExecution[]>([])
  const executionsLoading = ref(false)
  const selectedExecution = ref<RunbookExecution | null>(null)

  async function loadRunbooks(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.getRunbooks()
      runbooks.value = res.data || []
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e)
    } finally {
      loading.value = false
      fetched.value = true
    }
  }

  function startAdd(): void {
    editingRunbook.value = null
    saveError.value = ''
    showModal.value = true
  }

  function startEdit(rb: Runbook): void {
    editingRunbook.value = rb
    saveError.value = ''
    showModal.value = true
  }

  function closeModal(): void {
    showModal.value = false
    editingRunbook.value = null
  }

  async function saveRunbook(name: string, description: string, steps: RunbookStepCreate[]): Promise<boolean> {
    saving.value = true
    saveError.value = ''
    try {
      const payload: RunbookCreate = { name, description, steps }
      if (editingRunbook.value) {
        await apiClient.updateRunbook(editingRunbook.value.id, payload)
      } else {
        await apiClient.createRunbook(payload)
      }
      await loadRunbooks()
      closeModal()
      return true
    } catch (e: unknown) {
      saveError.value = getApiErrorMessage(e)
      return false
    } finally {
      saving.value = false
    }
  }

  async function deleteRunbook(rb: Runbook): Promise<void> {
    try {
      await apiClient.deleteRunbook(rb.id)
      await loadRunbooks()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e)
    }
  }

  const runningIds = ref(new Set<string>())
  let executionPollTimer: ReturnType<typeof setInterval> | null = null

  function stopExecutionPoll(): void {
    if (executionPollTimer) {
      clearInterval(executionPollTimer)
      executionPollTimer = null
    }
  }

  // Polls the open history modal while its runbook has a non-terminal
  // execution — the API only dispatches the first step and advances via the
  // agent's own command-completion callback (see runbook.Service.NotifyComplete
  // in the backend), so there's no push channel here; a short poll is the
  // simplest way to reflect step-by-step progress without one.
  function startExecutionPoll(rb: Runbook): void {
    stopExecutionPoll()
    executionPollTimer = setInterval(async () => {
      if (!historyRunbook.value || historyRunbook.value.id !== rb.id) {
        stopExecutionPoll()
        return
      }
      try {
        const res = await apiClient.getRunbookExecutions(rb.id)
        executions.value = res.data || []
      } catch {
        return
      }
      const top = executions.value[0]
      if (!top) return
      if (selectedExecution.value?.id === top.id || !selectedExecution.value) {
        await selectExecution(top)
      }
      if (isRunbookTerminal(executions.value[0]?.status)) {
        stopExecutionPoll()
      }
    }, EXECUTION_POLL_MS)
  }

  async function runRunbook(rb: Runbook): Promise<void> {
    runningIds.value.add(rb.id)
    try {
      await apiClient.runRunbook(rb.id)
      // Drop the user straight into the execution they just launched instead
      // of leaving them staring at a spinner-only button — see the history
      // modal's step table + live log panel for actual progress.
      await openHistory(rb)
      if (executions.value.length) {
        await selectExecution(executions.value[0])
        startExecutionPoll(rb)
      }
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e)
    } finally {
      runningIds.value.delete(rb.id)
    }
  }

  async function openHistory(rb: Runbook): Promise<void> {
    historyRunbook.value = rb
    selectedExecution.value = null
    executionsLoading.value = true
    try {
      const res = await apiClient.getRunbookExecutions(rb.id)
      executions.value = res.data || []
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e)
    } finally {
      executionsLoading.value = false
    }
  }

  function closeHistory(): void {
    stopExecutionPoll()
    closeStepLogs()
    historyRunbook.value = null
    executions.value = []
    selectedExecution.value = null
  }

  async function selectExecution(exec: RunbookExecution): Promise<void> {
    if (!historyRunbook.value) return
    try {
      const res = await apiClient.getRunbookExecution(historyRunbook.value.id, exec.id)
      selectedExecution.value = res.data
      if (!isRunbookTerminal(res.data.status) && !executionPollTimer) {
        startExecutionPoll(historyRunbook.value)
      }
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e)
    }
  }

  // ── Live step logs ────────────────────────────────────────────────────────────
  const selectedStepCommand = ref<{ id: string; host_id?: string; module?: string; action?: string; target?: string; status?: string; output?: string } | null>(null)
  const showStepLogPanel = ref(false)
  const { openCommandStream, closeStream } = useCommandStream()

  function openStepLogs(step: RunbookExecutionStep): void {
    if (!step.command_id) return
    if (selectedStepCommand.value?.id === step.command_id) {
      showStepLogPanel.value = true
      return
    }
    closeStream()
    selectedStepCommand.value = {
      id: step.command_id,
      host_id: step.host_id,
      module: step.module,
      action: step.action,
      target: step.target,
      status: step.status,
      output: step.output || '',
    }
    showStepLogPanel.value = true
    if (step.status === 'pending' || step.status === 'running') {
      openCommandStream(step.command_id, {
        closeOnTerminalStatus: true,
        onInit(p: CommandStreamInitMsg) {
          if (selectedStepCommand.value) { selectedStepCommand.value.status = p.status; selectedStepCommand.value.output = p.output || '' }
        },
        onChunk(p: CommandStreamChunkMsg) {
          if (selectedStepCommand.value) selectedStepCommand.value.output = (selectedStepCommand.value.output || '') + p.chunk
        },
        onStatus(p: CommandStatusUpdateMsg) {
          if (selectedStepCommand.value) { selectedStepCommand.value.status = p.status; if (p.output) selectedStepCommand.value.output = p.output }
        },
      })
    }
  }

  function closeStepLogs(): void {
    closeStream()
    selectedStepCommand.value = null
    showStepLogPanel.value = false
  }

  onUnmounted(() => {
    stopExecutionPoll()
    closeStream()
  })

  return {
    hostsStore,
    runbooks,
    loading,
    fetched,
    error,
    showModal,
    editingRunbook,
    saving,
    saveError,
    historyRunbook,
    executions,
    executionsLoading,
    selectedExecution,
    runningIds,
    loadRunbooks,
    startAdd,
    startEdit,
    closeModal,
    saveRunbook,
    deleteRunbook,
    runRunbook,
    openHistory,
    closeHistory,
    selectExecution,
    selectedStepCommand,
    showStepLogPanel,
    openStepLogs,
    closeStepLogs,
  }
}
