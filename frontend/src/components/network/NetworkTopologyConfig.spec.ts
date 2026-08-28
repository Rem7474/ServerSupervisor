import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import NetworkTopologyConfig from './NetworkTopologyConfig.vue'

function mountConfig() {
  return mount(NetworkTopologyConfig, {
    props: {
      hosts: [{ id: 'host-1', name: 'srv-web' }],
      containers: [{ host_id: 'host-1', name: 'app', port_mappings: [{ host_port: 8080, container_port: 80, protocol: 'tcp' }] }],
      rootHostId: 'host-1',
      autheliaHostId: 'host-1',
      networkServices: [{
        id: 'svc-1', name: '', domain: '', path: '', internalPort: null, externalPort: null,
        hostId: '', tags: '', linkToProxy: false, linkToAuthelia: false, exposedToInternet: false,
      }],
    },
  })
}

describe('NetworkTopologyConfig (characterization)', () => {
  it('renders the root/Authelia/Internet section fields with associated labels or aria-labels', () => {
    const wrapper = mountConfig()

    for (const id of [
      'network-config-root-node-name',
      'network-config-root-node-ip',
      'network-config-root-host-id',
      'network-config-root-port-id', // shown because rootHostId has a discovered non-internal port
      'network-config-authelia-host-id',
      'network-config-authelia-port-id',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }

    // Authelia / Internet free-text inputs use aria-label instead of a
    // sibling <label> (grouped under a section heading, not one per field).
    expect(wrapper.find('input[aria-label="Label affiché dans le graphe"]').exists()).toBe(true)
    expect(wrapper.find('input[aria-label="IP ou domaine Authelia"]').exists()).toBe(true)
    expect(wrapper.find('input[aria-label="IP publique ou domaine"]').exists()).toBe(true)
  })

  it('renders one manual-service table row with an aria-label per cell', () => {
    const wrapper = mountConfig()

    for (const label of ['Nom du service', 'Domaine', 'Chemin', 'Port interne', 'Host', 'Lier au proxy', 'Lier à Authelia', 'Exposer sur Internet', 'Port externe']) {
      expect(wrapper.find(`[aria-label="${label}"]`).exists(), `aria-label="${label}"`).toBe(true)
    }
  })

  it('renders one discovered-port table row (per host) with an aria-label per cell', () => {
    const wrapper = mountConfig()

    expect(wrapper.text()).toContain('srv-web')
    for (const label of ['Nom du service', 'Domaine', 'Chemin', 'Afficher ce port', 'Lier au proxy', 'Lier à Authelia', 'Exposer sur Internet', 'Port externe']) {
      expect(wrapper.find(`[aria-label="${label}"]`).exists(), `aria-label="${label}"`).toBe(true)
    }
  })
})
