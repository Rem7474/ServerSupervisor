import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxClusterCard from './ProxmoxClusterCard.vue'

beforeEach(() => {
  setLocale('fr')
})

const stubs = { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } }

const nodes = [
  { id: 1, node_name: 'pve1', status: 'online', cluster_name: 'prod', cpu_usage: 0.5, cpu_count: 8, mem_used: 4_000_000_000, mem_total: 16_000_000_000, vm_count: 3, lxc_count: 2 },
  { id: 2, node_name: 'pve2', status: 'offline', cpu_usage: 0, cpu_count: 4, mem_used: 0, mem_total: 8_000_000_000, vm_count: 0, lxc_count: 1 },
]

describe('ProxmoxClusterCard', () => {
  it('renders the translated title, details link, and KPI labels', () => {
    const wrapper = mount(ProxmoxClusterCard, { props: { nodes }, global: { stubs } })
    expect(wrapper.text()).toContain('Cluster Proxmox')
    expect(wrapper.text()).toContain('Détails')
    expect(wrapper.text()).toContain('Nœuds')
    expect(wrapper.text()).toContain('CPU cluster')
    expect(wrapper.text()).toContain('RAM cluster')
  })

  it('shows the offline-node count and the core count', () => {
    const wrapper = mount(ProxmoxClusterCard, { props: { nodes }, global: { stubs } })
    expect(wrapper.text()).toContain('1 hors ligne')
    expect(wrapper.text()).toContain('12 cœurs')
  })

  it('formats cluster memory usage with translated units', () => {
    const wrapper = mount(ProxmoxClusterCard, { props: { nodes }, global: { stubs } })
    // 4GB used / 24GB total, base-1024 -> "Go"
    expect(wrapper.text()).toMatch(/Go \/ .*Go/)
  })

  it('renders one row per node with a link to its detail page', () => {
    const wrapper = mount(ProxmoxClusterCard, { props: { nodes }, global: { stubs } })
    const links = wrapper.findAll('a[href^="/proxmox/nodes/"]')
    expect(links).toHaveLength(2)
    expect(links[0].attributes('href')).toBe('/proxmox/nodes/1')
  })
})
