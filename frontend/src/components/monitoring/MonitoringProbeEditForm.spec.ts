import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { setLocale } from '../../i18n'
import MonitoringProbeEditForm from './MonitoringProbeEditForm.vue'
import type { ProbeForm } from '../../composables/useUptimeProbes'

function form(over: Partial<ProbeForm> = {}): ProbeForm {
  return reactive({
    id: 'p1', name: 'API', type: 'http', target: 'https://x.io', interval_sec: 60, timeout_sec: 10,
    expected_status: 200, expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true, ...over,
  })
}

function mountForm(probeForm: ProbeForm, props: { saving?: boolean; error?: string } = {}) {
  return mount(MonitoringProbeEditForm, { props: { probeForm, saving: false, error: '', ...props } })
}

describe('MonitoringProbeEditForm', () => {
  beforeEach(() => setLocale('fr'))

  it('writes edits back to the bound form', async () => {
    const f = form()
    const w = mountForm(f)
    await w.find('#monitoring-edit-probe-name').setValue('Renamed')
    await w.find('#monitoring-edit-probe-target').setValue('https://y.io')
    await w.find('#monitoring-edit-probe-interval').setValue('120')
    await w.find('#monitoring-edit-probe-expected-status').setValue('204')
    expect(f.name).toBe('Renamed')
    expect(f.target).toBe('https://y.io')
    expect(f.interval_sec).toBe(120)
    expect(f.expected_status).toBe(204)
  })

  it('shows the http-only fields for http and hides them for tcp / icmp', async () => {
    const f = form()
    const w = mountForm(f)
    expect(w.find('#monitoring-edit-probe-expected-body-regex').exists()).toBe(true)
    expect(w.text()).toContain('URL')
    f.type = 'tcp'
    await w.vm.$nextTick()
    expect(w.find('#monitoring-edit-probe-expected-body-regex').exists()).toBe(false)
    expect(w.text()).toContain('host:port')
    f.type = 'icmp'
    await w.vm.$nextTick()
    expect(w.find('.form-hint').exists()).toBe(true)
    expect(w.text()).toContain('Hôte ou IP')
  })

  it('has no icmp hint for http probes', () => {
    expect(mountForm(form()).find('.form-hint').exists()).toBe(false)
  })

  it('toggles the enabled checkbox on the form', async () => {
    const f = form({ enabled: true })
    const w = mountForm(f)
    const boxes = w.findAll('input[type="checkbox"]')
    await boxes[boxes.length - 1].setValue(false)
    expect(f.enabled).toBe(false)
  })

  it('displays the error only when there is one', () => {
    expect(mountForm(form()).find('.alert-danger').exists()).toBe(false)
    expect(mountForm(form(), { error: 'nom déjà pris' }).find('.alert-danger').text()).toBe('nom déjà pris')
  })

  it('emits save on submit and close on cancel', async () => {
    const w = mountForm(form())
    await w.find('form').trigger('submit')
    expect(w.emitted('save')).toHaveLength(1)
    await w.find('button[type="button"]').trigger('click')
    expect(w.emitted('close')).toHaveLength(1)
  })

  it('disables both buttons and shows the saving label while saving', () => {
    const idle = mountForm(form())
    expect(idle.find('button[type="submit"]').text()).toBe('Enregistrer')
    expect(idle.find('button[type="submit"]').attributes('disabled')).toBeUndefined()

    const busy = mountForm(form(), { saving: true })
    expect(busy.find('button[type="submit"]').text()).toBe('Enregistrement…')
    expect(busy.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(busy.find('button[type="button"]').attributes('disabled')).toBeDefined()
  })
})
