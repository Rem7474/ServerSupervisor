import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'
import type { APIKey } from '../types/generated'

const { listMyApiKeys, createApiKey, revokeApiKey } = vi.hoisted(() => ({
  listMyApiKeys: vi.fn(),
  createApiKey: vi.fn(),
  revokeApiKey: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { listMyApiKeys, createApiKey, revokeApiKey },
}))

const writeText = vi.fn(async () => {})
Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

import { apiKeyStatus, useApiKeys } from './useApiKeys'

function key(overrides: Partial<APIKey> = {}): APIKey {
  return {
    id: 1, name: 'grafana', prefix: 'ssk_abcdefgh', scope: 'read', username: 'alice',
    created_at: '2026-11-01T00:00:00Z', ...overrides,
  }
}

function mountHost() {
  let api: ReturnType<typeof useApiKeys> | undefined
  mount(defineComponent({
    setup() {
      api = useApiKeys()
      return () => h('div')
    },
  }))
  return api!
}

function apiError(message: string) {
  return Object.assign(new Error(message), { response: { data: { error: message } } })
}

// A transport failure with no server message: the composable falls back to its own text.
function axiosError() {
  return Object.assign(new Error('Network Error'), { isAxiosError: true })
}

describe('apiKeyStatus', () => {
  const now = Date.parse('2026-11-25T10:00:00Z')
  it('reports revoked before expired, and active otherwise', () => {
    expect(apiKeyStatus(key({ revoked_at: '2026-11-02T00:00:00Z', expires_at: '2026-11-03T00:00:00Z' }), now)).toBe('revoked')
    expect(apiKeyStatus(key({ expires_at: '2026-11-25T10:00:00Z' }), now)).toBe('expired')
    expect(apiKeyStatus(key({ expires_at: '2026-11-25T10:00:01Z' }), now)).toBe('active')
    expect(apiKeyStatus(key(), now)).toBe('active')
  })
})

describe('useApiKeys', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    listMyApiKeys.mockResolvedValue({ data: [key()] })
  })

  it('loads the keys and reports a load failure', async () => {
    const api = mountHost()
    await api.load()
    expect(api.keys.value).toHaveLength(1)

    listMyApiKeys.mockRejectedValueOnce(axiosError())
    await api.load()
    expect(api.error.value).toBe('Impossible de charger les clés API')
    expect(api.loading.value).toBe(false)
  })

  it('treats a null body as an empty list and ignores an aborted request', async () => {
    const api = mountHost()
    listMyApiKeys.mockResolvedValueOnce({ data: null })
    await api.load()
    expect(api.keys.value).toEqual([])

    listMyApiKeys.mockRejectedValueOnce(Object.assign(new Error('canceled'), { name: 'CanceledError' }))
    await api.load()
    expect(api.error.value).toBe('')
  })

  it('creates a key with the trimmed name, keeps the secret and prepends the key', async () => {
    createApiKey.mockResolvedValue({ data: { key: key({ id: 2, name: 'ci', scope: 'write' }), secret: 'ssk_new_secret' } })
    const api = mountHost()
    await api.load()
    api.openCreate()
    api.name.value = '  ci  '
    api.scope.value = 'write'
    api.expiresInDays.value = 0
    await api.create()

    expect(createApiKey).toHaveBeenCalledWith({ name: 'ci', scope: 'write', expires_in_days: 0 })
    expect(api.secret.value).toBe('ssk_new_secret')
    expect(api.keys.value.map(k => k.id)).toEqual([2, 1])

    await api.copySecret()
    expect(writeText).toHaveBeenCalledWith('ssk_new_secret')
    expect(api.copied.value).toBe(true)

    api.closeCreate()
    expect(api.secret.value).toBe('')
    expect(api.copied.value).toBe(false)
    expect(api.showCreate.value).toBe(false)
  })

  it('does not call the API for a blank name and surfaces a server error', async () => {
    const api = mountHost()
    api.openCreate()
    api.name.value = '   '
    await api.create()
    expect(createApiKey).not.toHaveBeenCalled()

    createApiKey.mockRejectedValueOnce(apiError('vous avez déjà 25 clés API actives'))
    api.name.value = 'x'
    await api.create()
    expect(api.createError.value).toBe('vous avez déjà 25 clés API actives')
    expect(api.secret.value).toBe('')
    expect(api.creating.value).toBe(false)
  })

  it('reports a failed copy as not copied', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'))
    const api = mountHost()
    api.secret.value = 'ssk_x'
    await api.copySecret()
    expect(api.copied.value).toBe(false)
  })

  it('revokes only after confirmation, then reloads', async () => {
    const api = mountHost()
    const dialog = useConfirmDialog()

    const cancelled = api.revoke(key())
    expect(dialog.message.value).toContain('grafana')
    dialog.onCancel()
    await cancelled
    expect(revokeApiKey).not.toHaveBeenCalled()

    revokeApiKey.mockResolvedValue({})
    const confirmed = api.revoke(key())
    dialog.onConfirm()
    await confirmed
    await flushPromises()
    expect(revokeApiKey).toHaveBeenCalledWith(1)
    expect(api.success.value).toBe('Clé « grafana » révoquée')
    expect(listMyApiKeys).toHaveBeenCalled()

    revokeApiKey.mockRejectedValueOnce(axiosError())
    const failing = api.revoke(key())
    dialog.onConfirm()
    await failing
    expect(api.error.value).toBe('Impossible de révoquer la clé API')
  })
})
