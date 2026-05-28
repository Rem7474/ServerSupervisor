import { api, type JsonObject } from './client'

export const trackersApi = {
  getReleaseTrackers: () => api.get('/v1/release-trackers'),
  getReleaseTracker: (id: string) => api.get(`/v1/release-trackers/${id}`),
  createReleaseTracker: (payload: JsonObject) => api.post('/v1/release-trackers', payload),
  createReleaseTrackersBulk: (trackers: JsonObject[]) =>
    api.post('/v1/release-trackers/bulk', { trackers }),
  getTrackableContainers: () => api.get('/v1/release-trackers/trackable-containers'),
  updateReleaseTracker: (id: string, payload: JsonObject) =>
    api.put(`/v1/release-trackers/${id}`, payload),
  deleteReleaseTracker: (id: string) => api.delete(`/v1/release-trackers/${id}`),
  checkReleaseTrackerNow: (id: string) => api.post(`/v1/release-trackers/${id}/check-now`),
  runReleaseTracker: (id: string) => api.post(`/v1/release-trackers/${id}/run`),
  getReleaseTrackerExecutions: (id: string, limit?: number) =>
    api.get(`/v1/release-trackers/${id}/executions`, { params: { limit: limit ?? 50 } }),
  getReleaseTrackerVersionHistory: (id: string, limit?: number) =>
    api.get(`/v1/release-trackers/${id}/version-history`, { params: { limit: limit ?? 20 } }),
  getRegistryCredentials: () => api.get('/v1/registry-credentials'),
  createRegistryCredential: (payload: JsonObject) => api.post('/v1/registry-credentials', payload),
  updateRegistryCredential: (id: string, payload: JsonObject) =>
    api.put(`/v1/registry-credentials/${id}`, payload),
  deleteRegistryCredential: (id: string) => api.delete(`/v1/registry-credentials/${id}`),
}
