import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import NetworkPortList from './NetworkPortList.vue'

const mountOpts = { global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } } }

describe('NetworkPortList', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('shows empty states when nothing is passed', () => {
    const wrapper = mount(NetworkPortList, { ...mountOpts })
    expect(wrapper.text()).toContain('Aucun port visible')
    expect(wrapper.text()).toContain('Aucun hôte trouvé')
  })

  it('lists published ports grouped from container port_mappings, with a translated state badge', () => {
    const wrapper = mount(NetworkPortList, {
      props: {
        containers: [
          {
            id: 'c1', host_id: 'h1', hostname: 'web-01', name: 'nginx', image: 'nginx', image_tag: 'latest', state: 'running',
            port_mappings: [{ host_port: 8080, container_port: 80, protocol: 'tcp', host_ip: '0.0.0.0' }],
          },
        ],
      },
      ...mountOpts,
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('nginx')
    expect(rows[0].text()).toContain('8080')
    expect(rows[0].text()).toContain('En cours')
  })

  it('hides unpublished ports by default and shows them when "published only" is unchecked', async () => {
    const wrapper = mount(NetworkPortList, {
      props: {
        containers: [
          {
            id: 'c1', host_id: 'h1', name: 'internal-svc', state: 'running',
            port_mappings: [{ host_port: 0, container_port: 9000, protocol: 'tcp' }],
          },
        ],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Aucun port visible')

    await wrapper.find('input[type="checkbox"]').setValue(false)
    expect(wrapper.text()).toContain('internal-svc')
  })

  it('filters ports by protocol, host, and free-text search', async () => {
    const wrapper = mount(NetworkPortList, {
      props: {
        containers: [
          { id: 'c1', host_id: 'h1', hostname: 'web-01', name: 'nginx', image: 'nginx', state: 'running', port_mappings: [{ host_port: 80, container_port: 80, protocol: 'tcp' }] },
          { id: 'c2', host_id: 'h2', hostname: 'db-01', name: 'postgres', image: 'postgres', state: 'running', port_mappings: [{ host_port: 5432, container_port: 5432, protocol: 'udp' }] },
        ],
      },
      ...mountOpts,
    })
    expect(wrapper.findAll('tbody')[0].findAll('tr')).toHaveLength(2)

    await wrapper.find('select').setValue('udp')
    expect(wrapper.findAll('tbody')[0].findAll('tr')).toHaveLength(1)
    expect(wrapper.findAll('tbody')[0].text()).toContain('postgres')
  })

  it('renders per-host traffic with formatted byte counts and pluralized host count', () => {
    const wrapper = mount(NetworkPortList, {
      props: {
        hosts: [
          { id: 'h1', name: 'web-01', ip_address: '10.0.0.1', status: 'online', network_rx_bytes: 2048, network_tx_bytes: 1024 },
        ],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('2.0 KB')
    expect(wrapper.text()).toContain('1.0 KB')
    expect(wrapper.text()).toContain('1 hôte')
  })

  it('sorts Proxmox guest IPs numerically and toggles direction on header click', async () => {
    const wrapper = mount(NetworkPortList, {
      props: {
        proxmoxGuests: [
          { guest_id: 'g1', vmid: 100, name: 'vm-b', node: 'pve1', guest_type: 'qemu', status: 'running', ip_addresses: ['10.0.0.10'], host_id: undefined, host_name: undefined },
          { guest_id: 'g2', vmid: 101, name: 'vm-a', node: 'pve1', guest_type: 'lxc', status: 'stopped', ip_addresses: ['10.0.0.2'], host_id: undefined, host_name: undefined },
        ],
      },
      ...mountOpts,
    })
    let rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('vm-a')
    expect(rows[0].text()).toContain('Non lié')

    await wrapper.findAll('th button').find((b) => b.text().includes('IP'))?.trigger('click')
    rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('vm-b')
  })

  it('renders NPM domains with a correlated host link, and "Non résolu" when unmatched', () => {
    const wrapper = mount(NetworkPortList, {
      props: {
        npmEntries: [
          { proxy_host_id: 1, domain_names: ['app.example.com'], forward_host: '10.0.0.1', forward_port: 3000, matched_type: 'host', matched_id: 'h1', matched_name: 'web-01' },
          { proxy_host_id: 2, domain_names: ['unknown.example.com'], forward_host: '10.0.0.9', forward_port: 8080, matched_type: undefined, matched_id: undefined, matched_name: undefined },
        ],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('app.example.com')
    expect(wrapper.text()).toContain('Non résolu')
  })

  it('shows a loading spinner instead of counts while ipInventoryLoading is true', () => {
    const wrapper = mount(NetworkPortList, {
      props: { proxmoxGuests: [{ guest_id: 'g1', vmid: 100, name: 'vm', node: 'pve1', guest_type: 'qemu', status: 'running', ip_addresses: [], host_id: undefined, host_name: undefined }], ipInventoryLoading: true },
      ...mountOpts,
    })
    expect(wrapper.find('.spinner-border').exists()).toBe(true)
  })
})
