import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const {
  getProxmoxInstances,
  testProxmoxConnection,
  testProxmoxInstanceById,
} = vi.hoisted(() => ({
  getProxmoxInstances: vi.fn(),
  testProxmoxConnection: vi.fn(),
  testProxmoxInstanceById: vi.fn(),
}))

vi.mock('../../api/index', () => ({
  default: { getProxmoxInstances, testProxmoxConnection, testProxmoxInstanceById },
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
})
