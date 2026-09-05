import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { getUsers, createUser, updateUserRole, deleteUser } = vi.hoisted(() => ({
  getUsers: vi.fn(),
  createUser: vi.fn(),
  updateUserRole: vi.fn(),
  deleteUser: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getUsers, createUser, updateUserRole, deleteUser },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

import UsersView from './UsersView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function user(overrides: Record<string, unknown> = {}) {
  return { id: 'u1', username: 'alice', role: 'viewer', created_at: '2026-01-01T00:00:00Z', ...overrides }
}

describe('UsersView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    getUsers.mockResolvedValue({ data: [] })
  })

  it('renders the translated page header, form labels and empty state', async () => {
    const wrapper = mount(UsersView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Utilisateurs')
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Gestion des rôles')
    expect(wrapper.text()).toContain('Ajouter un utilisateur')
    expect(wrapper.text()).toContain("Nom d'utilisateur")
    expect(wrapper.text()).toContain('Mot de passe')
    expect(wrapper.text()).toContain('Aucun utilisateur')
  })

  it('renders the "you" badge and the disabled-role tooltip for the current user', async () => {
    getUsers.mockResolvedValue({ data: [user({ username: 'admin', role: 'admin' })] })
    const wrapper = mount(UsersView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Vous')
    const select = wrapper.find('select.form-select-sm')
    expect(select.attributes('title')).toBe('Impossible de modifier votre propre rôle')
  })

  it('shows the translated field-required error when submitting an empty create form', async () => {
    const wrapper = mount(UsersView, mountOpts)
    await flushPromises()
    await wrapper.find('form').trigger('submit.prevent')
    expect(wrapper.text()).toContain('Veuillez remplir tous les champs')
  })

  it('shows the translated delete-confirmation title and message', async () => {
    getUsers.mockResolvedValue({ data: [user()] })
    const dialog = useConfirmDialog()
    const wrapper = mount(UsersView, mountOpts)
    await flushPromises()

    wrapper.find('button.btn-ghost-danger').trigger('click')
    await flushPromises()
    expect(dialog.title.value).toBe("Supprimer l'utilisateur")
    expect(dialog.message.value).toBe('Cette action est irréversible.')
    dialog.onCancel()
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(UsersView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Users')
    expect(wrapper.text()).toContain('Role management')
    expect(wrapper.text()).toContain('Add a user')
    expect(wrapper.text()).toContain('No user')
  })
})
