import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { ref, defineComponent } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'

const mockPush = vi.fn()
const mockQuery = ref<Record<string, string>>({})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mockPush }),
  useRoute: () => ({ query: mockQuery.value }),
}))

const mockGetOIDCStatus = vi.fn()
const mockLogin = vi.fn()
const mockBeginWebAuthnLogin = vi.fn()
const mockFinishWebAuthnLogin = vi.fn()
const mockBeginDiscoverableWebAuthnLogin = vi.fn()
const mockFinishDiscoverableWebAuthnLogin = vi.fn()

vi.mock('../api', () => ({
  default: {
    getOIDCStatus: () => mockGetOIDCStatus(),
    login: (...args: unknown[]) => mockLogin(...args),
    beginWebAuthnLogin: (...args: unknown[]) => mockBeginWebAuthnLogin(...args),
    finishWebAuthnLogin: (...args: unknown[]) => mockFinishWebAuthnLogin(...args),
    beginDiscoverableWebAuthnLogin: () => mockBeginDiscoverableWebAuthnLogin(),
    finishDiscoverableWebAuthnLogin: (...args: unknown[]) => mockFinishDiscoverableWebAuthnLogin(...args),
  },
}))

vi.mock('../utils/webauthn', () => ({
  isWebAuthnSupported: () => false,
  isConditionalMediationAvailable: () => Promise.resolve(false),
  getWebAuthnAssertion: vi.fn(),
}))

import { useLogin } from './useLogin'

function withSetup<T>(composable: () => T) {
  let result!: T
  const wrapper = mount(
    defineComponent({
      setup() {
        result = composable()
        return () => null
      },
    }),
  )
  return { result, wrapper }
}

describe('composables/useLogin', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockPush.mockReset()
    mockQuery.value = {}
    mockGetOIDCStatus.mockReset()
    mockLogin.mockReset()
  })

  it('initializes default state and loads OIDC status on mount', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({
      data: { enabled: true, display_name: 'Keycloak SSO', allow_local_login: true },
    })

    const { result: login } = withSetup(() => useLogin())
    expect(login.username.value).toBe('')
    expect(login.password.value).toBe('')
    expect(login.needsMFA.value).toBe(false)

    await flushPromises()

    expect(login.oidcStatus.value?.enabled).toBe(true)
    expect(login.oidcStatus.value?.display_name).toBe('Keycloak SSO')
  })

  it('parses error query params on mount', async () => {
    mockQuery.value = { error: 'sso_failed', error_description: 'Access denied' }
    mockGetOIDCStatus.mockRejectedValueOnce(new Error('Network error'))

    const { result: login } = withSetup(() => useLogin())
    await flushPromises()

    expect(login.error.value).toBe('sso_failed: Access denied')
  })

  it('handles standard successful login', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({ data: { enabled: false } })
    mockLogin.mockResolvedValueOnce({
      data: { role: 'admin', username: 'admin' },
    })

    const { result: login } = withSetup(() => useLogin())
    login.username.value = 'admin'
    login.password.value = 'secret123'

    await login.handleLogin()

    expect(mockPush).toHaveBeenCalledWith('/')
    expect(login.error.value).toBe('')
  })

  it('handles MFA requirement response', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({ data: { enabled: false } })
    mockLogin.mockResolvedValueOnce({
      data: { require_mfa: true, mfa_methods: { totp: true, webauthn: false } },
    })

    const { result: login } = withSetup(() => useLogin())
    login.username.value = 'admin'
    login.password.value = 'secret123'

    await login.handleLogin()

    expect(login.needsMFA.value).toBe(true)
    expect(login.mfaMethods.value?.totp).toBe(true)
  })

  it('handles login failure and surfaces error message', async () => {
    mockGetOIDCStatus.mockResolvedValueOnce({ data: { enabled: false } })
    mockLogin.mockRejectedValueOnce({
      response: { data: { error: 'Identifiants invalides' } },
    })

    const { result: login } = withSetup(() => useLogin())
    login.username.value = 'admin'
    login.password.value = 'badpass'

    await login.handleLogin()

    expect(login.error.value).toBe('Identifiants invalides')
  })
})
