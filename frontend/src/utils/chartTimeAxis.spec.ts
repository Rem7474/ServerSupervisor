import { describe, it, expect, vi, afterEach } from 'vitest'
import { clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp, breakLargeGaps } from './chartTimeAxis'

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
