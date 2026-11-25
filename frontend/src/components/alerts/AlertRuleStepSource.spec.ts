import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AlertRuleStepSource from './AlertRuleStepSource.vue'
import { useAlertRuleForm } from '../../composables/useAlertRuleForm'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

beforeEach(() => {
  setLocale('fr')
})

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
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form: formFor('cpu', { target_type: 'host' }) }) })
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

  it('renders translated source-type buttons and selects the "all hosts" target by default', () => {
    const wrapper = mount(AlertRuleStepSource, { props: baseProps() })
    expect(wrapper.text()).toContain('Synthétique')
    expect(wrapper.text()).toContain('🐳 Docker')
    const pressed = wrapper.findAll('fieldset button[aria-pressed="true"]')
    expect(pressed.map(b => b.text())).toEqual(['Tous les hôtes'])
    expect(wrapper.find('#alert-source-host-id').exists()).toBe(false)
  })

  it('pluralizes the compose project service count', () => {
    const form = formFor('docker_compose_degraded_services', {
      docker_scope: { ...formFor('cpu').docker_scope, host_id: 'host-1' },
    })
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form,
        dockerHosts: [{
          host_id: 'host-1',
          host_name: 'srv-web',
          containers: [],
          projects: [
            { name: 'solo', services: ['web'] },
            { name: 'multi', services: ['web', 'db'] },
          ],
        }],
      }),
    })
    expect(wrapper.text()).toContain('solo (1 service)')
    expect(wrapper.text()).toContain('multi (2 services)')
  })
})

describe('AlertRuleStepSource — partial host metrics notice', () => {
  const capabilities = { metrics: [{ metric: 'cpu' }, { metric: 'memory' }, { metric: 'disk' }] }

  it.each([
    [1, 'fr', 'Cet hôte dispose de 1 métrique —'],
    [2, 'fr', 'Cet hôte dispose de 2 métriques —'],
    [1, 'en', 'This host has 1 metric —'],
    [2, 'en', 'This host has 2 metrics —'],
  ])('%i reported metric(s) in %s', (count, locale, expected) => {
    setLocale(locale as 'fr' | 'en')
    const wrapper = mount(AlertRuleStepSource, {
      props: baseProps({
        form: formFor('cpu', { host_id: 'host-1' }),
        capabilities,
        hostMetrics: { metrics: capabilities.metrics.slice(0, count) },
      }),
    })
    expect(wrapper.text()).toContain(expected)
  })
})

describe('AlertRuleStepSource — field writes emit a whole-object update:form, never mutate the prop', () => {
  it('hostIdModel (plain fieldModel) emits the merged form without touching sibling fields', async () => {
    const form = formFor('cpu', { target_type: 'host', host_id: '' })
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

describe('AlertRuleStepSource — agent rule target', () => {
  const hosts = [
    { id: 'h1', name: 'web-1', tags: ['prod', 'web'] },
    { id: 'h2', name: 'db-1', tags: ['prod', 'db'] },
    { id: 'h3', name: 'lab', tags: [] },
  ]
  const hostGroups = [{ id: 'g1', name: 'Prod', member_ids: ['h1', 'h2'] }]

  function lastForm(wrapper: ReturnType<typeof mount>): AlertRuleFormData {
    const events = wrapper.emitted('update:form')!
    return events[events.length - 1][0] as AlertRuleFormData
  }

  function targetButton(wrapper: ReturnType<typeof mount>, label: string) {
    return wrapper.findAll('fieldset .btn-group button').find(b => b.text() === label)!
  }

  it('switching the target type drops the fields the new type does not use', async () => {
    const form = formFor('cpu', { target_type: 'host', host_id: 'h1' })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, hosts, hostGroups }) })

    await targetButton(wrapper, 'Un groupe').trigger('click')

    const next = lastForm(wrapper)
    expect(next.target_type).toBe('group')
    expect(next.host_id).toBeNull()
    expect(next.target_tags).toEqual([])
  })

  it('keeps the current pick when re-selecting the same target type', async () => {
    const form = formFor('cpu', { target_type: 'tag', target_tags: ['db'] })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, hosts }) })

    await targetButton(wrapper, 'Des tags').trigger('click')

    expect(lastForm(wrapper).target_tags).toEqual(['db'])
  })

  it('group target: lists the groups with their member count and previews the coverage', async () => {
    const form = formFor('cpu', { target_type: 'group', host_group_id: 'g1' })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, hosts, hostGroups }) })

    const options = wrapper.findAll('#alert-source-host-group option')
    expect(options[1].text()).toBe('Prod (2 hôtes)')
    expect(wrapper.find('label[for="alert-source-host-group"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('2 hôtes couverts actuellement.')

    await wrapper.find('#alert-source-host-group').setValue('g1')
    expect(lastForm(wrapper).host_group_id).toBe('g1')
  })

  it('group target without any group points to the settings page', () => {
    const form = formFor('cpu', { target_type: 'group' })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, hosts }) })
    expect(wrapper.text()).toContain('Paramètres › Groupes d\'hôtes')
    expect(wrapper.text()).not.toContain('couvert')
  })

  it('tag target: offers the union of host tags and toggles them case-insensitively', async () => {
    const form = formFor('cpu', { target_type: 'tag', target_tags: ['PROD'] })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, hosts }) })

    const labels = wrapper.findAll('.form-check-label').map(l => l.text())
    expect(labels).toEqual(['db', 'prod', 'web'])
    expect(wrapper.text()).toContain('2 hôtes couverts actuellement.')

    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect((boxes[1].element as HTMLInputElement).checked).toBe(true)

    await boxes[1].trigger('change') // uncheck prod
    expect(lastForm(wrapper).target_tags).toEqual([])
    await boxes[0].trigger('change') // check db
    expect(lastForm(wrapper).target_tags).toEqual(['PROD', 'db'])
  })

  it('tag target without any tagged host explains where tags come from', () => {
    const form = formFor('cpu', { target_type: 'tag' })
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ form, hosts: [{ id: 'h', name: 'x' }] }) })
    expect(wrapper.text()).toContain('Aucun hôte n\'a de tag')
    expect(wrapper.text()).not.toContain('couvert')
  })

  it('a global metric only allows the "all hosts" target', () => {
    const wrapper = mount(AlertRuleStepSource, { props: baseProps({ metricSupportsHostFilter: false }) })
    const disabled = wrapper.findAll('fieldset .btn-group button').filter(b => b.attributes('disabled') !== undefined)
    expect(disabled.map(b => b.text())).toEqual(['Un hôte', 'Un groupe', 'Des tags'])
    expect(wrapper.text()).toContain('Cette métrique est globale')
  })
})
