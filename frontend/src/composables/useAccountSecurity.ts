import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import apiClient from '../api'
import type { LoginEvent } from '../types/generated'
import type { WebAuthnCredential } from '../types/webauthn'
import { useAuthStore } from '../stores/auth'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import { useConfirmDialog } from './useConfirmDialog'
import { createWebAuthnCredential, isWebAuthnSupported } from '../utils/webauthn'

export function useAccountSecurity() {
  const { t } = useI18n()
  const auth = useAuthStore()
  const signal = useAbortSignal()
  const dialog = useConfirmDialog()

  const mfaEnabled = ref(false)
  const setupVisible = ref(false)
  const setup = ref<{ secret: string; qr_code: string; backup_codes: string[] }>({ secret: '', qr_code: '', backup_codes: [] })
  const verifyCode = ref('')
  const disablePassword = ref('')
  const showDisable = ref(false)
  const loading = ref(false)
  const error = ref('')
  const success = ref('')
  const copiedBackup = ref(false)
  const copiedSecret = ref(false)

  // TOTP countdown
  const SETUP_TIMEOUT = 600
  const setupSecondsLeft = ref(0)
  let setupTimer: ReturnType<typeof setInterval> | undefined

  const setupProgressPct = computed(() =>
    Math.round((setupSecondsLeft.value / SETUP_TIMEOUT) * 100)
  )

  function formatCountdown(secs: number): string {
    const m = Math.floor(secs / 60)
    const s = secs % 60
    return `${m}:${String(s).padStart(2, '0')}`
  }

  function startSetupTimer(): void {
    if (setupTimer) clearInterval(setupTimer)
    setupSecondsLeft.value = SETUP_TIMEOUT
    setupTimer = setInterval(() => {
      setupSecondsLeft.value = Math.max(0, setupSecondsLeft.value - 1)
      if (setupSecondsLeft.value === 0) {
        if (setupTimer) clearInterval(setupTimer)
        setupVisible.value = false
        error.value = t('account.setupTimeoutError')
      }
    }, 1000)
  }

  function stopSetupTimer(): void {
    if (setupTimer) clearInterval(setupTimer)
    setupSecondsLeft.value = 0
  }

  const loginEvents = ref<LoginEvent[]>([])
  const sessionsLoading = ref(false)
  const revokeLoading = ref(false)
  const revokeError = ref('')
  const revokeSuccess = ref('')

  // WebAuthn (passkeys / security keys) — an additional MFA factor alongside TOTP.
  const webauthnSupported = isWebAuthnSupported()
  const webauthnCredentials = ref<WebAuthnCredential[]>([])
  const webauthnLoading = ref(false)
  const webauthnError = ref('')
  const webauthnSuccess = ref('')
  const addingPasskey = ref(false)
  const newPasskeyName = ref('')
  const registeringPasskey = ref(false)

  async function loadStatus() {
    try {
      const res = await apiClient.getMFAStatus(signal)
      mfaEnabled.value = !!res.data?.mfa_enabled
    } catch (e) {
      if (isApiAbort(e)) return
      mfaEnabled.value = false
    }
  }

  async function loadLoginEvents() {
    sessionsLoading.value = true
    try {
      const res = await apiClient.getLoginEvents(signal)
      loginEvents.value = (res.data?.events || []).slice(0, 15)
    } catch (e) {
      if (isApiAbort(e)) return
      loginEvents.value = []
    } finally {
      sessionsLoading.value = false
    }
  }

  async function startSetup() {
    loading.value = true
    error.value = ''
    success.value = ''
    try {
      const res = await apiClient.setupMFA()
      setup.value = res.data
      setupVisible.value = true
      startSetupTimer()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('account.mfaSetupError'))
    } finally {
      loading.value = false
    }
  }

  async function verifySetup() {
    loading.value = true
    error.value = ''
    success.value = ''
    try {
      await apiClient.verifyMFA(setup.value.secret, verifyCode.value, setup.value.backup_codes)
      success.value = t('account.mfaEnabledSuccessMessage')
      setupVisible.value = false
      verifyCode.value = ''
      stopSetupTimer()
      await loadStatus()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('account.invalidCodeError'))
    } finally {
      loading.value = false
    }
  }

  async function disableMFA() {
    const confirmed = await dialog.confirm({
      title: t('account.disableMfaButton'),
      message: t('account.disableMfaConfirmMessage'),
      variant: 'danger',
      okLabel: t('account.disableWord'),
    })
    if (!confirmed) return

    loading.value = true
    error.value = ''
    success.value = ''
    try {
      await apiClient.disableMFA(disablePassword.value)
      success.value = t('account.mfaDisabledMessage')
      showDisable.value = false
      disablePassword.value = ''
      await loadStatus()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('account.disableMfaError'))
    } finally {
      loading.value = false
    }
  }

  async function revokeOtherSessions() {
    if (!auth.isAuthenticated) return
    const confirmed = await dialog.confirm({
      title: t('account.revokeOtherSessionsButton'),
      message: t('account.revokeSessionsConfirmMessage'),
      variant: 'warning',
      okLabel: t('account.revokeWord'),
    })
    if (!confirmed) return

    revokeLoading.value = true
    revokeError.value = ''
    revokeSuccess.value = ''
    try {
      await apiClient.revokeAllSessions()
      revokeSuccess.value = t('account.sessionsRevokedMessage')
      await loadLoginEvents()
    } catch (e: unknown) {
      revokeError.value = getApiErrorMessage(e, t('account.revokeSessionsError'))
    } finally {
      revokeLoading.value = false
    }
  }

  async function copySecret() {
    await navigator.clipboard.writeText(setup.value.secret)
    copiedSecret.value = true
    setTimeout(() => { copiedSecret.value = false }, 1500)
  }

  async function copyBackupCodes() {
    if (!setup.value.backup_codes?.length) return
    await navigator.clipboard.writeText(setup.value.backup_codes.join('\n'))
    copiedBackup.value = true
    setTimeout(() => { copiedBackup.value = false }, 1500)
  }

  async function loadWebAuthnCredentials() {
    webauthnLoading.value = true
    try {
      const res = await apiClient.listWebAuthnCredentials(signal)
      webauthnCredentials.value = res.data?.credentials || []
    } catch (e) {
      if (isApiAbort(e)) return
      webauthnCredentials.value = []
    } finally {
      webauthnLoading.value = false
    }
  }

  function startAddPasskey(): void {
    newPasskeyName.value = ''
    webauthnError.value = ''
    webauthnSuccess.value = ''
    addingPasskey.value = true
  }

  function cancelAddPasskey(): void {
    addingPasskey.value = false
  }

  async function registerPasskey(): Promise<void> {
    registeringPasskey.value = true
    webauthnError.value = ''
    webauthnSuccess.value = ''
    try {
      const begin = await apiClient.beginWebAuthnRegistration()
      const credential = await createWebAuthnCredential(begin.data.options)
      await apiClient.finishWebAuthnRegistration(begin.data.session_token, newPasskeyName.value.trim(), credential)
      webauthnSuccess.value = t('account.keyAddedMessage')
      addingPasskey.value = false
      await loadWebAuthnCredentials()
    } catch (e: unknown) {
      webauthnError.value = getApiErrorMessage(e, t('account.couldNotAddKeyError'))
    } finally {
      registeringPasskey.value = false
    }
  }

  async function deletePasskey(cred: WebAuthnCredential): Promise<void> {
    const confirmed = await dialog.confirm({
      title: t('account.deleteKeyConfirmTitle'),
      message: t('account.deleteKeyConfirmMessage', { name: cred.name || t('account.securityKeyFallbackName') }),
      okLabel: t('account.deleteButton'),
      destructive: true,
    })
    if (!confirmed) return
    webauthnError.value = ''
    webauthnSuccess.value = ''
    try {
      await apiClient.deleteWebAuthnCredential(cred.id)
      webauthnSuccess.value = t('account.keyDeletedMessage')
      await loadWebAuthnCredentials()
    } catch (e: unknown) {
      webauthnError.value = getApiErrorMessage(e, t('account.deleteFailedError'))
    }
  }

  onMounted(() => {
    loadStatus()
    loadLoginEvents()
    if (webauthnSupported) loadWebAuthnCredentials()
  })

  onUnmounted(() => { stopSetupTimer() })

  return {
    auth,
    mfaEnabled,
    setupVisible,
    setup,
    verifyCode,
    disablePassword,
    showDisable,
    loading,
    error,
    success,
    copiedBackup,
    copiedSecret,
    setupSecondsLeft,
    setupProgressPct,
    formatCountdown,
    loginEvents,
    sessionsLoading,
    revokeLoading,
    revokeError,
    revokeSuccess,
    startSetup,
    verifySetup,
    disableMFA,
    revokeOtherSessions,
    copySecret,
    copyBackupCodes,

    webauthnSupported,
    webauthnCredentials,
    webauthnLoading,
    webauthnError,
    webauthnSuccess,
    addingPasskey,
    newPasskeyName,
    registeringPasskey,
    startAddPasskey,
    cancelAddPasskey,
    registerPasskey,
    deletePasskey,
  }
}
