import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { getGitWebhook, getHosts, getWebhookExecutions, getCommandStatus } = vi.hoisted(() => ({
  getGitWebhook: vi.fn(),
  getHosts: vi.fn(),
  getWebhookExecutions: vi.fn(),
  getCommandStatus: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getGitWebhook, getHosts, getWebhookExecutions, getCommandStatus },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'wh-1' } }),
}))

import GitWebhookDetailView from './GitWebhookDetailView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function baseWebhook(overrides: Record<string, unknown> = {}) {
  return {
    id: 'wh-1',
    name: 'Deploy app',
    provider: 'github',
    event_filter: 'push',
    repo_filter: '',
    branch_filter: '',
    host_id: 'h1',
    host_name: 'srv-web',
    custom_task_id: 'deploy-app',
    notify_channels: [],
    enabled: true,
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('GitWebhookDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getGitWebhook.mockResolvedValue({ data: { webhook: baseWebhook(), executions: [] } })
    getHosts.mockResolvedValue({ data: [] })
    getWebhookExecutions.mockResolvedValue({ data: { executions: [] } })
  })

  it('renders the translated config labels, breadcrumb and env vars table for an enabled webhook', async () => {
    const wrapper = mount(GitWebhookDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Git Webhooks')
    expect(wrapper.text()).toContain('Configuration')
    expect(wrapper.text()).toContain('Modifier')
    expect(wrapper.text()).toContain('Événement')
    expect(wrapper.text()).toContain('Filtre repo')
    expect(wrapper.text()).toContain('<tous>')
    expect(wrapper.text()).toContain('Filtre branche')
    expect(wrapper.text()).toContain('<toutes>')
    expect(wrapper.text()).toContain('VM cible')
    expect(wrapper.text()).toContain('Variables disponibles dans le script')
    expect(wrapper.text()).toContain('SS_REPO_NAME')
    expect(wrapper.text()).toContain('Historique des exécutions')
    expect(wrapper.text()).not.toContain('Désactivé')
  })

  it('shows the disabled badge and the notifications summary for a disabled webhook with channels', async () => {
    getGitWebhook.mockResolvedValue({
      data: {
        webhook: baseWebhook({ enabled: false, notify_channels: ['smtp'], notify_on_success: true, notify_on_failure: false }),
        executions: [],
      },
    })
    const wrapper = mount(GitWebhookDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('Notifications')
    expect(wrapper.text()).toContain('succès')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(GitWebhookDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Configuration')
    expect(wrapper.text()).toContain('Edit')
    expect(wrapper.text()).toContain('Variables available in the script')
  })
})
