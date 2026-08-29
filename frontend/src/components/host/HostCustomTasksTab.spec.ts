import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { getHostCustomTasks, runCustomTask } = vi.hoisted(() => ({
  getHostCustomTasks: vi.fn(),
  runCustomTask: vi.fn(),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))

vi.mock('../../api', () => ({
  default: { getHostCustomTasks, runCustomTask },
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

import HostCustomTasksTab from './HostCustomTasksTab.vue'

const baseTask = { id: 'task-1', name: 'Renew certs' }

describe('HostCustomTasksTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    track.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows a loading skeleton, then an empty state when there are no tasks', async () => {
    getHostCustomTasks.mockResolvedValue({ data: [] })

    const wrapper = mount(HostCustomTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    expect(wrapper.findComponent({ name: 'LoadingSkeleton' }).exists()).toBe(true)

    await flushPromises()

    expect(getHostCustomTasks).toHaveBeenCalledWith('host-1')
    expect(wrapper.text()).toContain('Aucune tâche personnalisée')
    expect(wrapper.emitted('tasks-count')?.at(-1)).toEqual([0])
  })

  it('does not load until the tab becomes active', async () => {
    getHostCustomTasks.mockResolvedValue({ data: [baseTask] })

    const wrapper = mount(HostCustomTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: false },
    })
    await flushPromises()
    expect(getHostCustomTasks).not.toHaveBeenCalled()

    await wrapper.setProps({ active: true })
    await flushPromises()
    expect(getHostCustomTasks).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Renew certs')
    expect(wrapper.emitted('tasks-count')?.at(-1)).toEqual([1])
  })

  it('runs a task, shows the toast, tracks it, and emits open-command/history-changed', async () => {
    getHostCustomTasks.mockResolvedValue({ data: [baseTask] })
    runCustomTask.mockResolvedValue({ data: { command_id: 'cmd-7' } })

    const wrapper = mount(HostCustomTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    const runButton = wrapper.find('button[aria-label="Exécuter la tâche maintenant"]')
    expect(runButton.exists()).toBe(true)
    await runButton.trigger('click')
    await flushPromises()

    expect(runCustomTask).toHaveBeenCalledWith('host-1', 'task-1')
    expect(track).toHaveBeenCalledWith('cmd-7')
    expect(wrapper.emitted('open-command')?.[0]?.[0]).toMatchObject({
      id: 'cmd-7',
      module: 'custom',
      target: 'task-1',
      status: 'pending',
    })
    expect(wrapper.emitted('history-changed')).toBeTruthy()
    expect(document.body.textContent).toContain('Renew certs')
    expect(document.body.textContent).toContain('cmd-7')
  })

  it('hides the run button when canRunApt is false', async () => {
    getHostCustomTasks.mockResolvedValue({ data: [baseTask] })

    const wrapper = mount(HostCustomTasksTab, {
      props: { hostId: 'host-1', canRunApt: false, active: true },
    })
    await flushPromises()

    expect(wrapper.find('button[aria-label="Exécuter la tâche maintenant"]').exists()).toBe(false)
  })

  it('surfaces a run failure without tracking a command', async () => {
    getHostCustomTasks.mockResolvedValue({ data: [baseTask] })
    runCustomTask.mockRejectedValue(new Error('dispatch failed'))

    const wrapper = mount(HostCustomTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    await wrapper.find('button[aria-label="Exécuter la tâche maintenant"]').trigger('click')
    await flushPromises()

    expect(track).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('dispatch failed')
  })

  it('surfaces a load failure as an alert', async () => {
    getHostCustomTasks.mockRejectedValue(new Error('boom'))

    const wrapper = mount(HostCustomTasksTab, {
      props: { hostId: 'host-1', canRunApt: true, active: true },
    })
    await flushPromises()

    expect(wrapper.find('.alert-danger').text()).toBe('boom')
  })
})
