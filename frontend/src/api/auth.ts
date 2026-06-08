import { api } from './client'
import type { IPTimelineResponse } from '../types/security'

export const authApi = {
  login: (username: string, password: string, totpCode?: string) =>
    api.post('/auth/login', { username, password, ...(totpCode ? { totp_code: totpCode } : {}) }),
  getProfile: () => api.get('/v1/auth/profile'),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.post('/v1/auth/change-password', { current_password: currentPassword, new_password: newPassword }),
  getLoginEvents: () => api.get('/v1/auth/login-events'),
  getLoginEventsAdmin: (page?: number, limit?: number) =>
    api.get('/v1/auth/login-events/admin', { params: { page: page ?? 1, limit: limit ?? 50 } }),
  revokeAllSessions: () => api.post('/v1/auth/revoke-all-sessions', {}),
  logout: () => api.post('/auth/logout', {}),
  refreshSession: () => api.post('/auth/refresh', {}),
  getSecuritySummary: (hours?: number) => api.get('/v1/auth/security', { params: { hours: hours ?? 24 } }),
  getWebLogsSummary: (period: string = '24h', hostId?: string, source?: string) =>
    api.get('/v1/security/web-logs', { params: { period, host_id: hostId ?? '', source: source ?? '' } }),
  getWebLogsTimeseries: (period: string = '24h', bucket: 'hour' | 'minute' = 'hour', hostId?: string, source?: string) =>
    api.get('/v1/security/web-logs/timeseries', {
      params: { period, bucket, host_id: hostId ?? '', source: source ?? '' },
    }),
  getWebLogsLive: (hostId?: string, source?: string, limit: number = 100) =>
    api.get('/v1/security/web-logs/live', {
      params: { host_id: hostId ?? '', source: source ?? '', limit },
    }),
  getIPTimeline: (ip: string, hostId?: string, period: string = '24h', limit: number = 500) =>
    api.get<IPTimelineResponse>(`/v1/security/web-logs/ip/${encodeURIComponent(ip)}`, {
      params: { host_id: hostId ?? '', period, limit },
    }),
  getDomainDetails: (domain: string, period: string = '24h', hostId?: string, source?: string, limit: number = 300) =>
    api.get(`/v1/security/web-logs/domain/${encodeURIComponent(domain)}`, {
      params: { period, host_id: hostId ?? '', source: source ?? '', limit },
    }),
  getCommand: (id: string) => api.get(`/v1/commands/${id}`),
  blockCrowdSecIP: (ip: string, hostId: string, duration: string = '4h') =>
    api.post(`/v1/security/web-logs/ip/${encodeURIComponent(ip)}/decisions`, null, {
      params: { host_id: hostId, duration },
    }),
  unblockCrowdSecIP: (ip: string, hostId: string) =>
    api.delete(`/v1/security/web-logs/ip/${encodeURIComponent(ip)}/decisions`, {
      params: { host_id: hostId },
    }),
  unblockIP: (ip: string) => api.delete(`/v1/auth/blocked-ips/${ip}`),
  getMFAStatus: () => api.get('/v1/auth/mfa/status'),
  setupMFA: () => api.post('/v1/auth/mfa/setup'),
  verifyMFA: (secret: string, totpCode: string, backupCodes: string[]) =>
    api.post('/v1/auth/mfa/verify', { secret, totp_code: totpCode, backup_codes: backupCodes }),
  disableMFA: (password: string) => api.post('/v1/auth/mfa/disable', { password }),
}
