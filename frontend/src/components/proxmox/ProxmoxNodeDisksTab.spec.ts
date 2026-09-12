import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeDisksTab from './ProxmoxNodeDisksTab.vue'
import type { ProxmoxDisk } from '../../types/proxmox'

beforeEach(() => {
  setLocale('fr')
})

function disk(overrides: Partial<ProxmoxDisk> = {}): ProxmoxDisk {
  return {
    id: 'd1', connection_id: 'c1', node_name: 'pve1', dev_path: '/dev/sda',
    model: 'Samsung SSD', serial: 'S1', size_bytes: 500_000_000_000,
    disk_type: 'ssd', health: 'PASSED', wearout: 90, last_seen_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('ProxmoxNodeDisksTab', () => {
  it('shows the translated empty state when there are no disks', () => {
    const wrapper = mount(ProxmoxNodeDisksTab, { props: { disks: [] } })
    expect(wrapper.text()).toContain('Aucun disque détecté sur ce nœud.')
  })

  it('renders translated sortable column headers', () => {
    const wrapper = mount(ProxmoxNodeDisksTab, { props: { disks: [disk()] } })
    for (const label of ['Périphérique', 'Modèle', 'Type', 'Taille', 'Santé SMART', 'Usure SSD']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('formats disk size with translated byte units', () => {
    const wrapper = mount(ProxmoxNodeDisksTab, { props: { disks: [disk({ size_bytes: 500_000_000_000 })] } })
    expect(wrapper.text()).toContain('Go')
  })

  it('re-sorts the rows when the already-active sort header is toggled to descending', async () => {
    const wrapper = mount(ProxmoxNodeDisksTab, {
      props: { disks: [disk({ id: 'd1', dev_path: '/dev/sdb' }), disk({ id: 'd2', dev_path: '/dev/sda' })] },
    })
    const rows = () => wrapper.findAll('tbody tr')
    // Default sort: dev_path ascending.
    expect(rows()[0].text()).toContain('/dev/sda')

    await wrapper.find('th button').trigger('click')
    expect(rows()[0].text()).toContain('/dev/sdb')
  })
})
