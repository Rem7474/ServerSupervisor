import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getTrackableContainers, createReleaseTrackersBulk } = vi.hoisted(() => ({
  getTrackableContainers: vi.fn(),
  createReleaseTrackersBulk: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { getTrackableContainers, createReleaseTrackersBulk },
}))

import TrackableContainersModal from './TrackableContainersModal.vue'

const container = { host_id: 'h1', host_name: 'web-01', image: 'nginx', image_tag: 'latest', compose_project: 'stack', compose_service: 'web' }

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

describe('TrackableContainersModal', () => {
  it('renders the translated title, explanation and options', async () => {
    getTrackableContainers.mockResolvedValue({ data: { containers: [container] } })
    const wrapper = mount(TrackableContainersModal, { props: { visible: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Activer la mise à jour automatique')
    expect(wrapper.text()).toContain('cette liste sert à activer en plus la mise à jour automatique')
    expect(wrapper.text()).toContain('Healthcheck (s)')
    expect(wrapper.text()).toContain('Rollback si échec')
    expect(wrapper.text()).toContain('Nettoyer images orphelines')
    for (const label of ['Hôte', 'Projet / Service']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('shows the translated empty state when there is no trackable container', async () => {
    getTrackableContainers.mockResolvedValue({ data: { containers: [] } })
    const wrapper = mount(TrackableContainersModal, { props: { visible: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun conteneur à suivre')
    expect(wrapper.text()).toContain('Tous les conteneurs compose détectés ont déjà un tracker')
  })

  it('shows the translated error when loading containers fails', async () => {
    getTrackableContainers.mockRejectedValue(new Error('boom'))
    const wrapper = mount(TrackableContainersModal, { props: { visible: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Impossible de charger les conteneurs détectés.')
  })

  it('shows the translated pluralized "create trackers" button and close button', async () => {
    getTrackableContainers.mockResolvedValue({ data: { containers: [container] } })
    const wrapper = mount(TrackableContainersModal, { props: { visible: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Créer 1 tracker')
    expect(wrapper.text()).toContain('Fermer')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    getTrackableContainers.mockResolvedValue({ data: { containers: [] } })
    const wrapper = mount(TrackableContainersModal, { props: { visible: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Enable automatic update')
    expect(wrapper.text()).toContain('No container to track')
  })
})
