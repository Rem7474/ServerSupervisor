import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'

vi.mock('../../api', () => ({
  default: { sendJournalCommand: vi.fn() },
}))

vi.mock('../../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { message?: string })?.message || fallback || 'erreur',
}))

vi.mock('./HostSystemdPanel.vue', () => ({
  default: { name: 'HostSystemdPanelStub', render: () => null },
}))

import apiClient from '../../api'
import HostSystemTab from './HostSystemTab.vue'

function mountTab() {
  return mount(HostSystemTab, { props: { hostId: 'host-1', canRunApt: true } })
}

describe('HostSystemTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('renders translated chrome', () => {
    const w = mountTab()

    expect(w.text()).toContain('Logs système (journalctl)')
    expect(w.find('input').attributes('placeholder')).toBe('Nom du service (ex : nginx, ssh, docker)')
    expect(w.text()).toContain('Charger les logs')
  })

  it('keeps the button disabled until a service name is typed', async () => {
    const w = mountTab()
    const button = w.find('button')

    expect(button.attributes('disabled')).toBeDefined()

    await w.find('input').setValue('nginx')

    expect(button.attributes('disabled')).toBeUndefined()
  })

  it('ignores a whitespace-only service name without calling the API', async () => {
    const w = mountTab()
    await w.find('input').setValue('   ')

    await w.find('button').trigger('click')

    expect(apiClient.sendJournalCommand).not.toHaveBeenCalled()
  })

  it('dispatches the command and emits the console + history events', async () => {
    vi.mocked(apiClient.sendJournalCommand).mockResolvedValue({ data: { command_id: 42 } } as never)
    const w = mountTab()
    await w.find('input').setValue('  nginx  ')

    await w.find('button').trigger('click')
    await w.vm.$nextTick()

    // The service name is trimmed before it reaches the agent.
    expect(apiClient.sendJournalCommand).toHaveBeenCalledWith('host-1', 'nginx')
    const opened = w.emitted('open-command')?.[0]?.[0] as Record<string, unknown>
    expect(opened).toMatchObject({ id: 42, module: 'journal', action: 'read', target: 'nginx' })
    expect(w.emitted('history-changed')).toHaveLength(1)
    expect(w.text()).toContain('Stream → commande #42')
  })

  it('shows a translated error and emits nothing when the dispatch fails', async () => {
    vi.mocked(apiClient.sendJournalCommand).mockRejectedValue({})
    const w = mountTab()
    await w.find('input').setValue('nginx')

    await w.find('button').trigger('click')
    await w.vm.$nextTick()

    expect(w.text()).toContain("Impossible d'envoyer la commande")
    expect(w.emitted('open-command')).toBeUndefined()
    expect(w.emitted('history-changed')).toBeUndefined()
  })

  it('renders the stream hint in the active language', async () => {
    setLocale('en')
    vi.mocked(apiClient.sendJournalCommand).mockResolvedValue({ data: { command_id: 7 } } as never)
    const w = mountTab()
    await w.find('input').setValue('sshd')

    await w.find('button').trigger('click')
    await w.vm.$nextTick()

    expect(w.text()).toContain('System logs (journalctl)')
    expect(w.text()).toContain('Stream → command #7')
  })
})
