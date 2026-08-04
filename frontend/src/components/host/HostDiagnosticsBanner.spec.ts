import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import HostDiagnosticsBanner from './HostDiagnosticsBanner.vue'
import type { DiagnosticIssue } from '../../types/host'

describe('HostDiagnosticsBanner', () => {
  it('renders nothing when there are no issues', () => {
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics: [] } })
    expect(wrapper.find('.alert').exists()).toBe(false)
  })

  it('renders nothing when diagnostics is undefined', () => {
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics: undefined } })
    expect(wrapper.find('.alert').exists()).toBe(false)
  })

  it('renders an error alert for error-severity issues', () => {
    const diagnostics: DiagnosticIssue[] = [
      { collector: 'restic', severity: 'error', message: 'restic_conf_path n\'est pas configuré' },
    ]
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics } })
    const alert = wrapper.find('.alert-danger')
    expect(alert.exists()).toBe(true)
    expect(alert.text()).toContain('restic')
    expect(alert.text()).toContain('restic_conf_path')
    expect(wrapper.find('.alert-warning').exists()).toBe(false)
  })

  it('renders a warning alert for warning-severity issues, separate from errors', () => {
    const diagnostics: DiagnosticIssue[] = [
      { collector: 'restic', severity: 'error', message: 'resticconf introuvable' },
      { collector: 'web_logs', severity: 'warning', message: 'aucun fichier ne correspond' },
    ]
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics } })
    expect(wrapper.find('.alert-danger').exists()).toBe(true)
    expect(wrapper.find('.alert-warning').exists()).toBe(true)
    expect(wrapper.find('.alert-warning').text()).toContain('web_logs')
    expect(wrapper.find('.alert-danger').text()).not.toContain('web_logs')
  })
})
