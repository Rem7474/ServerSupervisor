import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { rules, useFormValidator } from './useFormValidator'

describe('useFormValidator', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  describe('rules', () => {
    it('required rejects every falsy value and names the field', () => {
      const rule = rules.required('Hostname')
      for (const empty of ['', 0, null, undefined, false]) {
        expect(rule.validate(empty)).toBe('Hostname est requis')
      }
      expect(rule.validate('web-01')).toBeNull()
    })

    it('minLength counts characters and ignores non-strings', () => {
      const rule = rules.minLength(3)
      expect(rule.validate('ab')).toBe('Minimum 3 caractères')
      // Boundary: exactly `min` passes.
      expect(rule.validate('abc')).toBeNull()
      // A non-string isn't this rule's business — `required` covers absence.
      expect(rule.validate(12)).toBeNull()
      expect(rule.validate(null)).toBeNull()
    })

    it('email accepts an address and rejects malformed ones', () => {
      const rule = rules.email()
      expect(rule.validate('admin@example.com')).toBeNull()
      for (const bad of ['admin', 'admin@', '@example.com', 'a b@example.com']) {
        expect(rule.validate(bad)).toBe('Email invalide')
      }
      expect(rule.validate(null)).toBeNull()
    })

    it('url accepts a parseable URL and rejects the rest', () => {
      const rule = rules.url()
      expect(rule.validate('https://pve.example.com:8006')).toBeNull()
      expect(rule.validate('not a url')).toBe('URL invalide')
      expect(rule.validate(null)).toBeNull()
    })

    it('reports messages in the active locale', () => {
      setLocale('en')
      expect(rules.required('Hostname').validate('')).toBe('Hostname is required')
      expect(rules.minLength(3).validate('ab')).toBe('Minimum 3 characters')
      expect(rules.email().validate('nope')).toBe('Invalid email')
      expect(rules.url().validate('nope')).toBe('Invalid URL')
    })
  })

  describe('validateField', () => {
    it('records the first failing rule only and stops there', () => {
      const v = useFormValidator()
      // `required` fails first, so the email message must not overwrite it.
      const ok = v.validateField('email', '', [rules.required('Email'), rules.email()])

      expect(ok).toBe(false)
      expect(v.fieldErrors.value.email).toBe('Email est requis')
      expect(v.hasErrors.value).toBe(true)
    })

    it('clears a previously recorded error once the value becomes valid', () => {
      const v = useFormValidator()
      v.validateField('email', 'bogus', [rules.email()])
      expect(v.hasErrors.value).toBe(true)

      const ok = v.validateField('email', 'admin@example.com', [rules.email()])

      expect(ok).toBe(true)
      expect(v.fieldErrors.value.email).toBeUndefined()
      expect(v.hasErrors.value).toBe(false)
    })

    it('keeps errors on other fields when one field passes', () => {
      const v = useFormValidator()
      v.validateField('name', '', [rules.required('Nom')])
      v.validateField('email', 'admin@example.com', [rules.email()])

      expect(Object.keys(v.fieldErrors.value)).toEqual(['name'])
    })
  })

  describe('setFieldError / clearErrors', () => {
    it('setFieldError(null) removes the entry rather than blanking it', () => {
      const v = useFormValidator()
      v.setFieldError('host', 'boom')
      expect(v.fieldErrors.value.host).toBe('boom')

      v.setFieldError('host', null)
      expect('host' in v.fieldErrors.value).toBe(false)
    })

    it('clearErrors empties every field at once', () => {
      const v = useFormValidator()
      v.setFieldError('a', 'x')
      v.setFieldError('b', 'y')

      v.clearErrors()

      expect(v.fieldErrors.value).toEqual({})
      expect(v.hasErrors.value).toBe(false)
    })
  })
})
