import { api } from './client'
import type { UptimeProbe, UptimeProbeRequest, UptimeProbeResult, UptimeStats } from '../types/uptime'

export const uptimeApi = {
  getUptimeProbes: () => api.get<{ probes: UptimeProbe[] }>('/v1/uptime/probes'),
  getUptimeProbe: (id: string) => api.get<UptimeProbe>(`/v1/uptime/probes/${id}`),
  createUptimeProbe: (payload: Partial<UptimeProbeRequest>) => api.post('/v1/uptime/probes', payload),
  updateUptimeProbe: (id: string, payload: Partial<UptimeProbeRequest>) => api.put(`/v1/uptime/probes/${id}`, payload),
  deleteUptimeProbe: (id: string) => api.delete(`/v1/uptime/probes/${id}`),
  checkUptimeProbeNow: (id: string) => api.post(`/v1/uptime/probes/${id}/check-now`),
  getUptimeHistory: (id: string, limit?: number) =>
    api.get<{ results: UptimeProbeResult[] }>(`/v1/uptime/probes/${id}/history`, { params: { limit: limit ?? 200 } }),
  getUptimeStats: (id: string, hours?: number) =>
    api.get<UptimeStats>(`/v1/uptime/probes/${id}/stats`, { params: { hours: hours ?? 24 } }),
}
