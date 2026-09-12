import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { runReleaseTracker } = vi.hoisted(() => ({
  runReleaseTracker: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { runReleaseTracker },
}))

import ComposeProjectsTab from './ComposeProjectsTab.vue'

const project = {
  id: 'p1', name: 'my-stack', hostname: 'web-01', host_id: 'h1',
  config_file: 'docker-compose.yml', working_dir: '/opt/my-stack',
  raw_config: 'services:\n  web:\n    image: nginx',
  services: ['web', 'db'],
}

const runningContainer = {
  id: 'c1', host_id: 'h1', state: 'running', image: 'nginx',
  labels: { 'com.docker.compose.project': 'my-stack' },
}

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
})

describe('ComposeProjectsTab', () => {
  it('shows a generic empty state when there are no projects at all', () => {
    const wrapper = mount(ComposeProjectsTab, { props: { composeProjects: [] } })
    expect(wrapper.text()).toContain('Aucun projet Compose trouvé')
  })

  it('shows a filter-specific empty state when a search excludes every project', async () => {
    const wrapper = mount(ComposeProjectsTab, { props: { composeProjects: [project] } })
    await wrapper.find('input[type="search"], input').setValue('nope')
    await new Promise((r) => setTimeout(r, 350))
    await flushPromises()
    expect(wrapper.text()).toContain('Aucun résultat pour ces filtres')
  })

  it('derives a running status from a matching container and shows the row', () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [project], containers: [runningContainer] },
    })
    const row = wrapper.find('tbody tr')
    expect(row.text()).toContain('my-stack')
    expect(row.text()).toContain('En cours')
  })

  it('falls back to stopped status when no container matches the project', () => {
    const wrapper = mount(ComposeProjectsTab, { props: { composeProjects: [project], containers: [] } })
    expect(wrapper.find('tbody tr').text()).toContain('Arrêté')
  })

  it('shows an update badge and trigger button when a tracked image has an update available', () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: {
        composeProjects: [project],
        containers: [runningContainer],
        versionComparisons: [{
          tracker_id: 't1', host_id: 'h1', docker_image: 'nginx',
          status: 'update_available', latest_version: '1.27.0',
        }],
        canRunDocker: true,
      },
    })
    expect(wrapper.text()).toContain('1 MAJ')
    expect(wrapper.find('[aria-label="Déclencher le tracker"]').exists()).toBe(true)
  })

  it('hides all lifecycle action buttons when canRunDocker is false, but keeps the config button', () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [project], containers: [runningContainer], canRunDocker: false },
    })
    expect(wrapper.find('[aria-label="Arrêter le projet"]').exists()).toBe(false)
    expect(wrapper.find('[title="Config"]').exists()).toBe(true)
  })

  it('emits compose-action with compose_up when starting a stopped project', async () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [project], containers: [], canRunDocker: true },
    })
    await wrapper.find('[aria-label="Démarrer le projet"]').trigger('click')
    const emitted = wrapper.emitted('compose-action')
    expect(emitted?.[0][0]).toMatchObject({ hostId: 'h1', name: 'my-stack', action: 'compose_up' })
  })

  it('emits compose-action with compose_down/compose_restart for a running project', async () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [project], containers: [runningContainer], canRunDocker: true },
    })
    await wrapper.find('[aria-label="Arrêter le projet"]').trigger('click')
    expect(wrapper.emitted('compose-action')?.[0][0]).toMatchObject({ action: 'compose_down' })

    await wrapper.find('[aria-label="Redémarrer le projet"]').trigger('click')
    expect(wrapper.emitted('compose-action')?.[1][0]).toMatchObject({ action: 'compose_restart' })
  })

  it('emits compose-action with compose_logs from the logs button', async () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [project], containers: [runningContainer], canRunDocker: true },
    })
    await wrapper.find('[aria-label="Voir les logs du projet"]').trigger('click')
    expect(wrapper.emitted('compose-action')?.[0][0]).toMatchObject({ action: 'compose_logs' })
  })

  it('opens the config modal, shows the raw config, and closes it', async () => {
    const wrapper = mount(ComposeProjectsTab, { props: { composeProjects: [project] } })
    await wrapper.find('[title="Config"]').trigger('click')
    expect(wrapper.text()).toContain('services:')
    expect(wrapper.find('.modal').exists()).toBe(true)

    await wrapper.find('.modal-footer button').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it('shows a placeholder when the project has no raw config', async () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [{ ...project, raw_config: undefined }] },
    })
    await wrapper.find('[title="Config"]').trigger('click')
    expect(wrapper.text()).toContain('Config non disponible')
  })

  it('copies the raw config to the clipboard and shows a confirmation', async () => {
    const wrapper = mount(ComposeProjectsTab, { props: { composeProjects: [project] } })
    await wrapper.find('[title="Config"]').trigger('click')
    const copyBtn = wrapper.findAll('button').find((b) => b.text() === 'Copier')
    await copyBtn!.trigger('click')
    await flushPromises()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(project.raw_config)
    expect(wrapper.text()).toContain('✓ Copié')
  })

  it('filters by host and by state', async () => {
    const other = { ...project, id: 'p2', name: 'other-stack', hostname: 'web-02', host_id: 'h2' }
    const wrapper = mount(ComposeProjectsTab, {
      props: { composeProjects: [project, other], containers: [runningContainer] },
    })
    expect(wrapper.findAll('tbody tr').length).toBe(2)

    const [hostSelect, stateSelect] = wrapper.findAll('select')
    await hostSelect.setValue('web-01')
    expect(wrapper.findAll('tbody tr').length).toBe(1)
    expect(wrapper.text()).toContain('my-stack')

    await hostSelect.setValue('')
    await stateSelect.setValue('stopped')
    expect(wrapper.findAll('tbody tr').length).toBe(1)
    expect(wrapper.text()).toContain('other-stack')
  })

  it('triggers a tracker run and reports success', async () => {
    runReleaseTracker.mockResolvedValue({ data: {} })
    const wrapper = mount(ComposeProjectsTab, {
      props: {
        composeProjects: [project],
        containers: [runningContainer],
        versionComparisons: [{
          tracker_id: 't1', host_id: 'h1', docker_image: 'nginx',
          status: 'update_available', latest_version: '1.27.0',
        }],
        canRunDocker: true,
      },
    })
    await wrapper.find('[aria-label="Déclencher le tracker"]').trigger('click')
    await flushPromises()
    expect(runReleaseTracker).toHaveBeenCalledWith('t1')
    expect(wrapper.text()).toContain('Déclenchement lancé pour my-stack.')
    expect(wrapper.find('.alert-success').exists()).toBe(true)
  })

  it('reports a translated error when the tracker run fails', async () => {
    runReleaseTracker.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(ComposeProjectsTab, {
      props: {
        composeProjects: [project],
        containers: [runningContainer],
        versionComparisons: [{
          tracker_id: 't1', host_id: 'h1', docker_image: 'nginx',
          status: 'update_available', latest_version: '1.27.0',
        }],
        canRunDocker: true,
      },
    })
    await wrapper.find('[aria-label="Déclencher le tracker"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Échec du déclenchement manuel.')
    expect(wrapper.find('.alert-danger').exists()).toBe(true)
  })

  it('disables the tracker button and shows the not-admin tooltip when canRunDocker is false', () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: {
        composeProjects: [project],
        containers: [runningContainer],
        versionComparisons: [{
          tracker_id: 't1', host_id: 'h1', docker_image: 'nginx',
          status: 'update_available', latest_version: '1.27.0',
        }],
        canRunDocker: false,
      },
    })
    const btn = wrapper.find('[aria-label="Déclencher le tracker"]')
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.attributes('title')).toBe('Action réservée admin/opérateur')
  })

  it('shows the "wait for check" tooltip when the tracker has no manual version data yet', () => {
    const wrapper = mount(ComposeProjectsTab, {
      props: {
        composeProjects: [project],
        containers: [runningContainer],
        versionComparisons: [{
          tracker_id: 't1', host_id: 'h1', docker_image: 'nginx',
          status: 'update_available', latest_version: '',
        }],
        canRunDocker: true,
      },
    })
    const btn = wrapper.find('[aria-label="Déclencher le tracker"]')
    expect(btn.attributes('title')).toBe('Attendez la première vérification automatique')
  })
})
