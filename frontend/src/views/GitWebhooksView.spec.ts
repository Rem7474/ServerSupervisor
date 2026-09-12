import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { getGitWebhooks, getReleaseTrackers, getHosts } = vi.hoisted(() => ({
  getGitWebhooks: vi.fn(),
  getReleaseTrackers: vi.fn(),
  getHosts: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getGitWebhooks, getReleaseTrackers, getHosts },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))

import GitWebhooksView from './GitWebhooksView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function webhook(overrides: Record<string, unknown> = {}) {
  return {
    id: 'wh-1', name: 'Deploy app', provider: 'github', enabled: true,
    repo_filter: '', branch_filter: '', host_id: 'h1', host_name: 'srv-web', custom_task_id: 'deploy',
    ...overrides,
  }
}

function tracker(overrides: Record<string, unknown> = {}) {
  return {
    id: 'tr-1', name: 'HA tracker', enabled: true, provider: 'github',
    repo_owner: 'home-assistant', repo_name: 'core', tracker_type: 'git',
    ...overrides,
  }
}

describe('GitWebhooksView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getGitWebhooks.mockResolvedValue({ data: { webhooks: [] } })
    getReleaseTrackers.mockResolvedValue({ data: { trackers: [] } })
    getHosts.mockResolvedValue({ data: [] })
  })

  it('renders the translated page header, tabs and empty states', async () => {
    const wrapper = mount(GitWebhooksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Git / Automatisation')
    expect(wrapper.text()).toContain('Webhooks entrants et suivi de releases')
    expect(wrapper.text()).toContain('Webhooks entrants')
    expect(wrapper.text()).toContain('Suivi de versions')
    expect(wrapper.text()).toContain('Nouveau webhook')
    expect(wrapper.text()).toContain('Aucun webhook configuré.')
    expect(wrapper.text()).toContain('Créer le premier webhook')
  })

  it('renders the translated webhook card content', async () => {
    getGitWebhooks.mockResolvedValue({ data: { webhooks: [webhook({ enabled: false })] } })
    const wrapper = mount(GitWebhooksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('<tous>')
    expect(wrapper.text()).toContain('<toutes>')
    expect(wrapper.text()).toContain('Jamais déclenché')
    expect(wrapper.text()).toContain('Détails')
    expect(wrapper.text()).toContain('Modifier')
    expect(wrapper.text()).toContain('Activer')
  })

  it('renders the translated tracker card content and empty state on the trackers tab', async () => {
    getReleaseTrackers.mockResolvedValue({ data: { trackers: [tracker()] } })
    const wrapper = mount(GitWebhooksView, mountOpts)
    await flushPromises()
    await wrapper.find('a.nav-link:nth-of-type(1)').trigger('click')
    const trackersTabLink = wrapper.findAll('a.nav-link')[1]
    await trackersTabLink.trigger('click')

    expect(wrapper.text()).toContain('Repo')
    expect(wrapper.text()).toContain('Vérifiée')
    expect(wrapper.text()).toContain('En attente du premier check...')
    expect(wrapper.text()).toContain('Dernières exécutions des trackers')
  })

  it('renders the monitor-only badge for a tracker with no dispatch target', async () => {
    getReleaseTrackers.mockResolvedValue({
      data: { trackers: [tracker({ host_id: '', custom_task_id: '', update_action: 'custom' })] },
    })
    const wrapper = mount(GitWebhooksView, mountOpts)
    await flushPromises()
    const trackersTabLink = wrapper.findAll('a.nav-link')[1]
    await trackersTabLink.trigger('click')

    expect(wrapper.text()).toContain('Surveillance seule')
  })

  it('shows the translated delete-confirmation title and message for a webhook and a tracker', async () => {
    const dialog = useConfirmDialog()
    getGitWebhooks.mockResolvedValue({ data: { webhooks: [webhook({ name: 'Deploy app' })] } })
    getReleaseTrackers.mockResolvedValue({ data: { trackers: [tracker({ name: 'HA tracker' })] } })
    const wrapper = mount(GitWebhooksView, mountOpts)
    await flushPromises()

    const deleteWebhookBtn = wrapper.find('button.btn-ghost-danger')
    deleteWebhookBtn.trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe('Supprimer le webhook "Deploy app" ?')
    expect(dialog.message.value).toBe('Toutes les exécutions associées seront également supprimées.')
    dialog.onCancel()
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    getGitWebhooks.mockResolvedValue({ data: { webhooks: [webhook()] } })
    const wrapper = mount(GitWebhooksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Git / Automation')
    expect(wrapper.text()).toContain('Incoming webhooks')
    expect(wrapper.text()).toContain('Version tracking')
    expect(wrapper.text()).toContain('Never triggered')
  })
})
