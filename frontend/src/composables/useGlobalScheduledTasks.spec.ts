import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'

const {
  getAllScheduledTasks, runScheduledTask, getHosts, createScheduledTask, updateScheduledTask,
  deleteScheduledTask, getScheduledTaskExecutions,
} = vi.hoisted(() => ({
  getAllScheduledTasks: vi.fn(),
  runScheduledTask: vi.fn(),
  getHosts: vi.fn(),
  createScheduledTask: vi.fn(),
  updateScheduledTask: vi.fn(),
  deleteScheduledTask: vi.fn(),
  getScheduledTaskExecutions: vi.fn(),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))

vi.mock('../api', () => ({
  default: {
    getAllScheduledTasks, runScheduledTask, getHosts, createScheduledTask, updateScheduledTask,
    deleteScheduledTask, getScheduledTaskExecutions,
  },
}))

vi.mock('./usePendingCommand', () => ({
  usePendingCommand: () => ({ isPending: () => false, track }),
}))

import { useGlobalScheduledTasks } from './useGlobalScheduledTasks'

function mountUseGlobalScheduledTasks() {
  let api!: ReturnType<typeof useGlobalScheduledTasks>
  const wrapper = mount({
    setup() {
      api = useGlobalScheduledTasks()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

const taskA = {
  id: 'task-a', name: 'A task', host_name: 'zeta-host', module: 'apt', action: 'update',
  target: '', cron_expression: '0 3 * * *', enabled: true, payload: '',
}
const taskB = {
  id: 'task-b', name: 'B task', host_name: 'Alpha-host', module: 'docker', action: 'restart',
  target: '', cron_expression: '0 4 * * *', enabled: true, payload: '',
}

describe('useGlobalScheduledTasks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    getHosts.mockResolvedValue({ data: [] })
    getAllScheduledTasks.mockResolvedValue({ data: [taskA, taskB] })
    track.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('sorts the distinct host list with locale-aware comparison, not default lexicographic sort', async () => {
    // Plain Array.sort() (pre-PR) would sort "Alpha-host"/"zeta-host" the
    // same way, but localeCompare is what actually needs covering here —
    // use a case pair where the two differ (default sort is ASCII-cased:
    // uppercase < lowercase, putting "Alpha-host" before "zeta-host" either
    // way) so a mismatched sort function would show up as different output
    // in a genuinely case-mixed set.
    getAllScheduledTasks.mockResolvedValue({
      data: [
        { ...taskA, host_name: 'échalote' },
        { ...taskB, host_name: 'Zebra' },
      ],
    })

    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()

    // localeCompare places accented "échalote" before "Zebra" per French
    // collation — a plain sort() would instead sort by raw code point,
    // putting the uppercase-starting "Zebra" first.
    expect(api.hostList.value).toEqual(['échalote', 'Zebra'])
  })

  it('runs a task, tracks the dispatched command to completion, then reloads twice', async () => {
    runScheduledTask.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    getAllScheduledTasks
      .mockResolvedValueOnce({ data: [taskA, taskB] })
      .mockResolvedValueOnce({ data: [taskA, taskB] })
      .mockResolvedValueOnce({ data: [{ ...taskA, last_run_status: 'completed' }, taskB] })

    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()

    await api.runNow(taskA as never)
    await flushPromises()

    expect(runScheduledTask).toHaveBeenCalledWith('task-a')
    expect(track).toHaveBeenCalledWith('cmd-1')
    // initial load + pre-track reload + post-track reload
    expect(getAllScheduledTasks).toHaveBeenCalledTimes(3)
    expect(api.runningId.value).toBeNull()
  })

  it('surfaces a run failure without calling pendingCommand.track', async () => {
    runScheduledTask.mockRejectedValue(new Error('dispatch failed'))

    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()

    await api.runNow(taskA as never)
    await flushPromises()

    expect(track).not.toHaveBeenCalled()
    expect(api.error.value).toBe('dispatch failed')
    expect(api.runningId.value).toBeNull()
  })

  it('shows the translated delete-confirmation dialog and skips the API call when declined', async () => {
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.confirmDelete(taskA as never)
    expect(dialog.title.value).toBe('Supprimer la tâche')
    expect(dialog.message.value).toBe('Supprimer « A task » sur zeta-host ?')
    dialog.onCancel()
    await p
    expect(deleteScheduledTask).not.toHaveBeenCalled()
  })

  it('deletes a task on confirmation and reports a translated error on failure', async () => {
    deleteScheduledTask.mockRejectedValue(new Error(''))
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.confirmDelete(taskA as never)
    dialog.onConfirm()
    await p
    expect(api.error.value).toBe('Erreur lors de la suppression')
  })

  it('shows the translated enable/disable confirmation messages depending on current state', async () => {
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    const dialog = useConfirmDialog()

    const p1 = api.toggleTask(taskA as never) // enabled -> disabling
    expect(dialog.title.value).toBe('Désactiver la tâche')
    expect(dialog.message.value).toBe('Voulez-vous désactiver « A task » sur zeta-host ?')
    dialog.onCancel()
    await p1

    const p2 = api.toggleTask({ ...taskA, enabled: false } as never) // disabled -> enabling
    expect(dialog.title.value).toBe('Activer la tâche')
    expect(dialog.message.value).toBe('Voulez-vous activer « A task » sur zeta-host ?')
    dialog.onCancel()
    await p2
  })

  it('surfaces a translated error when creating a task fails', async () => {
    createScheduledTask.mockRejectedValue(new Error(''))
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    api.createForm.value.host_id = 'h1'
    await api.saveCreate()
    expect(api.createError.value).toBe('Erreur lors de la création')
  })

  it('surfaces a translated error when loading execution history fails', async () => {
    getScheduledTaskExecutions.mockRejectedValue(new Error(''))
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    await api.openHistory(taskA as never)
    expect(api.historyError.value).toBe('Erreur de chargement')
  })

  it('reports a pluralized success toast after a bulk action, and a partial-failure toast on mixed results', async () => {
    updateScheduledTask.mockResolvedValueOnce({}).mockRejectedValueOnce(new Error(''))
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.handleBulkEnable([taskA, taskB] as never)
    dialog.onConfirm()
    await p
    expect(updateScheduledTask).toHaveBeenCalledTimes(2)
  })

  it('translates confirmation dialogs to English when the locale is switched', async () => {
    setLocale('en')
    const { api } = mountUseGlobalScheduledTasks()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.confirmDelete(taskA as never)
    expect(dialog.title.value).toBe('Delete the task')
    dialog.onCancel()
    await p
  })
})
