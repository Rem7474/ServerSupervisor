import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const {
  getReleaseTracker, getHosts, getReleaseTrackerVersionHistory, getReleaseTrackerExecutions,
  getHostComposeProjects, getHostTasksYaml, runReleaseTracker, checkReleaseTrackerNow, updateReleaseTracker,
  getCommandStatus,
} = vi.hoisted(() => ({
  getReleaseTracker: vi.fn(),
  getHosts: vi.fn(),
  getReleaseTrackerVersionHistory: vi.fn(),
  getReleaseTrackerExecutions: vi.fn(),
  getHostComposeProjects: vi.fn(),
  getHostTasksYaml: vi.fn(),
  runReleaseTracker: vi.fn(),
  checkReleaseTrackerNow: vi.fn(),
  updateReleaseTracker: vi.fn(),
  getCommandStatus: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getReleaseTracker, getHosts, getReleaseTrackerVersionHistory, getReleaseTrackerExecutions,
    getHostComposeProjects, getHostTasksYaml, runReleaseTracker, checkReleaseTrackerNow, updateReleaseTracker,
    getCommandStatus,
  },
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error && e.message ? e.message : fallback),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'tr-1' } }),
}))

import { useReleaseTrackerDetail } from './useReleaseTrackerDetail'

function mountHost() {
  let api: ReturnType<typeof useReleaseTrackerDetail> | undefined
  mount(defineComponent({
    setup() {
      api = useReleaseTrackerDetail()
      return () => h('div')
    },
  }))
  return api!
}

function baseTracker(overrides: Record<string, unknown> = {}) {
  return {
    id: 'tr-1', name: 'HA tracker', enabled: true, provider: 'github',
    repo_owner: 'home-assistant', repo_name: 'core', tracker_type: 'git',
    ...overrides,
  }
}

describe('useReleaseTrackerDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker(), executions: [] } })
    getHosts.mockResolvedValue({ data: [] })
    getReleaseTrackerVersionHistory.mockResolvedValue({ data: { history: [] } })
    getReleaseTrackerExecutions.mockResolvedValue({ data: { executions: [] } })
    getHostComposeProjects.mockResolvedValue({ data: [] })
    getHostTasksYaml.mockResolvedValue({ data: { yaml: '' } })
  })

  it('reports the translated monitor-only hint when no host/task is configured', async () => {
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker({ host_id: '', custom_task_id: '' }), executions: [] } })
    const api = mountHost()
    await flushPromises()
    expect(api.canRunManually.value).toBe(false)
    expect(api.runDisabledReason.value).toBe(
      "Mode surveillance seule: configurez une VM cible et une tâche pour autoriser l'exécution manuelle."
    )
  })

  it('allows manual runs once a host and task are configured', async () => {
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker({ host_id: 'h1', custom_task_id: 't1' }), executions: [] } })
    const api = mountHost()
    await flushPromises()
    expect(api.canRunManually.value).toBe(true)
    expect(api.runDisabledReason.value).toBe('')
  })

  it('loads compose/tasks-yaml snippet data when the tracker has a host', async () => {
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker({ host_id: 'h1' }), executions: [] } })
    getHostComposeProjects.mockResolvedValue({ data: [{ id: 'p1' }] })
    getHostTasksYaml.mockResolvedValue({ data: { yaml: 'tasks: []' } })
    const api = mountHost()
    await flushPromises()
    expect(api.composeProjects.value).toHaveLength(1)
    expect(api.tasksYaml.value).toBe('tasks: []')
  })

  it('shows the translated fallback error when the tracker fails to load', async () => {
    getReleaseTracker.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    expect(api.error.value).toBe('Erreur lors du chargement')
  })

  it('formats a multi-day cooldown remaining label with the French day suffix', async () => {
    getReleaseTracker.mockResolvedValue({
      data: {
        tracker: baseTracker({ cooldown_hours: 240, last_release_detected_at: new Date(Date.now() - 1000).toISOString() }),
        executions: [],
      },
    })
    const api = mountHost()
    await flushPromises()
    expect(api.cooldownActive.value).toBe(true)
    expect(api.cooldownRemainingText.value).toMatch(/^\d+j \d+h$/)
  })

  it('switches the day suffix to English when the locale changes', async () => {
    setLocale('en')
    getReleaseTracker.mockResolvedValue({
      data: {
        tracker: baseTracker({ cooldown_hours: 240, last_release_detected_at: new Date(Date.now() - 1000).toISOString() }),
        executions: [],
      },
    })
    const api = mountHost()
    await flushPromises()
    expect(api.cooldownRemainingText.value).toMatch(/^\d+d \d+h$/)
  })

  it('reports a translated error when execution logs fail to load', async () => {
    getCommandStatus.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.openExecutionLogs('c1')
    expect(api.error.value).toBe('Impossible de charger les logs de la commande.')
  })

  it('refuses a manual run when disabled and reports the translated reason', async () => {
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker({ host_id: '', custom_task_id: '' }), executions: [] } })
    const api = mountHost()
    await flushPromises()
    await api.runManually()
    expect(api.error.value).toContain('Mode surveillance seule')
    expect(runReleaseTracker).not.toHaveBeenCalled()
  })

  it('shows a translated error when triggering a manual run fails', async () => {
    getReleaseTracker.mockResolvedValue({ data: { tracker: baseTracker({ host_id: 'h1', custom_task_id: 't1' }), executions: [] } })
    runReleaseTracker.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.runManually()
    expect(api.error.value).toBe('Erreur lors du déclenchement')
    expect(api.running.value).toBe(false)
  })

  it('shows a translated error when the immediate check fails', async () => {
    checkReleaseTrackerNow.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.triggerCheck()
    expect(api.error.value).toBe('Erreur')
    expect(api.checking.value).toBe(false)
  })

  it('shows a translated error when saving the edited tracker fails', async () => {
    updateReleaseTracker.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.saveEdit({} as never)
    expect(api.modalError.value).toBe('Erreur')
    expect(api.saving.value).toBe(false)
  })

  it('resolves a provider badge class, defaulting to secondary for an unknown provider', () => {
    const api = mountHost()
    expect(api.providerBadge('github')).toContain('blue')
    expect(api.providerBadge('unknown')).toContain('secondary')
  })
})
