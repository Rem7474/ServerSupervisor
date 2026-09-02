import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'

const { getScheduledTasks, runScheduledTask, createScheduledTask, updateScheduledTask, deleteScheduledTask } = vi.hoisted(() => ({
  getScheduledTasks: vi.fn(),
  runScheduledTask: vi.fn(),
  createScheduledTask: vi.fn(),
  updateScheduledTask: vi.fn(),
  deleteScheduledTask: vi.fn(),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))

vi.mock('../../api', () => ({
  default: {
    getScheduledTasks, runScheduledTask, createScheduledTask, updateScheduledTask, deleteScheduledTask,
    getBackupProfiles: vi.fn().mockResolvedValue({ data: { profiles: [] } }),
    getBackupGroups: vi.fn().mockResolvedValue({ data: { groups: [] } }),
  },
}))

vi.mock('../../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

// runTaskNow awaits pendingCommand.track(), which itself opens a real WS
// stream (useCommandStream) — irrelevant to what this component's own logic
// does with the result, so it's stubbed to resolve immediately.
vi.mock('../../composables/usePendingCommand', () => ({
  usePendingCommand: () => ({ isPending: () => false, track }),
}))

import { useConfirmDialog } from '../../composables/useConfirmDialog'
import HostTasksTab from './HostTasksTab.vue'

const baseTask = {
  id: 'task-1',
  name: 'Backup nightly',
  module: 'restic',
  action: 'run_backup',
  target: 'files',
  cron_expression: '0 3 * * *',
  enabled: true,
  next_run_at: null,
  last_run_status: null,
}

describe('HostTasksTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    // DispatchStepEditor (rendered inside the create/edit modal) reads the
    // hosts store via useHostsStore() — needs an active Pinia even though
    // this test never exercises host-scoping directly.
    setActivePinia(createPinia())
    getScheduledTasks.mockResolvedValue({ data: [baseTask] })
    track.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.clearAllMocks()
    // The create/edit modal and toast render via <Teleport to="body">, and
    // successive test mounts here are never explicitly unmounted — without
    // this, a modal left open by one test (e.g. the validation-failure test,
    // which never closes it) leaks into the next test's document.body query.
    document.body.innerHTML = ''
  })

  it('runs a task, tracks the dispatched command to completion, then reloads the task list twice', async () => {
    runScheduledTask.mockResolvedValue({ data: { command_id: 'cmd-42' } })
    // A second loadTasks() call (post-track) should reflect the task's
    // updated last_run_status — simulate that by changing the mock's
    // second-call response.
    getScheduledTasks
      .mockResolvedValueOnce({ data: [baseTask] })
      .mockResolvedValueOnce({ data: [baseTask] })
      .mockResolvedValueOnce({ data: [{ ...baseTask, last_run_status: 'completed' }] })

    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    const runButton = wrapper.find('button[aria-label="Exécuter la tâche maintenant"]')
    expect(runButton.exists()).toBe(true)
    await runButton.trigger('click')
    await flushPromises()

    expect(runScheduledTask).toHaveBeenCalledWith('task-1')
    // track() is awaited between the two loadTasks() calls added for this PR.
    expect(track).toHaveBeenCalledWith('cmd-42')
    expect(getScheduledTasks).toHaveBeenCalledTimes(3) // initial + pre-track + post-track
    expect(wrapper.emitted('open-command')?.[0]?.[0]).toMatchObject({ id: 'cmd-42', module: 'restic' })
    expect(wrapper.emitted('history-changed')).toBeTruthy()
    // getExecutionStateClass/commandStatusLabel render the French label for
    // a 'completed' status ("Terminé"), confirming the post-track reload's
    // fresh data actually made it into the table.
    expect(wrapper.text()).toContain('Terminé')
  })

  it('surfaces a run failure without ever calling pendingCommand.track', async () => {
    runScheduledTask.mockRejectedValue(new Error('dispatch failed'))

    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    await wrapper.find('button[aria-label="Exécuter la tâche maintenant"]').trigger('click')
    await flushPromises()

    expect(track).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('dispatch failed')
  })

  it('blocks saving an empty task form client-side without calling the API', async () => {
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    const newTaskButton = wrapper.findAll('button').find((b) => b.text().includes('Nouvelle tâche'))
    expect(newTaskButton).toBeTruthy()
    await newTaskButton?.trigger('click')
    await flushPromises()

    // The create/edit modal renders via <Teleport to="body">, outside the
    // wrapper's own root subtree — query the real document for it instead
    // of wrapper.find().
    const createButton = Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.includes('Créer'))
    expect(createButton).toBeTruthy()
    createButton?.dispatchEvent(new Event('click', { bubbles: true }))
    await flushPromises()

    // Name (and, per this PR's change, action for a non-custom module) are
    // required client-side — saveTask() must short-circuit before the API call.
    expect(createScheduledTask).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Nom et action sont obligatoires.')
  })

  it('shows the empty state and a "Nouvelle tâche" CTA when there are no tasks', async () => {
    getScheduledTasks.mockResolvedValue({ data: [] })
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucune tâche planifiée')
  })

  it('toggles a task\'s enabled state via the checkbox', async () => {
    updateScheduledTask.mockResolvedValue({ data: {} })
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    await wrapper.find('input[type="checkbox"]').trigger('change')
    await flushPromises()

    expect(updateScheduledTask).toHaveBeenCalledWith('task-1', expect.objectContaining({ enabled: false }))
  })

  it('surfaces a toggle failure', async () => {
    updateScheduledTask.mockRejectedValue(new Error('toggle failed'))
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    await wrapper.find('input[type="checkbox"]').trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('toggle failed')
  })

  it('opens the edit modal pre-filled with the task, and saves it', async () => {
    updateScheduledTask.mockResolvedValue({ data: {} })
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    await wrapper.find('button[aria-label="Modifier la tâche"]').trigger('click')
    await flushPromises()

    const nameInput = document.querySelector('input.form-control:not(.form-control-sm)') as HTMLInputElement
    expect(nameInput.value).toBe('Backup nightly')
    expect(document.body.textContent).toContain('Modifier la tâche')

    const saveButton = Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Enregistrer')
    saveButton?.dispatchEvent(new Event('click', { bubbles: true }))
    await flushPromises()

    expect(updateScheduledTask).toHaveBeenCalledWith('task-1', expect.objectContaining({ name: 'Backup nightly' }))
  })

  it('deletes a task only after the confirm dialog is accepted', async () => {
    deleteScheduledTask.mockResolvedValue({ data: {} })
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('button[aria-label="Supprimer la tâche"]').trigger('click')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.title.value).toBe('Supprimer la tâche')
    expect(dialog.message.value).toContain('Backup nightly')
    expect(deleteScheduledTask).not.toHaveBeenCalled()

    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(deleteScheduledTask).toHaveBeenCalledWith('task-1')
  })

  it('does not delete when the confirm dialog is cancelled', async () => {
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('button[aria-label="Supprimer la tâche"]').trigger('click')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    dialog.onCancel()
    await clickPromise
    await flushPromises()

    expect(deleteScheduledTask).not.toHaveBeenCalled()
  })

  it('toggles sort direction on the Nom column', async () => {
    getScheduledTasks.mockResolvedValue({
      data: [
        { ...baseTask, id: 'a', name: 'Zebra task' },
        { ...baseTask, id: 'b', name: 'Alpha task' },
      ],
    })
    const wrapper = mount(HostTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    let rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('Alpha task')

    await wrapper.find('th button, th [role="button"]').trigger('click')
    await flushPromises()
    rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('Zebra task')
  })
})
