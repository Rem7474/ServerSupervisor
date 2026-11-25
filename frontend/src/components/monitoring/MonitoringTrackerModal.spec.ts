import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { setLocale } from '../../i18n'
import MonitoringTrackerModal from './MonitoringTrackerModal.vue'
import type { ProbeForm } from '../../composables/useUptimeProbes'
import type { CertForm } from '../../composables/useSslCertificates'

function probe(id = ''): ProbeForm {
  return reactive({
    id, name: '', type: 'http', target: '', interval_sec: 60, timeout_sec: 10, expected_status: 200,
    expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true, slo_target: 99.9,
  })
}
function cert(id = ''): CertForm {
  return reactive({ id, name: '', host: '', port: 443, server_name: '', enabled: true })
}

function mountModal(o: {
  probeOpen?: boolean; certOpen?: boolean; probeId?: string; certId?: string
  savingProbe?: boolean; savingCert?: boolean; probeError?: string; certError?: string
} = {}) {
  const closeProbeModal = vi.fn()
  const closeCertModal = vi.fn()
  const wrapper = mount(MonitoringTrackerModal, {
    attachTo: document.body,
    props: {
      probeModalOpen: o.probeOpen ?? false,
      certModalOpen: o.certOpen ?? false,
      savingProbe: o.savingProbe ?? false,
      savingCert: o.savingCert ?? false,
      probeFormError: o.probeError ?? '',
      certFormError: o.certError ?? '',
      isProbeModalOpen: () => o.probeOpen ?? false,
      isCertModalOpen: () => o.certOpen ?? false,
      saveProbe: vi.fn(async () => {}),
      saveCert: vi.fn(async () => {}),
      closeProbeModal, closeCertModal,
      probeForm: probe(o.probeId), certForm: cert(o.certId),
    },
  })
  return { wrapper, closeProbeModal, closeCertModal }
}

describe('MonitoringTrackerModal', () => {
  beforeEach(() => {
    setLocale('fr')
    document.body.innerHTML = ''
  })

  it('renders nothing while both modals are closed', () => {
    const { wrapper } = mountModal()
    expect(wrapper.find('.modal').exists()).toBe(false)
    expect(wrapper.find('.modal-backdrop').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows the create form titled "Nouveau suivi" for a new probe or cert', () => {
    const p = mountModal({ probeOpen: true })
    expect(p.wrapper.find('.modal-title').text()).toBe('Nouveau suivi')
    expect(p.wrapper.find('#monitoring-create-name').exists()).toBe(true)
    expect(p.wrapper.find('.modal-backdrop').exists()).toBe(true)
    p.wrapper.unmount()
    const c = mountModal({ certOpen: true })
    expect(c.wrapper.find('.modal-title').text()).toBe('Nouveau suivi')
    expect(c.wrapper.find('#monitoring-create-cert-host').exists()).toBe(true)
    c.wrapper.unmount()
  })

  it('shows the probe edit form and title when the probe has an id', () => {
    const { wrapper } = mountModal({ probeOpen: true, probeId: 'p1' })
    expect(wrapper.find('.modal-title').text()).toBe('Modifier la sonde')
    expect(wrapper.find('#monitoring-edit-probe-name').exists()).toBe(true)
    expect(wrapper.find('#monitoring-create-name').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows the cert edit form and title when the cert has an id', () => {
    const { wrapper } = mountModal({ certOpen: true, certId: 'c1' })
    expect(wrapper.find('.modal-title').text()).toBe('Modifier le certificat')
    expect(wrapper.find('#monitoring-edit-cert-name').exists()).toBe(true)
    wrapper.unmount()
  })

  it('passes the matching error to the form (probe error for edit probe, either for create)', () => {
    const edit = mountModal({ probeOpen: true, probeId: 'p1', probeError: 'sonde KO', certError: 'cert KO' })
    expect(edit.wrapper.find('.alert-danger').text()).toBe('sonde KO')
    edit.wrapper.unmount()
    const create = mountModal({ certOpen: true, certError: 'cert KO' })
    expect(create.wrapper.find('.alert-danger').text()).toBe('cert KO')
    create.wrapper.unmount()
  })

  it('closes both underlying modals from the header close button', async () => {
    const { wrapper, closeProbeModal, closeCertModal } = mountModal({ probeOpen: true })
    await wrapper.find('.btn-close').trigger('click')
    expect(closeProbeModal).toHaveBeenCalledOnce()
    expect(closeCertModal).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('closes from the form cancel button', async () => {
    const { wrapper, closeProbeModal } = mountModal({ probeOpen: true, probeId: 'p1' })
    await wrapper.find('.modal-footer button[type="button"]').trigger('click')
    expect(closeProbeModal).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('closes on Escape', async () => {
    const { wrapper, closeProbeModal, closeCertModal } = mountModal({ probeOpen: true })
    await wrapper.vm.$nextTick()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(closeProbeModal).toHaveBeenCalledOnce()
    expect(closeCertModal).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('moves focus to the first input when opened', async () => {
    // The chrome only finds its element once the modal toggles open after mount.
    const { wrapper } = mountModal()
    await wrapper.setProps({ probeModalOpen: true, isProbeModalOpen: () => true })
    await wrapper.vm.$nextTick()
    expect(document.activeElement?.id).toBe('monitoring-create-name')
    wrapper.unmount()
  })

  it('disables the header close button while a save is in flight', () => {
    const probeSaving = mountModal({ probeOpen: true, savingProbe: true })
    expect(probeSaving.wrapper.find('.btn-close').attributes('disabled')).toBeDefined()
    probeSaving.wrapper.unmount()
    const certSaving = mountModal({ certOpen: true, savingCert: true })
    expect(certSaving.wrapper.find('.btn-close').attributes('disabled')).toBeDefined()
    certSaving.wrapper.unmount()
    const idle = mountModal({ probeOpen: true })
    expect(idle.wrapper.find('.btn-close').attributes('disabled')).toBeUndefined()
    idle.wrapper.unmount()
  })
})
