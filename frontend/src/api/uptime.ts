import { api, type JsonObject } from './client'

export const uptimeApi = {
  getUptimeProbes: () => api.get('/v1/uptime/probes'),
  getUptimeProbe: (id: string) => api.get(`/v1/uptime/probes/${id}`),
  createUptimeProbe: (payload: JsonObject) => api.post('/v1/uptime/probes', payload),
  updateUptimeProbe: (id: string, payload: JsonObject) => api.put(`/v1/uptime/probes/${id}`, payload),
  deleteUptimeProbe: (id: string) => api.delete(`/v1/uptime/probes/${id}`),
  checkUptimeProbeNow: (id: string) => api.post(`/v1/uptime/probes/${id}/check-now`),
  getUptimeHistory: (id: string, limit?: number) =>
    api.get(`/v1/uptime/probes/${id}/history`, { params: { limit: limit ?? 200 } }),
  getUptimeStats: (id: string, hours?: number) =>
    api.get(`/v1/uptime/probes/${id}/stats`, { params: { hours: hours ?? 24 } }),
}
