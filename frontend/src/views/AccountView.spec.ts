import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'

const { getProfile, getCommandsHistory, changePassword } = vi.hoisted(() => ({
  getProfile: vi.fn(),
  getCommandsHistory: vi.fn(),
  changePassword: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getProfile, getCommandsHistory, changePassword },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

import AccountView from './AccountView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

describe('AccountView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getProfile.mockResolvedValue({ data: { username: 'admin', role: 'admin', mfa_enabled: false, created_at: '2026-01-01T00:00:00Z' } })
    getCommandsHistory.mockResolvedValue({ data: { commands: [] } })
  })

  it('renders the translated header, tabs and profile card', async () => {
    const wrapper = mount(AccountView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Mon compte')
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Gérez vos informations personnelles')
    expect(wrapper.text()).toContain('Profil')
    expect(wrapper.text()).toContain('Historique')
    expect(wrapper.text()).toContain('Connexions')
    expect(wrapper.text()).toContain('Membre depuis')
    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('Actif')
  })

  it('renders the translated MFA card and password form', async () => {
    const wrapper = mount(AccountView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Authentification à deux facteurs')
    expect(wrapper.text()).toContain('Inactif')
    expect(wrapper.text()).toContain('Gérer le MFA du compte')
    expect(wrapper.text()).toContain('Changer le mot de passe')
    expect(wrapper.text()).toContain('Mot de passe actuel')
    expect(wrapper.text()).toContain('Nouveau mot de passe')
    expect(wrapper.text()).toContain('Confirmer le nouveau mot de passe')
    expect(wrapper.text()).toContain('Au moins 8 caractères.')
    expect(wrapper.text()).toContain('Mettre à jour le mot de passe')
  })

  it('shows the translated password-strength label once typing starts', async () => {
    const wrapper = mount(AccountView, mountOpts)
    await flushPromises()

    const inputs = wrapper.findAll('input[type="password"]')
    await inputs[1].setValue('abc12345XY')
    expect(wrapper.text()).toContain('Force :')
  })

  it('renders the translated history table headers and empty state', async () => {
    const wrapper = mount(AccountView, mountOpts)
    await flushPromises()
    await wrapper.findAll('button.nav-link')[1].trigger('click')

    expect(wrapper.text()).toContain('Date')
    expect(wrapper.text()).toContain('Hôte')
    expect(wrapper.text()).toContain('Commande')
    expect(wrapper.text()).toContain('Durée')
    expect(wrapper.text()).toContain('Aucune activité récente')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(AccountView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('My account')
    expect(wrapper.text()).toContain('Profile')
    expect(wrapper.text()).toContain('Two-factor authentication')
    expect(wrapper.text()).toContain('Current password')
  })
})
