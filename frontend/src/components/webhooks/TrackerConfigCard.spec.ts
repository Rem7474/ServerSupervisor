import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import TrackerConfigCard from './TrackerConfigCard.vue'
import type { ReleaseTracker } from '../../types/tracker'

function tracker(overrides: Partial<ReleaseTracker> = {}): ReleaseTracker {
  return {
    id: 't1', name: '', tracker_type: 'git', provider: 'github', repo_owner: 'home-assistant', repo_name: 'core',
    docker_image: '', docker_tag: '', host_id: '', custom_task_id: '', last_release_tag: '', cooldown_hours: 0,
    notify_channels: [], notify_on_release: true, enabled: true, created_at: '2026-01-01T00:00:00Z', update_action: 'custom',
    ...overrides,
  } as ReleaseTracker
}

function mountCard(overrides: Record<string, unknown> = {}) {
  return mount(TrackerConfigCard, {
    props: {
      tracker: tracker(),
      checking: false,
      running: false,
      canRunManually: true,
      runDisabledReason: '',
      cooldownActive: false,
      cooldownEtaText: '',
      ...overrides,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('TrackerConfigCard', () => {
  it('renders the translated title and action buttons for a git tracker', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('Configuration')
    expect(wrapper.text()).toContain('Vérifier maintenant')
    expect(wrapper.text()).toContain('Exécuter')
    expect(wrapper.text()).toContain('Modifier')
    expect(wrapper.text()).toContain('Release Git')
    expect(wrapper.text()).toContain('Dépôt')
    expect(wrapper.text()).toContain('En attente...')
    expect(wrapper.text()).toContain('Surveillance seule')
  })

  it('shows the translated in-flight labels while checking/running', () => {
    const wrapper = mountCard({ checking: true, running: true })
    expect(wrapper.text()).toContain('Vérification...')
    expect(wrapper.text()).toContain('Déclenchement...')
  })

  it('renders the translated docker-specific labels and "never checked" state', () => {
    const wrapper = mountCard({ tracker: tracker({ tracker_type: 'docker', docker_image: 'nginx:latest' }) })
    expect(wrapper.text()).toContain('Image Docker')
    expect(wrapper.text()).toContain('Tag surveillé')
    expect(wrapper.text()).toContain('Dernier check')
    expect(wrapper.text()).toContain('Jamais')
  })

  it('shows the translated linked-repo label and release-notes link for a docker tracker with a linked repo', () => {
    const wrapper = mountCard({
      tracker: tracker({ tracker_type: 'docker', docker_image: 'nginx:latest', repo_owner: 'nginx', repo_name: 'nginx' }),
    })
    expect(wrapper.text()).toContain('Repo lié')
    expect(wrapper.text()).toContain('Voir les release notes')
  })

  it('shows the translated target VM/task labels when both are set, and the error/cooldown/notifications labels', () => {
    const wrapper = mountCard({
      tracker: tracker({
        host_id: 'h1', custom_task_id: 'task-1', host_name: 'web-01', last_error: 'boom',
        last_triggered_at: '2026-01-01T00:00:00Z', notify_channels: ['smtp'], cooldown_hours: 4,
      }),
    })
    expect(wrapper.text()).toContain('VM cible')
    expect(wrapper.text()).toContain('Tâche')
    expect(wrapper.text()).toContain('Erreur')
    expect(wrapper.text()).toContain('Dernier déclench.')
    expect(wrapper.text()).toContain('Notifications')
    expect(wrapper.text()).toContain('Cooldown')
  })

  it('shows the translated planned-deployment label when a cooldown is active', () => {
    const wrapper = mountCard({ cooldownActive: true, cooldownEtaText: 'dans 2h' })
    expect(wrapper.text()).toContain('Déploiement prévu')
    expect(wrapper.text()).toContain('dans 2h')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('Configuration')
    expect(wrapper.text()).toContain('Check now')
    expect(wrapper.text()).toContain('Git release')
  })
})
