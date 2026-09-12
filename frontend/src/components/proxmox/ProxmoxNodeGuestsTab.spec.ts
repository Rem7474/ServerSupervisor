import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import ProxmoxNodeGuestsTab from './ProxmoxNodeGuestsTab.vue'
import { useAuthStore } from '../../stores/auth'

const stubs = { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } }

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    kind: 'vm' as const,
    guests: [],
    guestNetworks: {},
    peerNodes: [],
    nodeId: 'n1',
    ...overrides,
  }
}

beforeEach(() => {
  setLocale('fr')
  setActivePinia(createPinia())
})

describe('ProxmoxNodeGuestsTab', () => {
  it('shows the VM-specific empty state and column headers', () => {
    const wrapper = mount(ProxmoxNodeGuestsTab, { props: baseProps(), global: { stubs } })
    expect(wrapper.text()).toContain('Aucune VM sur ce nœud.')
    for (const label of ['Nom', 'Statut', 'IP', 'Domaines', 'CPU', 'RAM', 'Disque']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('shows the LXC-specific empty state', () => {
    const wrapper = mount(ProxmoxNodeGuestsTab, { props: baseProps({ kind: 'lxc' }), global: { stubs } })
    expect(wrapper.text()).toContain('Aucun conteneur LXC sur ce nœud.')
  })

  it('shows translated entity-state label for a running guest', () => {
    const wrapper = mount(ProxmoxNodeGuestsTab, {
      props: baseProps({ guests: [{ id: 'g1', vmid: 100, name: 'web-01', status: 'running', cpu_usage: 0.1 }] }),
      global: { stubs },
    })
    expect(wrapper.text()).toContain('En cours')
  })

  it('shows admin power-action buttons with translated tooltips for an admin', () => {
    const auth = useAuthStore()
    auth.role = 'admin'
    const wrapper = mount(ProxmoxNodeGuestsTab, {
      props: baseProps({ guests: [{ id: 'g1', vmid: 100, name: 'web-01', status: 'running', cpu_usage: 0.1 }] }),
      global: { stubs },
    })
    expect(wrapper.find('button[title="Redémarrer"]').exists()).toBe(true)
    expect(wrapper.find('button[title="Arrêter"]').exists()).toBe(true)
  })

  it('shows the translated Migrer button for a VM with peer nodes available', () => {
    const wrapper = mount(ProxmoxNodeGuestsTab, {
      props: baseProps({
        guests: [{ id: 'g1', vmid: 100, name: 'web-01', status: 'running', cpu_usage: 0.1 }],
        peerNodes: [{ id: 'n2' }],
      }),
      global: { stubs },
    })
    const migrateButton = wrapper.findAll('button').find((b) => b.text() === 'Migrer')
    expect(migrateButton).toBeTruthy()
    expect(migrateButton!.attributes('title')).toBe('Migrer vers un autre nœud')
  })
})
