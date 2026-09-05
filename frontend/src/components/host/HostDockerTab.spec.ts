import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { useGlobalToast } from '../../composables/useGlobalToast'
import HostDockerTab from './HostDockerTab.vue'

const { sendDockerCommand } = vi.hoisted(() => ({ sendDockerCommand: vi.fn() }))

vi.mock('../../api', () => ({
  default: { sendDockerCommand },
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

// Regression test: the host-detail Docker tab used to have no way to tell a
// compose-managed container from a standalone one, unlike the /docker global
// page (DockerContainersTab.vue) which already showed the compose project/
// service. Both now share DockerComposeBadge.vue.
describe('HostDockerTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    useGlobalToast().toasts.splice(0)
  })

  it('shows the version-comparison badges by status', () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        containers: [
          { id: 'c1', name: 'app-uptodate', image: 'app', image_tag: 'latest', state: 'running', status: 'Up' },
          { id: 'c2', name: 'app-update', image: 'app2', image_tag: 'latest', state: 'running', status: 'Up' },
          { id: 'c3', name: 'app-unknown', image: 'app3', image_tag: 'latest', state: 'running', status: 'Up' },
        ],
        versionComparisons: [
          { docker_image: 'app', image_tag: 'latest', status: 'up_to_date' },
          { docker_image: 'app2', image_tag: 'latest', status: 'update_available', latest_version: '2.0' },
          { docker_image: 'app3', image_tag: 'latest', status: 'unknown', last_error: 'registry unreachable' },
        ],
      },
    })
    expect(wrapper.text()).toContain('À jour')
    expect(wrapper.text()).toContain('Mise à jour disponible')
    expect(wrapper.text()).toContain('Version inconnue')
    const unknownBadge = wrapper.findAll('.badge').find((b) => b.text() === 'Version inconnue')
    expect(unknownBadge?.attributes('title')).toBe('registry unreachable')
  })

  it('shows an empty state when there are no containers', () => {
    const wrapper = mount(HostDockerTab, { props: { hostId: 'h1', containers: [] } })
    expect(wrapper.text()).toContain('Aucun conteneur Docker actif sur cet hôte.')
  })

  it('hides the actions column when canRun is false', () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        canRun: false,
        containers: [{ id: 'c1', name: 'redis', image: 'redis', state: 'running', status: 'Up' }],
      },
    })
    expect(wrapper.find('button[aria-label="Arrêter le conteneur"]').exists()).toBe(false)
  })

  it('starts a stopped container without a confirmation dialog', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        canRun: true,
        containers: [{ id: 'c1', name: 'redis', image: 'redis', state: 'exited', status: 'Exited' }],
      },
    })

    await wrapper.find('button[aria-label="Démarrer le conteneur"]').trigger('click')
    await flushPromises()

    expect(sendDockerCommand).toHaveBeenCalledWith('h1', 'redis', 'start')
    expect(wrapper.emitted('open-command')?.[0]?.[0]).toMatchObject({
      id: 'cmd-1',
      module: 'docker',
      action: 'start',
      target: 'redis',
      status: 'pending',
    })
    expect(wrapper.emitted('history-changed')).toBeTruthy()
  })

  it('stops a running container only after confirming the dialog', async () => {
    sendDockerCommand.mockResolvedValue({ data: { command_id: 'cmd-2' } })
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        canRun: true,
        containers: [{ id: 'c1', name: 'redis', image: 'redis', state: 'running', status: 'Up' }],
      },
    })

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('button[aria-label="Arrêter le conteneur"]').trigger('click')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.title.value).toBe('Arrêter le conteneur')
    expect(dialog.message.value).toBe('Confirmer : arrêter du conteneur « redis » ?')
    expect(sendDockerCommand).not.toHaveBeenCalled()

    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(sendDockerCommand).toHaveBeenCalledWith('h1', 'redis', 'stop')
  })

  it('does not dispatch when the stop confirmation is cancelled', async () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        canRun: true,
        containers: [{ id: 'c1', name: 'redis', image: 'redis', state: 'running', status: 'Up' }],
      },
    })

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('button[aria-label="Arrêter le conteneur"]').trigger('click')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    dialog.onCancel()
    await clickPromise
    await flushPromises()

    expect(sendDockerCommand).not.toHaveBeenCalled()
  })

  it('surfaces a dispatch failure as a toast and does not emit open-command', async () => {
    sendDockerCommand.mockRejectedValue(new Error('agent unreachable'))
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        canRun: true,
        containers: [{ id: 'c1', name: 'redis', image: 'redis', state: 'exited', status: 'Exited' }],
      },
    })

    await wrapper.find('button[aria-label="Démarrer le conteneur"]').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('open-command')).toBeFalsy()
    const { toasts } = useGlobalToast()
    expect(toasts[toasts.length - 1]).toMatchObject({ type: 'error', message: 'agent unreachable' })
  })
})

describe('HostDockerTab — compose column parity with the /docker page', () => {
  it('shows the compose project/service for a compose-managed container', () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        containers: [
          {
            id: 'c1', name: 'nextcloud-app', image: 'nextcloud', image_tag: 'latest', state: 'running', status: 'Up 2 days',
            labels: { 'com.docker.compose.project': 'nextcloud', 'com.docker.compose.service': 'app' },
          },
        ],
      },
    })
    expect(wrapper.text()).toContain('nextcloud')
    expect(wrapper.text()).toContain('app')
  })

  it('shows a dash for a standalone container', () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        containers: [
          { id: 'c2', name: 'redis', image: 'redis', image_tag: 'alpine', state: 'running', status: 'Up 2 days' },
        ],
      },
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('-')
  })
})
