import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { runReleaseTracker } = vi.hoisted(() => ({ runReleaseTracker: vi.fn() }))
const { push } = vi.hoisted(() => ({ push: vi.fn() }))
vi.mock('../../api', () => ({ default: { runReleaseTracker } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

import DockerContainersTab from './DockerContainersTab.vue'

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

// BulkActionBar teleports to document.body regardless of whether a given
// test mounted with `attachTo` — without this, an earlier test's teleported
// content (never unmounted) lingers in the real document.body and a later
// test's document.body.querySelectorAll() can pick up a stale button.
afterEach(() => {
  document.body.innerHTML = ''
})

function makeContainers(n: number) {
  return Array.from({ length: n }, (_, i) => ({
    id: `c${i}`,
    name: `container-${i}`,
    hostname: 'host-1',
    host_id: 'h1',
    image: 'nginx',
    image_tag: 'latest',
    state: i % 2 === 0 ? 'running' : 'exited',
    labels: {},
    env_vars: {},
    volumes: [],
    networks: [],
    ports: '80/tcp',
  }))
}

describe('DockerContainersTab mount/unmount', () => {
  it('mounts a table of rows and unmounts cleanly', () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: makeContainers(5),
        versionComparisons: [],
        canRunDocker: true,
        actionLoading: {},
      },
    })
    expect(wrapper.findAll('tbody tr').length).toBe(5)
    expect(() => wrapper.unmount()).not.toThrow()
  })

  it('survives repeated WS-style patches then unmount', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: makeContainers(3),
        versionComparisons: [],
        canRunDocker: true,
        actionLoading: {},
      },
    })
    // Simulate WS snapshots: list grows/shrinks/reorders, action spinners toggle.
    await wrapper.setProps({ containers: makeContainers(8), actionLoading: { 'container-0': 'start' } })
    await wrapper.setProps({ containers: makeContainers(2), actionLoading: {} })
    await wrapper.setProps({ containers: makeContainers(30) })
    expect(() => wrapper.unmount()).not.toThrow()
  })

  it('survives duplicate container ids in a snapshot (dup v-for keys)', async () => {
    const dup = makeContainers(3)
    dup.push({ ...dup[0] }) // same id as row 0 → duplicate :key
    const wrapper = mount(DockerContainersTab, {
      props: { containers: dup, versionComparisons: [], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.setProps({ containers: makeContainers(5) })
    expect(() => wrapper.unmount()).not.toThrow()
  })
})

describe('DockerContainersTab version badges', () => {
  const container = (over: Record<string, unknown> = {}) => ({
    id: 'c0', name: 'web', hostname: 'host-1', host_id: 'h1',
    image: 'nginx', image_tag: 'latest', state: 'running',
    labels: {}, env_vars: {}, volumes: [], networks: [], ports: '80/tcp',
    ...over,
  })

  function badgeText(props: Record<string, unknown>): string {
    const wrapper = mount(DockerContainersTab, {
      props: { canRunDocker: false, actionLoading: {}, ...props },
    })
    return wrapper.find('tbody tr').text()
  }

  // The ambient engine emits a row for every running container, tracker or
  // not — that's the whole point of the shared engine, so an untracked
  // container must render a real verdict instead of no badge at all.
  it('renders a badge for a container with no tracker, from an ambient row', () => {
    expect(badgeText({
      containers: [container()],
      versionComparisons: [{
        host_id: 'h1', docker_image: 'nginx', image_tag: 'latest',
        status: 'update_available', running_version: '1.25.0', latest_version: '1.27.4',
        is_up_to_date: false, update_confirmed: true,
      }],
    })).toContain('Mise à jour disponible')

    expect(badgeText({
      containers: [container()],
      versionComparisons: [{
        host_id: 'h1', docker_image: 'nginx', image_tag: 'latest',
        status: 'up_to_date', running_version: '1.27.4', latest_version: '1.27.4', is_up_to_date: true,
      }],
    })).toContain('À jour')
  })

  // An image the engine could not check (private registry, no credential) is
  // explicitly unknown — never silently classified as up to date.
  it('renders "Version inconnue" when the ambient row carries no verdict', () => {
    expect(badgeText({
      containers: [container()],
      versionComparisons: [{
        host_id: 'h1', docker_image: 'nginx', image_tag: 'latest',
        status: 'unknown', running_version: '', latest_version: '',
        last_error: 'registre privé : aucun identifiant enregistré',
      }],
    })).toContain('Version inconnue')
  })

  // Two containers of the same image on different tags must not share a badge.
  it('keys ambient rows by tag', () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        canRunDocker: false,
        actionLoading: {},
        containers: [
          container({ id: 'a', name: 'a', image_tag: 'latest' }),
          container({ id: 'b', name: 'b', image_tag: '1.25' }),
        ],
        versionComparisons: [
          { host_id: 'h1', docker_image: 'nginx', image_tag: 'latest', status: 'up_to_date', is_up_to_date: true },
          { host_id: 'h1', docker_image: 'nginx', image_tag: '1.25', status: 'update_available', running_version: '1.25', latest_version: '1.27.4' },
        ],
      },
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('À jour')
    expect(rows[1].text()).toContain('Mise à jour disponible')
  })

  // A tracker row is tag-agnostic (it aggregates every tag of its image), so it
  // stays reachable as the fallback for containers it covers.
  it('falls back to a tracker row when no tag-specific row exists', () => {
    expect(badgeText({
      containers: [container()],
      versionComparisons: [{
        tracker_id: 't1', host_id: 'h1', docker_image: 'nginx',
        status: 'update_available', running_version: '1.25.0', latest_version: '1.27.4',
        is_up_to_date: false, update_confirmed: true,
      }],
    })).toContain('Mise à jour disponible')
  })
})

const container = (over: Record<string, unknown> = {}) => ({
  id: 'c0', name: 'web', hostname: 'host-1', host_id: 'h1',
  image: 'nginx', image_tag: 'latest', state: 'running',
  labels: {}, env_vars: {}, volumes: [], networks: [], ports: '80/tcp',
  ...over,
})

describe('DockerContainersTab lifecycle action buttons', () => {
  it('shows start for a stopped container and stop/restart for a running one, and emits container-action', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: [container({ state: 'exited' }), container({ id: 'c1', name: 'db', state: 'running' })],
        canRunDocker: true,
        actionLoading: {},
      },
    })
    const [stoppedRow, runningRow] = wrapper.findAll('tbody tr')

    expect(stoppedRow.find('[aria-label="Démarrer le conteneur"]').exists()).toBe(true)
    expect(stoppedRow.find('[aria-label="Arrêter le conteneur"]').exists()).toBe(false)
    expect(runningRow.find('[aria-label="Arrêter le conteneur"]').exists()).toBe(true)
    expect(runningRow.find('[aria-label="Redémarrer le conteneur"]').exists()).toBe(true)

    await runningRow.find('[aria-label="Arrêter le conteneur"]').trigger('click')
    expect(wrapper.emitted('container-action')?.[0][0]).toMatchObject({ hostId: 'h1', name: 'db', action: 'stop' })

    await runningRow.find('[aria-label="Voir les logs du conteneur"]').trigger('click')
    expect(wrapper.emitted('container-action')?.[1][0]).toMatchObject({ action: 'logs' })
  })

  it('hides every lifecycle action for a read-only user, but keeps inspect/track buttons', () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container({ state: 'running' })], canRunDocker: false, actionLoading: {} },
    })
    const row = wrapper.find('tbody tr')
    expect(row.find('[aria-label="Arrêter le conteneur"]').exists()).toBe(false)
    expect(row.find('[aria-label="Inspecter le conteneur"]').exists()).toBe(true)
  })
})

describe('DockerContainersTab selection + bulk action bar', () => {
  it('selects an individual row and shows it in the bulk bar count', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container(), container({ id: 'c1', name: 'db' })], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Sélectionner web"]').setValue(true)
    expect(wrapper.find('tbody tr').classes()).toContain('table-active')
  })

  it('selects all visible rows via the header checkbox, scoped to the current page', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container(), container({ id: 'c1', name: 'db' })], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Sélectionner tous les conteneurs affichés"]').setValue(true)
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].classes()).toContain('table-active')
    expect(rows[1].classes()).toContain('table-active')
  })

  it('emits bulk-container-action with the selected containers and clears selection', async () => {
    // BulkActionBar teleports to document.body, so its buttons live outside
    // the mounted wrapper's own tree — query the body directly.
    const wrapper = mount(DockerContainersTab, {
      attachTo: document.body,
      props: { containers: [container(), container({ id: 'c1', name: 'db' })], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Sélectionner web"]').setValue(true)
    const startBtn = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Démarrer')
    expect(startBtn).toBeTruthy()
    startBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('bulk-container-action')
    expect(emitted).toBeTruthy()
    expect((emitted![0][0] as { name: string }[]).map((c) => c.name)).toEqual(['web'])
    expect(emitted![0][1]).toBe('start')
    wrapper.unmount()
  })
})

describe('DockerContainersTab inspect modal', () => {
  it('opens on the inspect button, defaults to the Env Vars tab, and shows the no-data message when empty', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container()], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(true)
    expect(wrapper.text()).toContain("Aucune variable d'environnement")
  })

  it('lists env vars in a table when present', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container({ env_vars: { NODE_ENV: 'production' } })], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    expect(wrapper.text()).toContain('NODE_ENV')
    expect(wrapper.text()).toContain('production')
  })

  it('switches to the Volumes tab and shows the empty message, then lists volumes when present', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container({ volumes: ['/data:/data'] })], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    const volumesTab = wrapper.findAll('.nav-link').find((a) => a.text().includes('Volumes'))
    await volumesTab!.trigger('click')
    expect(wrapper.text()).toContain('/data:/data')
  })

  it('shows no-volumes message when there are none', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container()], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    const volumesTab = wrapper.findAll('.nav-link').find((a) => a.text().includes('Volumes'))
    await volumesTab!.trigger('click')
    expect(wrapper.text()).toContain('Aucun volume monté')
  })

  it('switches to the Networks tab, lists networks, and shows cumulative I/O when traffic is present', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: [container({ networks: ['bridge'], net_rx_bytes: 2048, net_tx_bytes: 1024 })],
        canRunDocker: false,
        actionLoading: {},
      },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    const networksTab = wrapper.findAll('.nav-link').find((a) => a.text().includes('Réseaux'))
    await networksTab!.trigger('click')
    expect(wrapper.text()).toContain('bridge')
    expect(wrapper.text()).toContain('I/O réseau (cumulatif)')
    expect(wrapper.text()).toContain('2 KiB')
  })

  it('shows no-network message and hides the I/O panel when there is no traffic', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container()], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    const networksTab = wrapper.findAll('.nav-link').find((a) => a.text().includes('Réseaux'))
    await networksTab!.trigger('click')
    expect(wrapper.text()).toContain('Aucun réseau connecté')
    expect(wrapper.text()).not.toContain('I/O réseau')
  })

  it('closes via the footer button', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container()], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Inspecter le conteneur"]').trigger('click')
    await wrapper.find('.modal-footer button').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(false)
  })
})

describe('DockerContainersTab compose info / labels modal', () => {
  it('shows the button only when the container has compose info or labels', () => {
    const bare = mount(DockerContainersTab, {
      props: { containers: [container({ labels: {} })], canRunDocker: false, actionLoading: {} },
    })
    expect(bare.find('[title="Labels"]').exists()).toBe(false)

    const labeled = mount(DockerContainersTab, {
      props: { containers: [container({ labels: { foo: 'bar' } })], canRunDocker: false, actionLoading: {} },
    })
    expect(labeled.find('[title="Labels"]').exists()).toBe(true)
  })

  it('opens the modal with compose project/service info and raw labels', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: [container({
          labels: {
            'com.docker.compose.project': 'my-stack',
            'com.docker.compose.service': 'web',
            'com.docker.compose.project.working_dir': '/opt/my-stack',
          },
        })],
        canRunDocker: false,
        actionLoading: {},
      },
    })
    const btn = wrapper.find('[title="Infos Compose + Labels"]')
    expect(btn.exists()).toBe(true)
    await btn.trigger('click')
    expect(wrapper.text()).toContain('my-stack')
    expect(wrapper.text()).toContain('com.docker.compose.project:')
  })

  it('closes via the footer button', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [container({ labels: { foo: 'bar' } })], canRunDocker: false, actionLoading: {} },
    })
    await wrapper.find('[title="Labels"]').trigger('click')
    const closeBtn = wrapper.findAll('.modal-footer button').find((b) => b.text() === 'Fermer')
    await closeBtn!.trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(false)
  })
})

describe('DockerContainersTab version tracker actions', () => {
  const trackedContainer = container({
    id: 'c-tracked',
  })
  const trackedVc = {
    tracker_id: 't1', host_id: 'h1', docker_image: 'nginx', image_tag: 'latest',
    status: 'update_available' as const, latest_version: '1.27.4',
  }

  it('shows the "view tracking" and "trigger tracker" buttons only when a tracker_id exists', () => {
    const untracked = mount(DockerContainersTab, {
      props: { containers: [container()], canRunDocker: true, actionLoading: {} },
    })
    expect(untracked.find('[title="Voir le suivi de version"]').exists()).toBe(false)

    const tracked = mount(DockerContainersTab, {
      props: { containers: [trackedContainer], versionComparisons: [trackedVc], canRunDocker: true, actionLoading: {} },
    })
    expect(tracked.find('[title="Voir le suivi de version"]').exists()).toBe(true)
  })

  it('navigates to the release tracker page from the view-tracking button', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [trackedContainer], versionComparisons: [trackedVc], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.find('[title="Voir le suivi de version"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/release-trackers/t1')
  })

  it('triggers the tracker and shows a success message', async () => {
    runReleaseTracker.mockResolvedValue({ data: {} })
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [trackedContainer], versionComparisons: [trackedVc], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Déclencher le tracker"]').trigger('click')
    await flushPromises()
    expect(runReleaseTracker).toHaveBeenCalledWith('t1')
    expect(wrapper.text()).toContain('Déclenchement lancé pour nginx.')
  })

  it('shows a translated error when the tracker trigger fails', async () => {
    runReleaseTracker.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(DockerContainersTab, {
      props: { containers: [trackedContainer], versionComparisons: [trackedVc], canRunDocker: true, actionLoading: {} },
    })
    await wrapper.find('[aria-label="Déclencher le tracker"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Échec du déclenchement manuel.')
  })

  it('navigates to the git-webhooks tracker-creation flow from the track-updates button', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: [container({ labels: { 'com.docker.compose.project': 'my-stack', 'com.docker.compose.service': 'web' } })],
        canRunDocker: true,
        actionLoading: {},
      },
    })
    await wrapper.find('[aria-label="Créer un tracker de mise à jour"]').trigger('click')
    expect(push).toHaveBeenCalledWith({
      path: '/git-webhooks',
      query: { tab: 'trackers', docker_image: 'nginx', docker_tag: 'latest', compose_project: 'my-stack', compose_service: 'web' },
    })
  })
})

describe('DockerContainersTab empty states', () => {
  it('shows the no-containers empty state with a CTA when there are none at all', () => {
    const wrapper = mount(DockerContainersTab, { props: { containers: [], canRunDocker: false, actionLoading: {} } })
    expect(wrapper.text()).toContain('Aucun conteneur trouvé')
    expect(wrapper.text()).toContain('Ajouter un hôte')
  })

  it('shows the filter-specific empty state without a CTA when a filter excludes everything', async () => {
    const wrapper = mount(DockerContainersTab, { props: { containers: [container()], canRunDocker: false, actionLoading: {} } })
    const stateSelect = wrapper.findAll('select')[1]
    await stateSelect.setValue('dead')
    expect(wrapper.text()).toContain('Aucun résultat pour ces filtres')
    expect(wrapper.text()).not.toContain('Ajouter un hôte')
  })
})

describe('DockerContainersTab filters and sorting', () => {
  it('filters by host, state and compose/standalone', async () => {
    const wrapper = mount(DockerContainersTab, {
      props: {
        containers: [
          container({ id: 'a', name: 'a', hostname: 'host-1', state: 'running' }),
          container({ id: 'b', name: 'b', hostname: 'host-2', state: 'exited', labels: { 'com.docker.compose.project': 'p' } }),
        ],
        canRunDocker: false,
        actionLoading: {},
      },
    })
    expect(wrapper.findAll('tbody tr').length).toBe(2)

    const [hostSelect, stateSelect, composeSelect] = wrapper.findAll('select')
    await hostSelect.setValue('host-2')
    expect(wrapper.findAll('tbody tr').length).toBe(1)

    await hostSelect.setValue('')
    await stateSelect.setValue('running')
    expect(wrapper.findAll('tbody tr').length).toBe(1)

    await stateSelect.setValue('')
    await composeSelect.setValue('compose')
    expect(wrapper.findAll('tbody tr').length).toBe(1)
    expect(wrapper.text()).toContain('b')
  })
})
