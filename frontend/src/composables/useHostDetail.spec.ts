import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const {
  getHostComplete, getHostProxmoxLink, getUUStatus, getUURuns, getNotifications, updateUU,
  openCommandStream, closeStream, collectCommandOutput,
} = vi.hoisted(() => ({
  getHostComplete: vi.fn(),
  getHostProxmoxLink: vi.fn(),
  getUUStatus: vi.fn(),
  getUURuns: vi.fn(),
  getNotifications: vi.fn(),
  updateUU: vi.fn(),
  openCommandStream: vi.fn(),
  closeStream: vi.fn(),
  collectCommandOutput: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'h1' }, query: {} }),
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}))

vi.mock('../api', () => ({
  default: { getHostComplete, getHostProxmoxLink, getUUStatus, getUURuns, getNotifications, updateUU },
  getApiErrorMessage: (e: unknown) => String(e),
}))

vi.mock('./useWebSocket', () => ({
  useWebSocket: () => ({
    wsStatus: { value: 'connected' },
    wsError: { value: '' },
    retryCount: { value: 0 },
    reconnect: vi.fn(),
  }),
}))

// Real WebSockets aren't available/meaningful under happy-dom — capture the
// stream open call instead, same pattern as useBackup.spec/HostBackupTab.spec.
// collectCommandOutput backs usePendingCommand.track(), used standalone
// (no console) by handleUUConfigure — see that test below.
vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({ openCommandStream, closeStream, collectCommandOutput }),
}))

import { useHostDetail } from './useHostDetail'

function mockEmptyBackend() {
  getHostComplete.mockResolvedValue({ data: {} })
  getHostProxmoxLink.mockResolvedValue({ data: null })
  getUUStatus.mockResolvedValue({ data: {} })
  getUURuns.mockResolvedValue({ data: [] })
  getNotifications.mockResolvedValue({ data: { notifications: [] } })
}

function mountUseHostDetail() {
  let api!: ReturnType<typeof useHostDetail>
  const wrapper = mount({
    setup() {
      api = useHostDetail()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  mockEmptyBackend()
})

describe('useHostDetail — resume live command tracking on mount', () => {
  it('resumes watching an already-running command found in command history', async () => {
    getHostComplete.mockResolvedValue({
      data: {
        host: { id: 'h1', hostname: 'web-01' },
        command_history: [
          { id: 'cmd-resume', module: 'apt', action: 'dist-upgrade', status: 'running', output: '', created_at: new Date().toISOString() },
        ],
      },
    })

    const { api } = mountUseHostDetail()
    await flushPromises()

    expect(openCommandStream).toHaveBeenCalledWith('cmd-resume', expect.anything())
    expect(api.showConsole.value).toBe(true)
    expect(api.liveCommand.value?.id).toBe('cmd-resume')
  })

  it('does not resume-subscribe when the latest command is already terminal', async () => {
    getHostComplete.mockResolvedValue({
      data: {
        host: { id: 'h1', hostname: 'web-01' },
        command_history: [
          { id: 'cmd-done', module: 'apt', action: 'update', status: 'completed', output: 'ok', created_at: new Date().toISOString() },
        ],
      },
    })

    const { api } = mountUseHostDetail()
    await flushPromises()

    expect(openCommandStream).not.toHaveBeenCalled()
    expect(api.showConsole.value).toBe(false)
  })
})

describe('useHostDetail — handleUUConfigure spinner and form sync', () => {
  it('keeps uuLoading set and does not reload (revert) the form until both dispatched commands actually complete', async () => {
    getUUStatus
      .mockResolvedValueOnce({ data: { enabled: false, config: { security_only: true, auto_reboot: false, auto_reboot_time: '02:00', remove_unused: false } } })
      .mockResolvedValueOnce({ data: { enabled: true, config: { security_only: true, auto_reboot: false, auto_reboot_time: '02:00', remove_unused: false } } })
    updateUU.mockResolvedValue({ data: { command_ids: ['cmd-a', 'cmd-b'], status: 'pending' } })

    // Both dispatched commands (configure_uu, toggle_uu) are tracked in
    // parallel (Promise.all) — capture a resolver per call, not just the last.
    const resolveTracks: Array<() => void> = []
    collectCommandOutput.mockImplementation(
      () => new Promise<string>((resolve) => { resolveTracks.push(() => resolve('')) })
    )

    const { api } = mountUseHostDetail()
    await flushPromises()
    expect(api.uuForm.value?.enabled).toBe(false)

    // Simulates the checkbox's v-model="uuForm.enabled" mutating the same
    // object HostAptTab.vue then emits back on save (@click="$emit('uu-configure', uuForm)").
    api.uuForm.value!.enabled = true
    const configurePromise = api.handleUUConfigure(api.uuForm.value as unknown as Record<string, unknown>)
    await flushPromises()

    // The dispatch HTTP round-trip has resolved, but the agent hasn't run the
    // commands yet (collectCommandOutput's promise is still pending) — the
    // spinner must still be up and the form must not have been reloaded yet.
    expect(api.uuLoading.value).toBe('configure')
    expect(getUUStatus).toHaveBeenCalledTimes(1) // only the initial mount load so far
    expect(api.uuForm.value?.enabled).toBe(true) // the user's own edit, untouched

    resolveTracks.forEach((resolve) => resolve())
    await configurePromise
    await flushPromises()

    expect(api.uuLoading.value).toBe('')
    expect(getUUStatus).toHaveBeenCalledTimes(2) // the post-completion reload
    expect(api.uuForm.value?.enabled).toBe(true) // reloaded from the now-updated server state, not reverted
  })
})

describe('useHostDetail — effectiveMetrics CPU core count', () => {
  it('uses the Proxmox link\'s cpu_alloc, not the stale agent cpu_cores, when metrics_source=proxmox', async () => {
    getHostComplete.mockResolvedValue({
      data: {
        host: { id: 'h1', hostname: 'web-01' },
        metrics: { cpu_cores: 2, cpu_usage_percent: 10 },
      },
    })
    getHostProxmoxLink.mockResolvedValue({
      data: { status: 'confirmed', metrics_source: 'proxmox', cpu_alloc: 4, cpu_usage: 0.3, mem_alloc: 4096, mem_usage: 2048 },
    })

    const { api } = mountUseHostDetail()
    await flushPromises()

    expect(api.effectiveMetrics.value?.cpu_cores).toBe(4)
  })

  it('keeps the agent-reported cpu_cores when metrics_source=agent', async () => {
    getHostComplete.mockResolvedValue({
      data: {
        host: { id: 'h1', hostname: 'web-01' },
        metrics: { cpu_cores: 2, cpu_usage_percent: 10 },
      },
    })
    getHostProxmoxLink.mockResolvedValue({
      data: { status: 'confirmed', metrics_source: 'agent', cpu_alloc: 4, cpu_usage: 0.3, mem_alloc: 4096, mem_usage: 2048 },
    })

    const { api } = mountUseHostDetail()
    await flushPromises()

    expect(api.effectiveMetrics.value?.cpu_cores).toBe(2)
  })
})
