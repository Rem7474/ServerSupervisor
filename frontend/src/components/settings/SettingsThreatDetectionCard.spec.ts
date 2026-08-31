import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsThreatDetectionCard from './SettingsThreatDetectionCard.vue'

beforeEach(() => {
  setLocale('fr')
})

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

  it('shows no incoherence warning when thresholds are strictly increasing', () => {
    const wrapper = mount(SettingsThreatDetectionCard, {
      props: { form: defaultForm() },
    })
    expect(wrapper.text()).not.toContain('devraient être croissants')
  })

  it('shows an incoherence warning when thresholds are not increasing', () => {
    const wrapper = mount(SettingsThreatDetectionCard, {
      props: { form: { ...defaultForm(), threatThresholdMedium: 50, threatThresholdHigh: 30 } },
    })
    expect(wrapper.text()).toContain('devraient être croissants')
  })

  it('shows an incoherence warning when medium < high but high is not < critical', () => {
    const wrapper = mount(SettingsThreatDetectionCard, {
      props: { form: { ...defaultForm(), threatThresholdMedium: 10, threatThresholdHigh: 60, threatThresholdCritical: 30 } },
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

describe('SettingsThreatDetectionCard — every field writes emit a whole-object update:form, never mutate the prop', () => {
  // Each field is wired to its own fieldModel('<key>') call by hand in the
  // component — a copy-paste error there (e.g. two fields sharing a key)
  // would make one field silently control another's value. Checking every
  // id individually catches exactly that, which a single representative
  // field wouldn't.
  const fieldIdToKey: Record<string, keyof ReturnType<typeof defaultForm>> = {
    'threat-weight-wordpress': 'threatWeightWordpress',
    'threat-weight-adminpanel': 'threatWeightAdminpanel',
    'threat-weight-pathtraversal': 'threatWeightPathtraversal',
    'threat-weight-knownscanner': 'threatWeightKnownscanner',
    'threat-weight-suspiciousmethod': 'threatWeightSuspiciousmethod',
    'threat-weight-status-2xx': 'threatWeightStatus2xx',
    'threat-weight-status-3xx': 'threatWeightStatus3xx',
    'threat-weight-status-404': 'threatWeightStatus404',
    'threat-weight-status-4xx': 'threatWeightStatus4xx',
    'threat-weight-status-5xx': 'threatWeightStatus5xx',
    'threat-weight-breadth': 'threatWeightBreadth',
    'threat-weight-hits': 'threatWeightHits',
    'threat-threshold-medium': 'threatThresholdMedium',
    'threat-threshold-high': 'threatThresholdHigh',
    'threat-threshold-critical': 'threatThresholdCritical',
  }

  for (const [id, key] of Object.entries(fieldIdToKey)) {
    it(`#${id} writes only ${key}`, async () => {
      const form = defaultForm()
      const wrapper = mount(SettingsThreatDetectionCard, { props: { form } })

      await wrapper.find(`#${id}`).setValue(999)

      const next = wrapper.emitted('update:form')![0][0] as ReturnType<typeof defaultForm>
      expect(next[key]).toBe(999)
      for (const otherKey of Object.keys(form) as Array<keyof ReturnType<typeof defaultForm>>) {
        if (otherKey !== key) expect(next[otherKey]).toBe(form[otherKey])
      }
      expect(form[key]).not.toBe(999) // prop untouched
    })
  }
})
