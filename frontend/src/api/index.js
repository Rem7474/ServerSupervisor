import axios from 'axios'
import { useAuthStore } from '../stores/auth'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// Add JWT token to requests
api.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  return config
})

// Handle 401 responses
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      const auth = useAuthStore()
      auth.logout()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default {
  // Auth
  login: (username, password, totpCode = '') =>
    api.post('/auth/login', { username, password, ...(totpCode ? { totp_code: totpCode } : {}) }),
  getProfile: () => api.get('/v1/auth/profile'),
  changePassword: (currentPassword, newPassword) =>
    api.post('/v1/auth/change-password', { current_password: currentPassword, new_password: newPassword }),
  getLoginEvents: () => api.get('/v1/auth/login-events'),
  getLoginEventsAdmin: (page = 1, limit = 50) => api.get(`/v1/auth/login-events/admin?page=${page}&limit=${limit}`),
  revokeAllSessions: (refreshToken) => api.post('/v1/auth/revoke-all-sessions', { refresh_token: refreshToken }),
  getSecuritySummary: () => api.get('/v1/auth/security'),
  unblockIP: (ip) => api.delete(`/v1/auth/blocked-ips/${ip}`),
  getMFAStatus: () => api.get('/v1/auth/mfa/status'),
  setupMFA: () => api.post('/v1/auth/mfa/setup'),
  verifyMFA: (secret, totpCode, backupCodes) =>
    api.post('/v1/auth/mfa/verify', { secret, totp_code: totpCode, backup_codes: backupCodes }),
  disableMFA: (password) => api.post('/v1/auth/mfa/disable', { password }),

  // Hosts
  getHosts: () => api.get('/v1/hosts'),
  getHost: (id) => api.get(`/v1/hosts/${id}`),
  getHostDashboard: (id) => api.get(`/v1/hosts/${id}/dashboard`),
  registerHost: (data) => api.post('/v1/hosts', data),
  updateHost: (id, data) => api.patch(`/v1/hosts/${id}`, data),
  deleteHost: (id) => api.delete(`/v1/hosts/${id}`),
  rotateHostKey: (id) => api.post(`/v1/hosts/${id}/rotate-key`),

  // Disk
  getDiskMetrics: (hostId) => api.get(`/v1/hosts/${hostId}/disk/metrics`),
  getDiskHealth: (hostId) => api.get(`/v1/hosts/${hostId}/disk/health`),

  // Metrics
  getMetricsHistory: (hostId, hours = 24) => api.get(`/v1/hosts/${hostId}/metrics/history?hours=${hours}`),
  getMetricsAggregated: (hostId, hours = 24) => api.get(`/v1/hosts/${hostId}/metrics/aggregated?hours=${hours}`),
  getMetricsSummary: (hours = 24, bucketMinutes = 5) =>
    api.get(`/v1/metrics/summary?hours=${hours}&bucket_minutes=${bucketMinutes}`),

  // Docker
  getContainers: (hostId) => api.get(`/v1/hosts/${hostId}/containers`),
  getAllContainers: () => api.get('/v1/docker/containers'),
  getComposeProjects: () => api.get('/v1/docker/compose'),
  getVersionComparisons: () => api.get('/v1/docker/versions'),
  sendDockerCommand: (hostId, containerName, action, workingDir = '') =>
    api.post('/v1/docker/command', { host_id: hostId, container_name: containerName, action, working_dir: workingDir }),
  sendJournalCommand: (hostId, serviceName) =>
    api.post('/v1/system/journalctl', { host_id: hostId, service_name: serviceName }),
  sendSystemdCommand: (hostId, serviceName, action) =>
    api.post('/v1/system/service', { host_id: hostId, service_name: serviceName, action }),
  sendProcessesCommand: (hostId) =>
    api.post('/v1/system/processes', { host_id: hostId }),
  getDockerHistory: (hostId) => api.get(`/v1/hosts/${hostId}/docker/history`),
  getHostCommandHistory: (hostId, limit = 50) => api.get(`/v1/hosts/${hostId}/commands/history?limit=${limit}`),

  // Tracked Repos
  getTrackedRepos: () => api.get('/v1/repos'),
  addTrackedRepo: (data) => api.post('/v1/repos', data),
  deleteTrackedRepo: (id) => api.delete(`/v1/repos/${id}`),

  // APT
  getAptStatus: (hostId) => api.get(`/v1/hosts/${hostId}/apt`),
  getAptHistory: (hostId) => api.get(`/v1/hosts/${hostId}/apt/history`),
  sendAptCommand: (hostIds, command) => api.post('/v1/apt/command', { host_ids: hostIds, command }),

  // Network Topology
  getNetworkSnapshot: () => api.get('/v1/network'),
  getTopologySnapshot: () => api.get('/v1/network/topology'),
  getTopologyConfig: () => api.get('/v1/network/config'),
  saveTopologyConfig: (config) => api.put('/v1/network/config', config),

  // Audit
  getAuditLogs: (page = 1, limit = 50) => api.get(`/v1/audit/logs?page=${page}&limit=${limit}`),
  getMyAuditLogs: (limit = 10) => api.get(`/v1/audit/logs/me?limit=${limit}`),
  getAuditLogsByHost: (hostId, limit = 100) => api.get(`/v1/audit/logs/host/${hostId}?limit=${limit}`),
  getAuditLogsByUser: (username, limit = 100) => api.get(`/v1/audit/logs/user/${username}?limit=${limit}`),
  getCommandsHistory: (page = 1, limit = 50) => api.get(`/v1/audit/commands?page=${page}&limit=${limit}`),
  getCommandStatus: (id) => api.get(`/v1/commands/${id}`),

  // Users
  getUsers: () => api.get('/v1/users'),
  createUser: (username, password, role) => api.post('/v1/users', { username, password, role }),
  updateUserRole: (id, role) => api.patch(`/v1/users/${id}/role`, { role }),
  deleteUser: (id) => api.delete(`/v1/users/${id}`),

  // Alert Rules
  getAlertRules: () => api.get('/v1/alert-rules'),
  createAlertRule: (payload) => api.post('/v1/alert-rules', payload),
  updateAlertRule: (id, payload) => api.patch(`/v1/alert-rules/${id}`, payload),
  deleteAlertRule: (id) => api.delete(`/v1/alert-rules/${id}`),
  testAlertRule: (payload) => api.post('/v1/alert-rules/test', payload),

  // Notifications
  getNotifications: () => api.get('/v1/notifications'),

  // Settings
  getSettings: () => api.get('/v1/settings'),
  updateSettings: (payload) => api.put('/v1/settings', payload),
  testSmtp: () => api.post('/v1/settings/test-smtp'),
  testNtfy: () => api.post('/v1/settings/test-ntfy'),
  cleanupMetrics: () => api.post('/v1/settings/cleanup-metrics'),
  cleanupAudit: () => api.post('/v1/settings/cleanup-audit'),
}
