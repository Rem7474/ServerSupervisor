interface UseStatusBadgeOptions {
  map?: Record<string, string>
}

interface UseStatusBadgeApi {
  getStatusBadgeClass: (status: string | null | undefined, fallback?: string) => string
}

const STATUS_BADGE_CLASS_MAP: Record<string, string> = {
  completed: 'badge bg-green-lt text-green',
  success: 'badge bg-green-lt text-green',
  succeeded: 'badge bg-green-lt text-green',
  failed: 'badge bg-red-lt text-red',
  error: 'badge bg-red-lt text-red',
  pending: 'badge bg-yellow-lt text-yellow',
  running: 'badge bg-yellow-lt text-yellow',
}

export function useStatusBadge(options: UseStatusBadgeOptions = {}): UseStatusBadgeApi {
  const mergedMap: Record<string, string> = {
    ...STATUS_BADGE_CLASS_MAP,
    ...(options.map || {}),
  }

  function getStatusBadgeClass(
    status: string | null | undefined,
    fallback: string = 'badge bg-secondary-lt text-secondary'
  ): string {
    if (!status) return fallback
    return mergedMap[String(status).toLowerCase()] || fallback
  }

  return { getStatusBadgeClass }
}
