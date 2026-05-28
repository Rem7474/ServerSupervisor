import { api } from './client'

export const auditApi = {
  getAuditLogs: (page?: number, limit?: number) =>
    api.get('/v1/audit/logs', { params: { page: page ?? 1, limit: limit ?? 50 } }),
  getMyAuditLogs: (limit?: number) => api.get('/v1/audit/logs/me', { params: { limit: limit ?? 10 } }),
  getAuditLogsByHost: (hostId: string, limit?: number) =>
    api.get(`/v1/audit/logs/host/${hostId}`, { params: { limit: limit ?? 100 } }),
  getAuditLogsByUser: (username: string, limit?: number) =>
    api.get(`/v1/audit/logs/user/${username}`, { params: { limit: limit ?? 100 } }),
  getCommandsHistory: (page?: number, limit?: number, filters?: { search?: string; module?: string; status?: string }) =>
    api.get('/v1/audit/commands', { params: { page: page ?? 1, limit: limit ?? 50, ...filters } }),
  getCommandStatus: (id: string) => api.get(`/v1/commands/${id}`),
}
