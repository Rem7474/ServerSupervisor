import { api, rangeParams } from './client'
import type { TimeRange } from './client'
import type { Host, HostExposure, HostRegistration, HostUpdate } from '../types/host'
import type { DiscoveredHost } from '../types/discovery'

interface BulkHostResult {
  name: string
  ip_address: string
  created: boolean
  host_id?: string
  api_key?: string
  error?: string
}

export const hostsApi = {
  // Hosts
  getHosts: (signal?: AbortSignal) => api.get<Host[]>('/v1/hosts', { signal }),
  getHost: (id: string) => api.get<Host>(`/v1/hosts/${id}`),
  getHostComplete: (id: string) => api.get(`/v1/hosts/${id}/complete`),
  getHostDashboard: (id: string) => api.get(`/v1/hosts/${id}/dashboard`),
  getHostExposure: (id: string, period?: string) =>
    api.get<HostExposure>(`/v1/hosts/${id}/exposure`, { params: { period: period || '24h' } }),
  registerHost: (data: Partial<HostRegistration>) => api.post('/v1/hosts', data),
  registerHostsBulk: (hosts: Partial<HostRegistration>[]) =>
    api.post<{ created: number; results: BulkHostResult[] }>('/v1/hosts/bulk', { hosts }),
  discoverHosts: (cidr: string) =>
    api.post<{ results: DiscoveredHost[] }>('/v1/hosts/discover', { cidr }),
  updateHost: (id: string, data: Partial<HostUpdate>) => api.patch(`/v1/hosts/${id}`, data),
  deleteHost: (id: string) => api.delete(`/v1/hosts/${id}`),
  rotateHostKey: (id: string) => api.post(`/v1/hosts/${id}/rotate-key`),
  updateHostAgent: (id: string) => api.post(`/v1/hosts/${id}/agent/update`),

  // Disk
  getDiskMetrics: (hostId: string) => api.get(`/v1/hosts/${hostId}/disk/metrics`),
  getDiskHealth: (hostId: string) => api.get(`/v1/hosts/${hostId}/disk/health`),
  getDiskMetricsAggregated: (hostId: string, mountPoint: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/disk/metrics/aggregated`, { params: { mount_point: mountPoint, hours: hours ?? 24 } }),
  // Physical disks (SMART health) of the Proxmox node hosting a linked host
  getHostProxmoxDisks: (hostId: string) => api.get(`/v1/hosts/${hostId}/proxmox-disks`),

  // Network flows ("top talkers"). period is a Go duration string ('24h',
  // '168h', ...), matching the web-logs endpoints' convention — the backend
  // parses both through the same shared parseTimeRange helper, which also
  // accepts an optional from/to (see range).
  getNetworkFlows: (hostId: string) => api.get(`/v1/hosts/${hostId}/network/flows`),
  getNetworkFlowsHistory: (
    hostId: string,
    params: { remote_ip: string; remote_port?: number; protocol: string; period?: string },
    range?: TimeRange,
  ) => api.get(`/v1/hosts/${hostId}/network/flows/history`, { params: { period: '24h', ...params, ...rangeParams(range) } }),
  getNetworkFlowsSummary: (hostId: string, period?: string, range?: TimeRange) =>
    api.get(`/v1/hosts/${hostId}/network/flows/summary`, { params: { period: period ?? '24h', ...rangeParams(range) } }),

  // Metrics
  getMetricsHistory: (hostId: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/metrics/history`, { params: { hours: hours ?? 24 } }),
  getMetricsAggregated: (hostId: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/metrics/aggregated`, { params: { hours: hours ?? 24 } }),
  getMetricsSummary: (hours?: number, bucketMinutes?: number) =>
    api.get('/v1/metrics/summary', { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),
}
