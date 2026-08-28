import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlertRuleStepConditions from './AlertRuleStepConditions.vue'
import { useAlertRuleForm } from '../../composables/useAlertRuleForm'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

function formFor(metric: string, overrides: Partial<AlertRuleFormData> = {}): AlertRuleFormData {
  const base = useAlertRuleForm().defaultForm()
  return { ...base, metric, ...overrides }
}

function mountStep(form: AlertRuleFormData) {
  return mount(AlertRuleStepConditions, { props: { form } })
}

describe('AlertRuleStepConditions (characterization, per-metric branches)', () => {
  it('docker_container_state: renders the warn/crit state checklists and a duration field', () => {
    const wrapper = mountStep(formFor('docker_container_state'))
    expect(wrapper.text()).toContain('warn')
    expect(wrapper.text()).toContain('crit')
    expect(wrapper.find('#alert-cond-duration').exists()).toBe(true)
  })

  it('docker_compose_degraded_services: renders warn/crit thresholds and a duration field', () => {
    const wrapper = mountStep(formFor('docker_compose_degraded_services'))
    for (const id of ['alert-cond-threshold-warn', 'alert-cond-threshold-crit', 'alert-cond-compose-duration']) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
    }
  })

  it('bandwidth_vs_rolling_avg: renders the baseline window select and both thresholds', () => {
    const wrapper = mountStep(formFor('bandwidth_vs_rolling_avg', { baseline_window_seconds: 3600 }))
    for (const id of ['alert-cond-baseline-window', 'alert-cond-bandwidth-threshold-warn', 'alert-cond-bandwidth-threshold-crit']) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
    }
  })

  it('heartbeat_timeout: renders the max-silence field', () => {
    const wrapper = mountStep(formFor('heartbeat_timeout'))
    expect(wrapper.find('#alert-cond-heartbeat-timeout').exists()).toBe(true)
  })

  it('generic metric (e.g. cpu): renders operator, thresholds, hysteresis and duration', () => {
    const wrapper = mountStep(formFor('cpu'))
    for (const id of [
      'alert-cond-operator',
      'alert-cond-generic-threshold-warn',
      'alert-cond-generic-threshold-crit',
      'alert-cond-threshold-clear-warn',
      'alert-cond-threshold-clear-crit',
      'alert-cond-generic-duration',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
  })
})
