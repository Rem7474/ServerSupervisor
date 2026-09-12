import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  default: {
    getGitWebhooks: vi.fn(async () => ({ data: { webhooks: [] } })),
    getReleaseTrackers: vi.fn(async () => ({ data: { trackers: [] } })),
    getHosts: vi.fn(async () => ({ data: [] })),
  },
  getApiErrorMessage: (e: unknown, fallback?: string) => fallback || String(e),
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

describe('useGitWebhooksPage — locale-dependent formatting', () => {
  beforeEach(() => {
    setLocale('fr')
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
