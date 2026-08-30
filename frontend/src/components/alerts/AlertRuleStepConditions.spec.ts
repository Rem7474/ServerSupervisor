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

describe('AlertRuleStepConditions — field writes emit a whole-object update:form, never mutate the prop', () => {
  it('toggleState adds/removes a docker state without touching the other severity list', async () => {
    const form = formFor('docker_container_state')
    const wrapper = mountStep(form)

    const warnExited = wrapper.find('input[type="checkbox"]')
    await warnExited.setValue(true)

    let emitted = wrapper.emitted('update:form')
    expect(emitted).toHaveLength(1)
    let next = emitted![0][0] as AlertRuleFormData
    expect(next.docker_scope.warn_states).toContain('created')
    expect(next.docker_scope.crit_states).toEqual(form.docker_scope.crit_states)
    // The original prop object must be untouched — this is exactly the
    // regression S8951 flagged (mutating props.form in place).
    expect(form.docker_scope.warn_states).not.toContain('created')

    // Feed the emitted value back in (as the real v-model:form parent
    // would) and uncheck it again.
    await wrapper.setProps({ form: next })
    await warnExited.setValue(false)
    emitted = wrapper.emitted('update:form')
    next = emitted![1][0] as AlertRuleFormData
    expect(next.docker_scope.warn_states).not.toContain('created')
  })

  it('durationModel emits the merged form with only duration changed', async () => {
    const form = formFor('docker_container_state', { duration: 60 })
    const wrapper = mountStep(form)

    await wrapper.find('#alert-cond-duration').setValue(120)

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.duration).toBe(120)
    expect(next.metric).toBe('docker_container_state')
    expect(form.duration).toBe(60) // prop untouched
  })

  it('operatorModel (a non-numeric field) emits the merged form via the same fieldModel factory', async () => {
    const form = formFor('cpu', { operator: '>' })
    const wrapper = mountStep(form)

    await wrapper.find('#alert-cond-operator').setValue('<')

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.operator).toBe('<')
    expect(form.operator).toBe('>') // prop untouched
  })

  it('threshold_clear_warn/crit writes go through the same updateForm path as the trigger thresholds', async () => {
    const form = formFor('cpu', { operator: '>', threshold_warn: 80, threshold_clear_warn: 70 })
    const wrapper = mountStep(form)

    await wrapper.find('#alert-cond-threshold-clear-warn').setValue(75)

    const next = wrapper.emitted('update:form')![0][0] as AlertRuleFormData
    expect(next.threshold_clear_warn).toBe(75)
    expect(next.threshold_warn).toBe(80) // untouched sibling field
  })
})
