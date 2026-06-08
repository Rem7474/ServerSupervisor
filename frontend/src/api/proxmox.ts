import { api, type JsonObject } from './client'
import type {
  ProxmoxConnection,
  ProxmoxConnectionRequest,
  ProxmoxNode,
  ProxmoxGuest,
  ProxmoxSummary,
  ProxmoxTask,
  ProxmoxDisk,
  ProxmoxBackupJob,
  ProxmoxBackupRun,
} from '../types/proxmox'

// Re-exported so existing `import { ProxmoxConnection } from '.../api/proxmox'`
// sites keep working now that the type lives in the shared types layer.
export type { ProxmoxConnection } from '../types/proxmox'

export const proxmoxApi = {
  getProxmoxSummary: () => api.get<ProxmoxSummary>('/v1/proxmox/summary'),
  getProxmoxInstances: () => api.get<ProxmoxConnection[]>('/v1/proxmox/instances'),
  getProxmoxInstance: (id: string) => api.get<ProxmoxConnection>(`/v1/proxmox/instances/${id}`),
  createProxmoxInstance: (payload: Partial<ProxmoxConnectionRequest>) =>
    api.post('/v1/proxmox/instances', payload),
  updateProxmoxInstance: (id: string, payload: Partial<ProxmoxConnectionRequest>) =>
    api.put(`/v1/proxmox/instances/${id}`, payload),
  deleteProxmoxInstance: (id: string) => api.delete(`/v1/proxmox/instances/${id}`),
  testProxmoxConnection: (payload: Partial<ProxmoxConnectionRequest>) =>
    api.post('/v1/proxmox/instances/test', payload),
  testProxmoxInstanceById: (id: string) => api.post(`/v1/proxmox/instances/${id}/test`),
  pollProxmoxNow: (id: string) => api.post(`/v1/proxmox/instances/${id}/poll-now`),
  getProxmoxNodes: (connectionId?: string) =>
    api.get<ProxmoxNode[]>('/v1/proxmox/nodes', { params: connectionId ? { connection_id: connectionId } : {} }),
  getProxmoxNode: (id: string) => api.get<ProxmoxNode>(`/v1/proxmox/nodes/${id}`),
  getProxmoxNodeCpuTempHistory: (id: string, hours?: number) =>
    api.get(`/v1/proxmox/nodes/${id}/cpu-temp/history`, { params: { hours: hours ?? 24 } }),
  getProxmoxNodeFanRPMHistory: (id: string, hours?: number) =>
    api.get(`/v1/proxmox/nodes/${id}/fan-rpm/history`, { params: { hours: hours ?? 24 } }),
  getProxmoxNodeSensorSourceCandidates: (id: string) => api.get(`/v1/proxmox/nodes/${id}/sensor-source/candidates`),
  setProxmoxNodeSensorSource: (id: string, hostId: string | null) =>
    api.put(`/v1/proxmox/nodes/${id}/sensor-source`, { host_id: hostId ?? '' }),
  getProxmoxNodeMetrics: (hours?: number, bucketMinutes?: number) =>
    api.get('/v1/proxmox/nodes/metrics', { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),
  getProxmoxGuests: (params?: JsonObject) => api.get<ProxmoxGuest[]>('/v1/proxmox/guests', { params: params ?? {} }),
  getProxmoxGuestMetrics: (guestId: string, hours?: number, bucketMinutes?: number) =>
    api.get(`/v1/proxmox/guests/${guestId}/metrics`, { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),
  getProxmoxGuestLink: (guestId: string) => api.get(`/v1/proxmox/guests/${guestId}/link`),

  // Guest ↔ host links
  getProxmoxLinks: (status?: string) =>
    api.get('/v1/proxmox/links', { params: status ? { status } : {} }),
  getProxmoxLink: (id: string) => api.get(`/v1/proxmox/links/${id}`),
  createProxmoxLink: (payload: JsonObject) => api.post('/v1/proxmox/links', payload),
  updateProxmoxLink: (id: string, payload: JsonObject) =>
    api.put(`/v1/proxmox/links/${id}`, payload),
  deleteProxmoxLink: (id: string) => api.delete(`/v1/proxmox/links/${id}`),

  // Per-host Proxmox link
  getHostProxmoxLink: (hostId: string) => api.get(`/v1/hosts/${hostId}/proxmox-link`),
  getHostProxmoxCandidates: (hostId: string) => api.get(`/v1/hosts/${hostId}/proxmox-candidates`),

  // Extended: tasks
  getProxmoxTasks: (params?: JsonObject) =>
    api.get<ProxmoxTask[]>('/v1/proxmox/tasks', { params: params ?? {} }),
  getProxmoxNodeTasks: (nodeId: string, limit?: number) =>
    api.get<ProxmoxTask[]>(`/v1/proxmox/nodes/${nodeId}/tasks`, { params: { limit: limit ?? 50 } }),

  // Extended: disks
  getProxmoxNodeDisks: (nodeId: string) => api.get<ProxmoxDisk[]>(`/v1/proxmox/nodes/${nodeId}/disks`),

  // Extended: backups
  getProxmoxBackupJobs: (connectionId?: string) =>
    api.get<ProxmoxBackupJob[]>('/v1/proxmox/backup-jobs', { params: connectionId ? { connection_id: connectionId } : {} }),
  getProxmoxBackupRuns: (connectionId?: string) =>
    api.get<ProxmoxBackupRun[]>('/v1/proxmox/backup-runs', { params: connectionId ? { connection_id: connectionId } : {} }),

  // Node live data
  getProxmoxNodeStatus: (nodeId: string) => api.get(`/v1/proxmox/nodes/${nodeId}/status`),
  getProxmoxTaskLog: (nodeId: string, upid: string) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/tasks/${encodeURIComponent(upid)}/log`),
  getProxmoxNodeRRD: (nodeId: string, timeframe?: string) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/rrd`, { params: { timeframe: timeframe ?? 'hour' } }),
  getProxmoxNodeSyslog: (nodeId: string, params?: JsonObject) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/syslog`, { params: params ?? {} }),

  // Node services
  getProxmoxNodeServices: (nodeId: string) => api.get(`/v1/proxmox/nodes/${nodeId}/services`),
  proxmoxNodeServiceAction: (nodeId: string, service: string, action: string) =>
    api.post(`/v1/proxmox/nodes/${nodeId}/services/${encodeURIComponent(service)}/${action}`),

  // Guest network interfaces
  getProxmoxNodeGuestNetworks: (nodeId: string) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/guest-networks`),

  // Node actions
  refreshProxmoxNodeApt: (nodeId: string) => api.post(`/v1/proxmox/nodes/${nodeId}/apt-refresh`),
  migrateProxmoxGuest: (nodeId: string, vmid: number, payload: { target: string; guest_type: string; online: boolean }) =>
    api.post(`/v1/proxmox/nodes/${nodeId}/guests/${vmid}/migrate`, payload),
}
