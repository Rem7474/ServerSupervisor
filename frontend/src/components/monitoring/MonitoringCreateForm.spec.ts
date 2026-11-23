import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import { setLocale } from '../../i18n'
import MonitoringCreateForm from './MonitoringCreateForm.vue'
import type { ProbeForm } from '../../composables/useUptimeProbes'
import type { CertForm } from '../../composables/useSslCertificates'

function probe(): ProbeForm {
  return reactive({
    id: '', name: '', type: 'http', target: '', interval_sec: 60, timeout_sec: 10, expected_status: 200,
    expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true,
  })
}
function cert(): CertForm {
  return reactive({ id: '', name: '', host: '', port: 443, server_name: '', enabled: true })
}

interface Extra { saving?: boolean; error?: string; saveProbe?: () => Promise<void>; saveCert?: () => Promise<void> }

// The toggles' initial state comes from the open-state getters, so build them per-test.
function mountWith(probeOpen: boolean | (() => boolean), certOpen: boolean, extra: Extra = {}) {
  const probeForm = probe()
  const certForm = cert()
  const saveProbe = vi.fn(extra.saveProbe ?? (async () => {}))
  const saveCert = vi.fn(extra.saveCert ?? (async () => {}))
  const wrapper = mount(MonitoringCreateForm, {
    props: {
      probeForm, certForm, saving: extra.saving ?? false, error: extra.error ?? '',
      isProbeModalOpen: typeof probeOpen === 'function' ? probeOpen : () => probeOpen, isCertModalOpen: () => certOpen,
      saveProbe, saveCert,
    },
  })
  return { wrapper, probeForm, certForm, saveProbe, saveCert }
}

describe('MonitoringCreateForm', () => {
  beforeEach(() => setLocale('fr'))

  it('shows only the probe section when opened for a probe', () => {
    const { wrapper } = mountWith(true, false)
    expect(wrapper.find('#monitoring-create-probe-target').exists()).toBe(true)
    expect(wrapper.find('#monitoring-create-cert-host').exists()).toBe(false)
  })

  it('shows the cert host/port fields (no derived hint) when only the cert is selected', () => {
    const { wrapper } = mountWith(false, true)
    expect(wrapper.find('#monitoring-create-probe-target').exists()).toBe(false)
    expect(wrapper.find('#monitoring-create-cert-host').exists()).toBe(true)
    expect(wrapper.find('#monitoring-create-cert-port').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Certificat SSL vérifié sur')
  })

  it('toggling the cert on adds a derived host:port hint that follows the probe target', async () => {
    const { wrapper } = mountWith(true, false)
    const [, certToggle] = wrapper.findAll('.btn-group button')
    await certToggle.trigger('click')
    expect(wrapper.find('#monitoring-create-cert-host').exists()).toBe(false)
    await wrapper.find('#monitoring-create-probe-target').setValue('https://api.example.com:8443/x')
    expect(wrapper.find('.col-12.text-secondary code').text()).toBe('api.example.com:8443')
  })

  it('shows a dash for the derived host while the target is empty', async () => {
    const { wrapper } = mountWith(true, true)
    expect(wrapper.find('.col-12.text-secondary code').text()).toBe('—:443')
  })

  it('refuses to untoggle the only selected resource', async () => {
    const { wrapper } = mountWith(true, false)
    const [probeToggle] = wrapper.findAll('.btn-group button')
    await probeToggle.trigger('click')
    expect(probeToggle.classes()).toContain('btn-primary')
    expect(wrapper.find('#monitoring-create-probe-target').exists()).toBe(true)
  })

  it('writes the shared name into both forms when both are selected', async () => {
    const { wrapper, probeForm, certForm } = mountWith(true, true)
    await wrapper.find('#monitoring-create-name').setValue('Prod')
    expect(probeForm.name).toBe('Prod')
    expect(certForm.name).toBe('Prod')
  })

  it('switches target hints and http-only fields with the probe type', async () => {
    const { wrapper } = mountWith(true, false)
    expect(wrapper.find('#monitoring-create-probe-expected-body-regex').exists()).toBe(true)
    await wrapper.find('#monitoring-create-probe-type').setValue('icmp')
    expect(wrapper.find('#monitoring-create-probe-expected-body-regex').exists()).toBe(false)
    expect(wrapper.find('.form-hint').exists()).toBe(true)
    expect(wrapper.text()).toContain('Hôte ou IP')
    await wrapper.find('#monitoring-create-probe-type').setValue('tcp')
    expect(wrapper.find('.form-hint').exists()).toBe(false)
    expect(wrapper.text()).toContain('host:port')
  })

  it('submits through saveProbe only for a probe-only creation', async () => {
    const { wrapper, saveProbe, saveCert } = mountWith(true, false)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(saveProbe).toHaveBeenCalledOnce()
    expect(saveCert).not.toHaveBeenCalled()
  })

  it('submits probe then cert when both are selected and the probe save succeeds', async () => {
    let probeOpen = true
    const { wrapper, saveProbe, saveCert } = mountWith(() => probeOpen, true, {
      saveProbe: async () => { probeOpen = false },
    })
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(saveProbe).toHaveBeenCalledOnce()
    expect(saveCert).toHaveBeenCalledOnce()
  })

  it('does not attempt the cert when the probe save leaves its modal open (failure)', async () => {
    const { wrapper, saveProbe, saveCert } = mountWith(() => true, true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(saveProbe).toHaveBeenCalledOnce()
    expect(saveCert).not.toHaveBeenCalled()
  })

  it('displays the error and emits close from the cancel button', async () => {
    const { wrapper } = mountWith(true, false, { error: 'échec' })
    expect(wrapper.find('.alert-danger').text()).toBe('échec')
    await wrapper.find('.modal-footer button[type="button"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('has no error box without an error', () => {
    expect(mountWith(true, false).wrapper.find('.alert-danger').exists()).toBe(false)
  })

  it('disables the footer buttons and shows the saving label while saving', () => {
    const idle = mountWith(true, false).wrapper
    expect(idle.find('button[type="submit"]').text()).toBe('Enregistrer')
    const busy = mountWith(true, false, { saving: true }).wrapper
    expect(busy.find('button[type="submit"]').text()).toBe('Enregistrement…')
    expect(busy.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(busy.find('.modal-footer button[type="button"]').attributes('disabled')).toBeDefined()
  })
})
