import { api, type JsonObject } from './client'
import type { Host } from '../types/host'

export const hostsApi = {
  // Hosts
  getHosts: () => api.get<Host[]>('/v1/hosts'),
  getHost: (id: string) => api.get<Host>(`/v1/hosts/${id}`),
  getHostComplete: (id: string) => api.get(`/v1/hosts/${id}/complete`),
  getHostDashboard: (id: string) => api.get(`/v1/hosts/${id}/dashboard`),
  registerHost: (data: JsonObject) => api.post('/v1/hosts', data),
  updateHost: (id: string, data: JsonObject) => api.patch(`/v1/hosts/${id}`, data),
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

  // Metrics
  getMetricsHistory: (hostId: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/metrics/history`, { params: { hours: hours ?? 24 } }),
  getMetricsAggregated: (hostId: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/metrics/aggregated`, { params: { hours: hours ?? 24 } }),
  getMetricsSummary: (hours?: number, bucketMinutes?: number) =>
    api.get('/v1/metrics/summary', { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),
}
