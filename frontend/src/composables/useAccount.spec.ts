import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'

const { getProfile, getCommandsHistory, changePassword } = vi.hoisted(() => ({
  getProfile: vi.fn(),
  getCommandsHistory: vi.fn(),
  changePassword: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getProfile, getCommandsHistory, changePassword },
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error && e.message ? e.message : fallback),
}))

// Minimal fake standing in for the browser WebSocket so openLogViewer's
// running/pending branch (which calls connectStream) doesn't try a real
// network connection.
class FakeWebSocket {
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  close() { /* no-op */ }
}

import { useAccount } from './useAccount'

function mountHost() {
  let api: ReturnType<typeof useAccount> | undefined
  mount(defineComponent({
    setup() {
      api = useAccount()
      return () => h('div')
    },
  }))
  return api!
}

describe('useAccount', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('WebSocket', FakeWebSocket)
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getProfile.mockResolvedValue({ data: { username: 'admin', role: 'admin' } })
    getCommandsHistory.mockResolvedValue({ data: { commands: [] } })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('computes password strength across every threshold', () => {
    const api = mountHost()
    api.pwForm.value.next = ''
    expect(api.pwStrengthMeta.value).toBeNull()
    api.pwForm.value.next = 'abcdefg' // 7 chars: 0 points
    expect(api.pwStrengthMeta.value?.label).toBe('Faible')
    api.pwForm.value.next = 'abcdefgh' // 8 chars: 1 point
    expect(api.pwStrengthMeta.value?.label).toBe('Faible')
    api.pwForm.value.next = 'abcdefgh1' // 8+, digit: 2 points
    expect(api.pwStrengthMeta.value?.label).toBe('Moyen')
    api.pwForm.value.next = 'Abcdefgh1' // 8+, upper, digit: 3 points
    expect(api.pwStrengthMeta.value?.label).toBe('Bon')
    api.pwForm.value.next = 'Abcdefgh1!' // 8+, upper, digit, symbol: 4 points
    expect(api.pwStrengthMeta.value?.label).toBe('Fort')
  })

  it('validates the next-password field length and mirrors mismatch into confirm', async () => {
    const api = mountHost()
    api.pwForm.value.next = 'short'
    await flushPromises()
    expect(api.pwErrors.value.next).toBe('Au moins 8 caractères requis.')

    api.pwForm.value.confirm = 'different'
    await flushPromises()
    expect(api.pwErrors.value.confirm).toBe('La confirmation ne correspond pas.')

    api.pwForm.value.next = 'differentxx'
    api.pwForm.value.confirm = 'differentxx'
    await flushPromises()
    expect(api.pwErrors.value.confirm).toBe('')
  })

  it('clears the confirm error once it matches, and resets it to empty when blanked', async () => {
    const api = mountHost()
    api.pwForm.value.next = 'matching12'
    api.pwForm.value.confirm = 'matching12'
    await flushPromises()
    expect(api.pwErrors.value.confirm).toBe('')

    api.pwForm.value.confirm = ''
    await flushPromises()
    expect(api.pwErrors.value.confirm).toBe('')
  })

  it('formats command duration and falls back for missing timestamps', () => {
    const api = mountHost()
    expect(api.formatDuration(undefined, undefined)).toBe('—')
    expect(api.formatDuration('2026-01-01T00:00:00Z', '2026-01-01T00:00:45Z')).toBe('45s')
    expect(api.formatDuration('2026-01-01T00:00:00Z', '2026-01-01T00:01:05Z')).toBe('1m 5s')
    expect(api.formatDuration('2026-01-01T00:00:00Z', '2026-01-01T00:02:00Z')).toBe('2m')
  })

  it('builds the command label and status class', () => {
    const api = mountHost()
    expect(api.cmdLabel({ action: 'restart', target: 'nginx' } as never)).toBe('restart nginx')
    expect(api.statusClass('running')).toContain('badge')
  })

  it('rejects an incomplete password-change form', async () => {
    const api = mountHost()
    await api.submitChangePassword()
    expect(api.pwErrors.value.current).toBe('Le mot de passe actuel est requis.')
    expect(api.pwErrors.value.next).toBe('Le nouveau mot de passe doit faire au moins 8 caractères.')
    expect(changePassword).not.toHaveBeenCalled()
  })

  it('submits a valid password change and clears the forced-change flag', async () => {
    changePassword.mockResolvedValue({})
    const api = mountHost()
    api.pwForm.value = { current: 'old12345', next: 'newpass123', confirm: 'newpass123' }
    await api.submitChangePassword()
    expect(api.pwSuccess.value).toBe('Mot de passe mis à jour avec succès.')
    expect(api.pwForm.value.current).toBe('')
  })

  it('surfaces the translated fallback error when changing the password fails', async () => {
    changePassword.mockRejectedValue(new Error(''))
    const api = mountHost()
    api.pwForm.value = { current: 'old12345', next: 'newpass123', confirm: 'newpass123' }
    await api.submitChangePassword()
    expect(api.pwError.value).toBe('Erreur lors de la mise à jour du mot de passe.')
  })

  it('opens and closes the log viewer, streaming output for a running command', async () => {
    const api = mountHost()
    await flushPromises()
    api.openLogViewer({ id: 'c1', status: 'running', output: '' } as never)
    expect(api.selectedCmd.value?.id).toBe('c1')
    expect(api.showConsole.value).toBe(true)

    api.closeLogViewer()
    expect(api.selectedCmd.value).toBeNull()
    expect(api.showConsole.value).toBe(false)
  })

  it('opens the log viewer without streaming for a completed command', async () => {
    const api = mountHost()
    await flushPromises()
    api.openLogViewer({ id: 'c2', status: 'completed', output: 'done' } as never)
    expect(api.selectedCmd.value?.id).toBe('c2')
  })

  it('clears the command list on a fetch error other than abort', async () => {
    getCommandsHistory.mockRejectedValue(new Error('network down'))
    const api = mountHost()
    await flushPromises()
    api.switchToHistorique()
    await flushPromises()
    expect(api.allCommands.value).toEqual([])
  })

  it('translates password-strength labels and errors to English when the locale is switched', async () => {
    setLocale('en')
    const api = mountHost()
    api.pwForm.value.next = 'Abcdefgh1!'
    expect(api.pwStrengthMeta.value?.label).toBe('Strong')
    await api.submitChangePassword()
    expect(api.pwErrors.value.current).toBe('The current password is required.')
  })
})
