import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeServicesTab from './ProxmoxNodeServicesTab.vue'

beforeEach(() => {
  setLocale('fr')
})

const services = [
  { name: 'pveproxy', state: 'running', 'active-state': 'active', 'sub-state': 'running', 'unit-state': 'enabled', desc: 'PVE API Proxy' },
  { name: 'pvebanner', state: 'exited', 'active-state': 'inactive', 'sub-state': 'dead', 'unit-state': 'static', desc: 'PVE Login Banner' },
]

describe('ProxmoxNodeServicesTab', () => {
  it('shows the hint to click refresh when no services are loaded yet', () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services: [] } })
    expect(wrapper.text()).toContain('Cliquez sur "Actualiser" pour charger les services du nœud Proxmox.')
  })

  it('renders translated filter buttons and column headers', () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services } })
    expect(wrapper.text()).toContain('Actifs')
    expect(wrapper.text()).toContain('Tous')
    for (const label of ['Service', 'État', 'Sous-état', 'Démarrage', 'Description', 'Actions']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('filters to active services by default and shows all when toggled', async () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services } })
    expect(wrapper.text()).toContain('pveproxy')
    expect(wrapper.text()).not.toContain('pvebanner')

    const allButton = wrapper.findAll('button').find((b) => b.text() === 'Tous')
    await allButton!.trigger('click')
    expect(wrapper.text()).toContain('pvebanner')
  })

  it('shows Démarrer for an inactive service and Arrêter/Redémarrer/Recharger for an active one, with translated tooltips', async () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services } })
    const allButton = wrapper.findAll('button').find((b) => b.text() === 'Tous')
    await allButton!.trigger('click')

    expect(wrapper.find('button[aria-label="Démarrer le service"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="Arrêter le service"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="Redémarrer le service"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="Recharger le service"]').exists()).toBe(true)
  })

  it('emits "action" with the service name and action type when a lifecycle button is clicked', async () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services } })
    await wrapper.find('button[aria-label="Arrêter le service"]').trigger('click')
    expect(wrapper.emitted('action')?.[0]).toEqual([{ name: 'pveproxy', action: 'stop' }])
  })

  it('shows the "Chargement…" label instead of "Actualiser" while loading', () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services: [], loading: true } })
    expect(wrapper.text()).toContain('Chargement…')
    expect(wrapper.text()).not.toContain('Actualiser')
  })

  it('shows the translated permissions hint in the footer when there is an error', () => {
    const wrapper = mount(ProxmoxNodeServicesTab, { props: { services: [], error: 'boom' } })
    expect(wrapper.text()).toContain('Sys.Audit requis')
    expect(wrapper.text()).toContain('Sys.Modify requis')
  })
})
