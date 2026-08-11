// Shared helpers for datetime-axis charts, consolidating what used to be
// near-identical copies in DiskHistoryChart.vue, HostMetricsPanel.vue,
// NetworkFlowsHistoryChart.vue and useDashboard.ts.

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
