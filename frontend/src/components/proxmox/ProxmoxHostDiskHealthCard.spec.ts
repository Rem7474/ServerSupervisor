import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getHostProxmoxDisks } = vi.hoisted(() => ({
  getHostProxmoxDisks: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { getHostProxmoxDisks },
}))

import ProxmoxHostDiskHealthCard from './ProxmoxHostDiskHealthCard.vue'

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

describe('ProxmoxHostDiskHealthCard', () => {
  it('shows the translated empty state when the node reports no disks', async () => {
    getHostProxmoxDisks.mockResolvedValue({ data: [] })
    const wrapper = mount(ProxmoxHostDiskHealthCard, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucune donnée disque côté nœud Proxmox')
  })

  it('shows the node-name suffix and the table with translated headers and formatted size', async () => {
    getHostProxmoxDisks.mockResolvedValue({
      data: [
        { id: 'd1', node_name: 'pve1', dev_path: '/dev/sda', model: 'SSD 1TB', serial: 'ABC123', size_bytes: 1_000_000_000_000, disk_type: 'ssd', health: 'PASSED', wearout: 87 },
      ],
    })
    const wrapper = mount(ProxmoxHostDiskHealthCard, { props: { hostId: 'h1', nodeName: 'pve1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('nœud Proxmox pve1')
    expect(wrapper.text()).toContain('Périphérique')
    expect(wrapper.text()).toContain('Durée de vie restante')
    expect(wrapper.text()).toContain('/dev/sda')
    expect(wrapper.text()).toContain('Go') // ~931 GB from 1TB decimal bytes
  })

  it('shows N/A when a disk has no wearout data', async () => {
    getHostProxmoxDisks.mockResolvedValue({
      data: [
        { id: 'd1', node_name: 'pve1', dev_path: '/dev/sdb', size_bytes: 500_000_000_000, health: 'UNKNOWN', wearout: -1 },
      ],
    })
    const wrapper = mount(ProxmoxHostDiskHealthCard, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('N/A')
  })
})
