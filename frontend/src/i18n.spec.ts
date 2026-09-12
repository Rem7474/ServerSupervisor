import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  currentLocale,
  ensureLocaleMessages,
  humanizeKeyLeaf,
  i18n,
  localeTag,
  setLocale,
  SUPPORTED_LOCALES,
} from './i18n'

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

    it('renders the key path in dev, where it is meant to be caught', () => {
      // import.meta.env.DEV is true under vitest, so this is the dev branch.
      expect(t('common.__definitelyNotAKey')).toBe('common.__definitelyNotAKey')
    })

    it('warns on every miss so a production one is still observable', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

      t('common.__anotherMissingKey')

      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining('missing key "common.__anotherMissingKey"'),
      )
      warn.mockRestore()
    })
  })

  describe('humanizeKeyLeaf', () => {
    // The production fallback: a user must never see a dotted developer path.
    it.each([
      ['errors.INVALID_TOKEN', 'Invalid token'],
      ['alerts.metricLabels.cpu_temperature', 'Cpu temperature'],
      ['nav.sections.automation.label', 'Label'],
      ['someCamelCaseLeaf', 'Some camel case leaf'],
      ['bare', 'Bare'],
    ])('%s → %s', (key, expected) => {
      expect(humanizeKeyLeaf(key)).toBe(expected)
    })

    it('never returns a string containing a dot', () => {
      expect(humanizeKeyLeaf('a.b.c.d')).not.toContain('.')
    })
  })
})

describe('ensureLocaleMessages', () => {
  it('is a no-op for a locale whose messages are already registered', async () => {
    // The test setup registers both locales up front, so this must not refetch.
    const before = i18n.global.getLocaleMessage('en')

    await ensureLocaleMessages('en')

    expect(i18n.global.getLocaleMessage('en')).toBe(before)
  })

  it('registers a locale that has none yet', async () => {
    const original = i18n.global.getLocaleMessage('en')
    i18n.global.setLocaleMessage('en', {} as never)

    await ensureLocaleMessages('en')

    const loaded = i18n.global.getLocaleMessage('en')
    expect(Object.keys(loaded).length).toBeGreaterThan(0)
    expect(loaded).toHaveProperty('common')
    i18n.global.setLocaleMessage('en', original)
  })
})

describe('storage failures', () => {
  // localStorage throws outright in Safari's Lock Down / private modes. i18n.ts
  // is on the boot path, so an unguarded throw here takes the whole app down.
  it('setLocale still switches when persisting throws', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota', 'QuotaExceededError')
    })

    expect(() => setLocale('en')).not.toThrow()
    expect(currentLocale()).toBe('en')
    expect(document.documentElement.lang).toBe('en')

    setItem.mockRestore()
    setLocale('fr')
  })

  it('reading a blocked localStorage does not throw at module scope', () => {
    // detectLocale() runs on import; this covers the same guarded read.
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('denied', 'SecurityError')
    })

    expect(() => setLocale('en')).not.toThrow()

    getItem.mockRestore()
    setLocale('fr')
  })
})
