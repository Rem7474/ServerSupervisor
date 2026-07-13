import { ref, computed, onMounted, onUnmounted } from 'vue'
import apiClient from '../api'
import type { LoginEvent } from '../types/generated'
import { useAuthStore } from '../stores/auth'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'

export function useAccountSecurity() {
  const auth = useAuthStore()
  const signal = useAbortSignal()

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
        error.value = 'Le délai de configuration a expiré. Veuillez cliquer sur "Activer MFA" pour recommencer.'
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
      error.value = getApiErrorMessage(e, 'Erreur lors de la configuration MFA')
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
      success.value = 'MFA activé avec succès.'
      setupVisible.value = false
      verifyCode.value = ''
      stopSetupTimer()
      await loadStatus()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Code invalide')
    } finally {
      loading.value = false
    }
  }

  async function disableMFA() {
    loading.value = true
    error.value = ''
    success.value = ''
    try {
      await apiClient.disableMFA(disablePassword.value)
      success.value = 'MFA désactivé.'
      showDisable.value = false
      disablePassword.value = ''
      await loadStatus()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur lors de la désactivation')
    } finally {
      loading.value = false
    }
  }

  async function revokeOtherSessions() {
    if (!auth.isAuthenticated) return
    revokeLoading.value = true
    revokeError.value = ''
    revokeSuccess.value = ''
    try {
      await apiClient.revokeAllSessions()
      revokeSuccess.value = 'Toutes les autres sessions ont été révoquées.'
      await loadLoginEvents()
    } catch (e: unknown) {
      revokeError.value = getApiErrorMessage(e, 'Erreur lors de la révocation des sessions.')
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

  onMounted(() => {
    loadStatus()
    loadLoginEvents()
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
  }
}
