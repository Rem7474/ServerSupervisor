import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { getRunbooks, getHosts, getRunbookExecutions } = vi.hoisted(() => ({
  getRunbooks: vi.fn(),
  getHosts: vi.fn(),
  getRunbookExecutions: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getRunbooks, getHosts, getRunbookExecutions },
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
})
