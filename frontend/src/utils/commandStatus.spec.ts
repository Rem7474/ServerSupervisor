import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { commandStatusLabel } from './commandStatus'

beforeEach(() => {
  setLocale('fr')
})

describe('commandStatusLabel', () => {
  it('returns the translated label for each known status', () => {
    expect(commandStatusLabel('pending')).toBe('En attente')
    expect(commandStatusLabel('running')).toBe('En cours')
    expect(commandStatusLabel('completed')).toBe('Terminé')
    expect(commandStatusLabel('failed')).toBe('Échoué')
    expect(commandStatusLabel('cancelled')).toBe('Annulé')
    expect(commandStatusLabel('skipped')).toBe('Ignoré')
  })

  it('returns an empty string for a falsy status, and the raw value for an unknown one', () => {
    expect(commandStatusLabel(undefined)).toBe('')
    expect(commandStatusLabel(null)).toBe('')
    expect(commandStatusLabel('weird')).toBe('weird')
  })

  it('reflects a locale switch without re-importing the module', () => {
    expect(commandStatusLabel('running')).toBe('En cours')
    setLocale('en')
    expect(commandStatusLabel('running')).toBe('Running')
  })
})
