import axios, { AxiosInstance, AxiosError } from 'axios'
import { useAuthStore } from '../stores/auth'
import { emitHttpError } from '../utils/httpErrorBus'

const api: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

/**
 * Normalize API/Network error objects into a user-facing message.
 */
export function getApiErrorMessage(
  error: unknown,
  fallback: string = 'Une erreur est survenue'
): string {
  const message =
    (error as any)?.response?.data?.error ||
    (error as any)?.response?.data?.message ||
    (error as any)?.message

  return message ? String(message) : fallback
}

/**
 * Add JWT token and standard headers to every request.
 * X-Requested-With is a defense-in-depth measure to prevent CSRF attacks.
 */
api.interceptors.request.use((config: any) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  config.headers['X-Requested-With'] = 'XMLHttpRequest'
  return config
})

/**
 * Handle 401 (unauthorized) by logging out and redirecting to login.
 * Silently ignore aborted requests (AbortController / component unmount).
 */
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (axios.isCancel(error)) {
      return Promise.reject(error)
    }
    const status = error.response?.status ?? null
    if (status === 401) {
      const auth = useAuthStore()
      auth.logout()
      window.location.href = '/login'
    } else if (status === 403) {
      emitHttpError(403, "Vous n'avez pas les droits nécessaires pour cette action")
    } else if (status && status >= 500) {
      emitHttpError(status, 'Le serveur a rencontré une erreur. Réessayez dans quelques instants.')
    } else if (status === null) {
      emitHttpError(null, 'Erreur réseau: impossible de joindre le serveur')
    }
    return Promise.reject(error)
  }
)

/**
 * Check if an axios error was caused by intentional cancellation.
 */
export function isApiAbort(error: unknown): boolean {
  return (
    axios.isCancel(error) ||
    (error as any)?.name === 'CanceledError' ||
    (error as any)?.name === 'AbortError'
  )
}

interface AlertRule {
  id?: number
  name?: string
  enabled?: boolean
  host_id?: string
  metric?: string
  operator?: string
  threshold?: number
  duration?: number
  actions?: {
    channels?: string[]
    smtp_to?: string
    ntfy_topic?: string
    cooldown?: number
    command_trigger?: any
  }
}

interface ProxmoxConnection {
  id: string
  name: string
  api_url: string
  token_id: string
  insecure_skip_verify?: boolean
  enabled?: boolean
  poll_interval_sec?: number
}

export default {
  // Auth
  login: (username: string, password: string, totpCode?: string) =>
    api.post('/auth/login', { username, password, ...(totpCode ? { totp_code: totpCode } : {}) }),
  getProfile: () => api.get('/v1/auth/profile'),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.post('/v1/auth/change-password', { current_password: currentPassword, new_password: newPassword }),
  getLoginEvents: () => api.get('/v1/auth/login-events'),
  getLoginEventsAdmin: (page?: number, limit?: number) =>
    api.get('/v1/auth/login-events/admin', { params: { page: page ?? 1, limit: limit ?? 50 } }),
  revokeAllSessions: (refreshToken: string) => api.post('/v1/auth/revoke-all-sessions', { refresh_token: refreshToken }),
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
    api.get(`/v1/security/web-logs/ip/${encodeURIComponent(ip)}`, {
      params: { host_id: hostId ?? '', period, limit },
    }),
  getDomainDetails: (domain: string, period: string = '24h', hostId?: string, source?: string, limit: number = 300) =>
    api.get(`/v1/security/web-logs/domain/${encodeURIComponent(domain)}`, {
      params: { period, host_id: hostId ?? '', source: source ?? '', limit },
    }),
  unblockIP: (ip: string) => api.delete(`/v1/auth/blocked-ips/${ip}`),
  getMFAStatus: () => api.get('/v1/auth/mfa/status'),
  setupMFA: () => api.post('/v1/auth/mfa/setup'),
  verifyMFA: (secret: string, totpCode: string, backupCodes: string[]) =>
    api.post('/v1/auth/mfa/verify', { secret, totp_code: totpCode, backup_codes: backupCodes }),
  disableMFA: (password: string) => api.post('/v1/auth/mfa/disable', { password }),

  // Hosts
  getHosts: () => api.get('/v1/hosts'),
  getHost: (id: string) => api.get(`/v1/hosts/${id}`),
  getHostComplete: (id: string) => api.get(`/v1/hosts/${id}/complete`),
  getHostDashboard: (id: string) => api.get(`/v1/hosts/${id}/dashboard`),
  registerHost: (data: Record<string, any>) => api.post('/v1/hosts', data),
  updateHost: (id: string, data: Record<string, any>) => api.patch(`/v1/hosts/${id}`, data),
  deleteHost: (id: string) => api.delete(`/v1/hosts/${id}`),
  rotateHostKey: (id: string) => api.post(`/v1/hosts/${id}/rotate-key`),

  // Disk
  getDiskMetrics: (hostId: string) => api.get(`/v1/hosts/${hostId}/disk/metrics`),
  getDiskHealth: (hostId: string) => api.get(`/v1/hosts/${hostId}/disk/health`),

  // Metrics
  getMetricsHistory: (hostId: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/metrics/history`, { params: { hours: hours ?? 24 } }),
  getMetricsAggregated: (hostId: string, hours?: number) =>
    api.get(`/v1/hosts/${hostId}/metrics/aggregated`, { params: { hours: hours ?? 24 } }),
  getMetricsSummary: (hours?: number, bucketMinutes?: number) =>
    api.get('/v1/metrics/summary', { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),

  // Docker
  getContainers: (hostId: string) => api.get(`/v1/hosts/${hostId}/containers`),
  getAllContainers: () => api.get('/v1/docker/containers'),
  getComposeProjects: () => api.get('/v1/docker/compose'),
  sendDockerCommand: (hostId: string, containerName: string, action: string, workingDir?: string) =>
    api.post('/v1/docker/command', { host_id: hostId, container_name: containerName, action, working_dir: workingDir ?? '' }),
  sendJournalCommand: (hostId: string, serviceName: string) =>
    api.post('/v1/system/journalctl', { host_id: hostId, service_name: serviceName }),
  sendSystemdCommand: (hostId: string, serviceName: string, action: string) =>
    api.post('/v1/system/service', { host_id: hostId, service_name: serviceName, action }),
  sendProcessesCommand: (hostId: string) =>
    api.post('/v1/system/processes', { host_id: hostId }),
  getHostCommandHistory: (hostId: string, limit?: number) =>
    api.get(`/v1/hosts/${hostId}/commands/history`, { params: { limit: limit ?? 50 } }),

  // APT
  getAptStatus: (hostId: string) => api.get(`/v1/hosts/${hostId}/apt`),
  getAptCVESummary: () => api.get('/v1/apt/summary'),
  sendAptCommand: (hostIds: string[], command: string) =>
    api.post('/v1/apt/command', { host_ids: hostIds, command }),

  // Network Topology
  getNetworkSnapshot: () => api.get('/v1/network'),
  getTopologySnapshot: () => api.get('/v1/network/topology'),
  getTopologyConfig: () => api.get('/v1/network/config'),
  saveTopologyConfig: (config: Record<string, any>) => api.put('/v1/network/config', config),

  // Audit
  getAuditLogs: (page?: number, limit?: number) =>
    api.get('/v1/audit/logs', { params: { page: page ?? 1, limit: limit ?? 50 } }),
  getMyAuditLogs: (limit?: number) => api.get('/v1/audit/logs/me', { params: { limit: limit ?? 10 } }),
  getAuditLogsByHost: (hostId: string, limit?: number) =>
    api.get(`/v1/audit/logs/host/${hostId}`, { params: { limit: limit ?? 100 } }),
  getAuditLogsByUser: (username: string, limit?: number) =>
    api.get(`/v1/audit/logs/user/${username}`, { params: { limit: limit ?? 100 } }),
  getCommandsHistory: (page?: number, limit?: number) =>
    api.get('/v1/audit/commands', { params: { page: page ?? 1, limit: limit ?? 50 } }),
  getCommandStatus: (id: string) => api.get(`/v1/commands/${id}`),

  // Users
  getUsers: () => api.get('/v1/users'),
  createUser: (username: string, password: string, role: string) =>
    api.post('/v1/users', { username, password, role }),
  updateUserRole: (id: string, role: string) => api.patch(`/v1/users/${id}/role`, { role }),
  deleteUser: (id: string) => api.delete(`/v1/users/${id}`),

  // Host permissions
  getHostPermissions: (hostId: string) => api.get(`/v1/hosts/${hostId}/permissions`),
  setHostPermission: (hostId: string, username: string, level: string) =>
    api.put(`/v1/hosts/${hostId}/permissions/${username}`, { level }),
  deleteHostPermission: (hostId: string, username: string) =>
    api.delete(`/v1/hosts/${hostId}/permissions/${username}`),
  getMyHostPermissions: () => api.get('/v1/auth/host-permissions'),

  // Alert Rules
  getAlertRules: () => api.get('/v1/alert-rules'),
  createAlertRule: (payload: AlertRule) => api.post('/v1/alert-rules', payload),
  updateAlertRule: (id: number, payload: AlertRule) => api.patch(`/v1/alert-rules/${id}`, payload),
  deleteAlertRule: (id: number) => api.delete(`/v1/alert-rules/${id}`),
  testAlertRule: (payload: AlertRule) => api.post('/v1/alert-rules/test', payload),

  // Notifications
  getNotifications: () => api.get('/v1/notifications'),
  markNotificationsRead: () => api.post('/v1/notifications/mark-read'),

  // Push (Web Push / VAPID)
  getPushVapidPublicKey: () => api.get('/v1/push/vapid-public-key'),
  subscribePush: (subscription: PushSubscriptionJSON) => api.post('/v1/push/subscribe', subscription),
  unsubscribePush: (endpoint: string) => api.delete('/v1/push/subscribe', { data: { endpoint } }),

  // Scheduled Tasks
  getAllScheduledTasks: () => api.get('/v1/scheduled-tasks'),
  getScheduledTasks: (hostId: string) => api.get(`/v1/hosts/${hostId}/scheduled-tasks`),
  createScheduledTask: (hostId: string, payload: Record<string, any>) =>
    api.post(`/v1/hosts/${hostId}/scheduled-tasks`, payload),
  updateScheduledTask: (id: string, payload: Record<string, any>) =>
    api.put(`/v1/scheduled-tasks/${id}`, payload),
  deleteScheduledTask: (id: string) => api.delete(`/v1/scheduled-tasks/${id}`),
  runScheduledTask: (id: string) => api.post(`/v1/scheduled-tasks/${id}/run`),
  getScheduledTaskExecutions: (id: string, limit?: number) =>
    api.get(`/v1/scheduled-tasks/${id}/executions`, { params: { limit: limit ?? 20 } }),
  getHostCustomTasks: (hostId: string) => api.get(`/v1/hosts/${hostId}/custom-tasks`),

  // Release Trackers
  getReleaseTrackers: () => api.get('/v1/release-trackers'),
  getReleaseTracker: (id: string) => api.get(`/v1/release-trackers/${id}`),
  createReleaseTracker: (payload: Record<string, any>) => api.post('/v1/release-trackers', payload),
  updateReleaseTracker: (id: string, payload: Record<string, any>) =>
    api.put(`/v1/release-trackers/${id}`, payload),
  deleteReleaseTracker: (id: string) => api.delete(`/v1/release-trackers/${id}`),
  checkReleaseTrackerNow: (id: string) => api.post(`/v1/release-trackers/${id}/check-now`),
  runReleaseTracker: (id: string) => api.post(`/v1/release-trackers/${id}/run`),
  getReleaseTrackerExecutions: (id: string, limit?: number) =>
    api.get(`/v1/release-trackers/${id}/executions`, { params: { limit: limit ?? 50 } }),

  // Git Webhooks
  getGitWebhooks: () => api.get('/v1/webhooks/git'),
  getGitWebhook: (id: string) => api.get(`/v1/webhooks/git/${id}`),
  createGitWebhook: (payload: Record<string, any>) => api.post('/v1/webhooks/git', payload),
  updateGitWebhook: (id: string, payload: Record<string, any>) =>
    api.put(`/v1/webhooks/git/${id}`, payload),
  deleteGitWebhook: (id: string) => api.delete(`/v1/webhooks/git/${id}`),
  regenerateWebhookSecret: (id: string) => api.post(`/v1/webhooks/git/${id}/regenerate-secret`),
  getWebhookExecutions: (id: string, limit?: number) =>
    api.get(`/v1/webhooks/git/${id}/executions`, { params: { limit: limit ?? 50 } }),

  // Settings
  getSettings: () => api.get('/v1/settings'),
  updateSettings: (payload: Record<string, any>) => api.put('/v1/settings', payload),
  testSmtp: () => api.post('/v1/settings/test-smtp'),
  testNtfy: () => api.post('/v1/settings/test-ntfy'),
  cleanupMetrics: () => api.post('/v1/settings/cleanup-metrics'),
  cleanupAudit: () => api.post('/v1/settings/cleanup-audit'),

  // Proxmox
  getProxmoxSummary: () => api.get('/v1/proxmox/summary'),
  getProxmoxInstances: () => api.get('/v1/proxmox/instances'),
  getProxmoxInstance: (id: string) => api.get(`/v1/proxmox/instances/${id}`),
  createProxmoxInstance: (payload: Partial<ProxmoxConnection>) =>
    api.post('/v1/proxmox/instances', payload),
  updateProxmoxInstance: (id: string, payload: Partial<ProxmoxConnection>) =>
    api.put(`/v1/proxmox/instances/${id}`, payload),
  deleteProxmoxInstance: (id: string) => api.delete(`/v1/proxmox/instances/${id}`),
  testProxmoxConnection: (payload: Omit<ProxmoxConnection, 'id'>) =>
    api.post('/v1/proxmox/instances/test', payload),
  testProxmoxInstanceById: (id: string) => api.post(`/v1/proxmox/instances/${id}/test`),
  pollProxmoxNow: (id: string) => api.post(`/v1/proxmox/instances/${id}/poll-now`),
  getProxmoxNodes: (connectionId?: string) =>
    api.get('/v1/proxmox/nodes', { params: connectionId ? { connection_id: connectionId } : {} }),
  getProxmoxNode: (id: string) => api.get(`/v1/proxmox/nodes/${id}`),
  getProxmoxNodeCpuTempSourceCandidates: (id: string) => api.get(`/v1/proxmox/nodes/${id}/cpu-temp-source/candidates`),
  setProxmoxNodeCpuTempSource: (id: string, hostId: string | null) =>
    api.put(`/v1/proxmox/nodes/${id}/cpu-temp-source`, { host_id: hostId ?? '' }),
  getProxmoxNodeMetrics: (hours?: number, bucketMinutes?: number) =>
    api.get('/v1/proxmox/nodes/metrics', { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),
  getProxmoxGuests: (params?: Record<string, any>) => api.get('/v1/proxmox/guests', { params: params ?? {} }),
  getProxmoxGuestMetrics: (guestId: string, hours?: number, bucketMinutes?: number) =>
    api.get(`/v1/proxmox/guests/${guestId}/metrics`, { params: { hours: hours ?? 24, bucket_minutes: bucketMinutes ?? 5 } }),
  getProxmoxGuestLink: (guestId: string) => api.get(`/v1/proxmox/guests/${guestId}/link`),

  // Guest ↔ host links
  getProxmoxLinks: (status?: string) =>
    api.get('/v1/proxmox/links', { params: status ? { status } : {} }),
  getProxmoxLink: (id: string) => api.get(`/v1/proxmox/links/${id}`),
  createProxmoxLink: (payload: Record<string, any>) => api.post('/v1/proxmox/links', payload),
  updateProxmoxLink: (id: string, payload: Record<string, any>) =>
    api.put(`/v1/proxmox/links/${id}`, payload),
  deleteProxmoxLink: (id: string) => api.delete(`/v1/proxmox/links/${id}`),

  // Per-host Proxmox link
  getHostProxmoxLink: (hostId: string) => api.get(`/v1/hosts/${hostId}/proxmox-link`),
  getHostProxmoxCandidates: (hostId: string) => api.get(`/v1/hosts/${hostId}/proxmox-candidates`),

  // Extended: tasks
  getProxmoxTasks: (params?: Record<string, any>) =>
    api.get('/v1/proxmox/tasks', { params: params ?? {} }),
  getProxmoxNodeTasks: (nodeId: string, limit?: number) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/tasks`, { params: { limit: limit ?? 50 } }),

  // Extended: disks
  getProxmoxNodeDisks: (nodeId: string) => api.get(`/v1/proxmox/nodes/${nodeId}/disks`),

  // Extended: backups
  getProxmoxBackupJobs: (connectionId?: string) =>
    api.get('/v1/proxmox/backup-jobs', { params: connectionId ? { connection_id: connectionId } : {} }),
  getProxmoxBackupRuns: (connectionId?: string) =>
    api.get('/v1/proxmox/backup-runs', { params: connectionId ? { connection_id: connectionId } : {} }),

  // Node live data
  getProxmoxNodeStatus: (nodeId: string) => api.get(`/v1/proxmox/nodes/${nodeId}/status`),
  getProxmoxTaskLog: (nodeId: string, upid: string) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/tasks/${encodeURIComponent(upid)}/log`),
  getProxmoxNodeRRD: (nodeId: string, timeframe?: string) =>
    api.get(`/v1/proxmox/nodes/${nodeId}/rrd`, { params: { timeframe: timeframe ?? 'hour' } }),
  getProxmoxNodeSyslog: (nodeId: string, params?: Record<string, any>) =>
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
}

