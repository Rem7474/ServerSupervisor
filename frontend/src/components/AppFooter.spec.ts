import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import AppFooter from './AppFooter.vue'

describe('AppFooter', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders the translated copyright notice', () => {
    const wrapper = mount(AppFooter)
    expect(wrapper.text()).toContain('Tous droits réservés.')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(AppFooter)
    expect(wrapper.text()).toContain('All rights reserved.')
  })
})
