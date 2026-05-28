import { api } from './client'

export const aptApi = {
  // APT
  getAptStatus: (hostId: string) => api.get(`/v1/hosts/${hostId}/apt`),
  getAptCVESummary: () => api.get('/v1/apt/summary'),
  sendAptCommand: (hostIds: string[], command: string) =>
    api.post('/v1/apt/command', { host_ids: hostIds, command }),

  // Unattended-upgrades
  getUUStatus: (hostId: string) => api.get(`/v1/hosts/${hostId}/apt/unattended-upgrades`),
  updateUU: (hostId: string, data: { enabled: boolean; config: object }) =>
    api.put(`/v1/hosts/${hostId}/apt/unattended-upgrades`, data),
  installUU: (hostId: string) => api.post(`/v1/hosts/${hostId}/apt/unattended-upgrades/install`),
  runUUNow: (hostId: string) => api.post(`/v1/hosts/${hostId}/apt/unattended-upgrades/run-now`),
  getUURuns: (hostId: string, limit = 20) =>
    api.get(`/v1/hosts/${hostId}/apt/unattended-upgrades/runs`, { params: { limit } }),
}
