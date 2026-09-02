import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const {
  getProxmoxInstances,
  testProxmoxConnection,
  testProxmoxInstanceById,
  createProxmoxInstance,
  updateProxmoxInstance,
  deleteProxmoxInstance,
  pollProxmoxNow,
} = vi.hoisted(() => ({
  getProxmoxInstances: vi.fn(),
  testProxmoxConnection: vi.fn(),
  testProxmoxInstanceById: vi.fn(),
  createProxmoxInstance: vi.fn(),
  updateProxmoxInstance: vi.fn(),
  deleteProxmoxInstance: vi.fn(),
  pollProxmoxNow: vi.fn(),
}))

vi.mock('../../api/index', () => ({
  default: {
    getProxmoxInstances, testProxmoxConnection, testProxmoxInstanceById,
    createProxmoxInstance, updateProxmoxInstance, deleteProxmoxInstance, pollProxmoxNow,
  },
}))

import SettingsProxmoxCard from './SettingsProxmoxCard.vue'

const connection = {
  id: 'conn-1',
  name: 'main',
  api_url: 'https://pve.example.com:8006/api2/json',
  token_id: 'root@pam!token',
  insecure_skip_verify: false,
  enabled: true,
  poll_interval_sec: 60,
  pve_username: 'root@pam',
  console_configured: true,
  node_count: 1,
  guest_count: 2,
  last_error: '',
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
}

async function mountAndOpenEdit() {
  getProxmoxInstances.mockResolvedValue({ data: [connection] })
  const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
  await flushPromises()
  await wrapper.find('button[title="Modifier"]').trigger('click')
  return wrapper
}

describe('SettingsProxmoxCard — test connection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('editing an existing connection tests the stored connection by id, not the (possibly blank) form fields', async () => {
    testProxmoxInstanceById.mockResolvedValue({
      data: { success: true, console_configured: true, console_ok: true },
    })
    const wrapper = await mountAndOpenEdit()

    const testBtn = wrapper.findAll('button').find((b) => b.text().includes('Tester la connexion'))
    await testBtn!.trigger('click')
    await flushPromises()

    expect(testProxmoxInstanceById).toHaveBeenCalledWith('conn-1')
    expect(testProxmoxConnection).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('identifiants console valides')
  })

  it('shows a warning tone (not success) when the API is reachable but console credentials are missing', async () => {
    testProxmoxInstanceById.mockResolvedValue({
      data: { success: true, console_configured: false },
    })
    const wrapper = await mountAndOpenEdit()

    const testBtn = wrapper.findAll('button').find((b) => b.text().includes('Tester la connexion'))
    await testBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Console non configurée')
    const msg = wrapper.find('.card-body span.small')
    expect(msg.classes()).toContain('text-warning')
  })

  it('shows a warning tone when console credentials are configured but the PVE login itself fails', async () => {
    testProxmoxInstanceById.mockResolvedValue({
      data: { success: true, console_configured: true, console_ok: false, console_error: 'HTTP 401: bad credentials' },
    })
    const wrapper = await mountAndOpenEdit()

    const testBtn = wrapper.findAll('button').find((b) => b.text().includes('Tester la connexion'))
    await testBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('échec de connexion console')
    expect(wrapper.text()).toContain('HTTP 401: bad credentials')
  })

  it('creating a new connection tests the ad-hoc form values, including pve_username/pve_password', async () => {
    getProxmoxInstances.mockResolvedValue({ data: [] })
    testProxmoxConnection.mockResolvedValue({ data: { success: true, console_configured: false } })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button').trigger('click') // "Ajouter une connexion"
    await wrapper.find('input[placeholder="Mon cluster PVE"]').setValue('new-conn')
    await wrapper.find('input[placeholder="https://pve.example.com:8006/api2/json"]').setValue('https://pve2:8006/api2/json')
    await wrapper.find('input[placeholder="root@pam!supervision"]').setValue('root@pam!tok')
    // Two distinct fields share autocomplete="new-password" (token secret,
    // then PVE console password) — find() only grabs the first, so the
    // second is queried by id instead to actually exercise form.pve_password
    // (v-model'd there, otherwise never touched by any test).
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('input[placeholder="root@pam"]').setValue('root@pam')
    await wrapper.find('#proxmox-pve-password').setValue('pve-secret')

    const testBtn = wrapper.findAll('button').find((b) => b.text().includes('Tester la connexion'))
    await testBtn!.trigger('click')
    await flushPromises()

    expect(testProxmoxInstanceById).not.toHaveBeenCalled()
    expect(testProxmoxConnection).toHaveBeenCalledWith(expect.objectContaining({
      api_url: 'https://pve2:8006/api2/json',
      token_id: 'root@pam!tok',
      token_secret: 'secret',
      pve_username: 'root@pam',
      pve_password: 'pve-secret',
    }))
  })

  it('shows a translated network-error message when the test request itself fails', async () => {
    testProxmoxInstanceById.mockRejectedValue({ response: { data: {} } })
    const wrapper = await mountAndOpenEdit()

    const testBtn = wrapper.findAll('button').find((b) => b.text().includes('Tester la connexion'))
    await testBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur réseau.')
  })
})

describe('SettingsProxmoxCard — save flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getProxmoxInstances.mockResolvedValue({ data: [] })
  })

  async function fillRequiredFields(wrapper: ReturnType<typeof mount>) {
    await wrapper.find('input[placeholder="Mon cluster PVE"]').setValue('new-conn')
    await wrapper.find('input[placeholder="https://pve.example.com:8006/api2/json"]').setValue('https://pve2:8006/api2/json')
    await wrapper.find('input[placeholder="root@pam!supervision"]').setValue('root@pam!tok')
  }

  it('rejects saving without the required fields', async () => {
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await wrapper.find('.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Nom, URL API et Token ID sont obligatoires.')
    expect(createProxmoxInstance).not.toHaveBeenCalled()
  })

  it('rejects creating a connection without a token secret', async () => {
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await fillRequiredFields(wrapper)
    await wrapper.find('.btn-primary').trigger('click')

    expect(wrapper.text()).toContain('Le token secret est obligatoire à la création.')
    expect(createProxmoxInstance).not.toHaveBeenCalled()
  })

  it('creates a connection once every required field is filled', async () => {
    createProxmoxInstance.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await fillRequiredFields(wrapper)
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(createProxmoxInstance).toHaveBeenCalledWith(expect.objectContaining({
      api_url: 'https://pve2:8006/api2/json', token_id: 'root@pam!tok',
    }))
    expect(getProxmoxInstances).toHaveBeenCalledTimes(2)
  })

  it('shows a translated error when saving fails', async () => {
    createProxmoxInstance.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await fillRequiredFields(wrapper)
    await wrapper.find('input[autocomplete="new-password"]').setValue('secret')
    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain("Erreur lors de l'enregistrement.")
  })

  it('pre-fills the form and updates (not creates) when editing an existing connection', async () => {
    updateProxmoxInstance.mockResolvedValue({ data: {} })
    const wrapper = await mountAndOpenEdit()
    expect((wrapper.find('input[placeholder="Mon cluster PVE"]').element as HTMLInputElement).value).toBe(connection.name)

    await wrapper.find('.btn-primary').trigger('click')
    await flushPromises()

    expect(updateProxmoxInstance).toHaveBeenCalledWith('conn-1', expect.objectContaining({ name: connection.name }))
    expect(createProxmoxInstance).not.toHaveBeenCalled()
  })
})

describe('SettingsProxmoxCard — poll/delete error paths', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('triggers an immediate poll and shows the translated confirmation', async () => {
    getProxmoxInstances.mockResolvedValue({ data: [connection] })
    pollProxmoxNow.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button[title="Collecter maintenant"]').trigger('click')
    await flushPromises()

    expect(pollProxmoxNow).toHaveBeenCalledWith('conn-1')
    expect(wrapper.text()).toContain('[main] Collecte déclenchée.')
  })

  it('shows a translated error when the poll trigger fails', async () => {
    getProxmoxInstances.mockResolvedValue({ data: [connection] })
    pollProxmoxNow.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()

    await wrapper.find('button[title="Collecter maintenant"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur.')
  })

  it('asks for confirmation before deleting, and deletes once confirmed', async () => {
    getProxmoxInstances.mockResolvedValue({ data: [connection] })
    deleteProxmoxInstance.mockResolvedValue({ data: {} })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()

    const clickPromise = wrapper.find('button[title="Supprimer"]').trigger('click')
    const dialog = useConfirmDialog()
    expect(dialog.title.value).toBe('Supprimer la connexion Proxmox ?')
    expect(dialog.message.value).toContain('main')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(deleteProxmoxInstance).toHaveBeenCalledWith('conn-1')
    expect(wrapper.text()).toContain('Connexion supprimée.')
  })

  it('shows a translated error when deletion fails', async () => {
    getProxmoxInstances.mockResolvedValue({ data: [connection] })
    deleteProxmoxInstance.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(SettingsProxmoxCard, { props: { authIsAdmin: true } })
    await flushPromises()

    const clickPromise = wrapper.find('button[title="Supprimer"]').trigger('click')
    useConfirmDialog().onConfirm()
    await clickPromise
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur lors de la suppression.')
  })
})
