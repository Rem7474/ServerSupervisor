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

  it('renders an error alert for error-severity issues, with a French collector label', () => {
    const diagnostics: DiagnosticIssue[] = [
      { collector: 'restic', severity: 'error', message: 'restic_conf_path n\'est pas configuré' },
    ]
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics } })
    const alert = wrapper.find('.alert-danger')
    expect(alert.exists()).toBe(true)
    expect(alert.text()).toContain('Restic')
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
    expect(wrapper.find('.alert-warning').text()).toContain('Logs web')
    expect(wrapper.find('.alert-danger').text()).not.toContain('Logs web')
  })

  it('falls back to the raw collector key when no French label is mapped', () => {
    const diagnostics: DiagnosticIssue[] = [
      { collector: 'some_future_collector', severity: 'warning', message: 'not yet configured' },
    ]
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics } })
    expect(wrapper.find('.alert-warning').text()).toContain('some_future_collector')
  })

  it('links to the restic doc guide only for restic issues', () => {
    const diagnostics: DiagnosticIssue[] = [
      { collector: 'restic', severity: 'error', message: 'resticconf introuvable' },
      { collector: 'web_logs', severity: 'warning', message: 'aucun fichier ne correspond' },
    ]
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics } })
    const links = wrapper.findAll('a')
    expect(links.length).toBe(1)
    expect(links[0].attributes('href')).toContain('backup-restic.md')
  })

  const oneIssue: DiagnosticIssue[] = [{ collector: 'restic', severity: 'error', message: 'resticconf introuvable' }]

  it('shows a freshness note based on lastSeen when the host is online', () => {
    const lastSeen = new Date(Date.now() - 5 * 60_000).toISOString()
    const wrapper = mount(HostDiagnosticsBanner, {
      props: { diagnostics: oneIssue, lastSeen, hostStatus: 'online' },
    })
    expect(wrapper.text()).toContain("D'après le dernier rapport de l'agent")
    expect(wrapper.text()).not.toContain('Hôte hors ligne')
  })

  it('shows an offline note instead of the plain freshness note when the host is not online', () => {
    const lastSeen = new Date(Date.now() - 3 * 3600_000).toISOString()
    const wrapper = mount(HostDiagnosticsBanner, {
      props: { diagnostics: oneIssue, lastSeen, hostStatus: 'offline' },
    })
    expect(wrapper.text()).toContain('Hôte hors ligne')
    expect(wrapper.text()).not.toContain("D'après le dernier rapport de l'agent")
  })

  it('shows no freshness note when lastSeen is not provided', () => {
    const wrapper = mount(HostDiagnosticsBanner, { props: { diagnostics: oneIssue } })
    expect(wrapper.text()).not.toContain('Hôte hors ligne')
    expect(wrapper.text()).not.toContain("D'après le dernier rapport")
  })
})
