import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'

const {
  getMFAStatus, getLoginEvents, setupMFA, verifyMFA, disableMFA, revokeAllSessions,
  listWebAuthnCredentials, beginWebAuthnRegistration, finishWebAuthnRegistration, deleteWebAuthnCredential,
} = vi.hoisted(() => ({
  getMFAStatus: vi.fn(),
  getLoginEvents: vi.fn(),
  setupMFA: vi.fn(),
  verifyMFA: vi.fn(),
  disableMFA: vi.fn(),
  revokeAllSessions: vi.fn(),
  listWebAuthnCredentials: vi.fn(),
  beginWebAuthnRegistration: vi.fn(),
  finishWebAuthnRegistration: vi.fn(),
  deleteWebAuthnCredential: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getMFAStatus, getLoginEvents, setupMFA, verifyMFA, disableMFA, revokeAllSessions,
    listWebAuthnCredentials, beginWebAuthnRegistration, finishWebAuthnRegistration, deleteWebAuthnCredential,
  },
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error && e.message ? e.message : fallback),
}))

vi.mock('../utils/webauthn', () => ({
  isWebAuthnSupported: () => true,
  createWebAuthnCredential: vi.fn(async () => ({})),
}))

Object.defineProperty(navigator, 'clipboard', {
  value: { writeText: vi.fn(async () => {}) },
  configurable: true,
})

import { useAccountSecurity } from './useAccountSecurity'

function mountHost() {
  let api: ReturnType<typeof useAccountSecurity> | undefined
  mount(defineComponent({
    setup() {
      api = useAccountSecurity()
      return () => h('div')
    },
  }))
  return api!
}

describe('useAccountSecurity', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getMFAStatus.mockResolvedValue({ data: { mfa_enabled: false } })
    getLoginEvents.mockResolvedValue({ data: { events: [] } })
    listWebAuthnCredentials.mockResolvedValue({ data: { credentials: [] } })
  })

  it('falls back to mfaEnabled=false and an empty login list on a load error', async () => {
    getMFAStatus.mockRejectedValue(new Error('boom'))
    getLoginEvents.mockRejectedValue(new Error('boom'))
    const api = mountHost()
    await flushPromises()
    expect(api.mfaEnabled.value).toBe(false)
    expect(api.loginEvents.value).toEqual([])
  })

  it('starts MFA setup and surfaces the translated fallback error on failure', async () => {
    setupMFA.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.startSetup()
    expect(api.error.value).toBe('Erreur lors de la configuration MFA')
    expect(api.setupVisible.value).toBe(false)
  })

  it('shows the setup panel on a successful start and expires it after the countdown', async () => {
    vi.useFakeTimers()
    setupMFA.mockResolvedValue({ data: { secret: 'ABC', qr_code: 'x', backup_codes: [] } })
    const api = mountHost()
    await flushPromises()
    await api.startSetup()
    expect(api.setupVisible.value).toBe(true)
    expect(api.setupSecondsLeft.value).toBe(600)

    await vi.advanceTimersByTimeAsync(600_000)
    expect(api.setupVisible.value).toBe(false)
    expect(api.error.value).toBe('Le délai de configuration a expiré. Veuillez cliquer sur "Activer MFA" pour recommencer.')
    vi.useRealTimers()
  })

  it('verifies the code and reports the translated invalid-code error', async () => {
    verifyMFA.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.verifySetup()
    expect(api.error.value).toBe('Code invalide')
  })

  it('verifies successfully and refreshes MFA status', async () => {
    verifyMFA.mockResolvedValue({})
    getMFAStatus.mockResolvedValueOnce({ data: { mfa_enabled: false } }).mockResolvedValueOnce({ data: { mfa_enabled: true } })
    const api = mountHost()
    await flushPromises()
    await api.verifySetup()
    expect(api.success.value).toBe('MFA activé avec succès.')
    expect(api.mfaEnabled.value).toBe(true)
  })

  it('declining the disable-MFA confirmation makes no API call', async () => {
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.disableMFA()
    expect(dialog.title.value).toBe('Désactiver le MFA')
    dialog.onCancel()
    await p
    expect(disableMFA).not.toHaveBeenCalled()
  })

  it('disables MFA on confirmation and shows an error on failure', async () => {
    disableMFA.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.disableMFA()
    dialog.onConfirm()
    await p
    expect(api.error.value).toBe('Erreur lors de la désactivation')
  })

  it('revokes other sessions on confirmation and reports the translated success message', async () => {
    revokeAllSessions.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.revokeOtherSessions()
    expect(dialog.title.value).toBe('Révoquer les autres sessions')
    dialog.onConfirm()
    await p
    expect(api.revokeSuccess.value).toBe('Toutes les autres sessions ont été révoquées.')
  })

  it('shows a translated error when revoking sessions fails', async () => {
    revokeAllSessions.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.revokeOtherSessions()
    dialog.onConfirm()
    await p
    expect(api.revokeError.value).toBe('Erreur lors de la révocation des sessions.')
  })

  it('copies the secret and backup codes to the clipboard', async () => {
    const api = mountHost()
    await flushPromises()
    api.setup.value.secret = 'SECRET'
    api.setup.value.backup_codes = ['a', 'b']
    await api.copySecret()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('SECRET')
    expect(api.copiedSecret.value).toBe(true)
    await api.copyBackupCodes()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('a\nb')
    expect(api.copiedBackup.value).toBe(true)
  })

  it('registers a passkey successfully and reloads the credential list', async () => {
    beginWebAuthnRegistration.mockResolvedValue({ data: { options: {}, session_token: 'tok' } })
    finishWebAuthnRegistration.mockResolvedValue({})
    listWebAuthnCredentials.mockResolvedValue({ data: { credentials: [{ id: 'k1', name: 'My key' }] } })
    const api = mountHost()
    await flushPromises()
    api.newPasskeyName.value = 'My key'
    await api.registerPasskey()
    expect(api.webauthnSuccess.value).toBe('Clé de sécurité ajoutée.')
    expect(api.webauthnCredentials.value).toHaveLength(1)
  })

  it('surfaces the translated fallback error when passkey registration fails', async () => {
    beginWebAuthnRegistration.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.registerPasskey()
    expect(api.webauthnError.value).toBe('Impossible d\'ajouter cette clé de sécurité')
  })

  it('deletes a passkey on confirmation, falling back to a generic key name', async () => {
    deleteWebAuthnCredential.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.deletePasskey({ id: 'k1', name: '' } as never)
    expect(dialog.message.value).toBe('« Clé de sécurité » ne pourra plus être utilisée pour se connecter.')
    dialog.onConfirm()
    await p
    expect(api.webauthnSuccess.value).toBe('Clé de sécurité supprimée.')
  })

  it('shows a translated error when passkey deletion fails', async () => {
    deleteWebAuthnCredential.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.deletePasskey({ id: 'k1', name: 'YubiKey' } as never)
    dialog.onConfirm()
    await p
    expect(api.webauthnError.value).toBe('Suppression impossible')
  })

  it('translates error messages to English when the locale is switched', async () => {
    setLocale('en')
    setupMFA.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.startSetup()
    expect(api.error.value).toBe('Error during MFA setup')
  })
})
