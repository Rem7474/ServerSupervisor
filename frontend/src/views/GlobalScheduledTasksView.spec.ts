import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const {
  getAllScheduledTasks, getHosts, updateScheduledTask, runScheduledTask, getScheduledTaskExecutions,
} = vi.hoisted(() => ({
  getAllScheduledTasks: vi.fn(),
  getHosts: vi.fn(),
  updateScheduledTask: vi.fn(),
  runScheduledTask: vi.fn(),
  getScheduledTaskExecutions: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getAllScheduledTasks, getHosts, updateScheduledTask, runScheduledTask, getScheduledTaskExecutions },
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

  it('filters the task list by host, module and status', async () => {
    getAllScheduledTasks.mockResolvedValue({
      data: [task(), task({ id: 't2', name: 'Restart docker', host_name: 'other-host', module: 'docker', enabled: false })],
    })
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    const [hostSelect, moduleSelect, statusSelect] = wrapper.findAll('select.tasks-filter-select')
    await hostSelect.setValue('srv-web')
    expect(wrapper.text()).toContain('Update packages')
    expect(wrapper.text()).not.toContain('Restart docker')

    await hostSelect.setValue('')
    await moduleSelect.setValue('docker')
    expect(wrapper.text()).toContain('Restart docker')
    expect(wrapper.text()).not.toContain('Update packages')

    await moduleSelect.setValue('')
    await statusSelect.setValue('disabled')
    expect(wrapper.text()).toContain('Restart docker')
    expect(wrapper.text()).not.toContain('Update packages')
  })

  it('selects all visible tasks and runs a bulk action', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task(), task({ id: 't2', name: 'Other task' })] })
    updateScheduledTask.mockResolvedValue({})
    const dialog = useConfirmDialog()
    const wrapper = mount(GlobalScheduledTasksView, { ...mountOpts, attachTo: document.body })
    await flushPromises()

    await wrapper.find('.tasks-select-col input[type="checkbox"]').setValue(true)
    await flushPromises()
    expect(document.body.textContent).toContain('Désactiver')

    const bulkBar = document.querySelector('.bulk-action-bar') as HTMLElement
    const disableBtn = Array.from(bulkBar.querySelectorAll('button')).find((b) => b.textContent?.includes('Désactiver'))!
    disableBtn.click()
    await flushPromises()
    dialog.onConfirm()
    await flushPromises()
    expect(updateScheduledTask).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('toggles a task enabled state on confirmation', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task()] })
    updateScheduledTask.mockResolvedValue({})
    const dialog = useConfirmDialog()
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    const p = wrapper.find('td > input[type="checkbox"]').trigger('change')
    await flushPromises()
    expect(dialog.title.value).toBe('Désactiver la tâche')
    dialog.onConfirm()
    await p
    await flushPromises()
    expect(updateScheduledTask).toHaveBeenCalled()
  })

  it('runs a task now', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task()] })
    runScheduledTask.mockResolvedValue({ data: { command_id: 'c1' } })
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-ghost-success').trigger('click')
    await flushPromises()
    expect(runScheduledTask).toHaveBeenCalledWith('t1')
  })

  it('opens the edit modal pre-filled and saves via the composable', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task()] })
    updateScheduledTask.mockResolvedValue({})
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    await wrapper.findAll('button.btn-icon.btn-ghost-secondary')[1].trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Modifier la tâche')
    const nameInput = wrapper.find('.modal-dialog:not(.modal-lg) input[type="text"]')
    expect((nameInput.element as HTMLInputElement).value).toBe('Update packages')

    await wrapper.find('.modal-footer button.btn-primary').trigger('click')
    await flushPromises()
    expect(updateScheduledTask).toHaveBeenCalled()
  })

  it('opens the execution history modal and shows a translated empty state, then a row', async () => {
    getAllScheduledTasks.mockResolvedValue({ data: [task()] })
    getScheduledTaskExecutions.mockResolvedValueOnce({ data: [] })
    const wrapper = mount(GlobalScheduledTasksView, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-icon.btn-ghost-secondary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain("Aucune exécution enregistrée pour cette tâche.")

    await wrapper.find('.btn-close').trigger('click')
    getScheduledTaskExecutions.mockResolvedValueOnce({
      data: [{ id: 'e1', status: 'completed', started_at: '2026-01-01T00:00:00Z', ended_at: '2026-01-01T00:00:05Z', triggered_by: 'admin' }],
    })
    await wrapper.find('button.btn-icon.btn-ghost-secondary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Terminé')
    expect(wrapper.text()).toContain('admin')
  })
})
