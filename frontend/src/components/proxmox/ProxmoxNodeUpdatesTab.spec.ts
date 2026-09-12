import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeUpdatesTab from './ProxmoxNodeUpdatesTab.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('ProxmoxNodeUpdatesTab', () => {
  it('shows the empty state with the "no data yet" subtitle when there is no last check', () => {
    const wrapper = mount(ProxmoxNodeUpdatesTab, { props: { pendingUpdates: 0 } })
    expect(wrapper.text()).toContain('Aucune mise à jour en attente détectée.')
    expect(wrapper.text()).toContain('Données non encore disponibles')
  })

  it('shows the empty state with the last-check date when one is set', () => {
    const wrapper = mount(ProxmoxNodeUpdatesTab, {
      props: { pendingUpdates: 0, lastUpdateCheckAt: '2026-01-15T10:00:00Z' },
    })
    expect(wrapper.text()).toContain('Dernière vérification :')
  })

  it('shows the pending-count card and read-only apt hints when updates are pending', () => {
    const wrapper = mount(ProxmoxNodeUpdatesTab, {
      props: { pendingUpdates: 5, lastUpdateCheckAt: '2026-01-15T10:00:00Z' },
    })
    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('paquet(s) en attente de mise à jour')
    expect(wrapper.text()).toContain('Détecté le')
    expect(wrapper.text()).toContain('cache apt du nœud Proxmox')
    expect(wrapper.text()).toContain('connectez-vous directement au nœud')
  })

  it('emits "refresh-apt" when the apt update button is clicked', async () => {
    const wrapper = mount(ProxmoxNodeUpdatesTab, { props: { pendingUpdates: 0 } })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('refresh-apt')).toBeTruthy()
  })
})
