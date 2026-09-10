import { getExecutionStateClass, getExecutionStateLabel } from '../utils/statusClasses'

interface UseStatusBadgeOptions {
  map?: Record<string, string>
}

interface UseStatusBadgeApi {
  getStatusBadgeClass: (status: string | null | undefined, fallback?: string) => string
  getStatusBadgeLabel: (status: string | null | undefined, fallback?: string) => string
}

export function useStatusBadge(options: UseStatusBadgeOptions = {}): UseStatusBadgeApi {
  const extra = options.map || {}

  function getStatusBadgeClass(
    status: string | null | undefined,
    fallback: string = 'badge bg-secondary-lt text-secondary'
  ): string {
    if (!status) return fallback
    const key = String(status).toLowerCase()
    if (extra[key]) return extra[key]
    return getExecutionStateClass(status, fallback)
  }

  // Handed out alongside the class so a badge's colour and text cannot drift
  // apart — the same reasoning statusClasses.ts records for entity states.
  // Without it a caller has nothing to render but the raw value, which is how
  // an untranslated "failed" ended up in the task console.
  function getStatusBadgeLabel(
    status: string | null | undefined,
    fallback?: string
  ): string {
    return getExecutionStateLabel(status, fallback)
  }

  return { getStatusBadgeClass, getStatusBadgeLabel }
}
