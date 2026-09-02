import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

const { login, beginWebAuthnLogin, finishWebAuthnLogin, beginDiscoverableWebAuthnLogin, getOIDCStatus } = vi.hoisted(() => ({
  login: vi.fn(),
  beginWebAuthnLogin: vi.fn(),
  finishWebAuthnLogin: vi.fn(),
  beginDiscoverableWebAuthnLogin: vi.fn().mockRejectedValue(new DOMException('no credential', 'AbortError')),
  getOIDCStatus: vi.fn().mockResolvedValue({ data: { enabled: false, display_name: '', allow_local_login: true } }),
}))

vi.mock('../api', () => ({
  default: {
    login,
    beginWebAuthnLogin,
    finishWebAuthnLogin,
    beginDiscoverableWebAuthnLogin,
    finishDiscoverableWebAuthnLogin: vi.fn(),
    getOIDCStatus,
  },
}))

const mockPush = vi.fn()
const mockQuery = ref<Record<string, string>>({})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mockPush }),
  useRoute: () => ({ query: mockQuery.value }),
}))

vi.mock('../utils/webauthn', () => ({
  isWebAuthnSupported: () => true,
  isConditionalMediationAvailable: vi.fn().mockResolvedValue(false),
  getWebAuthnAssertion: vi.fn(),
}))

import LoginView from './LoginView.vue'
import { setLocale } from '../i18n'

beforeEach(() => {
  setLocale('fr')
  setActivePinia(createPinia())
  vi.clearAllMocks()
  mockQuery.value = {}
  getOIDCStatus.mockResolvedValue({ data: { enabled: false, display_name: '', allow_local_login: true } })
  beginDiscoverableWebAuthnLogin.mockRejectedValue(new DOMException('no credential', 'AbortError'))
})

describe('LoginView', () => {
  it('renders the login form in French by default', () => {
    const wrapper = mount(LoginView)
    expect(wrapper.text()).toContain('Connexion au dashboard')
    expect(wrapper.text()).toContain('Se connecter')
    expect(wrapper.find('input[name="username"]').exists()).toBe(true)
    expect(wrapper.find('input[name="password"]').exists()).toBe(true)
  })

  it('shows the TOTP prompt with a translated hint when the server requires MFA', async () => {
    login.mockResolvedValueOnce({ data: { require_mfa: true, mfa_methods: { totp: true, webauthn: false } } })
    const wrapper = mount(LoginView)

    await wrapper.find('input[name="username"]').setValue('admin')
    await wrapper.find('input[name="password"]').setValue('secret')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Code TOTP')
    expect(wrapper.text()).toContain('Entrez le code de votre application d\'authentification.')
    expect(wrapper.text()).toContain('Vérifier le code')
  })

  it('shows a translated error when the TOTP code is rejected', async () => {
    login.mockResolvedValueOnce({ data: { require_mfa: true, mfa_methods: { totp: true, webauthn: false } } })
    login.mockRejectedValueOnce({ response: { data: {} } })
    const wrapper = mount(LoginView)

    await wrapper.find('input[name="username"]').setValue('admin')
    await wrapper.find('input[name="password"]').setValue('secret')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    await wrapper.find('input[maxlength="6"]').setValue('000000')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Code invalide ou expiré — générez un nouveau code.')
  })

  it('shows a translated fallback error on a plain login failure', async () => {
    login.mockRejectedValueOnce({ response: { data: {} } })
    const wrapper = mount(LoginView)

    await wrapper.find('input[name="username"]').setValue('admin')
    await wrapper.find('input[name="password"]').setValue('wrong')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur de connexion')
  })

  it('shows a translated error when the login response is missing a role', async () => {
    login.mockResolvedValueOnce({ data: {} })
    const wrapper = mount(LoginView)

    await wrapper.find('input[name="username"]').setValue('admin')
    await wrapper.find('input[name="password"]').setValue('secret')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Réponse de connexion invalide.')
  })

  it('offers the security-key button when the server supports it, and shows a translated error if verification fails', async () => {
    login.mockResolvedValueOnce({ data: { require_mfa: true, mfa_methods: { totp: false, webauthn: true } } })
    beginWebAuthnLogin.mockRejectedValueOnce({ response: { data: {} } })
    const wrapper = mount(LoginView)

    await wrapper.find('input[name="username"]').setValue('admin')
    await wrapper.find('input[name="password"]').setValue('secret')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const webauthnButton = wrapper.findAll('button').find((b) => b.text().includes('clé de sécurité'))
    expect(webauthnButton).toBeTruthy()

    await webauthnButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Échec de la vérification de la clé de sécurité')
  })

  it('renders in English once the UI locale is switched', () => {
    setLocale('en')
    const wrapper = mount(LoginView)
    expect(wrapper.text()).toContain('Sign in to the dashboard')
    expect(wrapper.text()).toContain('Sign in')
  })

  it('renders standard username and password fields when OIDC is disabled', async () => {
    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.find('input[name="username"]').exists()).toBe(true)
    expect(wrapper.find('input[name="password"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Se connecter avec')
  })

  it('renders SSO button when OIDC is enabled', async () => {
    getOIDCStatus.mockResolvedValueOnce({
      data: { enabled: true, display_name: 'Authentik SSO', allow_local_login: true },
    })

    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.text()).toContain('Se connecter avec Authentik SSO')
    expect(wrapper.text()).toContain('ou identifiants locaux')
    expect(wrapper.find('input[name="username"]').exists()).toBe(true)
  })

  it('hides local form when OIDC is enabled and allow_local_login is false', async () => {
    getOIDCStatus.mockResolvedValueOnce({
      data: { enabled: true, display_name: 'Enterprise SSO', allow_local_login: false },
    })

    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.text()).toContain('Se connecter avec Enterprise SSO')
    expect(wrapper.text()).not.toContain('ou identifiants locaux')
    expect(wrapper.find('input[name="username"]').exists()).toBe(false)
    expect(wrapper.find('input[name="password"]').exists()).toBe(false)
  })

  it('displays error alert when query error is present', async () => {
    mockQuery.value = { error: 'invalid_token' }

    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.find('.alert-danger').exists()).toBe(true)
    expect(wrapper.find('.alert-danger').text()).toContain('invalid_token')
  })
})
