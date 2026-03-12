export const MANUAL_SENTINEL = '0 0 29 2 *'

export function isManualOnly(task) {
  return task.cron_expression === MANUAL_SENTINEL && !task.enabled
}

export function describeCron(expr) {
  if (!expr) return ''
  const presets = {
    '@daily':   'tous les jours à minuit',
    '@hourly':  'toutes les heures',
    '@weekly':  'hebdomadaire (dim. minuit)',
    '@monthly': 'mensuel (1er à minuit)',
    '@yearly':  'annuel (1er jan. à minuit)',
  }
  if (presets[expr]) return presets[expr]
  const parts = expr.split(' ')
  if (parts.length !== 5) return ''
  const [min, hour, dom, month, dow] = parts
  const dayNames = ['dim', 'lun', 'mar', 'mer', 'jeu', 'ven', 'sam']

  // */N * * * * — every N minutes
  const stepMinMatch = min.match(/^\*\/(\d+)$/)
  if (stepMinMatch && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    const n = parseInt(stepMinMatch[1])
    return n === 1 ? 'toutes les minutes' : `toutes les ${n} minutes`
  }

  // M */N * * * — every N hours
  const stepHourMatch = hour.match(/^\*\/(\d+)$/)
  if (stepHourMatch && dom === '*' && month === '*' && dow === '*') {
    const n = parseInt(stepHourMatch[1])
    const minLabel = min === '0' ? '' : ` (min ${min})`
    return n === 1 ? `toutes les heures${minLabel}` : `toutes les ${n} heures${minLabel}`
  }

  // M H1-H2 * * * — hourly within a time range
  const rangeHourMatch = hour.match(/^(\d+)-(\d+)$/)
  if (rangeHourMatch && dom === '*' && dow === '*') {
    const [, h1, h2] = rangeHourMatch
    const minLabel = min === '0' ? '' : ` (min ${min})`
    return `chaque heure de ${h1.padStart(2, '0')}h à ${h2.padStart(2, '0')}h${minLabel}`
  }

  // N * * * * — every hour at minute N
  if (/^\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `toutes les heures à :${min.padStart(2, '0')}`
  }

  // M H * * * — every day at H:M
  if (dom === '*' && dow === '*' && !/[*/,-]/.test(hour) && !/[*/,-]/.test(min)) {
    return `tous les jours à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
  }

  // M H D * * — specific day of month
  if (dom !== '*' && !dom.includes('*') && !dom.includes('/') && dow === '*' && !/[*/,-]/.test(hour) && !/[*/,-]/.test(min)) {
    return `le ${dom} du mois à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
  }

  // M H * * D — day(s) of week
  if (dom === '*' && dow !== '*') {
    const days = dow.split(',').map(d => {
      const n = parseInt(d)
      return !isNaN(n) && n <= 6 ? dayNames[n] : d
    })
    if (!/[*/,-]/.test(hour) && !/[*/,-]/.test(min)) {
      return `chaque ${days.join(', ')} à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
    }
    return `chaque ${days.join(', ')}`
  }

  return ''
}
