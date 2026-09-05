import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import TrackerScriptHelpCard from './TrackerScriptHelpCard.vue'
import type { ReleaseTracker } from '../../types/tracker'

function tracker(overrides: Partial<ReleaseTracker> = {}): ReleaseTracker {
  return {
    id: 't1', name: '', tracker_type: 'git', provider: 'github', repo_owner: 'home-assistant', repo_name: 'core',
    docker_image: '', docker_tag: '', host_id: 'h1', custom_task_id: '', last_release_tag: '', cooldown_hours: 1,
    notify_channels: [], notify_on_release: true, enabled: true, created_at: '2026-01-01T00:00:00Z', update_action: 'custom',
    ...overrides,
  } as ReleaseTracker
}

function mountCard(overrides: Record<string, unknown> = {}) {
  return mount(TrackerScriptHelpCard, {
    props: { tracker: tracker(), composeProjects: [], tasksYaml: '', loadingSnippet: false, ...overrides },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('TrackerScriptHelpCard', () => {
  it('renders the translated env-var table title and git tracker descriptions', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('Variables disponibles dans le script')
    expect(wrapper.text()).toContain('Tag de la nouvelle release (ex: v1.2.3)')
    expect(wrapper.text()).toContain('Nom du tracker dans ServerSupervisor')
  })

  it('renders the translated docker tracker descriptions instead of the git ones', () => {
    const wrapper = mountCard({ tracker: tracker({ tracker_type: 'docker', docker_image: 'nginx:latest' }) })
    expect(wrapper.text()).toContain('Digest manifest SHA256 précédent')
    expect(wrapper.text()).not.toContain('URL de la release sur le provider')
  })

  it('renders the translated tasks.yaml example title, prompt sentence, and generated deployment snippet', () => {
    const wrapper = mountCard({ tracker: tracker({ name: 'My App' }) })
    expect(wrapper.text()).toContain('Exemple de script tasks.yaml')
    expect(wrapper.text()).toContain('Ajoutez cette tâche dans')
    expect(wrapper.text()).toContain('/etc/serversupervisor/tasks.yaml')
    expect(wrapper.text()).toContain('sur l\'hôte :')
    expect(wrapper.find('pre').text()).toContain('Déploiement My App')
    expect(wrapper.find('pre').text()).toContain('Nouvelle release: $SS_TAG_NAME')
  })

  it('shows the translated "task to add" prompt and docker snippet when tasksYaml content is already present', () => {
    const wrapper = mountCard({
      tracker: tracker({ tracker_type: 'docker', docker_image: 'nginx:latest', name: 'Nginx' }),
      tasksYaml: 'tasks:\n  - id: existing',
    })
    expect(wrapper.text()).toContain('Contenu actuel de')
    expect(wrapper.text()).toContain('Tâche à ajouter dans la section tasks: :')
    expect(wrapper.find('pre.mb-0').text()).toContain('Pull et redémarrage Nginx')
  })

  it('shows the translated auto-detected path badge and tooltip when a compose project matches', () => {
    const wrapper = mountCard({
      tracker: tracker({ tracker_type: 'docker', docker_image: 'nginx:latest' }),
      composeProjects: [{ raw_config: 'image: nginx:latest', working_dir: '/opt/nginx' } as never],
    })
    expect(wrapper.text()).toContain('Chemin détecté automatiquement')
    expect(wrapper.find('[title="Chemin détecté depuis les projets Compose de l\'hôte"]').exists()).toBe(true)
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mountCard({ tracker: tracker({ name: 'My App' }) })
    expect(wrapper.text()).toContain('Variables available in the script')
    expect(wrapper.text()).toContain('Example tasks.yaml script')
    expect(wrapper.find('pre').text()).toContain('Deployment My App')
  })
})
