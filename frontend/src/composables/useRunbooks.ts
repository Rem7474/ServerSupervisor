import { ref } from 'vue'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import { useHostsStore } from '../stores/hosts'
import type { Runbook, RunbookCreate, RunbookStepCreate, RunbookExecution } from '../types/generated'

export interface ModuleOption {
  value: string
  label: string
}

// Mirrors server/internal/services/runbook/service.go's commandModuleActions
// whitelist exactly — a step can only ever name an action already valid for
// that module, same as the alert rule command_trigger picker.
export const RUNBOOK_MODULES: ModuleOption[] = [
  { value: 'docker', label: 'Docker' },
  { value: 'apt', label: 'APT' },
  { value: 'systemd', label: 'Service systemd' },
  { value: 'journal', label: 'Journal systemd' },
  { value: 'processes', label: 'Processus' },
  { value: 'custom', label: 'Tâche personnalisée' },
]

const ACTION_LABELS: Record<string, string> = {
  logs: 'Voir les logs', restart: 'Redémarrer', start: 'Démarrer', stop: 'Arrêter',
  compose_up: 'Compose up', compose_down: 'Compose down', compose_pull: 'Mettre à jour les images',
  compose_logs: 'Voir les logs Compose', compose_restart: 'Redémarrer (Compose)',
  update: 'apt update', upgrade: 'apt upgrade', 'full-upgrade': 'apt full-upgrade', autoremove: 'apt autoremove',
  status: 'Statut', list: 'Lister', read: 'Lire', run: 'Exécuter',
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

export function actionsForModule(module: string): ModuleOption[] {
  const actions = MODULE_ACTIONS[module] || []
  return actions.map((value) => ({ value, label: ACTION_LABELS[value] || value }))
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

  async function runRunbook(rb: Runbook): Promise<void> {
    runningIds.value.add(rb.id)
    try {
      await apiClient.runRunbook(rb.id)
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
    historyRunbook.value = null
    executions.value = []
    selectedExecution.value = null
  }

  async function selectExecution(exec: RunbookExecution): Promise<void> {
    if (!historyRunbook.value) return
    try {
      const res = await apiClient.getRunbookExecution(historyRunbook.value.id, exec.id)
      selectedExecution.value = res.data
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e)
    }
  }

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
  }
}
