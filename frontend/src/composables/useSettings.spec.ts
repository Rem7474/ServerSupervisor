import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'

const {
  getSettings,
  updateSettings,
  testSmtp,
  testNtfy,
  cleanupMetrics,
  cleanupAudit,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  testSmtp: vi.fn(),
  testNtfy: vi.fn(),
  cleanupMetrics: vi.fn(),
  cleanupAudit: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getSettings,
    updateSettings,
    testSmtp,
    testNtfy,
    cleanupMetrics,
    cleanupAudit,
  },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e instanceof Error && e.message ? e.message : fallback) || String(e),
}))

const replaceMock = vi.fn()
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: replaceMock }),
}))

import { useSettings } from './useSettings'

function mountComposable() {
  let api!: ReturnType<typeof useSettings>
  mount(
    defineComponent({
      setup() {
        api = useSettings()
        return () => h('div')
      },
    })
  )
  return api
}

describe('useSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    setLocale('fr')
    setActivePinia(createPinia())
    getSettings.mockResolvedValue({
      data: {
        settings: {
          smtpHost: 'mail.example.com',
          smtpPort: 587,
          smtpUser: 'mailer',
          smtpPass: '',
          smtpPassConfigured: true,
          smtpFrom: 'noreply@example.com',
          smtpTo: 'admin@example.com',
          smtpTls: true,
          ntfyUrl: 'https://ntfy.sh/topic',
          githubToken: '',
          githubTokenConfigured: true,
          metricsRetentionDays: 30,
          auditRetentionDays: 90,
          auditCategories: [],
        },
        dbStatus: { connected: true, auditLogCount: 1, metricsCount: 2, hostsCount: 3 },
      },
    })
    updateSettings.mockResolvedValue({})
  })

  it('does not call getSettings on mount when the user is not an admin', async () => {
    useAuthStore().setAuth({ role: 'operator', username: 'carol' }, 'carol')
    mountComposable()
    await flushPromises()
    expect(getSettings).not.toHaveBeenCalled()
  })

  it('fetches settings on mount for admin and hydrates secret configured flags without secrets', async () => {
    useAuthStore().setAuth({ role: 'admin', username: 'alice' }, 'alice')
    const api = mountComposable()
    await flushPromises()

    expect(getSettings).toHaveBeenCalledTimes(1)
    expect(api.settings.value.smtpPassConfigured).toBe(true)
    expect(api.settings.value.githubTokenConfigured).toBe(true)
    expect(api.form.value.smtpPass).toBe('')
    expect(api.form.value.githubToken).toBe('')
  })

  it('saveSmtp omits smtp_pass when blank or masked sentinel and never sends github_token', async () => {
    useAuthStore().setAuth({ role: 'admin', username: 'alice' }, 'alice')
    const api = mountComposable()
    await flushPromises()

    await api.saveSmtp()
    expect(updateSettings).toHaveBeenCalledTimes(1)
    const firstPayload = updateSettings.mock.calls[0][0]
    expect(firstPayload).not.toHaveProperty('smtp_pass')
    expect(firstPayload).not.toHaveProperty('github_token')
    expect(firstPayload).not.toHaveProperty('ntfy_url')

    api.form.value.smtpPass = '••••••••'
    await api.saveSmtp()
    const sentinelPayload = updateSettings.mock.calls[1][0]
    expect(sentinelPayload).not.toHaveProperty('smtp_pass')

    api.form.value.smtpPass = 'new-smtp-secret'
    await api.saveSmtp()
    const thirdPayload = updateSettings.mock.calls[2][0]
    expect(thirdPayload.smtp_pass).toBe('new-smtp-secret')
    expect(thirdPayload).not.toHaveProperty('github_token')
    expect(api.form.value.smtpPass).toBe('')
  })

  it('saveNotifications omits github_token when blank or masked sentinel and never sends smtp fields', async () => {
    useAuthStore().setAuth({ role: 'admin', username: 'alice' }, 'alice')
    const api = mountComposable()
    await flushPromises()

    await api.saveNotifications()
    expect(updateSettings).toHaveBeenCalledTimes(1)
    const firstPayload = updateSettings.mock.calls[0][0]
    expect(firstPayload).toEqual({ ntfy_url: 'https://ntfy.sh/topic' })
    expect(firstPayload).not.toHaveProperty('github_token')
    expect(firstPayload).not.toHaveProperty('smtp_pass')

    api.form.value.githubToken = '••••••••'
    await api.saveNotifications()
    expect(updateSettings.mock.calls[1][0]).not.toHaveProperty('github_token')

    api.form.value.githubToken = 'ghp_real_token'
    await api.saveNotifications()
    expect(updateSettings.mock.calls[2][0]).toEqual({
      ntfy_url: 'https://ntfy.sh/topic',
      github_token: 'ghp_real_token',
    })
    expect(api.form.value.githubToken).toBe('')
  })
})
