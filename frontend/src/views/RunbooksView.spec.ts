import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { getRunbooks, getHosts, getRunbookExecutions, getRunbookExecution, createRunbook, updateRunbook, runRunbook } = vi.hoisted(() => ({
  getRunbooks: vi.fn(),
  getHosts: vi.fn(),
  getRunbookExecutions: vi.fn(),
  getRunbookExecution: vi.fn(),
  createRunbook: vi.fn(),
  updateRunbook: vi.fn(),
  runRunbook: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getRunbooks, getHosts, getRunbookExecutions, getRunbookExecution, createRunbook, updateRunbook, runRunbook },
  getApiErrorMessage: (e: unknown, fallback?: string) => fallback || String(e),
}))

import RunbooksView from './RunbooksView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function runbook(overrides: Record<string, unknown> = {}) {
  return {
    id: 'r1', name: 'Restart stack', description: '',
    steps: [{ host_id: 'h1', module: 'docker', action: 'restart', target: '', continue_on_failure: false }],
    ...overrides,
  }
}

describe('RunbooksView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    getRunbooks.mockResolvedValue({ data: [] })
    getHosts.mockResolvedValue({ data: [{ id: 'h1', name: 'srv-web' }] })
  })

  it('renders the translated header and empty state', async () => {
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Runbooks')
    expect(wrapper.text()).toContain("Séquences d'actions réutilisables")
    expect(wrapper.text()).toContain('Nouveau runbook')
    expect(wrapper.text()).toContain('Aucun runbook configuré')
    expect(wrapper.text()).toContain('Créer mon premier runbook')
  })

  it('renders the translated table with a pluralized steps badge', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook()] })
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Nom')
    expect(wrapper.text()).toContain('Étapes')
    expect(wrapper.find('.badge').text()).toBe('1 étape')
  })

  it('pluralizes the steps badge for 2+ steps', async () => {
    getRunbooks.mockResolvedValue({
      data: [runbook({ steps: [{ host_id: 'h1', module: 'docker', action: 'restart' }, { host_id: 'h1', module: 'apt', action: 'update' }] })],
    })
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()
    expect(wrapper.text()).toContain('2 étapes')
  })

  it('opens the create modal with translated labels', async () => {
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Nouveau runbook')
    expect(wrapper.text()).toContain("Étapes (exécutées dans l'ordre)")
    expect(wrapper.text()).toContain('Continuer même si cette étape échoue')
    expect(wrapper.text()).toContain('Ajouter une étape')
  })

  it('shows the translated delete-confirmation dialog', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook()] })
    const dialog = useConfirmDialog()
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    wrapper.find('button.btn-ghost-danger').trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe('Supprimer le runbook')
    expect(dialog.message.value).toBe('Supprimer le runbook "Restart stack" ? Cette action est irréversible.')
    dialog.onCancel()
  })

  it('shows the translated run-confirmation dialog, pluralized by step count', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook()] })
    const dialog = useConfirmDialog()
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    wrapper.find('button.btn-ghost-success').trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe('Lancer le runbook')
    expect(dialog.message.value).toBe('Lancer le runbook "Restart stack" sur 1 étape ?')
    dialog.onCancel()
  })

  it('opens the history modal and shows the translated empty state', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook()] })
    getRunbookExecutions.mockResolvedValue({ data: [] })
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-ghost-secondary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Historique — Restart stack')
    expect(wrapper.text()).toContain('Aucune exécution pour ce runbook.')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('No runbook configured')
    expect(wrapper.text()).toContain('New runbook')
  })

  it('opens the edit modal pre-filled from the selected runbook, then closes it', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook({ description: 'nightly restart' })] })
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    await wrapper.findAll('button.btn-ghost-secondary')[1].trigger('click') // edit
    await flushPromises()

    const nameInput = wrapper.find('input[type="text"]')
    expect((nameInput.element as HTMLInputElement).value).toBe('Restart stack')
    expect(wrapper.text()).toContain('Modifier le runbook')

    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it('adds and removes a step in the create modal', async () => {
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')

    expect(wrapper.findAll('.border.rounded.p-2.mb-2')).toHaveLength(1)
    await wrapper.find('button.btn-outline-secondary.btn-sm').trigger('click')
    expect(wrapper.findAll('.border.rounded.p-2.mb-2')).toHaveLength(2)

    await wrapper.find('button.btn-ghost-danger').trigger('click')
    expect(wrapper.findAll('.border.rounded.p-2.mb-2')).toHaveLength(1)
  })

  it('shows a translated save error', async () => {
    createRunbook.mockRejectedValue(new Error('boom'))
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()
    await wrapper.find('button.btn-primary').trigger('click')
    await wrapper.find('input[type="text"]').setValue('New runbook')
    await wrapper.find('select').setValue('h1')
    await flushPromises()

    await wrapper.find('.modal-footer button.btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('boom')
  })

  it('renders execution history rows with status badges and expands step details', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook()] })
    getRunbookExecutions.mockResolvedValue({
      data: [
        { id: 'e1', status: 'completed', triggered_by: 'admin', started_at: '2026-01-01T00:00:00Z', completed_at: '2026-01-01T00:01:00Z' },
      ],
    })
    getRunbookExecution.mockResolvedValue({
      data: {
        id: 'e1', status: 'completed', steps: [
          { position: 0, host_id: 'h1', module: 'docker', action: 'restart', target: '', status: 'completed', command_id: 'c1' },
          { position: 1, host_id: 'h1', module: 'apt', action: 'update', status: '' },
        ],
      },
    })
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    await wrapper.find('button.btn-ghost-secondary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Terminé')

    await wrapper.find('tr.clickable-row').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('docker/restart')
    expect(wrapper.text()).toContain('en attente')

    await wrapper.find('button[title="Voir les logs de cette étape"]').trigger('click')
    expect(wrapper.text()).toContain("Logs de l'étape")
  })

  it('runs a runbook on confirmation', async () => {
    getRunbooks.mockResolvedValue({ data: [runbook()] })
    runRunbook.mockResolvedValue({})
    getRunbookExecutions.mockResolvedValue({ data: [] })
    const dialog = useConfirmDialog()
    const wrapper = mount(RunbooksView, mountOpts)
    await flushPromises()

    const p = wrapper.find('button.btn-ghost-success').trigger('click')
    await flushPromises()
    dialog.onConfirm()
    await p
    await flushPromises()
    expect(runRunbook).toHaveBeenCalledWith('r1')
  })
})
