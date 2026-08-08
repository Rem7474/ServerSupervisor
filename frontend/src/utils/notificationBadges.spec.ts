import { describe, it, expect } from 'vitest'
import { notificationStateLabel, notificationStateTone } from './notificationBadges'
import type { NotificationItem } from '../types/generated'

function incident(overrides: Partial<NotificationItem> = {}): Pick<NotificationItem, 'type' | 'resolved_at' | 'status' | 'acknowledged_at'> {
  return { type: 'alert_incident', resolved_at: undefined, status: undefined, acknowledged_at: undefined, ...overrides }
}

describe('notificationStateTone / notificationStateLabel', () => {
  it('renders an untouched open incident as "Actif" / danger', () => {
    const item = incident()
    expect(notificationStateLabel(item)).toBe('Actif')
    expect(notificationStateTone(item)).toBe('danger')
  })

  it('renders an acknowledged-but-open incident as "En cours" / warning', () => {
    const item = incident({ acknowledged_at: '2026-01-01T00:00:00Z' })
    expect(notificationStateLabel(item)).toBe('En cours')
    expect(notificationStateTone(item)).toBe('warning')
  })

  it('renders a resolved incident as "Terminé" / success even if it was acknowledged first', () => {
    const item = incident({ acknowledged_at: '2026-01-01T00:00:00Z', resolved_at: '2026-01-01T00:05:00Z' })
    expect(notificationStateLabel(item)).toBe('Terminé')
    expect(notificationStateTone(item)).toBe('success')
  })

  it('never shows "En cours" for a release-tracker item, which has no ack concept', () => {
    const item = incident({ type: 'release_tracker_execution', acknowledged_at: '2026-01-01T00:00:00Z', status: 'pending' })
    expect(notificationStateLabel(item)).not.toBe('En cours')
    expect(notificationStateLabel(item)).toBe('Actif')
  })
})
