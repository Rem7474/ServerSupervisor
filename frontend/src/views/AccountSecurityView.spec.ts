import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { getMFAStatus, getLoginEvents, setupMFA, disableMFA, listWebAuthnCredentials } = vi.hoisted(() => ({
  getMFAStatus: vi.fn(),
  getLoginEvents: vi.fn(),
  setupMFA: vi.fn(),
  disableMFA: vi.fn(),
  listWebAuthnCredentials: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getMFAStatus, getLoginEvents, setupMFA, disableMFA, listWebAuthnCredentials },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

vi.mock('../utils/webauthn', () => ({
  isWebAuthnSupported: () => true,
  createWebAuthnCredential: vi.fn(),
}))

import AccountSecurityView from './AccountSecurityView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

describe('AccountSecurityView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: false } })
    getLoginEvents.mockResolvedValue({ data: { events: [] } })
    listWebAuthnCredentials.mockResolvedValue({ data: { credentials: [] } })
  })

  it('renders the translated header and MFA-disabled state', async () => {
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Authentification MFA')
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Mon compte')
    expect(wrapper.text()).toContain('Sécurité du compte')
    expect(wrapper.text()).toContain('Authentification multi-facteur')
    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('Activez le MFA pour renforcer la sécurité du compte.')
    expect(wrapper.text()).toContain('Activer MFA')
  })

  it('shows the MFA-enabled state and disable panel', async () => {
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: true } })
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Activé')
    expect(wrapper.text()).toContain('Le MFA est actif.')
    await wrapper.find('button.btn-outline-danger').trigger('click')
    expect(wrapper.text()).toContain('Mot de passe')
    expect(wrapper.text()).toContain('Confirmer la désactivation')
  })

  it('shows the setup panel after starting MFA setup', async () => {
    setupMFA.mockResolvedValue({ data: { secret: 'ABC123', qr_code: 'data:image/png;base64,x', backup_codes: [] } })
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Configuration MFA')
    expect(wrapper.text()).toContain('Clé secrète')
    expect(wrapper.text()).toContain('Code TOTP')
    expect(wrapper.text()).toContain('Vérifier et activer')
  })

  it('renders the passkeys empty state and add-key form', async () => {
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Clés de sécurité / Passkeys')
    expect(wrapper.text()).toContain('Aucune clé de sécurité enregistrée.')
    await wrapper.find('button.btn-outline-primary').trigger('click')
    expect(wrapper.text()).toContain('Nom de la clé (facultatif)')
    expect(wrapper.text()).toContain('Enregistrer cette clé')
  })

  it('renders the translated login-history section and revoke button', async () => {
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Historique de connexion')
    expect(wrapper.text()).toContain('Révoquer les autres sessions')
    expect(wrapper.text()).toContain('Connexions récentes associées à votre compte.')
  })

  it('shows the translated revoke-sessions confirmation dialog', async () => {
    const dialog = useConfirmDialog()
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    wrapper.find('button.btn-outline-danger').trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe('Révoquer les autres sessions')
    expect(dialog.message.value).toContain('Tous vos autres appareils/onglets connectés')
    dialog.onCancel()
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(AccountSecurityView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('MFA authentication')
    expect(wrapper.text()).toContain('Account security')
    expect(wrapper.text()).toContain('Enable MFA')
    expect(wrapper.text()).toContain('Security keys / Passkeys')
    expect(wrapper.text()).toContain('Login history')
  })
})
