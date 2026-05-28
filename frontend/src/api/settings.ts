import { api, type JsonObject } from './client'

export const settingsApi = {
  getSettings: () => api.get('/v1/settings'),
  updateSettings: (payload: JsonObject) => api.put('/v1/settings', payload),
  testSmtp: () => api.post('/v1/settings/test-smtp'),
  testNtfy: () => api.post('/v1/settings/test-ntfy'),
  cleanupMetrics: () => api.post('/v1/settings/cleanup-metrics'),
  cleanupAudit: () => api.post('/v1/settings/cleanup-audit'),
}
