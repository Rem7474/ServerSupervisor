import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { getScheduledTasks, runScheduledTask, createScheduledTask } = vi.hoisted(() => ({
  getScheduledTasks: vi.fn(),
  runScheduledTask: vi.fn(),
  createScheduledTask: vi.fn(),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))

vi.mock('../../api', () => ({
  default: { getScheduledTasks, runScheduledTask, createScheduledTask },
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
    // DispatchStepEditor (rendered inside the create/edit modal) reads the
    // hosts store via useHostsStore() — needs an active Pinia even though
    // this test never exercises host-scoping directly.
    setActivePinia(createPinia())
    getScheduledTasks.mockResolvedValue({ data: [baseTask] })
    track.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.clearAllMocks()
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
})
