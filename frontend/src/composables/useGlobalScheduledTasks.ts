import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useHostsStore } from '../stores/hosts'
import { addToast } from './useGlobalToast'
import { confirmBulkAction } from '../utils/bulkActionHelpers'
import api from '../api'
import { isManualOnly, describeCron, nextCronRun, MANUAL_SENTINEL } from '../utils/cron'
import { useConfirmDialog } from './useConfirmDialog'
import { usePendingCommand } from './usePendingCommand'
import { useHostCommandConsole } from './useHostCommandConsole'
import { useCommandStream } from './useCommandStream'
import type { ScheduledTaskWithHost, ScheduledTaskExecution } from '../types/task'
import { getApiErrorMessage } from '../api/client'
import { getExecutionStateClass } from '../utils/statusClasses'
import { commandStatusLabel } from '../utils/commandStatus'

const DEFAULT_CRON = '0 3 * * *'
const TASKS_REFRESH_SEC = 30

function emptyCreateForm() {
  return { host_id: '', name: '', module: 'apt', action: 'update', target: '', cron_expression: DEFAULT_CRON, enabled: true }
}

function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

function statusBadge(status: string | undefined): string {
  return getExecutionStateClass(status, 'badge bg-warning-lt text-warning')
}

function durationSec(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime()
  return (ms / 1000).toFixed(1)
}

function updatePayloadFor(t: ScheduledTaskWithHost, overrides: Partial<{ enabled: boolean }> = {}) {
  return {
    name: t.name,
    module: t.module,
    action: t.action,
    target: t.target,
    payload: t.payload,
    cron_expression: t.cron_expression,
    enabled: t.enabled,
    ...overrides,
  }
}

export function useGlobalScheduledTasks() {
  const auth = useAuthStore()
  const hostsStore = useHostsStore()
  const dialog = useConfirmDialog()
  const pendingCommand = usePendingCommand()

  const tasks = ref<ScheduledTaskWithHost[]>([])
  const loading = ref(false)
  const error = ref('')
  const runningId = ref<string | number | null>(null)
  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  let refreshTimer: ReturnType<typeof setInterval> | null = null

  const filterText = ref('')
  const filterHost = ref('')
  const filterModule = ref('')
  const filterStatus = ref('')
  const sortKey = ref('name')
  const sortDir = ref<'asc' | 'desc'>('asc')

  const selectedIds = ref<Set<string | number>>(new Set())
  const bulkLoading = ref(false)

  const editTask = ref<ScheduledTaskWithHost | null>(null)
  const editForm = ref({ name: '', cron_expression: '', enabled: false })
  const editManualOnly = ref(false)
  const editSaving = ref(false)
  const editError = ref('')

  const historyTask = ref<ScheduledTaskWithHost | null>(null)
  const executions = ref<ScheduledTaskExecution[]>([])
  const historyLoading = ref(false)
  const historyError = ref('')

  // Executions are remote_commands rows themselves (see ScheduledTaskExecution's
  // doc comment) — reuse the same console/CommandLogPanel pattern every other
  // module (apt/docker/systemd/runbooks/trackers/webhooks) already converged
  // on for "Logs", instead of the inline expand/collapse this view used to
  // have as its own one-off.
  const { liveCommand, showConsole, openCommand, closeConsole, updateCommand } = useHostCommandConsole()
  const { openCommandStream, closeStream: closeExecutionStream } = useCommandStream()

  function watchExecution(task: ScheduledTaskWithHost | null, ex: ScheduledTaskExecution): void {
    openCommand({
      id: ex.id,
      host_name: task?.host_name,
      module: 'scheduled_task',
      action: task?.name || '',
      status: ex.status,
      output: ex.output || '',
    })
    openCommandStream(String(ex.id), {
      onInit: (p) => {
        const current = liveCommand.value
        if (!current) return
        updateCommand({ ...current, status: p.status, output: p.output || current.output })
      },
      onChunk: (p) => {
        const current = liveCommand.value
        if (!current) return
        updateCommand({ ...current, output: (current.output || '') + (p.chunk || '') })
      },
      onStatus: (p) => {
        const current = liveCommand.value
        if (!current) return
        updateCommand({ ...current, status: p.status })
      },
    })
  }

  function closeExecutionConsole(): void {
    closeExecutionStream()
    closeConsole()
  }

  const createModalOpen = ref(false)
  const createForm = ref(emptyCreateForm())
  const createManualOnly = ref(false)
  const createSaving = ref(false)
  const createError = ref('')

  const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

  const createNextRun = computed(() => {
    const expr = createForm.value.cron_expression
    if (!expr || expr === MANUAL_SENTINEL) return null
    return nextCronRun(expr)
  })

  const editNextRun = computed(() => {
    const expr = editForm.value.cron_expression
    if (!expr || expr === MANUAL_SENTINEL) return null
    return nextCronRun(expr)
  })

  // Mirrors HostTasksTab's manual-only toggle: an explicit checkbox instead
  // of inferring "manual" from an empty cron field, so both scheduled-task
  // editors (per-host and global) share the same mental model.
  watch(createManualOnly, (manual) => {
    if (manual) {
      createForm.value.enabled = false
      createForm.value.cron_expression = MANUAL_SENTINEL
    } else {
      createForm.value.enabled = true
      if (createForm.value.cron_expression === MANUAL_SENTINEL) {
        createForm.value.cron_expression = DEFAULT_CRON
      }
    }
  })

  watch(editManualOnly, (manual) => {
    if (manual) {
      editForm.value.enabled = false
      editForm.value.cron_expression = MANUAL_SENTINEL
    } else {
      editForm.value.enabled = true
      if (editForm.value.cron_expression === MANUAL_SENTINEL) {
        editForm.value.cron_expression = DEFAULT_CRON
      }
    }
  })

  const hostList = computed(() => {
    const names = [...new Set(tasks.value.map((t) => t.host_name))]
    return names.sort()
  })

  const filteredTasks = computed(() => {
    const filtered = tasks.value.filter((task) => {
      if (filterHost.value && task.host_name !== filterHost.value) return false
      if (filterModule.value && task.module !== filterModule.value) return false
      if (filterStatus.value === 'enabled' && (!task.enabled || isManualOnly(task))) return false
      if (filterStatus.value === 'disabled' && (task.enabled || isManualOnly(task))) return false
      if (filterStatus.value === 'manual' && !isManualOnly(task)) return false
      if (filterStatus.value === 'failed' && task.last_run_status !== 'failed') return false
      if (filterText.value) {
        const q = filterText.value.toLowerCase()
        if (!task.name.toLowerCase().includes(q) &&
            !task.host_name.toLowerCase().includes(q) &&
            !task.module.toLowerCase().includes(q) &&
            !(task.target || '').toLowerCase().includes(q)) return false
      }
      return true
    })

    return [...filtered].sort((a, b) => {
      const key = sortKey.value
      const av = (a as Record<string, unknown>)[key] ?? ''
      const bv = (b as Record<string, unknown>)[key] ?? ''
      const cmp = String(av).localeCompare(String(bv), 'fr', { numeric: true })
      return sortDir.value === 'asc' ? cmp : -cmp
    })
  })

  const allVisibleSelected = computed(() =>
    filteredTasks.value.length > 0 && filteredTasks.value.every((t) => selectedIds.value.has(t.id))
  )
  const selectedTasks = computed(() => filteredTasks.value.filter((t) => selectedIds.value.has(t.id)))

  function toggleSelected(id: string | number, checked: boolean): void {
    const next = new Set(selectedIds.value)
    if (checked) next.add(id)
    else next.delete(id)
    selectedIds.value = next
  }

  function toggleSelectAll(checked: boolean): void {
    selectedIds.value = checked ? new Set(filteredTasks.value.map((t) => t.id)) : new Set()
  }

  function clearSelection(): void {
    selectedIds.value = new Set()
  }

  function toggleSort(key: string): void {
    if (sortKey.value === key) {
      sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    } else {
      sortKey.value = key
      sortDir.value = 'asc'
    }
  }

  function openCreate(): void {
    createForm.value = emptyCreateForm()
    createManualOnly.value = false
    createError.value = ''
    createModalOpen.value = true
  }

  async function saveCreate(): Promise<void> {
    createSaving.value = true
    createError.value = ''
    try {
      const { host_id, ...body } = createForm.value
      await api.createScheduledTask(host_id, {
        name: body.name,
        module: body.module,
        action: body.action,
        target: body.target,
        payload: '',
        cron_expression: body.cron_expression,
        enabled: body.cron_expression !== MANUAL_SENTINEL && body.enabled,
      })
      createModalOpen.value = false
      await loadTasks()
    } catch (e: unknown) {
      createError.value = getApiErrorMessage(e, 'Erreur lors de la création')
    } finally {
      createSaving.value = false
    }
  }

  function openEdit(task: ScheduledTaskWithHost): void {
    editTask.value = task
    editForm.value = { name: task.name, cron_expression: task.cron_expression, enabled: task.enabled }
    editManualOnly.value = isManualOnly(task)
    editError.value = ''
  }

  async function saveEdit(): Promise<void> {
    if (!editTask.value) return
    editSaving.value = true
    editError.value = ''
    try {
      await api.updateScheduledTask(editTask.value.id, {
        name: editForm.value.name,
        module: editTask.value.module,
        action: editTask.value.action,
        target: editTask.value.target,
        payload: editTask.value.payload,
        cron_expression: editForm.value.cron_expression,
        enabled: editForm.value.enabled,
      })
      editTask.value = null
      await loadTasks()
    } catch (e: unknown) {
      editError.value = getApiErrorMessage(e, 'Erreur lors de la sauvegarde')
    } finally {
      editSaving.value = false
    }
  }

  async function confirmDelete(task: ScheduledTaskWithHost): Promise<void> {
    const ok = await dialog.confirm({
      title: 'Supprimer la tâche',
      message: `Supprimer « ${task.name} » sur ${task.host_name} ?`,
      variant: 'danger',
    })
    if (!ok) return
    try {
      await api.deleteScheduledTask(task.id)
      await loadTasks()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur lors de la suppression')
    }
  }

  async function openHistory(task: ScheduledTaskWithHost): Promise<void> {
    historyTask.value = task
    executions.value = []
    historyError.value = ''
    historyLoading.value = true
    try {
      const { data } = await api.getScheduledTaskExecutions(String(task.id), 20)
      executions.value = data
    } catch (e: unknown) {
      historyError.value = getApiErrorMessage(e, 'Erreur de chargement')
    } finally {
      historyLoading.value = false
    }
  }

  async function loadTasks(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const { data } = await api.getAllScheduledTasks()
      tasks.value = data
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur de chargement')
    } finally {
      loading.value = false
    }
  }

  function startRefreshTimer(): void {
    stopRefreshTimer()
    refreshTimer = setInterval(() => {
      // Skip while a task is manually being run — avoids the row's
      // "running" state flickering under a refetch mid-execution.
      if (autoRefresh.value && runningId.value === null) loadTasks()
    }, TASKS_REFRESH_SEC * 1000)
  }

  function stopRefreshTimer(): void {
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
  }

  async function toggleTask(task: ScheduledTaskWithHost): Promise<void> {
    const enabling = !task.enabled
    const ok = await dialog.confirm({
      title: enabling ? 'Activer la tâche' : 'Désactiver la tâche',
      message: `Voulez-vous ${enabling ? 'activer' : 'désactiver'} « ${task.name} » sur ${task.host_name} ?`,
      variant: 'warning',
    })
    if (!ok) return
    try {
      await api.updateScheduledTask(task.id, updatePayloadFor(task, { enabled: enabling }))
      await loadTasks()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur')
    }
  }

  async function runNow(task: ScheduledTaskWithHost): Promise<void> {
    runningId.value = task.id
    try {
      const { data } = await api.runScheduledTask(String(task.id))
      addToast(`${task.name} déclenchée — commande ${data.command_id}`, 'success')
      await loadTasks()
      await pendingCommand.track(data.command_id)
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur')
    } finally {
      runningId.value = null
    }
  }

  // Shared bulk runner: confirms once, fires every item's action in parallel,
  // reports a single success/failure summary toast, then refreshes.
  async function runBulk(
    items: ScheduledTaskWithHost[],
    verb: string,
    action: (t: ScheduledTaskWithHost) => Promise<unknown>
  ): Promise<void> {
    if (!items.length || bulkLoading.value) return
    const ok = await confirmBulkAction(verb, items.length)
    if (!ok) return
    clearSelection()
    bulkLoading.value = true
    try {
      const results = await Promise.allSettled(items.map(action))
      const succeeded = results.filter((r) => r.status === 'fulfilled').length
      const failed = results.length - succeeded
      if (failed === 0) {
        addToast(`${succeeded} tâche${succeeded > 1 ? 's' : ''} traitée${succeeded > 1 ? 's' : ''}`, 'success')
      } else {
        addToast(`${succeeded} traitée(s), ${failed} échec(s)`, failed === results.length ? 'error' : 'warning', 6000)
      }
    } finally {
      bulkLoading.value = false
    }
    await loadTasks()
  }

  async function handleBulkEnable(items: ScheduledTaskWithHost[]): Promise<void> {
    await runBulk(items.filter((t) => !isManualOnly(t)), 'Activer', (t) =>
      api.updateScheduledTask(t.id, updatePayloadFor(t, { enabled: true })))
  }

  async function handleBulkDisable(items: ScheduledTaskWithHost[]): Promise<void> {
    await runBulk(items.filter((t) => !isManualOnly(t)), 'Désactiver', (t) =>
      api.updateScheduledTask(t.id, updatePayloadFor(t, { enabled: false })))
  }

  async function handleBulkDelete(items: ScheduledTaskWithHost[]): Promise<void> {
    await runBulk(items, 'Supprimer', (t) => api.deleteScheduledTask(t.id))
  }

  async function handleBulkRun(items: ScheduledTaskWithHost[]): Promise<void> {
    await runBulk(items, 'Exécuter', (t) => api.runScheduledTask(String(t.id)))
  }

  onMounted(() => {
    hostsStore.fetchHosts()
    loadTasks()
    startRefreshTimer()
  })
  onUnmounted(stopRefreshTimer)

  return {
    hostsStore,
    tasks,
    loading,
    error,
    autoRefresh,
    lastUpdatedAt,
    TASKS_REFRESH_SEC,
    runningId,
    filterText,
    filterHost,
    filterModule,
    filterStatus,
    sortKey,
    sortDir,
    selectedIds,
    bulkLoading,
    editTask,
    editForm,
    editManualOnly,
    editSaving,
    editError,
    historyTask,
    executions,
    historyLoading,
    historyError,
    liveCommand,
    showConsole,
    watchExecution,
    closeExecutionConsole,
    createModalOpen,
    createForm,
    createManualOnly,
    createSaving,
    createError,
    canManage,
    createNextRun,
    editNextRun,
    hostList,
    filteredTasks,
    allVisibleSelected,
    selectedTasks,
    toggleSelected,
    toggleSelectAll,
    clearSelection,
    toggleSort,
    openCreate,
    saveCreate,
    formatDate,
    statusBadge,
    commandStatusLabel,
    durationSec,
    isManualOnly,
    describeCron,
    openEdit,
    saveEdit,
    confirmDelete,
    openHistory,
    loadTasks,
    toggleTask,
    runNow,
    handleBulkEnable,
    handleBulkDisable,
    handleBulkDelete,
    handleBulkRun,
  }
}
