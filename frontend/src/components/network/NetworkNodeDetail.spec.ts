import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import NetworkNodeDetail from './NetworkNodeDetail.vue'

const mountOpts = { global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } } }

describe('NetworkNodeDetail', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders host role, traffic and container counts for a host node', () => {
    const wrapper = mount(NetworkNodeDetail, {
      props: {
        selectedNode: { type: 'host', hostId: 'h1', label: 'web-01' },
        hosts: [{ id: 'h1', status: 'online', network_rx_bytes: 2048, network_tx_bytes: 1024 }],
        containers: [
          { host_id: 'h1', state: 'running' },
          { host_id: 'h1', state: 'exited' },
        ],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Rôle réseau')
    expect(wrapper.text()).toContain('online')
    expect(wrapper.text()).toContain('2 (1 actifs)')
    expect(wrapper.text()).toContain('Ouvrir l\'hôte')
  })

  it('shows "inconnu" when a host has no status', () => {
    const wrapper = mount(NetworkNodeDetail, {
      props: {
        selectedNode: { type: 'host', hostId: 'h1' },
        hosts: [{ id: 'h1' }],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('inconnu')
  })

  it('renders the no-agent hint for a Proxmox guest node', () => {
    const wrapper = mount(NetworkNodeDetail, {
      props: { selectedNode: { type: 'proxmox_guest', status: 'running', guestId: 'g1' } },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('VM Proxmox')
    expect(wrapper.text()).toContain('Sans agent ServerSupervisor')
    expect(wrapper.text()).toContain('Ouvrir la VM')
  })

  it('renders port node details with container list', () => {
    const wrapper = mount(NetworkNodeDetail, {
      props: {
        selectedNode: { type: 'port', portNumber: 8080, protocol: 'tcp', containers: ['nginx', 'app'] },
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('8080/TCP')
    expect(wrapper.text()).toContain('nginx, app')
  })

  it('renders a service node with a URL and copy/open actions', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })

    const wrapper = mount(NetworkNodeDetail, {
      props: {
        selectedNode: { type: 'service', internalPort: 3000, externalPort: 443, sublabel: 'app.example.com', isInternetExposed: true },
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('https://app.example.com')
    expect(wrapper.text()).toContain('Exposé (port 443)')
    expect(wrapper.find('a[href="https://app.example.com"]').exists()).toBe(true)

    const copyButton = wrapper.findAll('button').find((b) => b.text() === "Copier l'URL")
    await copyButton?.trigger('click')
    expect(writeText).toHaveBeenCalledWith('https://app.example.com')
  })

  it('shows "Non" / "Non exposé" for an unlinked service', () => {
    const wrapper = mount(NetworkNodeDetail, {
      props: { selectedNode: { type: 'service', internalPort: 3000 } },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Non exposé')
    const nonSpans = wrapper.findAll('.text-secondary.small').filter((el) => el.text() === 'Non')
    expect(nonSpans.length).toBe(2)
  })

  it('renders the port chips section for a host with services and discovered ports', () => {
    const wrapper = mount(NetworkNodeDetail, {
      props: {
        selectedNode: { type: 'host', hostId: 'h1' },
        combinedServices: [{ id: 's1', hostId: 'h1', internalPort: 80, name: 'web', linkToProxy: true }],
        discoveredPortsByHost: {
          h1: [
            { key: 'p1', port: 80, protocol: 'tcp', internal: false },
            { key: 'p2', port: 22, protocol: 'tcp', internal: false },
          ],
        },
        hostPortOverrides: { h1: { excludedPorts: [22] } },
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Ports & services exposés')
    expect(wrapper.text()).toContain('web')
    expect(wrapper.text()).toContain('22/TCP')
    expect(wrapper.text()).toContain('off')
  })

  it('emits close when the close button is clicked', async () => {
    const wrapper = mount(NetworkNodeDetail, { props: { selectedNode: null }, ...mountOpts })
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
