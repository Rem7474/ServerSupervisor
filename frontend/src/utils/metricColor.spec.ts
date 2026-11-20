import { describe, it, expect } from 'vitest'
import { getMetricColorClass, getTemperatureColorClass, getWearoutColorClass } from './metricColor'

describe('getMetricColorClass', () => {
  it.each([
    [0, 'text-success'],
    [75, 'text-success'],
    [75.1, 'text-warning'],
    [90, 'text-warning'],
    [90.1, 'text-danger'],
  ])('%s %% → %s', (pct, expected) => {
    expect(getMetricColorClass(pct)).toBe(expected)
  })

  it('is neutral without a value and honours the bg variant', () => {
    expect(getMetricColorClass(null)).toBe('text-secondary')
    expect(getMetricColorClass(Number.NaN, 'bg')).toBe('bg-secondary')
    expect(getMetricColorClass(95, 'bg')).toBe('bg-danger')
  })
})

describe('getTemperatureColorClass', () => {
  it.each([
    [undefined, 'text-secondary'],
    [0, 'text-secondary'],
    [69.9, 'text-success'],
    [70, 'text-warning'],
    [84.9, 'text-warning'],
    [85, 'text-danger'],
  ])('%s °C → %s', (temp, expected) => {
    expect(getTemperatureColorClass(temp)).toBe(expected)
  })
})

describe('getWearoutColorClass', () => {
  it.each([
    [100, 'bg-success'],
    [50, 'bg-success'],
    [49, 'bg-warning'],
    [20, 'bg-warning'],
    [19, 'bg-danger'],
  ])('%s %% remaining → %s', (remaining, expected) => {
    expect(getWearoutColorClass(remaining)).toBe(expected)
  })
})
