import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeStorageTab from './ProxmoxNodeStorageTab.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('ProxmoxNodeStorageTab', () => {
  it('shows the translated empty state when there is no storage', () => {
    const wrapper = mount(ProxmoxNodeStorageTab, { props: { storages: [] } })
    expect(wrapper.text()).toContain('Aucun stockage sur ce nœud.')
  })

  it('renders translated column headers', () => {
    const wrapper = mount(ProxmoxNodeStorageTab, { props: { storages: [] } })
    for (const label of ['Stockage', 'Type', 'Total', 'Utilisé', 'Disponible', 'Utilisation', 'Partagé', 'Statut']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('shows the active badge and translated shared "Oui" for an active shared storage', () => {
    const wrapper = mount(ProxmoxNodeStorageTab, {
      props: {
        storages: [{ id: 's1', connection_id: 'c1', node_name: 'pve1', last_seen_at: '2026-01-01T00:00:00Z', storage_name: 'local-lvm', storage_type: 'lvm', total: 100_000_000_000, used: 50_000_000_000, avail: 50_000_000_000, shared: true, active: true, enabled: true }],
      },
    })
    expect(wrapper.text()).toContain('Actif')
    expect(wrapper.text()).toContain('Oui')
    expect(wrapper.text()).toContain('Go')
  })

  it('shows the inactive badge for a disabled storage', () => {
    const wrapper = mount(ProxmoxNodeStorageTab, {
      props: {
        storages: [{ id: 's2', connection_id: 'c1', node_name: 'pve1', last_seen_at: '2026-01-01T00:00:00Z', storage_name: 'nfs-backup', storage_type: 'nfs', total: 0, used: 0, avail: 0, shared: false, active: false, enabled: false }],
      },
    })
    expect(wrapper.text()).toContain('Inactif')
  })
})
