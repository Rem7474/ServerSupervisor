import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AptPendingPackagesList from './AptPendingPackagesList.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('AptPendingPackagesList', () => {
  it('renders nothing when there are no pending packages', () => {
    const wrapper = mount(AptPendingPackagesList, { props: { packages: [] } })
    expect(wrapper.text()).toBe('')
  })

  it('lists every package up to the preview count, with a count badge', () => {
    const wrapper = mount(AptPendingPackagesList, { props: { packages: ['curl', 'openssl', 'vim'] } })
    expect(wrapper.text()).toContain('Paquets en attente')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('curl')
    expect(wrapper.text()).toContain('openssl')
    expect(wrapper.text()).toContain('vim')
  })

  it('truncates past the preview count behind a "Voir tout" toggle', async () => {
    const packages = Array.from({ length: 20 }, (_, i) => `pkg-${i}`)
    const wrapper = mount(AptPendingPackagesList, { props: { packages, previewCount: 15 } })

    expect(wrapper.text()).toContain('pkg-14')
    expect(wrapper.text()).not.toContain('pkg-15')
    expect(wrapper.text()).toContain('Voir tout (20)')

    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toContain('pkg-19')
    expect(wrapper.text()).toContain('Réduire')
  })
})
