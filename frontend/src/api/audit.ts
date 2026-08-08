import { api } from './client'
import type { AuditLog, RemoteCommand, RemoteCommandWithHost, HostTimelineEvent } from '../types/audit'

export interface AuditLogFilters {
  category?: string
  from?: string
  to?: string
}

export const auditApi = {
  getAuditLogs: (page?: number, limit?: number, filters?: AuditLogFilters) =>
    api.get<{ logs: AuditLog[], page: number, limit: number }>('/v1/audit/logs', { params: { page: page ?? 1, limit: limit ?? 50, ...filters } }),
  exportAuditLogs: (filters?: AuditLogFilters) =>
    api.get('/v1/audit/logs/export', { params: filters, responseType: 'blob' }),
  getMyAuditLogs: (limit?: number) => api.get<AuditLog[]>('/v1/audit/logs/me', { params: { limit: limit ?? 10 } }),
  getAuditLogsByHost: (hostId: string, limit?: number) =>
    api.get<AuditLog[]>(`/v1/audit/logs/host/${hostId}`, { params: { limit: limit ?? 100 } }),
  getAuditLogsByUser: (username: string, limit?: number) =>
    api.get<{ user: string, logs: AuditLog[] }>(`/v1/audit/logs/user/${username}`, { params: { limit: limit ?? 100 } }),
  getCommandsHistory: (page?: number, limit?: number, filters?: { search?: string; module?: string; status?: string }, signal?: AbortSignal) =>
    api.get<{ commands: RemoteCommandWithHost[], total: number, page: number, limit: number }>('/v1/audit/commands', { params: { page: page ?? 1, limit: limit ?? 50, ...filters }, signal }),
  getCommandStatus: (id: string) => api.get<RemoteCommand>(`/v1/commands/${id}`),
  cancelCommand: (id: string) => api.post<{ status: string }>(`/v1/commands/${id}/cancel`),
  getHostTimeline: (hostId: string, limit?: number) =>
    api.get<{ events: HostTimelineEvent[] }>(`/v1/hosts/${hostId}/timeline`, { params: { limit: limit ?? 50 } }),
}
