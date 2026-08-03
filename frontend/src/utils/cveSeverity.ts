// Shared CVE severity handling — previously duplicated verbatim (including
// the raw-Tabler-color badge map) in CVEBadge.vue and CVEList.vue.

export const CVE_SEVERITY_ORDER: Record<string, number> = {
  CRITICAL: 5,
  HIGH: 4,
  MEDIUM: 3,
  LOW: 2,
  NEGLIGIBLE: 1,
  UNKNOWN: 0,
}

// Semantic tokens only (danger/warning/primary/secondary) rather than the
// previous 4-hue raw palette (red/orange/yellow/blue) — MEDIUM maps to
// `primary` (the design system's "info/accent" token) since nothing in the
// 5-token system reads as a true 3rd severity tier between warning and a
// neutral secondary; LOW and NEGLIGIBLE collapse onto the same neutral tone.
const CVE_SEVERITY_CLASS: Record<string, string> = {
  CRITICAL: 'bg-danger-lt text-danger',
  HIGH: 'bg-warning-lt text-warning',
  MEDIUM: 'bg-primary-lt text-primary',
  LOW: 'bg-secondary-lt text-secondary',
  NEGLIGIBLE: 'bg-secondary-lt text-secondary',
  UNKNOWN: 'bg-secondary-lt text-secondary',
}

export function normalizeCveSeverity(severity: string | undefined | null): string {
  return severity?.toUpperCase() || 'UNKNOWN'
}

export function cveSeverityOrder(severity: string | undefined | null): number {
  return CVE_SEVERITY_ORDER[normalizeCveSeverity(severity)] || 0
}

export function cveSeverityClass(severity: string | undefined | null): string {
  const normalized = normalizeCveSeverity(severity)
  return CVE_SEVERITY_CLASS[normalized] || CVE_SEVERITY_CLASS.UNKNOWN
}
