import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import GuestLinkCell from './GuestLinkCell.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('GuestLinkCell', () => {
  it('shows a dash when there is no link', () => {
    const wrapper = mount(GuestLinkCell, { props: { link: null } })
    expect(wrapper.text()).toBe('—')
  })

  it('shows the suggested badge, host name, and confirm/ignore buttons', () => {
    const wrapper = mount(GuestLinkCell, {
      props: { link: { status: 'suggested', host_name: 'web-01' } },
    })
    expect(wrapper.text()).toContain('Suggéré')
    expect(wrapper.text()).toContain('web-01')
    expect(wrapper.findAll('button')).toHaveLength(2)
  })

  it('emits confirm/ignore when the suggested buttons are clicked', async () => {
    const wrapper = mount(GuestLinkCell, {
      props: { link: { status: 'suggested', host_name: 'web-01' } },
    })
    const buttons = wrapper.findAll('button')
    await buttons[0].trigger('click')
    await buttons[1].trigger('click')
    expect(wrapper.emitted('confirm')).toBeTruthy()
    expect(wrapper.emitted('ignore')).toBeTruthy()
  })

  it('shows the linked badge and a translated tooltip on the host button, and emits "go" on click', async () => {
    const wrapper = mount(GuestLinkCell, {
      props: { link: { status: 'confirmed', host_hostname: 'srv-web' } },
    })
    expect(wrapper.text()).toContain('Lié')
    const button = wrapper.find('button')
    expect(button.attributes('title')).toBe('Voir la fiche hôte')
    expect(button.text()).toContain('srv-web')
    await button.trigger('click')
    expect(wrapper.emitted('go')).toBeTruthy()
  })

  it('shows a dash for an unrecognized link status', () => {
    const wrapper = mount(GuestLinkCell, {
      props: { link: { status: 'ignored' } },
    })
    expect(wrapper.text()).toBe('—')
  })
})
