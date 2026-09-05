import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { getAlertMetricMeta } from './alertMetrics'

beforeEach(() => {
  setLocale('fr')
})

describe('getAlertMetricMeta', () => {
  it('returns the translated label plus static metadata for a known metric', () => {
    const meta = getAlertMetricMeta('cpu_temperature')
    expect(meta.label).toBe('Temp. CPU')
    expect(meta.unit).toBe('°C')
    expect(meta.category).toBe('host')
  })

  it('translates the label into English when the locale is switched', () => {
    setLocale('en')
    expect(getAlertMetricMeta('disk').label).toBe('Disk')
  })

  it('falls back to the raw metric name and a generic style for an unknown metric', () => {
    const meta = getAlertMetricMeta('some_unmapped_metric')
    expect(meta.label).toBe('some_unmapped_metric')
    expect(meta.badgeClass).toBe('bg-secondary-lt text-secondary')
    expect(meta.category).toBe('host')
  })
})
