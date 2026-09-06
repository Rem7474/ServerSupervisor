import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { dispatchModules } from './dispatchStep'

describe('dispatchModules()', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('returns the shared module set with French labels', () => {
    const modules = dispatchModules()
    expect(modules.map((m) => m.value)).toEqual(['docker', 'apt', 'systemd', 'journal', 'processes', 'custom'])
    expect(modules.find((m) => m.value === 'systemd')?.label).toBe('Service systemd')
    expect(modules.find((m) => m.value === 'custom')?.label).toBe('Tâche personnalisée')
  })

  it('re-resolves labels in English when the locale is switched', () => {
    setLocale('en')
    const modules = dispatchModules()
    expect(modules.find((m) => m.value === 'systemd')?.label).toBe('Systemd service')
    expect(modules.find((m) => m.value === 'custom')?.label).toBe('Custom task')
  })
})
