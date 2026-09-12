import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { updateHost, rotateHostKey } = vi.hoisted(() => ({
  updateHost: vi.fn(),
  rotateHostKey: vi.fn(),
}))
vi.mock('../../api', () => ({
  default: { updateHost, rotateHostKey },
}))

import HostEditForm from './HostEditForm.vue'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const host = { name: 'web-01', hostname: 'web-01.internal', ip_address: '10.0.0.5', os: 'Debian 12', tags: ['prod'] }

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
})

describe('HostEditForm', () => {
  it('pre-fills the form from the host prop', () => {
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    expect((wrapper.find('input[required]').element as HTMLInputElement).value).toBe('web-01')
    const tagsInput = wrapper.findAll('input').find((i) => i.attributes('placeholder') === 'prod, site-lyon')
    expect((tagsInput!.element as HTMLInputElement).value).toBe('prod')
  })

  it('emits close when cancel is clicked', async () => {
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    await wrapper.find('.btn-outline-secondary').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('saves the edited host and emits updated + close', async () => {
    updateHost.mockResolvedValue({ data: { id: 'h1', name: 'web-02' } })
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateHost).toHaveBeenCalledWith('h1', expect.objectContaining({ name: 'web-01', tags: ['prod'] }))
    expect(wrapper.emitted('updated')?.[0]).toEqual([{ id: 'h1', name: 'web-02' }])
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('shows an error and does not emit close/updated when saving fails', async () => {
    updateHost.mockRejectedValue({ response: { data: { error: 'nom déjà utilisé' } } })
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('nom déjà utilisé')
    expect(wrapper.emitted('updated')).toBeFalsy()
    expect(wrapper.emitted('close')).toBeFalsy()
  })

  it('asks for confirmation before rotating the API key, with a French title/message', async () => {
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    const dialog = useConfirmDialog()

    const clickPromise = wrapper.find('.btn-outline-warning').trigger('click')
    expect(dialog.title.value).toBe('Régénérer la clé API')
    expect(dialog.variant.value).toBe('warning')
    dialog.onCancel()
    await clickPromise

    expect(rotateHostKey).not.toHaveBeenCalled()
  })

  it('rotates the key on confirmation and shows the new key/install/config panel', async () => {
    rotateHostKey.mockResolvedValue({ data: { api_key: 'new-secret-key' } })
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    const dialog = useConfirmDialog()

    const clickPromise = wrapper.find('.btn-outline-warning').trigger('click')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(rotateHostKey).toHaveBeenCalledWith('h1')
    expect(wrapper.text()).toContain('new-secret-key')
    expect(wrapper.text()).toContain('Nouvelle clé générée')
  })

  it('copies the rotated key, install command, and agent config, each with its own "Copié" confirmation', async () => {
    rotateHostKey.mockResolvedValue({ data: { api_key: 'new-secret-key' } })
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('.btn-outline-warning').trigger('click')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    const copyButtons = wrapper.findAll('button').filter((b) => b.text() === 'Copier' || b.text() === 'Copier la config')
    expect(copyButtons.length).toBe(3)

    await copyButtons[0].trigger('click')
    await flushPromises()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('new-secret-key')
    expect(wrapper.text()).toContain('Copié')
  })

  it('logs to the console and clears loading state without throwing when the rotation request fails', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    rotateHostKey.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(HostEditForm, { props: { hostId: 'h1', host } })
    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('.btn-outline-warning').trigger('click')
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(consoleSpy).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Régénérer la clé')
    consoleSpy.mockRestore()
  })
})
