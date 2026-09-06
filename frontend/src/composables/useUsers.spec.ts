import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'

const { getUsers, createUser, updateUserRole, deleteUser } = vi.hoisted(() => ({
  getUsers: vi.fn(),
  createUser: vi.fn(),
  updateUserRole: vi.fn(),
  deleteUser: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getUsers, createUser, updateUserRole, deleteUser },
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error ? e.message : fallback),
}))

import { useUsers } from './useUsers'

function mountHost() {
  let api: ReturnType<typeof useUsers> | undefined
  mount(defineComponent({
    setup() {
      api = useUsers()
      return () => h('div')
    },
  }))
  return api!
}

describe('useUsers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
  })

  it('formats a date and falls back to a dash', async () => {
    getUsers.mockResolvedValue({ data: [] })
    const api = mountHost()
    await flushPromises()
    expect(api.formatDate(undefined)).toBe('-')
    expect(api.formatDate('2026-01-01T00:00:00Z')).toMatch(/2026-01-01/)
  })

  it('flags the last remaining admin and builds the delete-button tooltip accordingly', async () => {
    getUsers.mockResolvedValue({
      data: [
        { id: 'u1', username: 'admin', role: 'admin' },
        { id: 'u2', username: 'bob', role: 'viewer' },
      ],
    })
    const api = mountHost()
    await flushPromises()
    expect(api.isLastAdmin('u1')).toBe(true)
    expect(api.getDeleteButtonTitle(api.users.value[0])).toBe('Impossible de supprimer votre propre compte')
    expect(api.getDeleteButtonTitle(api.users.value[1])).toBe('Supprimer cet utilisateur')
  })

  it('reports the last-admin tooltip for a non-self last admin', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'root', role: 'admin' }] })
    const api = mountHost()
    await flushPromises()
    expect(api.getDeleteButtonTitle(api.users.value[0])).toBe('Impossible de supprimer le dernier admin')
  })

  it('clears the user list on a fetch error', async () => {
    getUsers.mockRejectedValue(new Error('boom'))
    const api = mountHost()
    await flushPromises()
    expect(api.users.value).toEqual([])
  })

  it('rejects creating a user with missing fields', async () => {
    getUsers.mockResolvedValue({ data: [] })
    const api = mountHost()
    await flushPromises()
    await api.createUser()
    expect(api.createMessage.value).toBe('Veuillez remplir tous les champs')
    expect(api.createSuccess.value).toBe(false)
  })

  it('rejects creating a user whose username already exists', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'bob', role: 'viewer' }] })
    const api = mountHost()
    await flushPromises()
    api.newUserForm.value = { username: 'bob', password: 'x', role: 'viewer' }
    await api.createUser()
    expect(api.createMessage.value).toBe("Ce nom d'utilisateur existe déjà")
  })

  it('creates a user successfully and refetches the list', async () => {
    getUsers.mockResolvedValue({ data: [] })
    createUser.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    api.newUserForm.value = { username: 'new', password: 'pw', role: 'viewer' }
    await api.createUser()
    expect(api.createSuccess.value).toBe(true)
    expect(api.createMessage.value).toBe('Utilisateur créé avec succès')
    expect(getUsers).toHaveBeenCalledTimes(2)
  })

  it('surfaces the translated fallback error when user creation fails', async () => {
    getUsers.mockResolvedValue({ data: [] })
    createUser.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    api.newUserForm.value = { username: 'new', password: 'pw', role: 'viewer' }
    await api.createUser()
    expect(api.createSuccess.value).toBe(false)
  })

  it('refetches without saving when the role-change confirmation is declined', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'bob', role: 'viewer' }] })
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.saveRole(api.users.value[0])
    dialog.onCancel()
    await p
    expect(updateUserRole).not.toHaveBeenCalled()
    expect(getUsers).toHaveBeenCalledTimes(2)
  })

  it('saves a role change and shows a success message', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'bob', role: 'viewer' }] })
    updateUserRole.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.saveRole(api.users.value[0])
    dialog.onConfirm()
    await p
    expect(api.actionSuccess.value).toBe(true)
    expect(api.actionMessage.value).toBe('Rôle de bob mis à jour.')
  })

  it('shows a translated error and refetches when the role update fails', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'bob', role: 'viewer' }] })
    updateUserRole.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.saveRole(api.users.value[0])
    dialog.onConfirm()
    await p
    expect(api.actionSuccess.value).toBe(false)
    expect(getUsers).toHaveBeenCalledTimes(2)
  })

  it('deletes a user and shows a success message', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'bob', role: 'viewer' }] })
    deleteUser.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.deleteUser(api.users.value[0])
    dialog.onConfirm()
    await p
    expect(api.actionSuccess.value).toBe(true)
    expect(api.actionMessage.value).toBe('Utilisateur bob supprimé.')
  })

  it('shows a translated error when user deletion fails', async () => {
    getUsers.mockResolvedValue({ data: [{ id: 'u1', username: 'bob', role: 'viewer' }] })
    deleteUser.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.deleteUser(api.users.value[0])
    dialog.onConfirm()
    await p
    expect(api.actionSuccess.value).toBe(false)
  })

  it('translates messages to English when the locale is switched', async () => {
    setLocale('en')
    getUsers.mockResolvedValue({ data: [] })
    const api = mountHost()
    await flushPromises()
    await api.createUser()
    expect(api.createMessage.value).toBe('Please fill in all fields')
  })
})
