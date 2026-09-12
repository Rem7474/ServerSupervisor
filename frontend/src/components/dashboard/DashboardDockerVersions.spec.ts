import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { runReleaseTracker } = vi.hoisted(() => ({
  runReleaseTracker: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { runReleaseTracker },
}))

import DashboardDockerVersions from './DashboardDockerVersions.vue'
import { useAuthStore } from '../../stores/auth'
import { setLocale } from '../../i18n'

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
  vi.clearAllMocks()
})

describe('DashboardDockerVersions', () => {
  it('shows the empty state, translated, when no versions are tracked', () => {
    const wrapper = mount(DashboardDockerVersions, { props: { versions: [] } })
    expect(wrapper.text()).toContain('Aucun suivi de version configuré.')
  })

  it('shows an outdated badge and status badges per row', () => {
    const wrapper = mount(DashboardDockerVersions, {
      props: {
        versions: [
          { docker_image: 'nginx', host_id: 'h1', hostname: 'web-01', is_up_to_date: false, running_version: '1.0', tracker_id: 't1', container_count: 3 },
          { docker_image: 'redis', host_id: 'h2', hostname: 'web-02', is_up_to_date: true, running_version: '2.0', container_count: 1 },
        ],
      },
    })
    expect(wrapper.text()).toContain('1 en retard')
    expect(wrapper.text()).toContain('Mise à jour disponible')
    expect(wrapper.text()).toContain('À jour')
    expect(wrapper.find('[title="3 conteneurs utilisent cette image"]').exists()).toBe(true)
    expect(wrapper.find('[title="1 conteneur utilise cette image"]').exists()).toBe(true)
  })

  it('disables the trigger button for a non-admin/operator with an explanatory tooltip', () => {
    useAuthStore().setAuth({ role: 'viewer', username: 'u' } as never, 'viewer')
    const wrapper = mount(DashboardDockerVersions, {
      props: { versions: [{ docker_image: 'nginx', host_id: 'h1', tracker_id: 't1', latest_version: '1.1' }] },
    })
    const button = wrapper.find('button[aria-label="Déclencher le tracker"]')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('title')).toBe('Action réservée admin/opérateur')
  })

  it('triggers the tracker and shows a translated success message for an admin', async () => {
    useAuthStore().setAuth({ role: 'admin', username: 'u' } as never, 'admin')
    runReleaseTracker.mockResolvedValue({ data: {} })
    const wrapper = mount(DashboardDockerVersions, {
      props: { versions: [{ docker_image: 'nginx', host_id: 'h1', tracker_id: 't1', latest_version: '1.1' }] },
    })

    await wrapper.find('button[aria-label="Déclencher le tracker"]').trigger('click')
    await flushPromises()

    expect(runReleaseTracker).toHaveBeenCalledWith('t1')
    expect(wrapper.text()).toContain('Déclenchement lancé pour nginx.')
  })

  it('shows a translated error message when triggering the tracker fails', async () => {
    useAuthStore().setAuth({ role: 'admin', username: 'u' } as never, 'admin')
    runReleaseTracker.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(DashboardDockerVersions, {
      props: { versions: [{ docker_image: 'nginx', host_id: 'h1', tracker_id: 't1', latest_version: '1.1' }] },
    })

    await wrapper.find('button[aria-label="Déclencher le tracker"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Échec du déclenchement manuel.')
  })
})
