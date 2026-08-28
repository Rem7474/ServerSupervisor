import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import WebhookTrackerFields from './WebhookTrackerFields.vue'
import type { WebhookFormData, PickableContainer } from '../../composables/useWebhookForm'

function baseForm(overrides: Partial<WebhookFormData> = {}): WebhookFormData {
  return {
    name: '',
    notify_channels: [],
    enabled: true,
    tracker_type: 'git',
    provider: 'github',
    ...overrides,
  }
}

function mountFields(form: WebhookFormData) {
  return mount(WebhookTrackerFields, {
    props: {
      form,
      registryCredentials: [{ id: 'cred-1', name: 'GHCR perso', registry_host: 'ghcr.io' }],
      containerHosts: [{ id: 'host-1', name: 'srv-web' }],
      containersForHost: [],
      containerKey: (c: PickableContainer) => `${c.host_id}:${c.image}`,
      selectedContainerKey: '',
      selectedContainerMissing: false,
      containerSourceHostId: '',
    },
  })
}

describe('WebhookTrackerFields (characterization)', () => {
  it('git mode: renders provider/owner/repo fields with associated labels', () => {
    const wrapper = mountFields(baseForm({ tracker_type: 'git' }))

    for (const id of ['webhook-tracker-provider', 'webhook-tracker-repo-owner', 'webhook-tracker-repo-name']) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
  })

  it('docker mode: renders host/container/registry fields plus the linked-repo sub-section', () => {
    const wrapper = mountFields(baseForm({ tracker_type: 'docker' }))

    for (const id of [
      'webhook-tracker-source-host',
      'webhook-tracker-container',
      'webhook-tracker-registry-credentials',
      // Linked git repo fields nested inside the docker branch — distinct
      // ids from the git-mode ones above even though the labels repeat.
      'webhook-tracker-linked-provider',
      'webhook-tracker-linked-repo-owner',
      'webhook-tracker-linked-repo-name',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
    }

    // The two "Provider" ids must never collide even though this branch
    // isn't visible at the same time as the git branch above.
    expect(wrapper.find('#webhook-tracker-provider').exists()).toBe(false)
  })
})
