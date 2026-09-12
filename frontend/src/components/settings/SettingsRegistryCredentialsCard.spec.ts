import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const {
  getRegistryCredentials, createRegistryCredential, updateRegistryCredential, deleteRegistryCredential,
} = vi.hoisted(() => ({
  getRegistryCredentials: vi.fn(),
  createRegistryCredential: vi.fn(),
  updateRegistryCredential: vi.fn(),
  deleteRegistryCredential: vi.fn(),
}))

vi.mock('../../api/index', () => ({
  default: { getRegistryCredentials, createRegistryCredential, updateRegistryCredential, deleteRegistryCredential },
}))

import SettingsRegistryCredentialsCard from './SettingsRegistryCredentialsCard.vue'

const credential = { id: 'c1', name: 'GHCR', registry_host: 'ghcr.io', username: 'bot' }

beforeEach(() => {
  vi.clearAllMocks()
  setLocale('fr')
  getRegistryCredentials.mockResolvedValue({ data: { credentials: [] } })
})

describe('SettingsRegistryCredentialsCard', () => {
  it('shows the empty state when there are no credentials', async () => {
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Aucun identifiant de registre configuré.')
  })

  it('lists credentials without ever showing the password', async () => {
    getRegistryCredentials.mockResolvedValue({ data: { credentials: [credential] } })
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('GHCR')
    expect(wrapper.text()).toContain('ghcr.io')
  })

  it('rejects saving without the required fields', async () => {
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click') // open add form
    await wrapper.find('.btn-primary').trigger('click') // save with empty form

    expect(wrapper.text()).toContain('Nom, hôte et utilisateur sont obligatoires.')
    expect(createRegistryCredential).not.toHaveBeenCalled()
  })

  it('rejects creating a credential without a password', async () => {
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="GHCR mon-org"]').setValue('GHCR')
    await wrapper.find('input[placeholder="ghcr.io"]').setValue('ghcr.io')
    await wrapper.find('input[autocomplete="off"]').setValue('bot')
    await wrapper.find('.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Le mot de passe est obligatoire à la création.')
    expect(createRegistryCredential).not.toHaveBeenCalled()
  })

  it('creates a credential once every required field is filled', async () => {
    createRegistryCredential.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="GHCR mon-org"]').setValue('GHCR')
    await wrapper.find('input[placeholder="ghcr.io"]').setValue('ghcr.io')
    await wrapper.find('input[autocomplete="off"]').setValue('bot')
    await wrapper.find('input[autocomplete="new-password"]').setValue('token123')
    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(createRegistryCredential).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'GHCR', registry_host: 'ghcr.io', username: 'bot', password: 'token123' })
    )
    // The form (and its success message) closes immediately on success —
    // only the reloaded list remains visible.
    expect(getRegistryCredentials).toHaveBeenCalledTimes(2)
  })

  it('shows a translated error when saving fails', async () => {
    createRegistryCredential.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="GHCR mon-org"]').setValue('GHCR')
    await wrapper.find('input[placeholder="ghcr.io"]').setValue('ghcr.io')
    await wrapper.find('input[autocomplete="off"]').setValue('bot')
    await wrapper.find('input[autocomplete="new-password"]').setValue('token123')
    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain("Erreur lors de l'enregistrement.")
  })

  it('pre-fills the form and updates (not creates) when editing an existing credential', async () => {
    getRegistryCredentials.mockResolvedValue({ data: { credentials: [credential] } })
    updateRegistryCredential.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button[title="Modifier"]').trigger('click')
    expect((wrapper.find('input[placeholder="GHCR mon-org"]').element as HTMLInputElement).value).toBe(credential.name)

    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(updateRegistryCredential).toHaveBeenCalledWith('c1', expect.objectContaining({ name: credential.name }))
    expect(createRegistryCredential).not.toHaveBeenCalled()
  })

  it('asks for confirmation before deleting, and deletes once confirmed', async () => {
    getRegistryCredentials.mockResolvedValue({ data: { credentials: [credential] } })
    deleteRegistryCredential.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()

    const clickPromise = wrapper.find('button[title="Supprimer"]').trigger('click')
    const dialog = useConfirmDialog()
    expect(dialog.title.value).toBe("Supprimer l'identifiant ?")
    expect(dialog.message.value).toContain('GHCR')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(deleteRegistryCredential).toHaveBeenCalledWith('c1')
  })

  it('shows a translated error when deletion fails', async () => {
    getRegistryCredentials.mockResolvedValue({ data: { credentials: [credential] } })
    deleteRegistryCredential.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()

    const clickPromise = wrapper.find('button[title="Supprimer"]').trigger('click')
    useConfirmDialog().onConfirm()
    await clickPromise
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur lors de la suppression.')
  })

  it('translates in English', async () => {
    setLocale('en')
    const wrapper = mount(SettingsRegistryCredentialsCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Private registries')
    expect(wrapper.text()).toContain('No registry credential configured.')
  })
})
