import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import RRDChartCard from './RRDChartCard.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('RRDChartCard', () => {
  it('shows the translated default empty-state text when there is no series', () => {
    const wrapper = mount(RRDChartCard, { props: { title: 'CPU' } })
    expect(wrapper.text()).toContain('Aucune donnée')
  })

  it('honors a custom emptyText override instead of the translated default', () => {
    const wrapper = mount(RRDChartCard, { props: { title: 'CPU', emptyText: 'Rien à afficher' } })
    expect(wrapper.text()).toContain('Rien à afficher')
    expect(wrapper.text()).not.toContain('Aucune donnée')
  })

  it('renders the card title', () => {
    const wrapper = mount(RRDChartCard, { props: { title: 'Memory' } })
    expect(wrapper.text()).toContain('Memory')
  })
})
