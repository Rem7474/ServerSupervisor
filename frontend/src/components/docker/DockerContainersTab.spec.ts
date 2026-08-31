import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'

vi.mock('../../api', () => ({ default: { runReleaseTracker: vi.fn() } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

import DockerContainersTab from './DockerContainersTab.vue'

beforeEach(() => {
  setLocale('fr')
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
