import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlertRuleStepSource from './AlertRuleStepSource.vue'
import { useAlertRuleForm } from '../../composables/useAlertRuleForm'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

function formFor(metric: string, overrides: Partial<AlertRuleFormData> = {}): AlertRuleFormData {
  const base = useAlertRuleForm().defaultForm()
  return { ...base, metric, ...overrides }
}

const scopeOptions = [{ id: 'opt-1', label: 'Option 1' }]

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    form: formFor('cpu'),
    hosts: [{ id: 'host-1', name: 'srv-web' }],
    metricCards: [],
    metricSupportsHostFilter: true,
    metricAllowsGuestScope: false,
    metricAllowsStorageScope: false,
    metricAllowsDiskScope: false,
    proxmoxConnections: scopeOptions,
    proxmoxNodes: scopeOptions,
    proxmoxStorages: scopeOptions,
    proxmoxGuests: scopeOptions,
    proxmoxDisks: scopeOptions,
    dockerHosts: [],
    ...overrides,
  }
}

describe('AlertRuleStepSource (characterization, per source-type branches)', () => {
  it('agent metric: renders name and host-target fields, both labelled', () => {
    const wrapper = mount(AlertRuleStepSource, { props: baseProps() })
    for (const id of ['alert-source-name', 'alert-source-host-id']) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
  })

  it('proxmox metric, connection scope: renders the scope-mode and connection selects', () => {
    const form = formFor('proxmox_node_cpu_percent', { proxmox_scope: { ...formFor('cpu').proxmox_scope, scope_mode: 'connection' } })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form }) })
    expect(wrapper.find('#alert-source-proxmox-scope-mode').exists()).toBe(true)
    expect(wrapper.find('#alert-source-proxmox-connection').exists()).toBe(true)
  })

  it('proxmox metric, node scope: renders the node select', () => {
    const form = formFor('proxmox_node_cpu_percent', { proxmox_scope: { ...formFor('cpu').proxmox_scope, scope_mode: 'node' } })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form }) })
    expect(wrapper.find('#alert-source-proxmox-node').exists()).toBe(true)
  })

  it('proxmox metric, guest scope (metricAllowsGuestScope): renders the guest select', () => {
    const form = formFor('proxmox_guest_cpu_percent', { proxmox_scope: { ...formFor('cpu').proxmox_scope, scope_mode: 'guest' } })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, metricAllowsGuestScope: true }) })
    expect(wrapper.find('#alert-source-proxmox-guest').exists()).toBe(true)
  })

  it('proxmox metric, storage scope: renders the storage select', () => {
    const form = formFor('proxmox_storage_percent', { proxmox_scope: { ...formFor('cpu').proxmox_scope, scope_mode: 'storage' } })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, metricAllowsStorageScope: true }) })
    expect(wrapper.find('#alert-source-proxmox-storage').exists()).toBe(true)
  })

  it('proxmox metric, disk scope: renders the disk select', () => {
    const form = formFor('proxmox_disk_min_wearout_percent', { proxmox_scope: { ...formFor('cpu').proxmox_scope, scope_mode: 'disk' } })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, metricAllowsDiskScope: true }) })
    expect(wrapper.find('#alert-source-proxmox-disk').exists()).toBe(true)
  })

  it('docker metric: renders the docker host and scope-mode selects', () => {
    const form = formFor('docker_container_state')
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form,
        dockerHosts: [{ host_id: 'host-1', host_name: 'srv-web', containers: [], projects: [] }],
      }),
    })
    expect(wrapper.find('#alert-source-docker-host').exists()).toBe(true)
    expect(wrapper.find('#alert-source-docker-scope-mode').exists()).toBe(true)
  })

  it('docker_compose_degraded_services with a host selected: renders the compose project select', () => {
    const form = formFor('docker_compose_degraded_services', {
      docker_scope: { ...formFor('cpu').docker_scope, host_id: 'host-1' },
    })
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form,
        dockerHosts: [{ host_id: 'host-1', host_name: 'srv-web', containers: [], projects: [{ name: 'app', services: ['web'] }] }],
      }),
    })
    expect(wrapper.find('#alert-source-docker-project').exists()).toBe(true)
  })
})
