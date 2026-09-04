import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'

const { getProxmoxSummary, getProxmoxNodes, getProxmoxInstances } = vi.hoisted(() => ({
  getProxmoxSummary: vi.fn(),
  getProxmoxNodes: vi.fn(),
  getProxmoxInstances: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getProxmoxSummary, getProxmoxNodes, getProxmoxInstances },
  isApiAbort: () => false,
}))

import ProxmoxView from './ProxmoxView.vue'
import { useAuthStore } from '../stores/auth'

const stubs = { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } }

const node = {
  id: 'n1',
  connection_id: 'c1',
  node_name: 'pve-1',
  status: 'online',
  cpu_count: 8,
  cpu_usage: 0.2,
  mem_total: 16_000_000_000,
  mem_used: 4_000_000_000,
  uptime: 100000,
  pve_version: '8.1',
  cluster_name: '',
  ip_address: '10.0.0.10',
  last_seen_at: '2026-01-01T10:00:00Z',
  pending_updates: 0,
  security_updates: 0,
  vm_count: 3,
  lxc_count: 2,
}

function mountView() {
  return mount(ProxmoxView, { global: { stubs } })
}

beforeEach(() => {
  vi.clearAllMocks()
  setLocale('fr')
  setActivePinia(createPinia())
  getProxmoxSummary.mockResolvedValue({ data: { connection_count: 1, node_count: 1, vm_count: 3, lxc_count: 2, storage_total: 0, storage_used: 0 } })
  getProxmoxNodes.mockResolvedValue({ data: [node] })
  getProxmoxInstances.mockResolvedValue({ data: [] })
})

describe('ProxmoxView', () => {
  it('renders the translated page header and table headers', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Proxmox VE')
    expect(wrapper.text()).toContain("Supervision de l'infrastructure de virtualisation")
    for (const label of ['Nœud', 'Instance / Cluster', 'VMs', 'LXC', 'Statut', 'Dernier contact']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('shows a translated online status badge and Détail link for a node', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('En ligne')
    expect(wrapper.text()).toContain('Détail')
  })

  it('shows the translated empty state when no node is found', async () => {
    getProxmoxNodes.mockResolvedValue({ data: [] })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun nœud Proxmox trouvé.')
  })

  it('shows a translated health filter badge and empty state when a filter matches nothing', async () => {
    getProxmoxSummary.mockResolvedValue({
      data: { connection_count: 1, node_count: 1, vm_count: 0, lxc_count: 0, storage_total: 0, storage_used: 0, nodes_down: 1, nodes_down_ids: ['other-node'] },
    })
    const wrapper = mountView()
    await flushPromises()

    const card = wrapper.findAll('.card-sm').find((c) => c.text().includes('Nœuds hors ligne'))
    expect(card).toBeTruthy()
    await card!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Filtré : Nœuds hors ligne')
    expect(wrapper.text()).toContain('Aucun nœud ne correspond au filtre')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const auth = useAuthStore()
    auth.role = 'admin'
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Virtualization infrastructure monitoring')
    expect(wrapper.text()).toContain('Online')
  })
})
