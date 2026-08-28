import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

// Mock the API barrel so the component never hits the network on mount/watchers
// (useWebhookForm's immediate watcher loads custom tasks / registry credentials
// / pickable containers as soon as the modal is visible).
vi.mock('../../api', () => ({
  default: {
    getHostCustomTasks: vi.fn(async () => ({ data: [] })),
    getRegistryCredentials: vi.fn(async () => ({ data: { credentials: [] } })),
    getPickableContainers: vi.fn(async () => ({ data: { containers: [] } })),
  },
}))

import WebhookModal from './WebhookModal.vue'

function mountModal(props: Record<string, unknown> = {}) {
  return mount(WebhookModal, {
    props: {
      visible: true,
      hosts: [{ id: 'host-1', name: 'srv-web' }],
      ...props,
    },
  })
}

describe('WebhookModal (characterization)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('webhook mode: renders the git-webhook fields, VM cible and the tasks.yaml field, each labelled', async () => {
    const wrapper = mountModal({ mode: 'webhook' })
    await nextTick()

    for (const id of [
      'webhook-name',
      'webhook-provider',
      'webhook-event-filter',
      'webhook-repo-filter',
      'webhook-branch-filter',
      'webhook-host-id',
      'webhook-custom-task-id',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
  })

  it('tracker mode, docker + compose with a dispatch task: renders cooldown, compose fields and both hooks', async () => {
    const wrapper = mountModal({
      mode: 'tracker',
      item: {
        name: 'Update Home Assistant',
        tracker_type: 'docker',
        host_id: 'host-1',
        docker_image: 'ghcr.io/home-assistant/home-assistant',
        docker_tag: 'latest',
        update_action: 'compose',
        compose_project: 'homeassistant',
      },
    })
    await nextTick()

    for (const id of [
      'webhook-cooldown-hours',
      'webhook-host-id',
      'webhook-compose-project',
      'webhook-compose-service',
      'webhook-healthcheck-timeout',
      'webhook-pre-update-task-id',
      'webhook-post-update-task-id',
      // Nested WebhookTrackerFields, docker branch.
      'webhook-tracker-source-host',
      'webhook-tracker-container',
      'webhook-tracker-registry-credentials',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
    }
    expect(wrapper.text()).toContain('Mode de mise a jour')
  })

  it('emits "close" when the close button is clicked', async () => {
    const wrapper = mountModal()
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
