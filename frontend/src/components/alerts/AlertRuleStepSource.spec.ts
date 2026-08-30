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

describe('AlertRuleStepSource — field writes emit a whole-object update:form, never mutate the prop', () => {
  it('hostIdModel (plain fieldModel) emits the merged form without touching sibling fields', async () => {
    const form = formFor('cpu', { host_id: '' })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form }) })

    await wrapper.find('#alert-source-host-id').setValue('host-1')

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.host_id).toBe('host-1')
    expect(next.metric).toBe('cpu')
    expect(form.host_id).toBe('') // prop untouched
  })

  it('onDockerHostChange atomically resets container_id/container_ids/project_name alongside host_id', async () => {
    const form = formFor('docker_container_state', {
      docker_scope: { ...formFor('cpu').docker_scope, host_id: 'host-1', container_ids: ['c1'], project_name: 'app' },
    })
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form,
        dockerHosts: [
          { host_id: 'host-1', host_name: 'srv-web', containers: [], projects: [] },
          { host_id: 'host-2', host_name: 'srv-db', containers: [], projects: [] },
        ],
      }),
    })

    await wrapper.find('#alert-source-docker-host').setValue('host-2')

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.docker_scope.host_id).toBe('host-2')
    expect(next.docker_scope.container_ids).toEqual([])
    expect(next.docker_scope.project_name).toBe('')
    // Sibling docker_scope fields (e.g. scope_mode) untouched by this reset.
    expect(next.docker_scope.scope_mode).toBe(form.docker_scope.scope_mode)
    expect(form.docker_scope.container_ids).toEqual(['c1']) // prop untouched
  })

  it('onDockerScopeModeChange resets the container selection when switching scope mode', async () => {
    const form = formFor('docker_container_state', {
      docker_scope: { ...formFor('cpu').docker_scope, scope_mode: 'container', host_id: 'host-1', container_ids: ['c1'] },
    })
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form,
        dockerHosts: [{ host_id: 'host-1', host_name: 'srv-web', containers: [{ id: 'c1', name: 'web', state: 'running' }], projects: [] }],
      }),
    })

    await wrapper.find('#alert-source-docker-scope-mode').setValue('host')

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.docker_scope.scope_mode).toBe('host')
    expect(next.docker_scope.container_ids).toEqual([])
    expect(form.docker_scope.scope_mode).toBe('container') // prop untouched
  })

  it('toggleContainer adds/removes one container id without clearing the others', async () => {
    const form = formFor('docker_container_state', {
      docker_scope: { ...formFor('cpu').docker_scope, scope_mode: 'container', host_id: 'host-1', container_ids: ['c1'] },
    })
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form,
        dockerHosts: [{
          host_id: 'host-1',
          host_name: 'srv-web',
          containers: [
            { id: 'c1', name: 'web', state: 'running' },
            { id: 'c2', name: 'worker', state: 'running' },
          ],
          projects: [],
        }],
      }),
    })

    const checkboxes = wrapper.findAll('.docker-container-checklist input[type="checkbox"]')
    expect(checkboxes).toHaveLength(2)
    await checkboxes[1].setValue(true) // check c2, c1 already selected in the form

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.docker_scope.container_ids.sort()).toEqual(['c1', 'c2'])
    expect(form.docker_scope.container_ids).toEqual(['c1']) // prop untouched
  })

  it('proxmoxScopeModeModel (proxmoxScopeModel factory) emits the merged form', async () => {
    const form = formFor('proxmox_node_cpu_percent', {
      proxmox_scope: { ...formFor('cpu').proxmox_scope, scope_mode: 'connection' },
    })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form }) })

    await wrapper.find('#alert-source-proxmox-scope-mode').setValue('node')

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.proxmox_scope.scope_mode).toBe('node')
    expect(form.proxmox_scope.scope_mode).toBe('connection') // prop untouched
  })
})
