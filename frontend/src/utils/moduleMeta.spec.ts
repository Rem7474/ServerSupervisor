import { describe, it, expect, beforeEach } from 'vitest'
import { moduleLabel, moduleClass, remoteCommandModuleOptions } from './moduleMeta'
import { setLocale } from '../i18n'

describe('remoteCommandModuleOptions()', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('includes every module the agent dispatcher registry can target', () => {
    // agent/internal/dispatcher/registry.go's module -> handler map. If this
    // starts failing, a module was added/removed there without updating the
    // whitelist here — the audit page's module filter went stale exactly
    // this way for `restic` before this test existed.
    const values = remoteCommandModuleOptions().map((o) => o.value)
    expect(values.sort()).toEqual(
      ['agent', 'apt', 'compose', 'crowdsec', 'custom', 'docker', 'journal', 'processes', 'restic', 'systemd'].sort()
    )
  })

  it('does not include proxmox (used elsewhere for a non-remote-command audit context)', () => {
    expect(remoteCommandModuleOptions().map((o) => o.value)).not.toContain('proxmox')
  })

  it('every option label matches moduleLabel/moduleClass so badges and the filter never disagree', () => {
    for (const opt of remoteCommandModuleOptions()) {
      expect(opt.label).toBe(moduleLabel(opt.value))
      expect(moduleClass(opt.value)).not.toBe('badge bg-secondary-lt text-secondary')
    }
  })

  it('translates module labels to English when the locale is switched', () => {
    setLocale('en')
    expect(moduleLabel('processes')).toBe('Processes')
    expect(moduleLabel('docker')).toBe('Docker')
  })
})
