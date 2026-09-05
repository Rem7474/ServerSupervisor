import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { getAllScheduledTasks, getHosts } = vi.hoisted(() => ({
  getAllScheduledTasks: vi.fn(),
  getHosts: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getAllScheduledTasks, getHosts },
}))

import GlobalScheduledTasksView from './GlobalScheduledTasksView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function task(overrides: Record<string, unknown> = {}) {
  return {
    id: 't1', name: 'Update packages', host_id: 'h1', host_name: 'srv-web',
    module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * *',
    enabled: true, payload: '',
    ...overrides,
  }
}

describe('GlobalScheduledTasksView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getAllScheduledTasks.mockResolvedValue({ data: [] })
    getHosts.mockResolvedValue({ data: [] })
  })

  it('renders the translated header and empty state', async () => {
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Tâches planifiées')
    expect(wrapper.text()).toContain('Nouvelle tâche')
    expect(wrapper.text()).toContain('Aucune tâche trouvée')
    expect(wrapper.text()).toContain('Cliquez sur « Nouvelle tâche » pour commencer.')
  })

  it('shows the no-task-configured hint for a viewer who cannot manage tasks', async () => {
    useAuthStore().setAuth({ role: 'viewer', username: 'bob' } as never, 'bob')
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()
    expect(wrapper.text()).toContain('Aucune tâche configurée.')
  })

  it('renders the translated filter options and table headers with a translated task row', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task()] })
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Tous les hôtes')
    expect(wrapper.text()).toContain('Tous les statuts')
    expect(wrapper.text()).toContain('Activées')
    expect(wrapper.text()).toContain('Hôte')
    expect(wrapper.text()).toContain('Module / Action')
    expect(wrapper.text()).toContain('Dernier résultat')
    expect(wrapper.text()).toContain('jamais')
  })

  it('shows the manual badge and dash for a manual-only task', async () => {
    getAllScheduledTasks.mockResolvedValue({
      data: [task({ cron_expression: '0 0 29 2 *', enabled: false })],
    })
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()
    expect(wrapper.text()).toContain('Manuel')
  })

  it('opens the create-task modal with translated labels', async () => {
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Nouvelle tâche planifiée')
    expect(wrapper.text()).toContain('Exécution manuelle uniquement')
    expect(wrapper.text()).toContain('Créer la tâche')
  })

  it('shows the translated delete-confirmation dialog', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task()] })
    const dialog = useConfirmDialog()
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    wrapper.find('button.btn-ghost-danger').trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe('Supprimer la tâche')
    expect(dialog.message.value).toBe('Supprimer « Update packages » sur srv-web ?')
    dialog.onCancel()
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Scheduled tasks')
    expect(wrapper.text()).toContain('No task found')
    expect(wrapper.text()).toContain('Click "New task" to get started.')
  })
})
