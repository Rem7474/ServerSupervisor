import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { getAllScheduledTasks, runScheduledTask, getHosts } = vi.hoisted(() => ({
  getAllScheduledTasks: vi.fn(),
  runScheduledTask: vi.fn(),
  getHosts: vi.fn(),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))

vi.mock('../api', () => ({
  default: { getAllScheduledTasks, runScheduledTask, getHosts },
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
})
