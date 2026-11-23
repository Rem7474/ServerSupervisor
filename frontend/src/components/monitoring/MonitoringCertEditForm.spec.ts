import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { setLocale } from '../../i18n'
import MonitoringCertEditForm from './MonitoringCertEditForm.vue'
import type { CertForm } from '../../composables/useSslCertificates'

function form(): CertForm {
  return reactive({ id: 'c1', name: 'api', host: 'api.example.com', port: 443, server_name: '', enabled: true })
}

function mountForm(certForm: CertForm, props: { saving?: boolean; error?: string } = {}) {
  return mount(MonitoringCertEditForm, { props: { certForm, saving: false, error: '', ...props } })
}

describe('MonitoringCertEditForm', () => {
  beforeEach(() => setLocale('fr'))

  it('shows the current values and writes edits back (port as a number)', async () => {
    const f = form()
    const w = mountForm(f)
    expect((w.find('#monitoring-edit-cert-host').element as HTMLInputElement).value).toBe('api.example.com')
    await w.find('#monitoring-edit-cert-name').setValue('new name')
    await w.find('#monitoring-edit-cert-host').setValue('b.example.com')
    await w.find('#monitoring-edit-cert-port').setValue('8443')
    await w.find('#monitoring-edit-cert-sni').setValue('sni.example.com')
    await w.find('input[type="checkbox"]').setValue(false)
    expect(f).toMatchObject({ name: 'new name', host: 'b.example.com', port: 8443, server_name: 'sni.example.com', enabled: false })
  })

  it('displays the error only when there is one', () => {
    expect(mountForm(form()).find('.alert-danger').exists()).toBe(false)
    expect(mountForm(form(), { error: 'hôte invalide' }).find('.alert-danger').text()).toBe('hôte invalide')
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
