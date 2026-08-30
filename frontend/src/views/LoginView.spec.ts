import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { login, beginWebAuthnLogin, finishWebAuthnLogin } = vi.hoisted(() => ({
  login: vi.fn(),
  beginWebAuthnLogin: vi.fn(),
  finishWebAuthnLogin: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    login,
    beginWebAuthnLogin,
    finishWebAuthnLogin,
    beginDiscoverableWebAuthnLogin: vi.fn().mockRejectedValue(new DOMException('no credential', 'AbortError')),
    finishDiscoverableWebAuthnLogin: vi.fn(),
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
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

  it('renders in English once the UI locale is switched', () => {
    setLocale('en')
    const wrapper = mount(LoginView)
    expect(wrapper.text()).toContain('Sign in to the dashboard')
    expect(wrapper.text()).toContain('Sign in')
  })
})
