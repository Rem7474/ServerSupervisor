import { describe, it, expect } from 'vitest'
import { isNeverConnectedHost } from './hosts'

describe('isNeverConnectedHost', () => {
  it('is true when offline and last_seen is within a few seconds of created_at', () => {
    const created = new Date('2026-01-01T00:00:00Z')
    const seen = new Date(created.getTime() + 5_000)
    expect(
      isNeverConnectedHost({ status: 'offline', created_at: created.toISOString(), last_seen: seen.toISOString() })
    ).toBe(true)
  })

  it('is false when online, even with matching timestamps', () => {
    const now = new Date().toISOString()
    expect(isNeverConnectedHost({ status: 'online', created_at: now, last_seen: now })).toBe(false)
  })

  it('is false for an offline host that reported long after registration', () => {
    const created = new Date('2026-01-01T00:00:00Z')
    const seen = new Date(created.getTime() + 3 * 24 * 60 * 60 * 1000)
    expect(
      isNeverConnectedHost({ status: 'offline', created_at: created.toISOString(), last_seen: seen.toISOString() })
    ).toBe(false)
  })

  it('is false for warning status', () => {
    const now = new Date().toISOString()
    expect(isNeverConnectedHost({ status: 'warning', created_at: now, last_seen: now })).toBe(false)
  })

  it('is false with missing timestamps', () => {
    expect(isNeverConnectedHost({ status: 'offline' })).toBe(false)
  })
})
