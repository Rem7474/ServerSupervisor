import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import LoginView from './LoginView.vue'

const mockPush = vi.fn()
const mockQuery = ref<Record<string, string>>({})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mockPush }),
  useRoute: () => ({ query: mockQuery.value }),
}))

const mockGetOIDCStatus = vi.fn()
const mockLogin = vi.fn()

vi.mock('../api', () => ({
  default: {
    getOIDCStatus: () => mockGetOIDCStatus(),
    login: (...args: unknown[]) => mockLogin(...args),
    beginDiscoverableWebAuthnLogin: vi.fn(),
  },
}))

vi.mock('../utils/webauthn', () => ({
  isWebAuthnSupported: () => false,
  isConditionalMediationAvailable: () => Promise.resolve(false),
  getWebAuthnAssertion: vi.fn(),
}))

describe('views/LoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockPush.mockReset()
    mockQuery.value = {}
    mockGetOIDCStatus.mockReset()
    mockLogin.mockReset()
  })

  it('renders standard username and password fields when OIDC is disabled', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({
      data: { enabled: false, display_name: '', allow_local_login: true },
    })

    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.find('input[name="username"]').exists()).toBe(true)
    expect(wrapper.find('input[name="password"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Se connecter avec')
  })

  it('renders SSO button when OIDC is enabled', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({
      data: { enabled: true, display_name: 'Authentik SSO', allow_local_login: true },
    })

    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.text()).toContain('Se connecter avec Authentik SSO')
    expect(wrapper.text()).toContain('ou identifiants locaux')
    expect(wrapper.find('input[name="username"]').exists()).toBe(true)
  })

  it('hides local form when OIDC is enabled and allow_local_login is false', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({
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
    mockGetOIDCStatus.mockResolvedValueOnce({ data: { enabled: false } })

    const wrapper = mount(LoginView)
    await flushPromises()

    expect(wrapper.find('.alert-danger').exists()).toBe(true)
    expect(wrapper.find('.alert-danger').text()).toContain('invalid_token')
  })
})
