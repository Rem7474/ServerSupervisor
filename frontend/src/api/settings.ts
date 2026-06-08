import { api } from './client'
import type { SettingsUpdateRequest } from '../types/settings'

export const settingsApi = {
  getSettings: () => api.get('/v1/settings'),
  updateSettings: (payload: Partial<SettingsUpdateRequest>) => api.put('/v1/settings', payload),
  testSmtp: () => api.post('/v1/settings/test-smtp'),
  testNtfy: () => api.post('/v1/settings/test-ntfy'),
  cleanupMetrics: () => api.post('/v1/settings/cleanup-metrics'),
  cleanupAudit: () => api.post('/v1/settings/cleanup-audit'),
}
