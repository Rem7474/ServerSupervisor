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
  login: (username, password) => api.post('/auth/login', { username, password }),
  changePassword: (currentPassword, newPassword) =>
    api.post('/v1/auth/change-password', { current_password: currentPassword, new_password: newPassword }),

  // Hosts
  getHosts: () => api.get('/v1/hosts'),
  getHost: (id) => api.get(`/v1/hosts/${id}`),
  getHostDashboard: (id) => api.get(`/v1/hosts/${id}/dashboard`),
  registerHost: (data) => api.post('/v1/hosts', data),
  updateHost: (id, data) => api.patch(`/v1/hosts/${id}`, data),
  deleteHost: (id) => api.delete(`/v1/hosts/${id}`),

  // Metrics
  getMetricsHistory: (hostId, hours = 24) => api.get(`/v1/hosts/${hostId}/metrics/history?hours=${hours}`),

  // Docker
  getContainers: (hostId) => api.get(`/v1/hosts/${hostId}/containers`),
  getAllContainers: () => api.get('/v1/docker/containers'),
  getVersionComparisons: () => api.get('/v1/docker/versions'),

  // Tracked Repos
  getTrackedRepos: () => api.get('/v1/repos'),
  addTrackedRepo: (data) => api.post('/v1/repos', data),
  deleteTrackedRepo: (id) => api.delete(`/v1/repos/${id}`),

  // APT
  getAptStatus: (hostId) => api.get(`/v1/hosts/${hostId}/apt`),
  getAptHistory: (hostId) => api.get(`/v1/hosts/${hostId}/apt/history`),
  sendAptCommand: (hostIds, command) => api.post('/v1/apt/command', { host_ids: hostIds, command }),
}
