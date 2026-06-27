import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('../../api', () => ({ default: { runReleaseTracker: vi.fn() } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

import DockerContainersTab from './DockerContainersTab.vue'

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
