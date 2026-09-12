import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import TrafficThreatsShell from './TrafficThreatsShell.vue'

let currentPath = '/traffic'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: currentPath }),
}))

vi.mock('../components/security/TrafficOverviewPanel.vue', () => ({
  default: { name: 'TrafficOverviewPanel', template: '<div class="overview-stub" />' },
}))
vi.mock('../components/security/ThreatsPanel.vue', () => ({
  default: { name: 'ThreatsPanel', template: '<div class="threats-stub" />' },
}))

const stubs = { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } }

describe('TrafficThreatsShell', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('shows the translated traffic-mode title/description and renders the overview panel on /traffic', () => {
    currentPath = '/traffic'
    const wrapper = mount(TrafficThreatsShell, { global: { stubs } })

    expect(wrapper.text()).toContain('Stats web')
    expect(wrapper.text()).toContain('Trafic HTTP, erreurs, endpoints, géographie des clients et actualisation automatique')
    expect(wrapper.find('.overview-stub').exists()).toBe(true)
    expect(wrapper.find('.threats-stub').exists()).toBe(false)
  })

  it('shows the translated threats-mode title/description and renders the threats panel on /threats', () => {
    currentPath = '/threats'
    const wrapper = mount(TrafficThreatsShell, { global: { stubs } })

    expect(wrapper.text()).toContain('Menaces web')
    expect(wrapper.text()).toContain('IPs suspectes, chemins scannés, corrélation multi-hôtes et chronologie détaillée')
    expect(wrapper.find('.threats-stub').exists()).toBe(true)
    expect(wrapper.find('.overview-stub').exists()).toBe(false)
  })
})
