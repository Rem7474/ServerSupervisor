import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import apiClient from '../api'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import type { APIKey } from '../types/generated'
import { useAbortSignal } from './useAbortSignal'
import { useConfirmDialog } from './useConfirmDialog'

export type ApiKeyScope = 'read' | 'write'
export type ApiKeyStatus = 'active' | 'expired' | 'revoked'

/** Validity choices offered at creation, in days (0 = never expires). */
export const API_KEY_EXPIRY_OPTIONS = [30, 90, 365, 0] as const

export function apiKeyStatus(key: APIKey, now: number = Date.now()): ApiKeyStatus {
  if (key.revoked_at) return 'revoked'
  if (key.expires_at && new Date(key.expires_at).getTime() <= now) return 'expired'
  return 'active'
}

/**
 * Personal API keys of the signed-in user: list, creation (the full key is
 * returned once and kept only until the secret dialog is closed) and
 * revocation.
 */
export function useApiKeys() {
  const { t } = useI18n()
  const signal = useAbortSignal()
  const dialog = useConfirmDialog()

  const keys = ref<APIKey[]>([])
  const loading = ref(false)
  const error = ref('')
  const success = ref('')

  const showCreate = ref(false)
  const name = ref('')
  const scope = ref<ApiKeyScope>('read')
  const expiresInDays = ref<number>(90)
  const creating = ref(false)
  const createError = ref('')
  // The full key, set right after creation and cleared when the dialog closes.
  const secret = ref('')
  const copied = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const { data } = await apiClient.listMyApiKeys(signal)
      keys.value = data ?? []
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, t('account.apiKeysLoadError'))
    } finally {
      loading.value = false
    }
  }

  function openCreate(): void {
    name.value = ''
    scope.value = 'read'
    expiresInDays.value = 90
    createError.value = ''
    secret.value = ''
    copied.value = false
    showCreate.value = true
  }

  function closeCreate(): void {
    showCreate.value = false
    secret.value = ''
    copied.value = false
  }

  async function create(): Promise<void> {
    if (!name.value.trim() || creating.value) return
    creating.value = true
    createError.value = ''
    success.value = ''
    try {
      const { data } = await apiClient.createApiKey({
        name: name.value.trim(),
        scope: scope.value,
        expires_in_days: expiresInDays.value,
      })
      secret.value = data.secret
      keys.value = [data.key, ...keys.value]
    } catch (e: unknown) {
      createError.value = getApiErrorMessage(e, t('account.apiKeyCreateError'))
    } finally {
      creating.value = false
    }
  }

  async function copySecret(): Promise<void> {
    try {
      await navigator.clipboard.writeText(secret.value)
      copied.value = true
    } catch {
      copied.value = false
    }
  }

  async function revoke(key: APIKey): Promise<void> {
    const confirmed = await dialog.confirm({
      title: t('account.apiKeyRevokeConfirmTitle'),
      message: t('account.apiKeyRevokeConfirmMessage', { name: key.name }),
      okLabel: t('account.apiKeyRevokeButton'),
      destructive: true,
    })
    if (!confirmed) return
    error.value = ''
    success.value = ''
    try {
      await apiClient.revokeApiKey(key.id)
      success.value = t('account.apiKeyRevokedMessage', { name: key.name })
      await load()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('account.apiKeyRevokeError'))
    }
  }

  return {
    keys,
    loading,
    error,
    success,
    showCreate,
    name,
    scope,
    expiresInDays,
    creating,
    createError,
    secret,
    copied,
    load,
    openCreate,
    closeCreate,
    create,
    copySecret,
    revoke,
  }
}
