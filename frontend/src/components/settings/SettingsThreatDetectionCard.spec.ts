import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsThreatDetectionCard from './SettingsThreatDetectionCard.vue'

function defaultForm() {
  return {
    threatWeightWordpress: 1,
    threatWeightAdminpanel: 1,
    threatWeightPathtraversal: 1,
    threatWeightKnownscanner: 1,
    threatWeightSuspiciousmethod: 1,
    threatWeightStatus2xx: 0.1,
    threatWeightStatus3xx: 0.2,
    threatWeightStatus404: 0.5,
    threatWeightStatus4xx: 0.3,
    threatWeightStatus5xx: 0.4,
    threatWeightBreadth: 1,
    threatWeightHits: 1,
    threatThresholdMedium: 10,
    threatThresholdHigh: 30,
    threatThresholdCritical: 60,
  }
}

describe('SettingsThreatDetectionCard (characterization)', () => {
  it('renders every weight/threshold field with its label correctly associated', () => {
    const wrapper = mount(SettingsThreatDetectionCard, {
      props: { form: defaultForm() },
    })

    const fieldIds = [
      'threat-weight-wordpress',
      'threat-weight-adminpanel',
      'threat-weight-pathtraversal',
      'threat-weight-knownscanner',
      'threat-weight-suspiciousmethod',
      'threat-weight-status-2xx',
      'threat-weight-status-3xx',
      'threat-weight-status-404',
      'threat-weight-status-4xx',
      'threat-weight-status-5xx',
      'threat-weight-breadth',
      'threat-weight-hits',
      'threat-threshold-medium',
      'threat-threshold-high',
      'threat-threshold-critical',
    ]

    for (const id of fieldIds) {
      const input = wrapper.find(`#${id}`)
      expect(input.exists(), `expected #${id} to exist`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `expected a label for #${id}`).toBe(true)
    }
  })

  it('shows an incoherence warning when thresholds are not increasing', () => {
    const wrapper = mount(SettingsThreatDetectionCard, {
      props: { form: { ...defaultForm(), threatThresholdMedium: 50, threatThresholdHigh: 30 } },
    })
    expect(wrapper.text()).toContain('devraient être croissants')
  })

  it('emits "save" when the admin-only save button is clicked', async () => {
    const wrapper = mount(SettingsThreatDetectionCard, {
      props: { form: defaultForm(), authIsAdmin: true },
    })
    await wrapper.find('.card-footer button').trigger('click')
    expect(wrapper.emitted('save')).toBeTruthy()
  })
})
