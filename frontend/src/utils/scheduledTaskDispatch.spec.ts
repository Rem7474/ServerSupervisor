import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { scheduledTaskModules, availableScheduledTaskModules, scheduledTaskActions, scheduledTaskTargetConfig } from './scheduledTaskDispatch'

describe('scheduledTaskDispatch', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('scheduledTaskModules() extends dispatchModules() with restic', () => {
    const modules = scheduledTaskModules()
    expect(modules.map((m) => m.value)).toEqual(['docker', 'apt', 'systemd', 'journal', 'processes', 'custom', 'restic'])
    expect(modules.find((m) => m.value === 'restic')?.label).toBe('Restic (backup)')
  })

  it('availableScheduledTaskModules filters out unavailable capabilities', () => {
    const modules = availableScheduledTaskModules({ docker: false, apt: true, restic: false })
    const values = modules.map((m) => m.value)
    expect(values).not.toContain('docker')
    expect(values).not.toContain('restic')
    expect(values).toContain('apt')
    expect(values).toContain('systemd') // no capability gate
  })

  it('availableScheduledTaskModules treats missing collectors as unavailable for gated modules', () => {
    const modules = availableScheduledTaskModules(null)
    expect(modules.map((m) => m.value)).not.toContain('docker')
  })

  it('scheduledTaskActions returns the advisory action list for a known module, empty for others', () => {
    expect(scheduledTaskActions('docker').map((a) => a.value)).toEqual(['start', 'stop', 'restart', 'pull', 'prune'])
    expect(scheduledTaskActions('unknown-module')).toEqual([])
  })

  it('scheduledTaskTargetConfig returns a translated label + placeholder, or null for an unconfigured module', () => {
    expect(scheduledTaskTargetConfig('docker')).toEqual({ label: 'Conteneur (nom ou ID)', placeholder: 'nginx' })
    expect(scheduledTaskTargetConfig('processes')).toBeNull()
  })

  it('translates target labels to English when the locale is switched', () => {
    setLocale('en')
    expect(scheduledTaskTargetConfig('docker')?.label).toBe('Container (name or ID)')
    expect(scheduledTaskModules().find((m) => m.value === 'restic')?.label).toBe('Restic (backup)')
  })
})
