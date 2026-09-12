import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import SettingsMaintenanceCard from './SettingsMaintenanceCard.vue'

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
})

describe('SettingsMaintenanceCard', () => {
  it('shows the retention days in both cleanup descriptions', () => {
    const wrapper = mount(SettingsMaintenanceCard, {
      props: { settings: { metricsRetentionDays: 30, auditRetentionDays: 90 } },
    })
    expect(wrapper.text()).toContain('30 jours')
    expect(wrapper.text()).toContain('90 jours')
  })

  it('shows the in-progress label and disables the button while cleaning', () => {
    const wrapper = mount(SettingsMaintenanceCard, {
      props: { settings: { metricsRetentionDays: 30, auditRetentionDays: 90 }, cleaningMetrics: true },
    })
    const buttons = wrapper.findAll('button')
    expect(buttons[0].text()).toBe('Nettoyage en cours...')
    expect(buttons[0].attributes('disabled')).toBeDefined()
  })

  it('asks for confirmation before emitting clean-metrics, and does not emit if cancelled', async () => {
    const wrapper = mount(SettingsMaintenanceCard, {
      props: { settings: { metricsRetentionDays: 30, auditRetentionDays: 90 } },
    })
    const clickPromise = wrapper.findAll('button')[0].trigger('click')
    const dialog = useConfirmDialog()
    expect(dialog.title.value).toBe('Confirmer le nettoyage')
    expect(dialog.message.value).toContain('30 jours')
    dialog.onCancel()
    await clickPromise

    expect(wrapper.emitted('clean-metrics')).toBeUndefined()
  })

  it('emits clean-audit once the audit cleanup is confirmed', async () => {
    const wrapper = mount(SettingsMaintenanceCard, {
      props: { settings: { metricsRetentionDays: 30, auditRetentionDays: 90 } },
    })
    const clickPromise = wrapper.findAll('button')[1].trigger('click')
    const dialog = useConfirmDialog()
    expect(dialog.message.value).toContain('90 jours')
    dialog.onConfirm()
    await clickPromise

    expect(wrapper.emitted('clean-audit')).toHaveLength(1)
  })

  it('shows the success/failure feedback message when present', () => {
    const wrapper = mount(SettingsMaintenanceCard, {
      props: {
        settings: { metricsRetentionDays: 30, auditRetentionDays: 90 },
        cleanMessage: 'Nettoyage effectué.', cleanSuccess: true,
      },
    })
    expect(wrapper.text()).toContain('Nettoyage effectué.')
    expect(wrapper.find('.alert').classes()).toContain('alert-success')
  })
})
