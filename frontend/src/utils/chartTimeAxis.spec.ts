import { describe, it, expect, vi, afterEach } from 'vitest'
import { setLocale } from '../i18n'
import {
  clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp, breakLargeGaps,
  chartTimeDetail, formatChartTimestamp,
} from './chartTimeAxis'

describe('clampTimestamp', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns NaN for a non-finite input', () => {
    expect(Number.isNaN(clampTimestamp(NaN))).toBe(true)
    expect(Number.isNaN(clampTimestamp(Infinity))).toBe(true)
  })

  it('clamps a future timestamp (clock skew) down to now', () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.useFakeTimers()
    vi.setSystemTime(now)

    expect(clampTimestamp(now + 60_000)).toBe(now)
  })

  it('leaves a past/present timestamp untouched', () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.useFakeTimers()
    vi.setSystemTime(now)

    expect(clampTimestamp(now - 60_000)).toBe(now - 60_000)
  })
})

describe('getMinPointTimestamp / getMaxPointTimestamp', () => {
  it('returns undefined for an empty or null/undefined-ish points array', () => {
    expect(getMinPointTimestamp([])).toBeUndefined()
    expect(getMaxPointTimestamp([])).toBeUndefined()
  })

  it('ignores points with a non-finite x when computing min/max', () => {
    const points = [
      { x: NaN, y: 1 },
      { x: 100, y: 2 },
      { x: 300, y: 3 },
      { x: Infinity, y: 4 },
    ]
    expect(getMinPointTimestamp(points)).toBe(100)
  })

  it('caps the max at "now" so a future/clock-skewed point cannot extend the axis', () => {
    const now = new Date('2026-08-24T12:00:00.000Z').getTime()
    vi.useFakeTimers()
    vi.setSystemTime(now)

    const points = [{ x: now - 1000, y: 1 }, { x: now + 60_000, y: 2 }]
    expect(getMaxPointTimestamp(points)).toBe(now)

    vi.useRealTimers()
  })

  it('finds the true min/max among normal points', () => {
    const points = [{ x: 500, y: 1 }, { x: 100, y: 2 }, { x: 300, y: 3 }]
    expect(getMinPointTimestamp(points)).toBe(100)
    expect(getMaxPointTimestamp(points)).toBe(500)
  })
})

describe('breakLargeGaps', () => {
  it('inserts a null point right after a gap exceeding maxGapMs', () => {
    const points = [
      { x: 0, y: 1 },
      { x: 100, y: 2 }, // gap of 100 > maxGapMs(50) from previous point
      { x: 110, y: 3 }, // gap of 10, no break
    ]
    const result = breakLargeGaps(points, 50)

    expect(result).toEqual([
      { x: 0, y: 1 },
      { x: 1, y: null }, // inserted break, timestamped just after the gap start
      { x: 100, y: 2 },
      { x: 110, y: 3 },
    ])
  })

  it('does not insert a break when every gap is within tolerance', () => {
    const points = [{ x: 0, y: 1 }, { x: 10, y: 2 }, { x: 20, y: 3 }]
    expect(breakLargeGaps(points, 50)).toEqual(points)
  })

  it('returns an empty array unchanged', () => {
    expect(breakLargeGaps([], 50)).toEqual([])
  })

  it('handles a single point with no next to compare against', () => {
    const points = [{ x: 0, y: 1 }]
    expect(breakLargeGaps(points, 50)).toEqual(points)
  })
})

describe('chartTimeDetail', () => {
  it.each([
    [1, 'time'],
    [24, 'time'],
    [25, 'dayTime'],
    [720, 'dayTime'],
    [721, 'day'],
  ])('%sh → %s', (hours, detail) => {
    expect(chartTimeDetail(hours)).toBe(detail)
  })
})

describe('formatChartTimestamp', () => {
  // Local time so the expected wall-clock values don't depend on the runner's TZ.
  const ts = new Date(2026, 8, 30, 14, 5, 9).getTime()

  afterEach(() => setLocale('fr'))

  it('orders day and month by UI locale instead of a hardcoded DD/MM', () => {
    setLocale('fr')
    expect(formatChartTimestamp(ts, 'day')).toBe('30/09')
    expect(formatChartTimestamp(ts, 'dayTime')).toBe('30/09 14:05')
    setLocale('en')
    expect(formatChartTimestamp(ts, 'day')).toBe('09/30')
    expect(formatChartTimestamp(ts, 'time')).toMatch(/^02:05\sPM$/)
  })

  it('shows seconds only when asked', () => {
    setLocale('fr')
    expect(formatChartTimestamp(ts, 'time')).toBe('14:05')
    expect(formatChartTimestamp(ts, 'timeSeconds')).toBe('14:05:09')
  })

  it('accepts ms, numeric strings (ApexCharts axis values), ISO strings and Dates', () => {
    setLocale('fr')
    const iso = new Date(ts).toISOString()
    for (const v of [ts, String(ts), iso, new Date(ts)]) {
      expect(formatChartTimestamp(v, 'time')).toBe('14:05')
    }
  })

  it('returns an empty label for missing or invalid input', () => {
    expect(formatChartTimestamp(undefined, 'time')).toBe('')
    expect(formatChartTimestamp('', 'time')).toBe('')
    expect(formatChartTimestamp('garbage', 'time')).toBe('')
  })
})
