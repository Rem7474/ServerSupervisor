import { api } from './client'
import type { IPTimelineResponse, DomainDetailsParams } from '../types/security'
import type { LoginEvent } from '../types/generated'
import type { SecurityData } from '../components/security/AuditSecurityPanel.vue'
import type { WebAuthnCredential } from '../types/webauthn'

export const authApi = {
  login: (username: string, password: string, totpCode?: string) =>
    api.post('/auth/login', { username, password, ...(totpCode ? { totp_code: totpCode } : {}) }),
  getProfile: (signal?: AbortSignal) => api.get('/v1/auth/profile', { signal }),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.post('/v1/auth/change-password', { current_password: currentPassword, new_password: newPassword }),
  getLoginEvents: (signal?: AbortSignal) => api.get('/v1/auth/login-events', { signal }),
  getLoginEventsAdmin: (page?: number, limit?: number) =>
    api.get<{ events: LoginEvent[], total: number, page: number, limit: number }>(
      '/v1/auth/login-events/admin', { params: { page: page ?? 1, limit: limit ?? 50 } }
    ),
  revokeAllSessions: () => api.post('/v1/auth/revoke-all-sessions', {}),
  logout: () => api.post('/auth/logout', {}),
  refreshSession: () => api.post('/auth/refresh', {}),
  getSecuritySummary: (hours?: number) => api.get<SecurityData>('/v1/auth/security', { params: { hours: hours ?? 24 } }),
  getWebLogsSummary: (period: string = '24h', hostId?: string, source?: string, scope?: 'threats' | 'full') =>
    api.get('/v1/security/web-logs', { params: { period, host_id: hostId ?? '', source: source ?? '', scope: scope ?? '' } }),
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
  getDomainDetails: (domain: string, period: string = '24h', params: DomainDetailsParams = {}) =>
    api.get(`/v1/security/web-logs/domain/${encodeURIComponent(domain)}`, {
      params: {
        period,
        host_id: params.hostId ?? '',
        source: params.source ?? '',
        page: params.page ?? 1,
        limit: params.limit ?? 50,
        status: params.status ?? '',
        method: params.method ?? '',
        path: params.path ?? '',
        ip: params.ip ?? '',
        sort: params.sort ?? '',
        dir: params.dir ?? '',
      },
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
  getMFAStatus: (signal?: AbortSignal) => api.get('/v1/auth/mfa/status', { signal }),
  setupMFA: () => api.post('/v1/auth/mfa/setup'),
  verifyMFA: (secret: string, totpCode: string, backupCodes: string[]) =>
    api.post('/v1/auth/mfa/verify', { secret, totp_code: totpCode, backup_codes: backupCodes }),
  disableMFA: (password: string) => api.post('/v1/auth/mfa/disable', { password }),

  // WebAuthn (passkeys / security keys) — an additional MFA factor alongside TOTP.
  listWebAuthnCredentials: (signal?: AbortSignal) =>
    api.get<{ credentials: WebAuthnCredential[] }>('/v1/auth/webauthn/credentials', { signal }),
  beginWebAuthnRegistration: () =>
    api.post<{ options: unknown; session_token: string }>('/v1/auth/webauthn/register/begin', {}),
  finishWebAuthnRegistration: (sessionToken: string, name: string, credential: unknown) =>
    api.post<WebAuthnCredential>('/v1/auth/webauthn/register/finish', { session_token: sessionToken, name, credential }),
  deleteWebAuthnCredential: (id: string) => api.delete(`/v1/auth/webauthn/credentials/${id}`),
  beginWebAuthnLogin: (username: string, password: string) =>
    api.post<{ options: unknown; session_token: string }>('/auth/webauthn/login/begin', { username, password }),
  finishWebAuthnLogin: (username: string, sessionToken: string, credential: unknown) =>
    api.post('/auth/webauthn/login/finish', { username, session_token: sessionToken, credential }),
}
