// Shared helpers for datetime-axis charts, consolidating what used to be
// near-identical copies in DiskHistoryChart.vue, HostMetricsPanel.vue,
// NetworkFlowsHistoryChart.vue and useDashboard.ts.

import { localeTag } from '../i18n'

export interface TimeSeriesPoint {
  x: number
  y: number | null
}

/**
 * Clamps a timestamp to "now" — a clock-skew guard so a point reported with
 * a future timestamp (agent/server clock drift) can't push a chart's visible
 * range past the present.
 */
export function clampTimestamp(timestampMs: number): number {
  if (!Number.isFinite(timestampMs)) return NaN
  return Math.min(timestampMs, Date.now())
}

export function getMinPointTimestamp(points: TimeSeriesPoint[]): number | undefined {
  let min = Infinity
  for (const p of points || []) {
    if (Number.isFinite(p?.x) && p.x < min) min = p.x
  }
  return Number.isFinite(min) ? min : undefined
}

export function getMaxPointTimestamp(points: TimeSeriesPoint[]): number | undefined {
  let max = -Infinity
  for (const p of points || []) {
    if (Number.isFinite(p?.x) && p.x > max) max = p.x
  }
  return Number.isFinite(max) ? Math.min(Date.now(), max) : undefined
}

/**
 * Inserts a `y: null` point right after any point whose gap to the next one
 * exceeds `maxGapMs`, so a real hole in the series (e.g. the source was
 * offline, or sampling stopped for a while) renders as a visual break
 * instead of a smooth/straight line silently interpolated across it —
 * ApexCharts otherwise connects any two consecutive points regardless of
 * how far apart in time they are.
 */
export function breakLargeGaps(points: TimeSeriesPoint[], maxGapMs: number): TimeSeriesPoint[] {
  const result: TimeSeriesPoint[] = []
  for (let i = 0; i < points.length; i++) {
    result.push(points[i])
    const next = points[i + 1]
    if (next && next.x - points[i].x > maxGapMs) {
      result.push({ x: points[i].x + 1, y: null })
    }
  }
  return result
}

/**
 * How much of a timestamp an axis label shows. Field order and the 12/24-hour
 * clock come from the UI locale (e.g. "30/09 14:05" in French, "09/30, 02:05 PM"
 * in English), never from a hardcoded day-first pattern.
 */
export type ChartTimeDetail = 'time' | 'timeSeconds' | 'dayHour' | 'dayTime' | 'day'

const CHART_TIME_OPTIONS: Record<ChartTimeDetail, Intl.DateTimeFormatOptions> = {
  time: { hour: '2-digit', minute: '2-digit' },
  timeSeconds: { hour: '2-digit', minute: '2-digit', second: '2-digit' },
  dayHour: { day: '2-digit', month: '2-digit', hour: '2-digit' },
  dayTime: { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' },
  day: { day: '2-digit', month: '2-digit' },
}

/**
 * Label detail for a chart covering `spanHours`: time of day up to a day,
 * date + time up to 30 days, date only beyond.
 */
export function chartTimeDetail(spanHours: number): ChartTimeDetail {
  if (spanHours <= 24) return 'time'
  if (spanHours <= 720) return 'dayTime'
  return 'day'
}

/** Formats an axis/tooltip timestamp (ms, numeric string, ISO string or Date). */
export function formatChartTimestamp(value: number | string | Date | null | undefined, detail: ChartTimeDetail): string {
  if (value == null || value === '') return ''
  const input = typeof value === 'string' && /^\d+$/.test(value) ? Number(value) : value
  const d = new Date(input)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(localeTag(), CHART_TIME_OPTIONS[detail])
}
