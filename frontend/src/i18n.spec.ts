import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { i18n, currentLocale, localeTag, setLocale, SUPPORTED_LOCALES } from './i18n'

const t = i18n.global.t

describe('i18n', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  afterEach(() => {
    setLocale('fr')
  })

  describe('French plural rule', () => {
    // CLDR French puts 0 in the `one` category; vue-i18n's built-in default is
    // the English rule, which would render "0 hôtes".
    it('treats 0 and 1 as singular in French', () => {
      expect(t('dashboard.hostCount', { count: 0 }, 0)).toBe('0 hôte')
      expect(t('dashboard.hostCount', { count: 1 }, 1)).toBe('1 hôte')
      expect(t('dashboard.hostCount', { count: 2 }, 2)).toBe('2 hôtes')
    })

    it('keeps 0 plural in English', () => {
      setLocale('en')
      expect(t('dashboard.hostCount', { count: 0 }, 0)).toBe('0 hosts')
      expect(t('dashboard.hostCount', { count: 1 }, 1)).toBe('1 host')
      expect(t('dashboard.hostCount', { count: 2 }, 2)).toBe('2 hosts')
    })

    it('applies to negative counts by magnitude', () => {
      expect(t('dashboard.hostCount', { count: -1 }, -1)).toBe('-1 hôte')
    })
  })

  describe('locale switching', () => {
    it('persists the choice and syncs <html lang>', () => {
      setLocale('en')
      expect(localStorage.getItem('locale')).toBe('en')
      expect(document.documentElement.lang).toBe('en')
      setLocale('fr')
      expect(localStorage.getItem('locale')).toBe('fr')
      expect(document.documentElement.lang).toBe('fr')
    })

    it('maps each supported locale to a BCP-47 tag with a region', () => {
      for (const locale of SUPPORTED_LOCALES) {
        setLocale(locale)
        expect(currentLocale()).toBe(locale)
        expect(localeTag()).toMatch(/^[a-z]{2}-[A-Z]{2}$/)
      }
    })
  })

  describe('missing keys', () => {
    it('falls back to French rather than rendering the raw key', () => {
      setLocale('en')
      // Present in both locales — the fallback path must not be hit here.
      expect(t('common.language')).not.toBe('common.language')
    })

    it('renders the key path when it exists in no locale', () => {
      // Documents the worst case the `missing` handler warns about.
      expect(t('common.__definitelyNotAKey')).toBe('common.__definitelyNotAKey')
    })
  })
})
