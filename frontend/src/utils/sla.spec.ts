import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { formatPercent, minutesBetween, publicStatusClass, slaStatusClass, slaStatusKey, uptimeDayClass } from './sla'

describe('sla utils', () => {
  beforeEach(() => setLocale('fr'))

  it('formats percentages in the active locale and blanks missing values', () => {
    expect(formatPercent(99.95)).toBe('99,95')
    expect(formatPercent(99.9999, 3)).toBe('100')
    expect(formatPercent(null)).toBe('')
    expect(formatPercent(undefined)).toBe('')
    expect(formatPercent(Number.NaN)).toBe('')
    setLocale('en')
    expect(formatPercent(99.95)).toBe('99.95')
  })

  it('maps SLA statuses to a badge and a label key', () => {
    expect(slaStatusClass('met')).toContain('success')
    expect(slaStatusClass('at_risk')).toContain('warning')
    expect(slaStatusClass('breached')).toContain('danger')
    expect(slaStatusClass('no_data')).toContain('secondary')
    expect(['met', 'at_risk', 'breached', 'no_data', 'other'].map(slaStatusKey))
      .toEqual(['statusMet', 'statusAtRisk', 'statusBreached', 'statusNoData', 'statusNoData'])
  })

  it('maps public statuses to a badge', () => {
    expect(publicStatusClass('operational')).toContain('bg-success')
    expect(publicStatusClass('partial_outage')).toContain('bg-warning')
    expect(publicStatusClass('major_outage')).toContain('bg-danger')
    expect(publicStatusClass('whatever')).toContain('bg-secondary')
  })

  it('colours a day of the uptime bar at the thresholds', () => {
    expect(uptimeDayClass(null)).toBe('uptime-day-none')
    expect(uptimeDayClass(undefined)).toBe('uptime-day-none')
    expect(uptimeDayClass(100)).toBe('uptime-day-ok')
    expect(uptimeDayClass(99.9)).toBe('uptime-day-ok')
    expect(uptimeDayClass(99.89)).toBe('uptime-day-warn')
    expect(uptimeDayClass(95)).toBe('uptime-day-warn')
    expect(uptimeDayClass(94.99)).toBe('uptime-day-down')
  })

  it('counts whole minutes, up to now for an ongoing period', () => {
    expect(minutesBetween('2026-11-25T10:00:00Z', '2026-11-25T11:30:20Z')).toBe(90)
    expect(minutesBetween('2026-11-25T10:00:00Z', null, new Date('2026-11-25T10:05:00Z'))).toBe(5)
    expect(minutesBetween('2026-11-25T10:00:00Z', '2026-11-25T09:00:00Z')).toBe(0)
  })
})
