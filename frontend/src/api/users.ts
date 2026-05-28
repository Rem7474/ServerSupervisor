import { api } from './client'

export const usersApi = {
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
}
