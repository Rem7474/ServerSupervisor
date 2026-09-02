import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AlertRuleList from './AlertRuleList.vue'

beforeEach(() => {
  setLocale('fr')
})

function mountList(props: Record<string, unknown> = {}) {
  return mount(AlertRuleList, {
    props: {
      formatDate: (d: string | undefined) => d ?? '',
      ...props,
    },
  })
}

describe('AlertRuleList', () => {
  it('shows the empty state with the admin CTA', () => {
    const wrapper = mountList({ rules: [], fetched: true, isAdmin: true })
    expect(wrapper.text()).toContain('Aucune règle d\'alerte configurée')
    expect(wrapper.text()).toContain('Créer ma première alerte')
  })

  it('shows the viewer empty-state subtitle without a CTA', () => {
    const wrapper = mountList({ rules: [], fetched: true, isAdmin: false })
    expect(wrapper.text()).toContain('Aucune règle visible sur les hôtes qui vous sont accessibles.')
    expect(wrapper.text()).not.toContain('Créer ma première alerte')
  })

  it('falls back to "Sans nom" and pluralizes the active-incident badge', () => {
    const wrapper = mountList({
      rules: [{ id: 1, metric: 'cpu', operator: '>', active_incident_count: 3 }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('Sans nom')
    expect(wrapper.text()).toContain('3 actifs')
  })

  it('singularizes the active-incident badge for one incident', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'CPU high', metric: 'cpu', operator: '>', active_incident_count: 1 }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('1 actif')
    expect(wrapper.text()).not.toContain('1 actifs')
  })

  it('shows an agent source badge with the host name, falling back to "Tous les hôtes"', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'cpu', operator: '>', source_type: 'agent', host_id: 'h1' }],
      hosts: [{ id: 'h1', name: 'web-01' }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('Agent › web-01')
  })

  it('shows a synthetic source badge', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'uptime_down_count', operator: '>', source_type: 'synthetic' }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('Synthétique')
  })

  it('formats the docker scope label for a compose project and unknown project', () => {
    const wrapper = mountList({
      rules: [
        { id: 1, name: 'r1', metric: 'docker_container_state', operator: '==', source_type: 'docker', docker_scope: { scope_mode: 'compose_project', project_name: 'vaultwarden' } },
        { id: 2, name: 'r2', metric: 'docker_container_state', operator: '==', source_type: 'docker', docker_scope: { scope_mode: 'compose_project' } },
        { id: 3, name: 'r3', metric: 'docker_container_state', operator: '==', source_type: 'docker', docker_scope: { scope_mode: 'container' } },
      ],
      fetched: true,
    })
    expect(wrapper.text()).toContain('Compose › vaultwarden')
    expect(wrapper.text()).toContain('Compose › Projet inconnu')
    expect(wrapper.text()).toContain('Docker › Conteneur')
  })

  it('formats the proxmox scope label for known scope modes', () => {
    const wrapper = mountList({
      rules: [
        { id: 1, name: 'r1', metric: 'proxmox_node_cpu_percent', operator: '>', proxmox_scope: { scope_mode: 'node', node_id: 'pve1' } },
      ],
      fetched: true,
    })
    expect(wrapper.text()).toContain('Proxmox › Nœud pve1')
  })

  it('formats every other proxmox scope mode (global/connection/guest/storage/disk/unknown)', () => {
    const cases: Array<{ scope: Record<string, unknown> | undefined, expected: string }> = [
      { scope: undefined, expected: 'Proxmox › Cluster' },
      { scope: { scope_mode: 'global' }, expected: 'Proxmox › Cluster' },
      { scope: { scope_mode: 'connection', connection_id: 'c1' }, expected: 'Proxmox › Connexion c1' },
      { scope: { scope_mode: 'guest', guest_id: 'g1' }, expected: 'Proxmox › VM/LXC g1' },
      { scope: { scope_mode: 'storage', storage_id: 's1' }, expected: 'Proxmox › Stockage s1' },
      { scope: { scope_mode: 'disk', disk_id: 'd1' }, expected: 'Proxmox › Disque d1' },
      { scope: { scope_mode: 'weird-future-mode' }, expected: 'Proxmox › Scope inconnu' },
    ]
    for (const { scope, expected } of cases) {
      const wrapper = mountList({
        rules: [{ id: 1, name: 'r', metric: 'proxmox_node_cpu_percent', operator: '>', proxmox_scope: scope }],
        fetched: true,
      })
      expect(wrapper.text(), JSON.stringify(scope)).toContain(expected)
    }
  })

  it('falls back to "Docker" with no docker_scope and to "all containers" for an unrecognized scope mode', () => {
    const noScope = mountList({
      rules: [{ id: 1, name: 'r', metric: 'docker_container_state', operator: '==', source_type: 'docker' }],
      fetched: true,
    })
    expect(noScope.text()).toContain('Docker')

    const unknownScopeMode = mountList({
      rules: [{ id: 2, name: 'r2', metric: 'docker_container_state', operator: '==', source_type: 'docker', docker_scope: { scope_mode: 'host' } }],
      fetched: true,
    })
    expect(unknownScopeMode.text()).toContain('Docker › Tous les conteneurs')
  })

  it('falls back to the raw channel string for an unrecognized channel', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'cpu', operator: '>', actions: { channels: ['some_future_channel'] } }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('some_future_channel')
  })

  it('shows the auto-resolve hysteresis hints when no clear threshold is set', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'cpu', operator: '>', threshold_warn: 70, threshold_crit: 90 }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('auto : résolution quand aucune condition n\'est vraie')
    expect(wrapper.text()).toContain('auto : résolution quand la condition crit n\'est plus vraie')
  })

  it('translates channel badges', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'cpu', operator: '>', actions: { channels: ['smtp', 'ntfy', 'browser', 'notify'] } }],
      fetched: true,
    })
    expect(wrapper.text()).toContain('Email')
    expect(wrapper.text()).toContain('Ntfy')
    expect(wrapper.text()).toContain('Navigateur')
    expect(wrapper.text()).toContain('Système')
  })

  it('shows admin edit/delete buttons with translated tooltips', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'cpu', operator: '>' }],
      fetched: true,
      isAdmin: true,
    })
    expect(wrapper.find('button[aria-label="Modifier la règle"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="Supprimer la règle"]').exists()).toBe(true)
  })

  it('hides admin actions and shows the admin-only tooltip on the toggle for a non-admin', () => {
    const wrapper = mountList({
      rules: [{ id: 1, name: 'r', metric: 'cpu', operator: '>' }],
      fetched: true,
      isAdmin: false,
    })
    expect(wrapper.find('button[aria-label="Modifier la règle"]').exists()).toBe(false)
    expect(wrapper.find('label.form-switch').attributes('title')).toBe('Réservé aux administrateurs')
  })
})
