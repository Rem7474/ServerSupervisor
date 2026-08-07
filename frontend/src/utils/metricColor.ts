/**
 * Single source of truth for CPU/RAM/disk usage percentage → color, matching
 * the dashboard's own legend (DashboardView.vue's RAM/Disk column tooltips):
 * green < 75 %, yellow 75-90 %, red > 90 %. Used for both badge/text colors
 * (`text-success`/`text-warning`/`text-danger`) and progress-bar fills
 * (`bg-success`/`bg-warning`/`bg-danger`) — pass the matching `variant`.
 */
const WARN_THRESHOLD = 75
const DANGER_THRESHOLD = 90

export function getMetricColorClass(pct: number | null | undefined, variant: 'text' | 'bg' = 'text'): string {
  if (pct == null || Number.isNaN(pct)) return `${variant}-secondary`
  if (pct > DANGER_THRESHOLD) return `${variant}-danger`
  if (pct > WARN_THRESHOLD) return `${variant}-warning`
  return `${variant}-success`
}
