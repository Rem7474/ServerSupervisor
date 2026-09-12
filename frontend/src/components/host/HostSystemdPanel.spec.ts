import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { sendSystemdCommand } = vi.hoisted(() => ({ sendSystemdCommand: vi.fn() }))
vi.mock('../../api', () => ({
  default: { sendSystemdCommand },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || 'error',
}))

const { collectCommandOutput } = vi.hoisted(() => ({ collectCommandOutput: vi.fn() }))
vi.mock('../../composables/useCommandStream', () => ({
  useCommandStream: () => ({ collectCommandOutput }),
}))

const { track } = vi.hoisted(() => ({ track: vi.fn() }))
vi.mock('../../composables/usePendingCommand', () => ({
  usePendingCommand: () => ({ isPending: () => false, track }),
}))

import HostSystemdPanel from './HostSystemdPanel.vue'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const services = [
  { name: 'nginx.service', active_state: 'active', sub_state: 'running', description: 'web server' },
  { name: 'cron.service', active_state: 'inactive', sub_state: 'dead', description: 'cron daemon' },
]

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  localStorage.clear()
})

describe('HostSystemdPanel', () => {
  it('renders nothing when canRun is false', () => {
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h1', canRun: false } })
    expect(wrapper.find('.card').exists()).toBe(false)
  })

  it('shows the prompt to load services before anything is fetched', () => {
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h1', canRun: true } })
    expect(wrapper.text()).toContain('Cliquez sur "Charger les services"')
  })

  it('loads and lists services, filtered to active by default', async () => {
    sendSystemdCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    collectCommandOutput.mockResolvedValue(JSON.stringify(services))
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h1', canRun: true } })

    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()

    expect(sendSystemdCommand).toHaveBeenCalledWith('h1', '', 'list')
    expect(wrapper.text()).toContain('nginx.service')
    expect(wrapper.text()).not.toContain('cron.service')
    expect(wrapper.emitted('history-changed')).toBeTruthy()
  })

  it('shows all services (active and inactive) when the "Tous" filter is selected', async () => {
    sendSystemdCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    collectCommandOutput.mockResolvedValue(JSON.stringify(services))
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h2', canRun: true } })

    const allFilterBtn = wrapper.findAll('button').find((b) => b.text() === 'Tous')
    await allFilterBtn!.trigger('click')
    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('nginx.service')
    expect(wrapper.text()).toContain('cron.service')
  })

  it('shows a translated error when the service list cannot be parsed', async () => {
    sendSystemdCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    collectCommandOutput.mockResolvedValue('not json')
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h3', canRun: true } })

    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Impossible de parser la liste des services')
  })

  it('shows a translated error when the command stream fails', async () => {
    sendSystemdCommand.mockResolvedValue({ data: { command_id: 'cmd1' } })
    collectCommandOutput.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h4', canRun: true } })

    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur lors du chargement des services')
  })

  it('shows a translated error when dispatching the list command itself throws', async () => {
    sendSystemdCommand.mockRejectedValue(new Error('network down'))
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h5', canRun: true } })

    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain("Impossible d'envoyer la commande")
  })

  it('runs start (inactive service) without confirmation, and restart (active service) after confirming', async () => {
    sendSystemdCommand.mockResolvedValue({ data: { command_id: 'cmd2' } })
    collectCommandOutput.mockResolvedValue(JSON.stringify(services))
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h6', canRun: true } })
    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()
    // "Tous" so both the active (nginx) and inactive (cron) rows are visible.
    const allFilterBtn = wrapper.findAll('button').find((b) => b.text() === 'Tous')
    await allFilterBtn!.trigger('click')

    await wrapper.find('[aria-label="Démarrer le service"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('open-console')?.[0][0]).toMatchObject({ command: 'start cron.service' })

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('[aria-label="Redémarrer le service"]').trigger('click')
    expect(dialog.title.value).toBe('Redémarrer le service')
    expect(dialog.message.value).toBe('Confirmer : systemctl restart nginx.service')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()
    expect(wrapper.emitted('open-console')?.[1][0]).toMatchObject({ command: 'restart nginx.service' })
  })

  it('skips the systemctl action when the confirmation is cancelled', async () => {
    sendSystemdCommand.mockResolvedValue({ data: { command_id: 'cmd2' } })
    collectCommandOutput.mockResolvedValue(JSON.stringify(services))
    const wrapper = mount(HostSystemdPanel, { props: { hostId: 'h7', canRun: true } })
    const loadBtn = wrapper.findAll('button').find((b) => b.text().includes('Charger les services'))
    await loadBtn!.trigger('click')
    await flushPromises()
    sendSystemdCommand.mockClear()

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('[aria-label="Redémarrer le service"]').trigger('click')
    dialog.onCancel()
    await clickPromise

    expect(sendSystemdCommand).not.toHaveBeenCalled()
  })
})
