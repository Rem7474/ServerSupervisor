import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const { listConnections, createConnection, updateConnection, testConnection, deleteConnection, refreshNow } = vi.hoisted(() => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  updateConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  refreshNow: vi.fn(),
}))

vi.mock('../../api/npm', () => ({
  npmApi: { listConnections, createConnection, updateConnection, testConnection, deleteConnection, refreshNow },
}))

import SettingsNPMCard from './SettingsNPMCard.vue'

const connection = {
  id: 'n1', name: 'main-npm', api_url: 'http://192.168.1.10:81', identity: 'admin@example.com',
  enabled: true, last_error: '', last_success_at: new Date().toISOString(), proxy_host_count: 3,
}

beforeEach(() => {
  vi.clearAllMocks()
  setLocale('fr')
  listConnections.mockResolvedValue({ data: { connections: [] } })
})

describe('SettingsNPMCard', () => {
  it('shows the empty state when there are no connections', async () => {
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Aucune connexion NPM configurée.')
  })

  it('shows an OK badge for a healthy connection and the proxy host count', async () => {
    listConnections.mockResolvedValue({ data: { connections: [connection] } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('main-npm')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.find('.badge.bg-success-lt').text()).toBe('OK')
  })

  it('shows a disabled badge for a disabled connection, regardless of last_error', async () => {
    listConnections.mockResolvedValue({ data: { connections: [{ ...connection, enabled: false, last_error: 'boom' }] } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Désactivé')
  })

  it('shows an error badge when enabled but the last poll failed', async () => {
    listConnections.mockResolvedValue({ data: { connections: [{ ...connection, last_success_at: undefined, last_error: 'boom' }] } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Erreur')
  })

  it('shows a pending badge for an enabled connection with neither success nor error yet', async () => {
    listConnections.mockResolvedValue({ data: { connections: [{ ...connection, last_success_at: undefined, last_error: '' }] } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('En attente')
  })

  it('rejects testing the connection until URL, identity and password are all set', async () => {
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click') // open add form
    await wrapper.find('.btn-outline-secondary.ms-2').trigger('click') // test with empty form

    expect(wrapper.text()).toContain("Renseignez l'URL, l'identifiant et le mot de passe pour tester.")
    expect(testConnection).not.toHaveBeenCalled()
  })

  it('shows a translated failure message when the test connection fails', async () => {
    testConnection.mockResolvedValue({ data: { success: false } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="http://192.168.1.10:81"]').setValue('http://npm:81')
    await wrapper.find('input[placeholder="admin@example.com"]').setValue('a@b.com')
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-outline-secondary.ms-2').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Échec de connexion.')
  })

  it('asks for confirmation before deleting a connection, and deletes once confirmed', async () => {
    listConnections.mockResolvedValue({ data: { connections: [connection] } })
    deleteConnection.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()

    const clickPromise = wrapper.find('button[title="Supprimer"]').trigger('click')
    const dialog = useConfirmDialog()
    expect(dialog.title.value).toBe('Supprimer la connexion NPM ?')
    expect(dialog.message.value).toContain('main-npm')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(deleteConnection).toHaveBeenCalledWith('n1')
  })

  it('triggers an immediate refresh', async () => {
    listConnections.mockResolvedValue({ data: { connections: [connection] } })
    refreshNow.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button[title="Rafraîchir maintenant"]').trigger('click')
    await flushPromises()

    expect(refreshNow).toHaveBeenCalledWith('n1')
    expect(wrapper.text()).toContain('[main-npm] Rafraîchissement déclenché.')
  })

  it('translates in English', async () => {
    setLocale('en')
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('No NPM connection configured.')
  })
})

describe('SettingsNPMCard — save flow', () => {
  it('rejects saving without the required fields', async () => {
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Nom, URL API et identifiant sont obligatoires.')
    expect(createConnection).not.toHaveBeenCalled()
  })

  it('rejects creating a connection without a password', async () => {
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="Mon NPM"]').setValue('main')
    await wrapper.find('input[placeholder="http://192.168.1.10:81"]').setValue('http://npm:81')
    await wrapper.find('input[placeholder="admin@example.com"]').setValue('a@b.com')
    await wrapper.find('.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Le mot de passe est obligatoire à la création.')
    expect(createConnection).not.toHaveBeenCalled()
  })

  it('creates a connection once every required field is filled', async () => {
    createConnection.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="Mon NPM"]').setValue('main')
    await wrapper.find('input[placeholder="http://192.168.1.10:81"]').setValue('http://npm:81')
    await wrapper.find('input[placeholder="admin@example.com"]').setValue('a@b.com')
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(createConnection).toHaveBeenCalledWith(expect.objectContaining({ api_url: 'http://npm:81', identity: 'a@b.com' }))
    expect(listConnections).toHaveBeenCalledTimes(2)
  })

  it('shows a translated error when saving fails', async () => {
    createConnection.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="Mon NPM"]').setValue('main')
    await wrapper.find('input[placeholder="http://192.168.1.10:81"]').setValue('http://npm:81')
    await wrapper.find('input[placeholder="admin@example.com"]').setValue('a@b.com')
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain("Erreur lors de l'enregistrement.")
  })

  it('pre-fills the form and updates (not creates) when editing an existing connection', async () => {
    listConnections.mockResolvedValue({ data: { connections: [connection] } })
    updateConnection.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button[title="Modifier"]').trigger('click')
    expect((wrapper.find('input[placeholder="http://192.168.1.10:81"]').element as HTMLInputElement).value).toBe(connection.api_url)

    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(updateConnection).toHaveBeenCalledWith('n1', expect.objectContaining({ name: connection.name }))
    expect(createConnection).not.toHaveBeenCalled()
  })
})

describe('SettingsNPMCard — test/refresh/delete error paths', () => {
  it('shows a translated success message when the test connection succeeds', async () => {
    testConnection.mockResolvedValue({ data: { success: true } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="http://192.168.1.10:81"]').setValue('http://npm:81')
    await wrapper.find('input[placeholder="admin@example.com"]').setValue('a@b.com')
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-outline-secondary.ms-2').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Connexion réussie !')
  })

  it('shows a translated network-error message when the test request itself fails', async () => {
    testConnection.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await wrapper.find('input[placeholder="http://192.168.1.10:81"]').setValue('http://npm:81')
    await wrapper.find('input[placeholder="admin@example.com"]').setValue('a@b.com')
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-outline-secondary.ms-2').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur réseau.')
  })

  it('shows a translated error when the immediate refresh fails', async () => {
    listConnections.mockResolvedValue({ data: { connections: [connection] } })
    refreshNow.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button[title="Rafraîchir maintenant"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur.')
  })

  it('shows a translated error when deletion fails', async () => {
    listConnections.mockResolvedValue({ data: { connections: [connection] } })
    deleteConnection.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsNPMCard, { props: { authIsAdmin: true } })
    await flushPromises()

    const clickPromise = wrapper.find('button[title="Supprimer"]').trigger('click')
    useConfirmDialog().onConfirm()
    await clickPromise
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur lors de la suppression.')
  })
})
