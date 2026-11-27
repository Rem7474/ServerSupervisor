import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getStatusPageConfig, updateStatusPageConfig, getUptimeProbes, addToast } = vi.hoisted(() => ({
  getStatusPageConfig: vi.fn(),
  updateStatusPageConfig: vi.fn(),
  getUptimeProbes: vi.fn(),
  addToast: vi.fn(),
}))
vi.mock('../../api', () => ({
  default: { getStatusPageConfig, updateStatusPageConfig, getUptimeProbes },
  getApiErrorMessage: (e: unknown, fallback: string) => (e as { message?: string })?.message || fallback,
}))
vi.mock('../../composables/useGlobalToast', () => ({ addToast }))

import StatusPageSettingsModal from './StatusPageSettingsModal.vue'

const probes = [
  { id: 'p1', name: 'zeta-internal' },
  { id: 'p2', name: 'alpha-internal' },
  { id: 'p3', name: 'beta-internal' },
]

async function mountModal(cfg = { enabled: true, title: 'Acme', description: '', services: [{ probe_id: 'p1', public_name: 'Website' }] }) {
  getStatusPageConfig.mockResolvedValue({ data: { updated_at: '', ...cfg } })
  getUptimeProbes.mockResolvedValue({ data: { probes } })
  const wrapper = mount(StatusPageSettingsModal, { attachTo: document.body })
  await flushPromises()
  return wrapper
}

describe('StatusPageSettingsModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('lists published services first, never prefills a public name, and labels its controls', async () => {
    const wrapper = await mountModal()
    expect(wrapper.find('[role="dialog"]').attributes('aria-modal')).toBe('true')
    const rows = wrapper.findAll('[data-probe]')
    expect(rows.map((r) => r.attributes('data-probe'))).toEqual(['p1', 'p2', 'p3'])
    const nameInputs = rows.map((r) => r.find('input[type="text"]').element as HTMLInputElement)
    expect(nameInputs.map((i) => i.value)).toEqual(['Website', '', ''])
    expect(nameInputs[1]!.disabled).toBe(true)
    expect(rows[1]!.find('input[type="text"]').attributes('aria-label')).toBe('Nom public de alpha-internal')
    expect(wrapper.text()).toContain('1 service publié')
    expect(wrapper.text()).toContain('/status')
    wrapper.unmount()
  })

  it('refuses to save a published service without a public name', async () => {
    const wrapper = await mountModal()
    await wrapper.findAll('[data-probe]')[1]!.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    expect(updateStatusPageConfig).not.toHaveBeenCalled()
    expect(wrapper.find('.alert-danger').text()).toContain('nom public')
    wrapper.unmount()
  })

  it('saves only the published services, trimmed and in order', async () => {
    updateStatusPageConfig.mockResolvedValue({ data: {} })
    const wrapper = await mountModal()
    const rows = wrapper.findAll('[data-probe]')
    await rows[2]!.find('input[type="checkbox"]').setValue(true)
    await rows[2]!.find('input[type="text"]').setValue('  Blog  ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(updateStatusPageConfig).toHaveBeenCalledWith({
      enabled: true, title: 'Acme', description: '',
      services: [{ probe_id: 'p1', public_name: 'Website' }, { probe_id: 'p3', public_name: 'Blog' }],
    })
    expect(addToast).toHaveBeenCalledWith('Page de statut enregistrée', 'success')
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('shows the server error and stays open when saving fails', async () => {
    updateStatusPageConfig.mockRejectedValue(new Error('une sonde ne peut être publiée qu\'une seule fois'))
    const wrapper = await mountModal()
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('.alert-danger').text()).toContain('une seule fois')
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('explains the 404 while disabled and reports a load failure', async () => {
    const wrapper = await mountModal({ enabled: false, title: '', description: '', services: [] })
    expect(wrapper.text()).toContain('répond 404')
    expect(wrapper.text()).toContain('0 service publié')
    wrapper.unmount()

    getStatusPageConfig.mockRejectedValue(new Error(''))
    getUptimeProbes.mockResolvedValue({ data: { probes } })
    const failing = mount(StatusPageSettingsModal, { attachTo: document.body })
    await flushPromises()
    expect(failing.find('.alert-danger').text()).toContain('Impossible de charger')
    failing.unmount()
  })

  it('closes from the header button', async () => {
    const wrapper = await mountModal()
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
})
