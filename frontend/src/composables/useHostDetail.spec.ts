import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const {
  getHostComplete, getHostProxmoxLink, getUUStatus, getUURuns, getNotifications,
  openCommandStream, closeStream,
} = vi.hoisted(() => ({
  getHostComplete: vi.fn(),
  getHostProxmoxLink: vi.fn(),
  getUUStatus: vi.fn(),
  getUURuns: vi.fn(),
  getNotifications: vi.fn(),
  openCommandStream: vi.fn(),
  closeStream: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'h1' }, query: {} }),
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}))

vi.mock('../api', () => ({
  default: { getHostComplete, getHostProxmoxLink, getUUStatus, getUURuns, getNotifications },
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
vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({ openCommandStream, closeStream }),
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
