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

describe('WebhookTrackerFields — field writes emit a whole-object update:form, never mutate the prop', () => {
  // provider is a fixed-option <select> (github/gitlab/gitea); repo_owner/
  // repo_name are free-text inputs.
  const gitFields: Array<{ id: string; key: keyof WebhookFormData; oldValue: string; newValue: string }> = [
    { id: 'webhook-tracker-provider', key: 'provider', oldValue: 'github', newValue: 'gitlab' },
    { id: 'webhook-tracker-repo-owner', key: 'repo_owner', oldValue: 'old-owner', newValue: 'new-owner' },
    { id: 'webhook-tracker-repo-name', key: 'repo_name', oldValue: 'old-repo', newValue: 'new-repo' },
  ]

  for (const { id, key, oldValue, newValue } of gitFields) {
    it(`#${id} (git mode) writes only ${key}`, async () => {
      const form = baseForm({ tracker_type: 'git', [key]: oldValue } as Partial<WebhookFormData>)
      const wrapper = mountFields(form)

      await wrapper.find(`#${id}`).setValue(newValue)

      const next = wrapper.emitted('update:form')![0][0] as WebhookFormData
      expect(next[key]).toBe(newValue)
      expect(next.tracker_type).toBe('git')
      expect(form[key]).toBe(oldValue) // prop untouched
    })
  }

  it('the linked-repo fields (docker mode) write through the exact same providerModel/repoOwnerModel/repoNameModel as git mode', async () => {
    const form = baseForm({ tracker_type: 'docker', provider: 'github', repo_owner: 'old-owner' })
    const wrapper = mountFields(form)

    await wrapper.find('#webhook-tracker-linked-repo-owner').setValue('new-owner')

    const next = wrapper.emitted('update:form')![0][0] as WebhookFormData
    expect(next.repo_owner).toBe('new-owner')
    expect(next.tracker_type).toBe('docker')
    expect(form.repo_owner).toBe('old-owner') // prop untouched
  })

  it('registryCredentialsIdModel (docker mode) emits the merged form', async () => {
    const form = baseForm({ tracker_type: 'docker' })
    const wrapper = mountFields(form)

    await wrapper.find('#webhook-tracker-registry-credentials').setValue('cred-1')

    const next = wrapper.emitted('update:form')![0][0] as WebhookFormData
    expect(next.registry_credentials_id).toBe('cred-1')
  })

  it('onContainerChange emits select-container for a real selection but not for the placeholder', async () => {
    const wrapper = mount(WebhookTrackerFields, {
      props: {
        form: baseForm({ tracker_type: 'docker' }),
        registryCredentials: [],
        containerHosts: [{ id: 'host-1', name: 'srv-web' }],
        containersForHost: [{ host_id: 'host-1', host_name: 'srv-web', image: 'nginx:latest' } as PickableContainer],
        containerKey: (c: PickableContainer) => `${c.host_id}:${c.image}`,
        selectedContainerKey: '',
        selectedContainerMissing: false,
        containerSourceHostId: 'host-1',
      },
    })

    await wrapper.find('#webhook-tracker-container').setValue('host-1:nginx:latest')
    expect(wrapper.emitted('select-container')?.[0]).toEqual(['host-1:nginx:latest'])

    // Re-selecting the already-active key (e.g. the placeholder round-tripping) must not re-emit.
    await wrapper.setProps({ selectedContainerKey: 'host-1:nginx:latest' })
    await wrapper.find('#webhook-tracker-container').setValue('host-1:nginx:latest')
    expect(wrapper.emitted('select-container')).toHaveLength(1)
  })
})
