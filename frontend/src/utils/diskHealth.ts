// Shared SMART-status → BadgePill tone mapping — was duplicated identically
// in DiskHealthCard.vue (host agent) and ProxmoxHostDiskHealthCard.vue
// (Proxmox-reported disks), same 4-state vocabulary either way.
export type SmartTone = 'success' | 'danger' | 'warning' | 'secondary'

export function smartStatusTone(status: string): SmartTone {
  switch (status) {
    case 'PASSED': return 'success'
    case 'FAILED': return 'danger'
    case 'UNKNOWN': return 'warning'
    case 'NOT_AVAILABLE': return 'secondary'
    default: return 'secondary'
  }
}
