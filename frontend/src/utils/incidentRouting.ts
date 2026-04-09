export function resolveIncidentHostRoute(hostId?: string | null, metric?: string | null): string {
  if (!hostId) {
    return '/alerts?tab=incidents'
  }

  if (hostId.startsWith('proxmox:')) {
    const [, scope, rawId] = hostId.split(':', 3)
    const id = rawId ? encodeURIComponent(rawId) : ''

    switch (scope) {
      case 'node':
        return id ? `/proxmox/nodes/${id}` : '/proxmox'
      case 'guest':
        return id ? `/proxmox/guests/${id}` : '/proxmox'
      case 'global':
      case 'connection':
      case 'storage':
      case 'disk':
      default:
        return '/proxmox'
    }
  }

  if (metric && metric.startsWith('proxmox_')) {
    return '/proxmox'
  }

  return `/hosts/${encodeURIComponent(hostId)}`
}
