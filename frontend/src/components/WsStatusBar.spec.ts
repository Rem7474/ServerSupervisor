import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import WsStatusBar from './WsStatusBar.vue'

describe('WsStatusBar', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders nothing when connected', () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'connected' } })
    expect(wrapper.text()).toBe('')
  })

  it('shows the reconnecting message without a retry count', () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'reconnecting', retryCount: 1 } })
    expect(wrapper.text()).toContain('Reconnexion en cours…')
    expect(wrapper.text()).not.toContain('tentative')
  })

  it('shows the retry attempt count when retryCount > 1', () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'reconnecting', retryCount: 3 } })
    expect(wrapper.text()).toContain('Reconnexion en cours (tentative 3)…')
  })

  it('shows the connecting message', () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'connecting' } })
    expect(wrapper.text()).toContain('Connexion au serveur…')
  })

  it('shows the error title and retry button', () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'error' } })
    expect(wrapper.text()).toContain('Erreur WebSocket')
    expect(wrapper.text()).toContain('Réessayer')
    expect(wrapper.text()).not.toContain('—')
  })

  it('shows the error detail when provided', () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'error', error: 'connexion refusée' } })
    expect(wrapper.text()).toContain('— connexion refusée')
  })

  it('emits reconnect when the retry button is clicked', async () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'error' } })
    await wrapper.find('button.btn-danger').trigger('click')
    expect(wrapper.emitted('reconnect')).toBeTruthy()
  })

  it('shows the data-stale alert and emits dismiss-stale-alert', async () => {
    const wrapper = mount(WsStatusBar, { props: { status: 'connected', dataStaleAlert: true } })
    expect(wrapper.text()).toContain('Données actualisées')
    expect(wrapper.text()).toContain('après reconnexion')
    const closeBtn = wrapper.find('.btn-close')
    expect(closeBtn.attributes('aria-label')).toBe("Fermer l'alerte de fraîcheur")
    await closeBtn.trigger('click')
    expect(wrapper.emitted('dismiss-stale-alert')).toBeTruthy()
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(WsStatusBar, { props: { status: 'reconnecting', retryCount: 2 } })
    expect(wrapper.text()).toContain('Reconnecting (attempt 2)…')
  })
})
