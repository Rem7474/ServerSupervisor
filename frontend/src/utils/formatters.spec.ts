import { describe, it, expect } from 'vitest'
import { formatBytes, formatDurationSecs, formatUptime } from './formatters'

describe('formatters', () => {
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

  it('formatUptime', () => {
    expect(formatUptime(null)).toBe('N/A')
    expect(formatUptime(90061)).toBe('1j 1h')
  })
})
