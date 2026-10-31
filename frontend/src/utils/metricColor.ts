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

/**
 * CPU temperature (°C) → color: green < 70, yellow 70-85, red ≥ 85. A missing
 * or zero reading means no sensor, not a cold CPU, so it stays neutral.
 */
export function getTemperatureColorClass(tempC: number | null | undefined, variant: 'text' | 'bg' = 'text'): string {
  if (!tempC) return `${variant}-secondary`
  if (tempC >= 85) return `${variant}-danger`
  if (tempC >= 70) return `${variant}-warning`
  return `${variant}-success`
}

/**
 * SSD wear-out as Proxmox reports it — *remaining* life in percent, so low is
 * bad: red < 20, yellow < 50. Not for SMART's `percentage_used` (the inverse
 * scale, see DiskHealthCard.vue), and not for usage percentages, which fail
 * at a far higher value than a worn-out disk.
 */
export function getWearoutColorClass(remainingPct: number, variant: 'text' | 'bg' = 'bg'): string {
  if (remainingPct < 20) return `${variant}-danger`
  if (remainingPct < 50) return `${variant}-warning`
  return `${variant}-success`
}
