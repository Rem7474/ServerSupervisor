import { api } from './client'
import type { AlertRule, AlertRulePayload } from '../types/alert'

// Re-exported so existing import sites keep working from the api barrel.
export type { AlertRule, AlertRulePayload } from '../types/alert'

export const alertsApi = {
  getAgentAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/agent'),
  getProxmoxAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/proxmox'),
  getSyntheticAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/synthetic'),
  getHostCapabilities: (hostId: string) => api.get(`/v1/hosts/${hostId}/capabilities`),
  getAlertRules: () => api.get<AlertRule[]>('/v1/alert-rules'),
  createAlertRule: (payload: AlertRulePayload) => api.post('/v1/alert-rules', payload),
  updateAlertRule: (id: number, payload: AlertRulePayload) => api.patch(`/v1/alert-rules/${id}`, payload),
  deleteAlertRule: (id: number) => api.delete(`/v1/alert-rules/${id}`),
  resolveAlertIncident: (id: number | string) => api.post(`/v1/alerts/incidents/${id}/resolve`),
  testAlertRule: (payload: AlertRulePayload) => api.post('/v1/alert-rules/test', payload),
  downloadAlertRuleTestLogs: (payload: AlertRulePayload) =>
    api.post('/v1/alert-rules/test/logs', payload, { responseType: 'blob' }),
}
