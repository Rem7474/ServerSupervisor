import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { getBackupStatus, getBackupRuns, getBackupProfiles, getBackupGroups, runBackup, openCommandStream, closeStream, getLastStreamOptions } = vi.hoisted(() => {
  let lastStreamOptions: {
    onChunk?: (p: { chunk: string }) => void
    onStatus?: (p: { status: string }) => void
  } | null = null
  return {
    getBackupStatus: vi.fn(),
    getBackupRuns: vi.fn(),
    getBackupProfiles: vi.fn(),
    getBackupGroups: vi.fn(),
    runBackup: vi.fn(),
    closeStream: vi.fn(),
    openCommandStream: vi.fn((_commandId: string, options: typeof lastStreamOptions) => {
      lastStreamOptions = options
    }),
    getLastStreamOptions: () => lastStreamOptions,
  }
})

vi.mock('../../api', () => ({
  default: { getBackupStatus, getBackupRuns, getBackupProfiles, getBackupGroups, runBackup },
  getApiErrorMessage: (e: unknown) => String(e),
}))

// Real WebSockets aren't available/meaningful under happy-dom — capture the
// stream callbacks so tests can drive them directly instead.
vi.mock('../../composables/useCommandStream', () => ({
  useCommandStream: () => ({ openCommandStream, closeStream }),
}))

import HostBackupTab from './HostBackupTab.vue'

function mockEmptyBackend() {
  getBackupStatus.mockResolvedValue({ data: {} })
  getBackupRuns.mockResolvedValue({ data: { runs: [] } })
  getBackupProfiles.mockResolvedValue({ data: { profiles: [] } })
  getBackupGroups.mockResolvedValue({ data: { groups: [] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  getBackupProfiles.mockResolvedValue({ data: { profiles: [] } })
  getBackupGroups.mockResolvedValue({ data: { groups: [] } })
})

describe('HostBackupTab', () => {
  it('shows an empty state when there is no backup data', async () => {
    mockEmptyBackend()
    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Aucune donnée de sauvegarde')
    expect(() => wrapper.unmount()).not.toThrow()
  })

  it('renders the latest run summary and history table when data is present', async () => {
    getBackupStatus.mockResolvedValue({
      data: {
        latest_run: {
          id: 'run-1', host_id: 'h1', status: 'ok', profile: 'files',
          started_at: new Date().toISOString(), finished_at: new Date().toISOString(),
          duration_sec: 125, snapshot_id: 'abc123',
        },
      },
    })
    getBackupRuns.mockResolvedValue({
      data: {
        runs: [
          { id: 'run-1', host_id: 'h1', status: 'ok', started_at: new Date().toISOString(), triggered_by: 'alice' },
          { id: 'run-0', host_id: 'h1', status: 'error', started_at: new Date().toISOString(), triggered_by: 'scheduled_task', error_message: 'repo locked' },
        ],
      },
    })

    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('abc123')
    expect(wrapper.findAll('tbody tr').length).toBe(2)
    expect(wrapper.text()).toContain('Planifié')
    expect(wrapper.text()).toContain('repo locked')
  })

  it('walks idle -> running -> completed and reloads history', async () => {
    mockEmptyBackend()
    runBackup.mockResolvedValue({ data: { command_id: 'cmd-1' } })

    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: true } })
    await flushPromises()

    const runButton = wrapper.findAll('button').find((b) => b.text().includes('Lancer un backup'))
    expect(runButton).toBeTruthy()
    await runButton!.trigger('click')
    await flushPromises()

    expect(runBackup).toHaveBeenCalledWith('h1', undefined)
    expect(openCommandStream).toHaveBeenCalledWith('cmd-1', expect.anything())
    expect(wrapper.text()).toContain('Backup en cours')

    // Live progress chunk (structured JSON, as streamed by the agent).
    getLastStreamOptions()?.onChunk?.({
      chunk: JSON.stringify({ phase: 'backup', percent_done: 42, files_done: 5, files_total: 10 }),
    })
    await flushPromises()
    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('10')

    getLastStreamOptions()?.onStatus?.({ status: 'completed' })
    await flushPromises()
    expect(wrapper.text()).toContain('Backup terminé')
  })

  it('shows a failed state on a failed run', async () => {
    mockEmptyBackend()
    runBackup.mockResolvedValue({ data: { command_id: 'cmd-2' } })

    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: true } })
    await flushPromises()
    const runButton = wrapper.findAll('button').find((b) => b.text().includes('Lancer un backup'))
    await runButton!.trigger('click')
    await flushPromises()

    getLastStreamOptions()?.onStatus?.({ status: 'failed' })
    await flushPromises()
    expect(wrapper.text()).toContain('Backup en échec')
  })

  it('hides the run button when the user cannot manage the host', async () => {
    mockEmptyBackend()
    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: false } })
    await flushPromises()
    const runButton = wrapper.findAll('button').find((b) => b.text().includes('Lancer un backup'))
    expect(runButton).toBeFalsy()
  })

  it('unmounts cleanly and stops watching the live stream', async () => {
    mockEmptyBackend()
    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: true } })
    await flushPromises()
    wrapper.unmount()
    expect(closeStream).toHaveBeenCalled()
  })

  it('offers discovered profiles and groups as a select, grouped, and runs a chosen group', async () => {
    mockEmptyBackend()
    getBackupProfiles.mockResolvedValue({ data: { profiles: ['files', 'db'] } })
    getBackupGroups.mockResolvedValue({ data: { groups: ['full-backup'] } })
    runBackup.mockResolvedValue({ data: { command_id: 'cmd-1' } })

    const wrapper = mount(HostBackupTab, { props: { hostId: 'h1', canRun: true } })
    await flushPromises()

    const select = wrapper.find('select')
    expect(select.exists()).toBe(true)
    const optgroups = select.findAll('optgroup')
    expect(optgroups.map((g) => g.attributes('label'))).toEqual(['Profils', 'Groupes'])

    await select.setValue('full-backup')
    const runButton = wrapper.findAll('button').find((b) => b.text().includes('Lancer un backup'))
    await runButton!.trigger('click')
    await flushPromises()

    expect(runBackup).toHaveBeenCalledWith('h1', 'full-backup')
  })
})
