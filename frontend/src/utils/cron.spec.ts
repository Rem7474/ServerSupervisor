import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import { describeCron, isManualOnly, MANUAL_SENTINEL } from './cron'

describe('describeCron', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('describes the @daily/@hourly/@weekly/@monthly/@yearly presets', () => {
    expect(describeCron('@daily')).toBe('tous les jours à minuit')
    expect(describeCron('@hourly')).toBe('toutes les heures')
    expect(describeCron('@weekly')).toBe('hebdomadaire (dim. minuit)')
    expect(describeCron('@monthly')).toBe('mensuel (1er à minuit)')
    expect(describeCron('@yearly')).toBe('annuel (1er jan. à minuit)')
  })

  it('describes every-N-minutes expressions, singular and plural', () => {
    expect(describeCron('*/1 * * * *')).toBe('toutes les minutes')
    expect(describeCron('*/15 * * * *')).toBe('toutes les 15 minutes')
  })

  it('describes every-N-hours expressions, with and without a minute offset', () => {
    expect(describeCron('0 */2 * * *')).toBe('toutes les 2 heures')
    expect(describeCron('30 */1 * * *')).toBe('toutes les heures (min 30)')
  })

  it('describes an hour range', () => {
    expect(describeCron('0 9-17 * * *')).toBe('chaque heure de 09h à 17h')
    expect(describeCron('15 9-17 * * *')).toBe('chaque heure de 09h à 17h (min 15)')
  })

  it('describes an every-hour-at-minute-N expression', () => {
    expect(describeCron('5 * * * *')).toBe('toutes les heures à :05')
  })

  it('describes a fixed daily time', () => {
    expect(describeCron('0 3 * * *')).toBe('tous les jours à 03h00')
  })

  it('describes a specific day of the month', () => {
    expect(describeCron('0 3 15 * *')).toBe('le 15 du mois à 03h00')
  })

  it('describes day(s) of the week, with and without a fixed time', () => {
    expect(describeCron('0 3 * * 1,3')).toBe('chaque lun, mer à 03h00')
    expect(describeCron('*/5 * * * 1')).toBe('chaque lun')
  })

  it('returns an empty string for an empty or malformed expression', () => {
    expect(describeCron('')).toBe('')
    expect(describeCron(null)).toBe('')
    expect(describeCron('not a cron')).toBe('')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    expect(describeCron('@daily')).toBe('every day at midnight')
    expect(describeCron('*/15 * * * *')).toBe('every 15 minutes')
    expect(describeCron('0 3 * * 1,3')).toBe('every Mon, Wed at 03h00')
  })
})

describe('isManualOnly', () => {
  it('is true only when the sentinel cron is set and the task is disabled', () => {
    expect(isManualOnly({ cron_expression: MANUAL_SENTINEL, enabled: false })).toBe(true)
    expect(isManualOnly({ cron_expression: MANUAL_SENTINEL, enabled: true })).toBe(false)
    expect(isManualOnly({ cron_expression: '0 3 * * *', enabled: false })).toBe(false)
  })
})
