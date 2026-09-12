import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { setLocale } from '../i18n'
import { translateError } from './translateError'

beforeEach(() => {
  setLocale('fr')
})

afterEach(() => {
  setLocale('fr')
})

describe('translateError', () => {
  it('returns the unknown-error message for a falsy error', () => {
    expect(translateError(null)).toBe('Une erreur est survenue')
    setLocale('en')
    expect(translateError(undefined)).toBe('An error occurred')
  })

  it('resolves a known i18nKey in the active UI language, ignoring the response text', () => {
    const error = {
      response: { data: { error: 'texte déjà traduit côté serveur', i18nKey: 'NODE_NOT_FOUND' } },
    }
    expect(translateError(error)).toBe('nœud non trouvé')
    setLocale('en')
    expect(translateError(error)).toBe('node not found')
  })

  it('interpolates params for a keyed message', () => {
    setLocale('en')
    const error = {
      response: { data: { error: 'ignored', i18nKey: 'GIT_PROVIDER_ERROR', params: { status: '503' } } },
    }
    expect(translateError(error)).toBe('GitHub API error (503)')
  })

  it('falls back to substring matching when there is no i18nKey', () => {
    const error = { response: { data: { error: 'host not found in database' } } }
    expect(translateError(error)).toBe('Hôte introuvable')
  })

  it('ignores an unrecognized i18nKey and falls back to substring matching', () => {
    const error = { response: { data: { error: 'permission denied', i18nKey: 'NOT_A_REAL_KEY' } } }
    expect(translateError(error)).toBe('Permission refusée')
  })

  it('capitalizes the raw message when nothing matches', () => {
    const error = { response: { data: { error: 'something unexpected happened' } } }
    expect(translateError(error)).toBe('Something unexpected happened')
  })

  it('reads message from a native Error when there is no response payload', () => {
    expect(translateError(new Error('timeout of 30000ms exceeded'))).toBe('Délai d\'attente dépassé')
  })
})
