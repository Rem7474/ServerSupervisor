import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { getReleaseTracker, getHosts, getReleaseTrackerVersionHistory, getReleaseTrackerExecutions } = vi.hoisted(() => ({
  getReleaseTracker: vi.fn(),
  getHosts: vi.fn(),
  getReleaseTrackerVersionHistory: vi.fn(),
  getReleaseTrackerExecutions: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getReleaseTracker, getHosts, getReleaseTrackerVersionHistory, getReleaseTrackerExecutions,
    getHostComposeProjects: vi.fn(async () => ({ data: [] })),
    getHostTasksYaml: vi.fn(async () => ({ data: { yaml: '' } })),
  },
  getApiErrorMessage: (e: unknown, fallback?: string) => fallback || String(e),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'tr-1' } }),
}))

import ReleaseTrackerDetailView from './ReleaseTrackerDetailView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function baseTracker(overrides: Record<string, unknown> = {}) {
  return {
    id: 'tr-1', name: 'HA tracker', enabled: true, provider: 'github',
    repo_owner: 'home-assistant', repo_name: 'core', tracker_type: 'git',
    host_id: '', custom_task_id: '',
    ...overrides,
  }
}

describe('ReleaseTrackerDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker(), executions: [] } })
    getHosts.mockResolvedValue({ data: [] })
    getReleaseTrackerVersionHistory.mockResolvedValue({ data: { history: [] } })
    getReleaseTrackerExecutions.mockResolvedValue({ data: { executions: [] } })
  })

  it('renders the translated breadcrumb, disabled badge and no-task alert', async () => {
    getReleaseTracker.mockResolvedValue({
      data: { tracker: baseTracker({ enabled: false, host_id: 'h1' }), executions: [] },
    })
    const wrapper = mount(ReleaseTrackerDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Suivi de versions')
    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('Aucune tâche configurée')
    expect(wrapper.text()).toContain('Créer une tâche')
    expect(wrapper.text()).toContain('Historique des exécutions')
  })

  it('renders the translated latest-release-detected card', async () => {
    getReleaseTracker.mockResolvedValue({
      data: {
        tracker: baseTracker({ last_release_tag: 'v1.2.3', docker_image: 'nginx', release_url: 'https://x' }),
        executions: [],
      },
    })
    const wrapper = mount(ReleaseTrackerDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Dernière version détectée')
    expect(wrapper.text()).toContain('Image & tag')
    expect(wrapper.text()).toContain('Voir sur GitHub')
  })

  it('shows the translated cooldown-active badge with a tooltip', async () => {
    getReleaseTracker.mockResolvedValue({
      data: {
        tracker: baseTracker({ cooldown_hours: 4, last_release_detected_at: new Date(Date.now() - 1000).toISOString() }),
        executions: [],
      },
    })
    const wrapper = mount(ReleaseTrackerDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Cooldown actif · reste')
    const badge = wrapper.find('.badge.bg-warning-lt')
    expect(badge.attributes('title')).toContain('Déploiement prévu:')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    getReleaseTracker.mockResolvedValue({
      data: { tracker: baseTracker({ host_id: 'h1' }), executions: [] },
    })
    const wrapper = mount(ReleaseTrackerDetailView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Version tracking')
    expect(wrapper.text()).toContain('No task configured')
    expect(wrapper.text()).toContain('Execution history')
  })
})
