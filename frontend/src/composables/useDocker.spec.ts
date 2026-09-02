import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import type { DockerContainer } from '../types/docker'

const { wsMessageHandler } = vi.hoisted(() => ({
  wsMessageHandler: { current: null as ((payload: unknown) => void) | null },
}))

vi.mock('./useWebSocket', () => ({
  useWebSocket: (_url: string, onMessage: (payload: unknown) => void) => {
    wsMessageHandler.current = onMessage
    return {
      wsStatus: { value: 'connected' },
      wsError: { value: '' },
      retryCount: { value: 0 },
      dataStaleAlert: { value: false },
      reconnect: vi.fn(),
    }
  },
}))

vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({
    openCommandStream: vi.fn(),
    closeStream: vi.fn(),
  }),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))
vi.mock('./usePendingCommand', () => ({
  usePendingCommand: () => ({ isPending: () => false, track }),
}))

const { sendDockerCommand } = vi.hoisted(() => ({ sendDockerCommand: vi.fn() }))
vi.mock('../api', () => ({
  default: { sendDockerCommand },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || 'Erreur réseau.',
}))

const { addToast } = vi.hoisted(() => ({ addToast: vi.fn() }))
vi.mock('./useGlobalToast', () => ({ addToast }))

import { useDocker } from './useDocker'
import { useConfirmDialog } from './useConfirmDialog'

const CONTAINER = { id: 'c1', name: 'web', host_id: 'h1', hostname: 'host-1', state: 'running', image: 'nginx' } as unknown as DockerContainer

function mountUseDocker() {
  let api!: ReturnType<typeof useDocker>
  const wrapper = mount({
    setup() {
      api = useDocker()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
  vi.clearAllMocks()
  wsMessageHandler.current = null
})

describe('useDocker — handleContainerAction', () => {
  it('starts a container without a confirmation dialog', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    await api.handleContainerAction({ hostId: 'h1', name: 'web', action: 'start' })

    expect(dialog.isOpen.value).toBe(false)
    expect(sendDockerCommand).toHaveBeenCalledWith('h1', 'web', 'start')
  })

  it('asks for confirmation before stopping a container, with a French title/message', async () => {
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    const promise = api.handleContainerAction({ hostId: 'h1', name: 'web', action: 'stop' })
    expect(dialog.isOpen.value).toBe(true)
    expect(dialog.title.value).toBe('Arrêter le conteneur')
    expect(dialog.message.value).toBe('Confirmer : stop du conteneur « web » ?')
    dialog.onCancel()
    await promise

    expect(sendDockerCommand).not.toHaveBeenCalled()
  })

  it('sends the command once the stop is confirmed', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    const promise = api.handleContainerAction({ hostId: 'h1', name: 'web', action: 'stop' })
    dialog.onConfirm()
    await promise

    expect(sendDockerCommand).toHaveBeenCalledWith('h1', 'web', 'stop')
  })

  it('shows a translated error toast and reverts the optimistic state on failure', async () => {
    sendDockerCommand.mockRejectedValue({ response: { data: { error: 'boom' } } })
    const { api } = mountUseDocker()
    api.containers.value = [CONTAINER]

    await api.handleContainerAction({ hostId: 'h1', name: 'web', action: 'start' })

    expect(addToast).toHaveBeenCalledWith('boom', 'error', 6000)
    expect(api.containers.value[0].state).toBe('running')
  })

  it('is a no-op while an action is already pending for that container', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    const { api } = mountUseDocker()
    api.dockerActionLoading.value = { web: 'start' }

    await api.handleContainerAction({ hostId: 'h1', name: 'web', action: 'start' })

    expect(sendDockerCommand).not.toHaveBeenCalled()
  })
})

describe('useDocker — handleBulkContainerAction', () => {
  it('does nothing for an empty selection', async () => {
    const { api } = mountUseDocker()
    await api.handleBulkContainerAction([], 'start')
    expect(sendDockerCommand).not.toHaveBeenCalled()
  })

  it('confirms with the pluralized French verb/count, then dispatches every container', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    const promise = api.handleBulkContainerAction(
      [CONTAINER, { ...CONTAINER, id: 'c2', name: 'db' }],
      'stop'
    )
    expect(dialog.title.value).toBe('Arrêter sur 2 éléments ?')
    expect(dialog.message.value).toContain('Arrêter 2 conteneurs')
    dialog.onConfirm()
    await promise

    expect(sendDockerCommand).toHaveBeenCalledTimes(2)
    expect(addToast).toHaveBeenCalledWith('2 commandes envoyées', 'success')
  })

  it('reports a mixed success/failure summary', async () => {
    sendDockerCommand
      .mockResolvedValueOnce({ data: { command_id: 'cmd1' } })
      .mockRejectedValueOnce(new Error('fail'))
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    const promise = api.handleBulkContainerAction(
      [CONTAINER, { ...CONTAINER, id: 'c2', name: 'db' }],
      'restart'
    )
    dialog.onConfirm()
    await promise

    expect(addToast).toHaveBeenCalledWith('1 envoyée(s), 1 échec(s)', 'warning', 6000)
  })
})

describe('useDocker — handleComposeAction', () => {
  it('starts a compose project without confirmation', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    await api.handleComposeAction({ hostId: 'h1', name: 'stack', action: 'compose_up' })

    expect(dialog.isOpen.value).toBe(false)
    expect(sendDockerCommand).toHaveBeenCalledWith('h1', 'stack', 'compose_up', undefined)
  })

  it('confirms before compose_down, with a French title/message', async () => {
    const { api } = mountUseDocker()
    const dialog = useConfirmDialog()

    const promise = api.handleComposeAction({ hostId: 'h1', name: 'stack', action: 'compose_down' })
    expect(dialog.title.value).toBe('Arrêter le projet')
    expect(dialog.message.value).toBe('Confirmer : down du projet « stack » ?')
    dialog.onCancel()
    await promise

    expect(sendDockerCommand).not.toHaveBeenCalled()
  })

  it('shows a translated error toast on failure', async () => {
    sendDockerCommand.mockRejectedValue({ response: { data: {} } })
    const { api } = mountUseDocker()

    await api.handleComposeAction({ hostId: 'h1', name: 'stack', action: 'compose_logs', workingDir: '/opt/stack' })

    expect(addToast).toHaveBeenCalledWith('Erreur Docker', 'error', 6000)
  })
})

describe('useDocker — WS snapshot', () => {
  it('populates containers/composeProjects/versionComparisons from a docker snapshot', () => {
    const { api } = mountUseDocker()
    wsMessageHandler.current?.({
      type: 'docker',
      containers: [CONTAINER],
      compose_projects: [{ id: 'p1', name: 'stack', host_id: 'h1' }],
      version_comparisons: [],
    })
    expect(api.containers.value).toEqual([CONTAINER])
    expect(api.runningCount.value).toBe(1)
  })

  it('ignores a non-docker snapshot', () => {
    const { api } = mountUseDocker()
    api.containers.value = [CONTAINER]
    wsMessageHandler.current?.({ type: 'other' })
    expect(api.containers.value).toEqual([CONTAINER])
  })
})
