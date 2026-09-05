import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import TrafficThreatsFilterBar from './TrafficThreatsFilterBar.vue'

function mountBar(props: Record<string, unknown> = {}) {
  return mount(TrafficThreatsFilterBar, {
    props: { source: '', hostId: '', searchTerm: '', ...props },
  })
}

beforeEach(() => {
  setLocale('fr')
  setActivePinia(createPinia())
})

describe('TrafficThreatsFilterBar', () => {
  it('renders the translated labels, options and buttons', () => {
    const wrapper = mountBar()
    expect(wrapper.text()).toContain('Source')
    expect(wrapper.text()).toContain('Toutes')
    expect(wrapper.text()).toContain('Hôte')
    expect(wrapper.text()).toContain('Tous les hôtes')
    expect(wrapper.text()).toContain('Rafraîchir')
    expect(wrapper.text()).toContain('Rechercher un domaine ou une IP')
    expect(wrapper.text()).toContain('Voir les requêtes')
    expect(wrapper.find('input').attributes('placeholder')).toBe('exemple.com ou 1.2.3.4')
  })

  it('shows the translated "no data for this source" tooltip when sourceHasNoData is true', () => {
    const wrapper = mountBar({ sourceHasNoData: true })
    expect(wrapper.find('[title]').attributes('title')).toBe('Aucune donnée pour cette source sur la période sélectionnée')
  })

  it('emits refresh and search', async () => {
    const wrapper = mountBar({ searchTerm: 'example.com' })
    await wrapper.find('button.btn-primary').trigger('click')
    expect(wrapper.emitted('refresh')).toBeTruthy()

    await wrapper.find('button.btn-outline-secondary').trigger('click')
    expect(wrapper.emitted('search')).toBeTruthy()
  })
})
