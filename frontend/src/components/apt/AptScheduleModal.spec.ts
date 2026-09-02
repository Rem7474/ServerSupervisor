import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { createScheduledTask } = vi.hoisted(() => ({ createScheduledTask: vi.fn() }))
vi.mock('../../api', () => ({
  default: { createScheduledTask },
}))

import AptScheduleModal from './AptScheduleModal.vue'

const host = { id: 'h1', name: 'web-01', hostname: 'web-01.internal' }

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

describe('AptScheduleModal', () => {
  it('renders nothing when host is null', () => {
    const wrapper = mount(AptScheduleModal, { props: { host: null } })
    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it('shows the host name and hostname distinctly when they differ', () => {
    const wrapper = mount(AptScheduleModal, { props: { host } })
    expect(wrapper.text()).toContain('web-01 (web-01.internal)')
  })

  it('shows only the name when hostname matches it', () => {
    const wrapper = mount(AptScheduleModal, { props: { host: { id: 'h1', name: 'web-01', hostname: 'web-01' } } })
    expect(wrapper.text()).toContain('web-01')
    expect(wrapper.text()).not.toContain('web-01 (web-01)')
  })

  it('shows the CronBuilder and the Enabled toggle by default, hides both in manual-only mode', async () => {
    const wrapper = mount(AptScheduleModal, { props: { host } })
    expect(wrapper.text()).toContain('Expression cron')
    expect(wrapper.find('#schedEnabled').exists()).toBe(true)

    const manualToggle = wrapper.find('input[type="checkbox"]')
    await manualToggle.setValue(true)
    expect(wrapper.text()).not.toContain('Expression cron')
    expect(wrapper.find('#schedEnabled').exists()).toBe(false)
  })

  it('emits close when the header close button is clicked', async () => {
    const wrapper = mount(AptScheduleModal, { props: { host } })
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits close when the footer cancel button is clicked', async () => {
    const wrapper = mount(AptScheduleModal, { props: { host } })
    const cancelBtn = wrapper.findAll('.modal-footer button').find((b) => b.text() === 'Annuler')
    await cancelBtn!.trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('creates a scheduled task with the default name derived from the action, and closes the modal', async () => {
    createScheduledTask.mockResolvedValue({ data: {} })
    const wrapper = mount(AptScheduleModal, { props: { host } })

    const saveBtn = wrapper.findAll('.modal-footer button').find((b) => b.text().includes('Créer la tâche'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(createScheduledTask).toHaveBeenCalledWith('h1', expect.objectContaining({
      name: 'apt update',
      module: 'apt',
      action: 'update',
      enabled: true,
    }))
    expect(wrapper.emitted('created')).toBeTruthy()
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('uses the custom task name when provided', async () => {
    createScheduledTask.mockResolvedValue({ data: {} })
    const wrapper = mount(AptScheduleModal, { props: { host } })

    await wrapper.find('input[type="text"]').setValue('nightly upgrade')
    const saveBtn = wrapper.findAll('.modal-footer button').find((b) => b.text().includes('Créer la tâche'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(createScheduledTask).toHaveBeenCalledWith('h1', expect.objectContaining({ name: 'nightly upgrade' }))
  })

  it('forces enabled=false and uses the manual sentinel cron when manual-only is checked', async () => {
    createScheduledTask.mockResolvedValue({ data: {} })
    const wrapper = mount(AptScheduleModal, { props: { host } })

    await wrapper.find('input[type="checkbox"]').setValue(true)
    const saveBtn = wrapper.findAll('.modal-footer button').find((b) => b.text().includes('Créer la tâche'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(createScheduledTask).toHaveBeenCalledWith('h1', expect.objectContaining({ enabled: false }))
    const payload = createScheduledTask.mock.calls[0][1]
    expect(payload.cron_expression).not.toBe('0 3 * * 0')
  })

  it('shows a translated error and keeps the modal open when saving fails', async () => {
    createScheduledTask.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(AptScheduleModal, { props: { host } })

    const saveBtn = wrapper.findAll('.modal-footer button').find((b) => b.text().includes('Créer la tâche'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Erreur lors de la création')
    expect(wrapper.emitted('close')).toBeFalsy()
    expect(wrapper.find('.modal').exists()).toBe(true)
  })

  it('resets the form when a new host is assigned', async () => {
    const wrapper = mount(AptScheduleModal, { props: { host } })
    await wrapper.find('input[type="text"]').setValue('custom name')
    await wrapper.find('input[type="checkbox"]').setValue(true)

    await wrapper.setProps({ host: { id: 'h2', name: 'db-01' } })

    expect((wrapper.find('input[type="text"]').element as HTMLInputElement).value).toBe('')
    expect((wrapper.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
  })
})
