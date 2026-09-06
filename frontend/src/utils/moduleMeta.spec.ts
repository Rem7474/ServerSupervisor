import { describe, it, expect } from 'vitest'
import { moduleLabel, moduleClass, REMOTE_COMMAND_MODULE_OPTIONS } from './moduleMeta'

describe('REMOTE_COMMAND_MODULE_OPTIONS', () => {
  it('includes every module the agent dispatcher registry can target', () => {
    // agent/internal/dispatcher/registry.go's module -> handler map. If this
    // starts failing, a module was added/removed there without updating the
    // whitelist here — the audit page's module filter went stale exactly
    // this way for `restic` before this test existed.
    const values = REMOTE_COMMAND_MODULE_OPTIONS.map((o) => o.value)
    expect(values.sort()).toEqual(
      ['agent', 'apt', 'compose', 'crowdsec', 'custom', 'docker', 'journal', 'processes', 'restic', 'systemd'].sort()
    )
  })

  it('does not include proxmox (used elsewhere for a non-remote-command audit context)', () => {
    expect(REMOTE_COMMAND_MODULE_OPTIONS.map((o) => o.value)).not.toContain('proxmox')
  })

  it('every option label matches moduleLabel/moduleClass so badges and the filter never disagree', () => {
    for (const opt of REMOTE_COMMAND_MODULE_OPTIONS) {
      expect(opt.label).toBe(moduleLabel(opt.value))
      expect(moduleClass(opt.value)).not.toBe('badge bg-secondary-lt text-secondary')
    }
  })
})
