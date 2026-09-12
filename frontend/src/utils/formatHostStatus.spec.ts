import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { formatHostStatus, hostStatusClass } from './formatHostStatus'

beforeEach(() => {
  setLocale('fr')
})

describe('formatHostStatus', () => {
  it('translates the known statuses through the active locale', () => {
    expect(formatHostStatus('online')).toBe('En ligne')
    expect(formatHostStatus('offline')).toBe('Hors ligne')
    expect(formatHostStatus('unknown-status')).toBe('Inconnu')
    expect(formatHostStatus('warning')).toBe('Warning')

    setLocale('en')
    expect(formatHostStatus('online')).toBe('Online')
    expect(formatHostStatus('offline')).toBe('Offline')
    expect(formatHostStatus('unknown-status')).toBe('Unknown')
  })
})

describe('hostStatusClass', () => {
  it('maps each status to its Tabler class, independent of locale', () => {
    expect(hostStatusClass('online')).toBe('status status-success')
    expect(hostStatusClass('warning')).toBe('status status-warning')
    expect(hostStatusClass('offline')).toBe('status status-danger')
    expect(hostStatusClass('unknown-status')).toBe('status status-secondary')
  })
})
