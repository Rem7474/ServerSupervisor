import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setLocale } from '../../i18n'
import AlertRuleTemplateModal from './AlertRuleTemplateModal.vue'

beforeEach(() => {
  setLocale('fr')
})

const agentMetrics = [
  { metric: 'cpu', label: 'CPU', unit: '%', icon: '', badge_class: '', supports_threshold: true, supports_duration: true, supports_host_filter: true },
  { metric: 'bandwidth_vs_rolling_avg', label: 'Bande passante', unit: '%', icon: '', badge_class: '', supports_threshold: true, supports_duration: false, supports_host_filter: true },
]

function mountModal(props: Record<string, unknown> = {}) {
  return mount(AlertRuleTemplateModal, {
    props: {
      visible: true,
      agentMetrics,
      ...props,
    },
  })
}

describe('AlertRuleTemplateModal (characterization)', () => {
  it('renders the generic fields (name, metric, operator, thresholds) with associated labels', () => {
    const wrapper = mountModal()

    for (const id of [
      'alert-template-name',
      'alert-template-metric',
      'alert-template-operator',
      'alert-template-threshold-warn',
      'alert-template-threshold-crit',
      'alert-template-duration',
      'alert-template-cooldown',
      'alert-template-escalate-after-minutes',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
    // "Duration" is hidden for the bandwidth metric — not selected by default.
    expect(wrapper.find('#alert-template-baseline-window').exists()).toBe(false)
  })

  it('shows the baseline-window field instead of duration for bandwidth_vs_rolling_avg', async () => {
    const wrapper = mountModal()
    await wrapper.find('#alert-template-metric').setValue('bandwidth_vs_rolling_avg')
    await nextTick()

    expect(wrapper.find('#alert-template-baseline-window').exists()).toBe(true)
    expect(wrapper.find('#alert-template-duration').exists()).toBe(false)
  })

  it('toggling a notification channel checkbox works under the "Canaux de notification" group', async () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Canaux de notification')

    const smtpCheckbox = wrapper.findAll('input[type="checkbox"]')[0]
    expect(smtpCheckbox).toBeTruthy()
    await smtpCheckbox.setValue(true)
    expect((smtpCheckbox.element as HTMLInputElement).checked).toBe(true)
  })

  it('emits "close" when the close button is clicked', async () => {
    const wrapper = mountModal()
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('pre-fills the form from an existing template (edit mode)', () => {
    const wrapper = mountModal({
      template: {
        id: 1,
        name: 'CPU élevé',
        metric: 'cpu',
        operator: '>',
        threshold_warn: 70,
        threshold_crit: 90,
        duration_seconds: 60,
        actions: { channels: ['smtp'], cooldown: 300, escalate_after_minutes: 0 },
      },
    })
    expect((wrapper.find('#alert-template-name').element as HTMLInputElement).value).toBe('CPU élevé')
  })

  it('switches the threshold labels to percent-of-average for bandwidth_vs_rolling_avg', async () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Seuil avertissement')
    expect(wrapper.text()).not.toContain('Seuil avertissement (% moyenne)')

    await wrapper.find('#alert-template-metric').setValue('bandwidth_vs_rolling_avg')
    await nextTick()
    expect(wrapper.text()).toContain('Seuil avertissement (% moyenne)')
    expect(wrapper.text()).toContain('Seuil critique (% moyenne)')
  })

  it('submits the form with the checked channels and the correct button label per mode', async () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Créer')

    await wrapper.find('#alert-template-name').setValue('High CPU')
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(true)
    await wrapper.find('form').trigger('submit.prevent')

    expect(wrapper.emitted('submit')?.[0]?.[0]).toMatchObject({
      name: 'High CPU',
      actions: expect.objectContaining({ channels: ['smtp'] }),
    })
  })

  it('shows "Enregistrer" instead of "Créer" when editing an existing template', () => {
    const wrapper = mountModal({
      template: {
        id: 1, name: 'CPU', metric: 'cpu', operator: '>', threshold_warn: 70, threshold_crit: 90,
        duration_seconds: 60, actions: { channels: [], cooldown: 0, escalate_after_minutes: 0 },
      },
    })
    const submitButton = wrapper.find('button[type="submit"]')
    expect(submitButton.text()).toBe('Enregistrer')
  })
})
