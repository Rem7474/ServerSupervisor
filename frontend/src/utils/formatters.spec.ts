import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import {
  compareStrings,
  formatBytes,
  formatDate,
  formatDateLong,
  formatDateTime,
  formatDurationSecs,
  formatNumber,
  formatUptime,
} from './formatters'

describe('formatters', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('formatBytes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(null)).toBe('-')
  })

  it('formatDurationSecs', () => {
    expect(formatDurationSecs(30)).toBe('30s')
    expect(formatDurationSecs(65)).toBe('1min 5s')
    expect(formatDurationSecs(3661)).toBe('1h 1min')
    expect(formatDurationSecs(null)).toBe('-')
  })

  it('formatUptime uses the active locale day abbreviation', () => {
    expect(formatUptime(null)).toBe('N/A')
    expect(formatUptime(90061)).toBe('1j 1h')
    setLocale('en')
    expect(formatUptime(90061)).toBe('1d 1h')
    // Sub-day uptimes carry no localized unit.
    expect(formatUptime(4200)).toBe('1h 10m')
  })

  describe('locale-sensitive formatting', () => {
    const dt = '2026-09-06T14:30:00Z'

    it('formatDate follows the active locale field order', () => {
      // 06/09 (day-first) in French vs 9/6 (month-first) in English — the same
      // instant reads as a different date if the locale is ignored.
      expect(formatDate(dt)).toBe('06/09/2026')
      setLocale('en')
      expect(formatDate(dt)).toBe('09/06/2026')
    })

    it('formatDateLong translates the month name', () => {
      expect(formatDateLong(dt)).toContain('septembre')
      setLocale('en')
      expect(formatDateLong(dt)).toContain('September')
    })

    it('formatDateTime switches to a 12-hour clock in English', () => {
      expect(formatDateTime(dt)).not.toMatch(/[AP]M/)
      setLocale('en')
      expect(formatDateTime(dt)).toMatch(/[AP]M/)
    })

    it('formatNumber uses the locale grouping separator', () => {
      // French groups with a narrow no-break space, English with a comma.
      expect(formatNumber(1234567)).not.toContain(',')
      setLocale('en')
      expect(formatNumber(1234567)).toBe('1,234,567')
    })

    it('formatNumber coerces nullish input to zero', () => {
      expect(formatNumber(null)).toBe('0')
      expect(formatNumber(undefined)).toBe('0')
    })

    it('compareStrings collates accents per locale', () => {
      expect(compareStrings('éclair', 'zebra')).toBeLessThan(0)
      expect(compareStrings('host10', 'host9', { numeric: true })).toBeGreaterThan(0)
    })
  })
})
