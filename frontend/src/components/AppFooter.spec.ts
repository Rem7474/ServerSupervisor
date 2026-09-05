import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import AppFooter from './AppFooter.vue'

describe('AppFooter', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders the translated copyright notice with no WS status by default', () => {
    const wrapper = mount(AppFooter)
    expect(wrapper.text()).toContain('Tous droits réservés.')
    expect(wrapper.text()).not.toContain('Connecté')
  })

  it('renders each translated WS status label', () => {
    const cases: Record<string, string> = {
      connected: 'Connecté',
      connecting: 'Connexion…',
      reconnecting: 'Reconnexion…',
      error: 'Erreur WS',
      disconnected: 'Déconnecté',
    }
    for (const [status, label] of Object.entries(cases)) {
      const wrapper = mount(AppFooter, { props: { wsStatus: status } })
      expect(wrapper.text()).toContain(label)
    }
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(AppFooter, { props: { wsStatus: 'reconnecting' } })
    expect(wrapper.text()).toContain('Reconnecting…')
    expect(wrapper.text()).toContain('All rights reserved.')
  })
})
