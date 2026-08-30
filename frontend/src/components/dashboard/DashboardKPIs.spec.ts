import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import { useHostsStore } from '../../stores/hosts'
import { useDashboardStore } from '../../stores/dashboard'
import DashboardKPIs from './DashboardKPIs.vue'

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
  useHostsStore().setHosts([
    { id: 'h1', status: 'online' } as never,
    { id: 'h2', status: 'online' } as never,
    { id: 'h3', status: 'offline' } as never,
  ])
})

describe('DashboardKPIs — hosts card', () => {
  it('shows the online/offline counts in French', () => {
    const wrapper = mount(DashboardKPIs)
    expect(wrapper.text()).toContain('Hôtes')
    expect(wrapper.text()).toContain('2 en ligne')
    expect(wrapper.text()).toContain('1 hors ligne')
  })

  it('translates the same counts in English', () => {
    setLocale('en')
    const wrapper = mount(DashboardKPIs)
    expect(wrapper.text()).toContain('Hosts')
    expect(wrapper.text()).toContain('2 online')
    expect(wrapper.text()).toContain('1 offline')
  })
})

describe('DashboardKPIs — updates card', () => {
  it('pluralizes the pending APT/Docker counts correctly', () => {
    useDashboardStore().setAptPending(1)
    useDashboardStore().setVersionComparisons([
      { docker_image: 'a', is_up_to_date: false, running_version: '1.0' },
      { docker_image: 'b', is_up_to_date: false, running_version: '1.0' },
    ])
    const wrapper = mount(DashboardKPIs)
    expect(wrapper.text()).toContain('1 paquet APT')
    expect(wrapper.text()).toContain('2 images Docker')
  })

  it('shows "everything up to date" when there is nothing outdated', () => {
    const wrapper = mount(DashboardKPIs)
    expect(wrapper.text()).toContain('Tout est à jour')
  })
})

describe('DashboardKPIs — Proxmox cards', () => {
  it('shows the Proxmox node/storage cards, translated, once Proxmox is configured', async () => {
    useDashboardStore().setProxmoxSummary({
      connection_count: 1, node_count: 3, nodes_down: 1,
      storage_used: 50, storage_total: 100,
    })
    const wrapper = mount(DashboardKPIs)
    expect(wrapper.text()).toContain('Proxmox — Nœuds')
    expect(wrapper.text()).toContain('Proxmox — Stockage')

    setLocale('en')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Proxmox — Nodes')
    expect(wrapper.text()).toContain('Proxmox — Storage')
  })

  it('falls back to the plain online/offline cards when Proxmox is not configured', () => {
    const wrapper = mount(DashboardKPIs)
    expect(wrapper.text()).not.toContain('Proxmox')
    expect(wrapper.text()).toContain('En ligne')
    expect(wrapper.text()).toContain('Hors ligne')
  })
})
