const STATUS_BADGE_CLASS_MAP = {
  completed: 'badge bg-green-lt text-green',
  success: 'badge bg-green-lt text-green',
  succeeded: 'badge bg-green-lt text-green',
  failed: 'badge bg-red-lt text-red',
  error: 'badge bg-red-lt text-red',
  pending: 'badge bg-yellow-lt text-yellow',
  running: 'badge bg-yellow-lt text-yellow',
}

/**
 * @param {{ map?: Record<string, string> }} [options]
 */
export function useStatusBadge(options = {}) {
  const mergedMap = {
    ...STATUS_BADGE_CLASS_MAP,
    ...(options.map || {}),
  }

  /**
   * @param {string | null | undefined} status
   * @param {string} [fallback='badge bg-secondary-lt text-secondary']
   */
  function getStatusBadgeClass(status, fallback = 'badge bg-secondary-lt text-secondary') {
    if (!status) return fallback
    return mergedMap[String(status).toLowerCase()] || fallback
  }

  return { getStatusBadgeClass }
}