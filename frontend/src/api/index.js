import axios from 'axios'
import { useAuthStore } from '../stores/auth'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

/**
 * Normalize API/Network error objects into a user-facing message.
 * @param {unknown} error
 * @param {string} [fallback='Une erreur est survenue']
 * @returns {string}
 */
export function getApiErrorMessage(error, fallback = 'Une erreur est survenue') {
  const message = error?.response?.data?.error || error?.response?.data?.message || error?.message
  return message ? String(message) : fallback
}

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
  getLoginEventsAdmin: (page = 1, limit = 50) => api.get('/v1/auth/login-events/admin', { params: { page, limit } }),
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
  getHostComplete: (id) => api.get(`/v1/hosts/${id}/complete`),
  getHostDashboard: (id) => api.get(`/v1/hosts/${id}/dashboard`),
  registerHost: (data) => api.post('/v1/hosts', data),
  updateHost: (id, data) => api.patch(`/v1/hosts/${id}`, data),
  deleteHost: (id) => api.delete(`/v1/hosts/${id}`),
  rotateHostKey: (id) => api.post(`/v1/hosts/${id}/rotate-key`),

  // Disk
  getDiskMetrics: (hostId) => api.get(`/v1/hosts/${hostId}/disk/metrics`),
  getDiskHealth: (hostId) => api.get(`/v1/hosts/${hostId}/disk/health`),

  // Metrics
  getMetricsHistory: (hostId, hours = 24) => api.get(`/v1/hosts/${hostId}/metrics/history`, { params: { hours } }),
  getMetricsAggregated: (hostId, hours = 24) => api.get(`/v1/hosts/${hostId}/metrics/aggregated`, { params: { hours } }),
  getMetricsSummary: (hours = 24, bucketMinutes = 5) =>
    api.get('/v1/metrics/summary', { params: { hours, bucket_minutes: bucketMinutes } }),

  // Docker
  getContainers: (hostId) => api.get(`/v1/hosts/${hostId}/containers`),
  getAllContainers: () => api.get('/v1/docker/containers'),
  getComposeProjects: () => api.get('/v1/docker/compose'),
  sendDockerCommand: (hostId, containerName, action, workingDir = '') =>
    api.post('/v1/docker/command', { host_id: hostId, container_name: containerName, action, working_dir: workingDir }),
  sendJournalCommand: (hostId, serviceName) =>
    api.post('/v1/system/journalctl', { host_id: hostId, service_name: serviceName }),
  sendSystemdCommand: (hostId, serviceName, action) =>
    api.post('/v1/system/service', { host_id: hostId, service_name: serviceName, action }),
  sendProcessesCommand: (hostId) =>
    api.post('/v1/system/processes', { host_id: hostId }),
  getHostCommandHistory: (hostId, limit = 50) => api.get(`/v1/hosts/${hostId}/commands/history`, { params: { limit } }),

  // APT
  getAptStatus: (hostId) => api.get(`/v1/hosts/${hostId}/apt`),
  sendAptCommand: (hostIds, command) => api.post('/v1/apt/command', { host_ids: hostIds, command }),

  // Network Topology
  getNetworkSnapshot: () => api.get('/v1/network'),
  getTopologySnapshot: () => api.get('/v1/network/topology'),
  getTopologyConfig: () => api.get('/v1/network/config'),
  saveTopologyConfig: (config) => api.put('/v1/network/config', config),

  // Audit
  getAuditLogs: (page = 1, limit = 50) => api.get('/v1/audit/logs', { params: { page, limit } }),
  getMyAuditLogs: (limit = 10) => api.get('/v1/audit/logs/me', { params: { limit } }),
  getAuditLogsByHost: (hostId, limit = 100) => api.get(`/v1/audit/logs/host/${hostId}`, { params: { limit } }),
  getAuditLogsByUser: (username, limit = 100) => api.get(`/v1/audit/logs/user/${username}`, { params: { limit } }),
  getCommandsHistory: (page = 1, limit = 50) => api.get('/v1/audit/commands', { params: { page, limit } }),
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
  markNotificationsRead: () => api.post('/v1/notifications/mark-read'),

  // Push (Web Push / VAPID)
  getPushVapidPublicKey: () => api.get('/v1/push/vapid-public-key'),
  subscribePush: (subscription) => api.post('/v1/push/subscribe', subscription),
  unsubscribePush: (endpoint) => api.delete('/v1/push/subscribe', { data: { endpoint } }),

  // Scheduled Tasks
  getAllScheduledTasks: () => api.get('/v1/scheduled-tasks'),
  getScheduledTasks: (hostId) => api.get(`/v1/hosts/${hostId}/scheduled-tasks`),
  createScheduledTask: (hostId, payload) => api.post(`/v1/hosts/${hostId}/scheduled-tasks`, payload),
  updateScheduledTask: (id, payload) => api.put(`/v1/scheduled-tasks/${id}`, payload),
  deleteScheduledTask: (id) => api.delete(`/v1/scheduled-tasks/${id}`),
  runScheduledTask: (id) => api.post(`/v1/scheduled-tasks/${id}/run`),
  getHostCustomTasks: (hostId) => api.get(`/v1/hosts/${hostId}/custom-tasks`),

  // Release Trackers
  getReleaseTrackers: () => api.get('/v1/release-trackers'),
  getReleaseTracker: (id) => api.get(`/v1/release-trackers/${id}`),
  createReleaseTracker: (payload) => api.post('/v1/release-trackers', payload),
  updateReleaseTracker: (id, payload) => api.put(`/v1/release-trackers/${id}`, payload),
  deleteReleaseTracker: (id) => api.delete(`/v1/release-trackers/${id}`),
  checkReleaseTrackerNow: (id) => api.post(`/v1/release-trackers/${id}/check-now`),
  runReleaseTracker: (id) => api.post(`/v1/release-trackers/${id}/run`),
  getReleaseTrackerExecutions: (id, limit = 50) => api.get(`/v1/release-trackers/${id}/executions`, { params: { limit } }),

  // Git Webhooks
  getGitWebhooks: () => api.get('/v1/webhooks/git'),
  getGitWebhook: (id) => api.get(`/v1/webhooks/git/${id}`),
  createGitWebhook: (payload) => api.post('/v1/webhooks/git', payload),
  updateGitWebhook: (id, payload) => api.put(`/v1/webhooks/git/${id}`, payload),
  deleteGitWebhook: (id) => api.delete(`/v1/webhooks/git/${id}`),
  regenerateWebhookSecret: (id) => api.post(`/v1/webhooks/git/${id}/regenerate-secret`),
  getWebhookExecutions: (id, limit = 50) => api.get(`/v1/webhooks/git/${id}/executions`, { params: { limit } }),

  // Settings
  getSettings: () => api.get('/v1/settings'),
  updateSettings: (payload) => api.put('/v1/settings', payload),
  testSmtp: () => api.post('/v1/settings/test-smtp'),
  testNtfy: () => api.post('/v1/settings/test-ntfy'),
  cleanupMetrics: () => api.post('/v1/settings/cleanup-metrics'),
  cleanupAudit: () => api.post('/v1/settings/cleanup-audit'),
}
