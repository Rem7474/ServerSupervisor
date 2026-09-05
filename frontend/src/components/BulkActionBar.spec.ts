import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import BulkActionBar from './BulkActionBar.vue'

describe('BulkActionBar', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders nothing when count is 0', () => {
    const wrapper = mount(BulkActionBar, { props: { count: 0 }, attachTo: document.body })
    expect(document.querySelector('.bulk-action-bar')).toBeNull()
    wrapper.unmount()
  })

  it('renders the translated singular label and aria attributes for one selection', () => {
    const wrapper = mount(BulkActionBar, { props: { count: 1 }, attachTo: document.body })
    const bar = document.querySelector('.bulk-action-bar') as HTMLElement
    expect(bar.textContent).toContain('1 hôte sélectionné')
    expect(bar.getAttribute('aria-label')).toBe('Actions groupées — 1 hôte(s) sélectionné(s)')
    expect(bar.querySelector('.bulk-action-bar__close')?.getAttribute('aria-label')).toBe('Annuler la sélection')
    wrapper.unmount()
  })

  it('pluralizes the count label for 2+ selections', () => {
    const wrapper = mount(BulkActionBar, { props: { count: 3 }, attachTo: document.body })
    const bar = document.querySelector('.bulk-action-bar') as HTMLElement
    expect(bar.textContent).toContain('3 hôtes sélectionnés')
    wrapper.unmount()
  })

  it('emits clear when the close button is clicked', async () => {
    const wrapper = mount(BulkActionBar, { props: { count: 1 }, attachTo: document.body })
    const closeBtn = document.querySelector('.bulk-action-bar__close') as HTMLButtonElement
    closeBtn.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('clear')).toBeTruthy()
    wrapper.unmount()
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(BulkActionBar, { props: { count: 2 }, attachTo: document.body })
    const bar = document.querySelector('.bulk-action-bar') as HTMLElement
    expect(bar.textContent).toContain('2 hosts selected')
    wrapper.unmount()
  })
})
