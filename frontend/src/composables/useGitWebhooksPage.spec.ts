import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const api = vi.hoisted(() => ({
  getGitWebhooks: vi.fn(),
  getReleaseTrackers: vi.fn(),
  getHosts: vi.fn(),
  createGitWebhook: vi.fn(),
  updateGitWebhook: vi.fn(),
  deleteGitWebhook: vi.fn(),
  updateReleaseTracker: vi.fn(),
  deleteReleaseTracker: vi.fn(),
  checkReleaseTrackerNow: vi.fn(),
}))

vi.mock('../api', async () => ({
  default: api,
  getApiErrorMessage: (await vi.importActual<typeof import('../api/client')>('../api/client')).getApiErrorMessage,
}))

vi.mock('./useConfirmDialog', () => ({
  useConfirmDialog: () => ({ confirm: vi.fn(async () => true) }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))

import { useGitWebhooksPage } from './useGitWebhooksPage'

// useI18n()/useConfirmDialog() both need an active component instance.
function mountHost() {
  let api: ReturnType<typeof useGitWebhooksPage> | undefined
  mount(defineComponent({
    setup() {
      api = useGitWebhooksPage()
      return () => h('div')
    },
  }))
  return api!
}

function resetApi() {
  for (const fn of Object.values(api)) fn.mockReset()
  api.getGitWebhooks.mockResolvedValue({ data: { webhooks: [] } })
  api.getReleaseTrackers.mockResolvedValue({ data: { trackers: [] } })
  api.getHosts.mockResolvedValue({ data: [] })
}

const serverError = (msg: string) => ({ isAxiosError: true, message: 'Request failed', response: { data: { error: msg } } })
const networkError = { isAxiosError: true, message: 'Network Error' }

describe('useGitWebhooksPage — locale-dependent formatting', () => {
  beforeEach(() => {
    setLocale('fr')
    resetApi()
  })

  it('formats a multi-day cooldown remaining label with the French day suffix', () => {
    const api = mountHost()
    const tracker = {
      id: 't1', name: 'x', enabled: true, provider: 'github', repo_owner: 'a', repo_name: 'b',
      cooldown_hours: 240, last_release_detected_at: new Date(Date.now() - 1000).toISOString(),
    }
    expect(api.cooldownRemainingLabel(tracker)).toMatch(/^\d+j \d+h$/)
  })

  it('switches the day suffix to English when the locale changes', () => {
    setLocale('en')
    const api = mountHost()
    const tracker = {
      id: 't1', name: 'x', enabled: true, provider: 'github', repo_owner: 'a', repo_name: 'b',
      cooldown_hours: 240, last_release_detected_at: new Date(Date.now() - 1000).toISOString(),
    }
    expect(api.cooldownRemainingLabel(tracker)).toMatch(/^\d+d \d+h$/)
  })

  it('formatRelative/formatDateOnly fall back to a dash for an empty date', () => {
    const api = mountHost()
    expect(api.formatRelative('')).toBe('-')
    expect(api.formatDateOnly(undefined)).toBe('-')
  })
})

describe('useGitWebhooksPage — error reporting', () => {
  const webhook = { id: 'w1', name: 'deploy', enabled: true } as never
  const tracker = { id: 't1', name: 'app', enabled: true } as never

  beforeEach(() => {
    setLocale('fr')
    resetApi()
  })

  it('shows the server message when loading webhooks fails', async () => {
    api.getGitWebhooks.mockRejectedValue(serverError('base indisponible'))
    const page = mountHost()
    await flushPromises()
    expect(page.error.value).toBe('base indisponible')
  })

  it('falls back to the translated message, not axios jargon, when trackers cannot load', async () => {
    api.getReleaseTrackers.mockRejectedValue(networkError)
    const page = mountHost()
    await flushPromises()
    expect(page.error.value).not.toContain('Network Error')
    expect(page.error.value).not.toBe('')
  })

  it('keeps the webhook modal open with the error when saving fails', async () => {
    const page = mountHost()
    await flushPromises()
    page.openCreateWebhook()
    api.createGitWebhook.mockRejectedValue(serverError('nom déjà utilisé'))
    await page.saveWebhook({} as never)
    expect(page.modalError.value).toBe('nom déjà utilisé')
    expect(page.showWebhookModal.value).toBe(true)
    expect(page.saving.value).toBe(false)
  })

  it('keeps the tracker modal open with the error when saving fails', async () => {
    const page = mountHost()
    await flushPromises()
    page.openEditTracker(tracker)
    api.updateReleaseTracker.mockRejectedValue(serverError('dépôt introuvable'))
    await page.saveTracker({} as never)
    expect(page.modalError.value).toBe('dépôt introuvable')
    expect(page.showTrackerModal.value).toBe(true)
  })

  it.each([
    ['toggleWebhook', 'updateGitWebhook', webhook],
    ['toggleTracker', 'updateReleaseTracker', tracker],
    ['checkNow', 'checkReleaseTrackerNow', tracker],
    ['confirmDeleteWebhook', 'deleteGitWebhook', webhook],
    ['confirmDeleteTracker', 'deleteReleaseTracker', tracker],
  ] as const)('%s surfaces a failed %s call on the page', async (action, endpoint, item) => {
    const page = mountHost()
    await flushPromises()
    api[endpoint].mockRejectedValue(serverError(`${endpoint} refusé`))
    await (page[action] as (x: unknown) => Promise<void>)(item)
    expect(page.error.value).toBe(`${endpoint} refusé`)
  })
})
