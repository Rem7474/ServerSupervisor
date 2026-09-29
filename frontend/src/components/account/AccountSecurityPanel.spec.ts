import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import { useAuthStore } from '../../stores/auth'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const { getMFAStatus, getLoginEvents, setupMFA, disableMFA, listWebAuthnCredentials } = vi.hoisted(() => ({
  getMFAStatus: vi.fn(),
  getLoginEvents: vi.fn(),
  setupMFA: vi.fn(),
  disableMFA: vi.fn(),
  listWebAuthnCredentials: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { getMFAStatus, getLoginEvents, setupMFA, disableMFA, listWebAuthnCredentials },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

vi.mock('../../utils/webauthn', () => ({
  isWebAuthnSupported: () => true,
  createWebAuthnCredential: vi.fn(),
}))

import AccountSecurityPanel from './AccountSecurityPanel.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

describe('AccountSecurityPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: false } })
    getLoginEvents.mockResolvedValue({ data: { events: [] } })
    listWebAuthnCredentials.mockResolvedValue({ data: { credentials: [] } })
  })

  it('renders the MFA-disabled state', async () => {
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Authentification multi-facteur')
    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('Activez le MFA pour renforcer la sécurité du compte.')
    expect(wrapper.text()).toContain('Activer MFA')
  })

  it('shows the MFA-enabled state and disable panel', async () => {
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: true } })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Activé')
    expect(wrapper.text()).toContain('Le MFA est actif.')
    await wrapper.find('button.btn-outline-danger').trigger('click')
    expect(wrapper.text()).toContain('Mot de passe')
    expect(wrapper.text()).toContain('Confirmer la désactivation')
  })

  it('shows the setup panel after starting MFA setup', async () => {
    setupMFA.mockResolvedValue({ data: { secret: 'ABC123', qr_code: 'data:image/png;base64,x', backup_codes: [] } })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Configuration MFA')
    expect(wrapper.text()).toContain('Clé secrète')
    expect(wrapper.text()).toContain('Code TOTP')
    expect(wrapper.text()).toContain('Vérifier et activer')
  })

  it('renders the passkeys empty state and add-key form', async () => {
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Clés de sécurité / Passkeys')
    expect(wrapper.text()).toContain('Aucune clé de sécurité enregistrée.')
    await wrapper.find('button.btn-outline-primary').trigger('click')
    expect(wrapper.text()).toContain('Nom de la clé (facultatif)')
    expect(wrapper.text()).toContain('Enregistrer cette clé')
  })

  it('renders the translated login-history section and revoke button', async () => {
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Historique de connexion')
    expect(wrapper.text()).toContain('Révoquer les autres sessions')
    expect(wrapper.text()).toContain('Connexions récentes associées à votre compte.')
  })

  it('shows the translated revoke-sessions confirmation dialog', async () => {
    const dialog = useConfirmDialog()
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    wrapper.find('button.btn-outline-danger').trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe('Révoquer les autres sessions')
    expect(dialog.message.value).toContain('Tous vos autres appareils/onglets connectés')
    dialog.onCancel()
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Enable MFA')
    expect(wrapper.text()).toContain('Security keys / Passkeys')
    expect(wrapper.text()).toContain('Login history')
  })
  it('lists registered passkeys with their column headers and last-used date', async () => {
    listWebAuthnCredentials.mockResolvedValue({
      data: { credentials: [
        { id: 'c1', name: 'YubiKey', created_at: '2026-01-02T10:00:00Z', last_used_at: '2026-02-03T10:00:00Z' },
        { id: 'c2', name: '', created_at: '2026-01-05T10:00:00Z' },
      ] },
    })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    expect(wrapper.findAll('table')[0].findAll('thead th').map((th) => th.text())).toEqual(['Nom', 'Détails', 'Actions'])
    expect(wrapper.findAll('table')[0].findAll('tbody tr')).toHaveLength(2)
    expect(wrapper.text()).toContain('YubiKey')
    expect(wrapper.findAll('table')[0].findAll('tbody tr')[0].text()).toContain('dernière utilisation')
    expect(wrapper.findAll('table')[0].findAll('tbody tr')[1].text()).not.toContain('dernière utilisation')
  })

  it('shows backup codes returned by the setup call', async () => {
    setupMFA.mockResolvedValue({ data: { secret: 'S', qr_code: 'data:image/png;base64,x', backup_codes: ['aaa-111', 'bbb-222'] } })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.find('pre').text()).toBe('aaa-111\nbbb-222')
  })

  it('surfaces a setup failure and keeps the setup panel closed', async () => {
    setupMFA.mockRejectedValue({ response: { data: { error: 'setup boom' } } })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.find('.alert-danger').text()).toContain('setup boom')
    expect(wrapper.text()).not.toContain('Configuration MFA')
  })

  it('disables MFA only once a password is entered and closes the panel on cancel', async () => {
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: true } })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-outline-danger').trigger('click')
    const confirm = wrapper.find('button.btn-danger')
    expect(confirm.attributes('disabled')).toBeDefined()
    await wrapper.find('#mfa-disable-password').setValue('secret')
    expect(wrapper.find('button.btn-danger').attributes('disabled')).toBeUndefined()

    const cancel = wrapper.findAll('button').find((b) => b.text() === 'Annuler')!
    await cancel.trigger('click')
    expect(wrapper.find('#mfa-disable-password').exists()).toBe(false)
  })

  it('associates every form label with its input', async () => {
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: false } })
    setupMFA.mockResolvedValue({ data: { secret: 'S', qr_code: 'data:image/png;base64,x', backup_codes: [] } })
    const wrapper = mount(AccountSecurityPanel, mountOpts)
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')
    await wrapper.find('button.btn-outline-primary').trigger('click')
    await flushPromises()

    for (const label of wrapper.findAll('label')) {
      const id = label.attributes('for')
      expect(id).toBeTruthy()
      expect(wrapper.find(`#${id}`).exists()).toBe(true)
    }
  })

})
