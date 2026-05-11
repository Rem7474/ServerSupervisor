import { getExecutionStateClass } from '../utils/statusClasses'

interface UseStatusBadgeOptions {
  map?: Record<string, string>
}

interface UseStatusBadgeApi {
  getStatusBadgeClass: (status: string | null | undefined, fallback?: string) => string
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

  return { getStatusBadgeClass }
}
