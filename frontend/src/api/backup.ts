import { api } from './client'

export const backupApi = {
  getBackupStatus: (hostId: string) => api.get(`/v1/hosts/${hostId}/backup`),
  getBackupRuns: (hostId: string, limit = 20) =>
    api.get(`/v1/hosts/${hostId}/backup/runs`, { params: { limit } }),
  runBackup: (hostId: string, profile?: string) =>
    api.post(`/v1/hosts/${hostId}/backup/run`, { profile }),
}
