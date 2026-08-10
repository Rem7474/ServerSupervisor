import { api } from './client'
import type { AlertRule, AlertRulePayload } from '../types/alert'
import type { AlertRuleTemplate, AlertRuleTemplateRequest, ApplyAlertRuleTemplateRequest, ApplyAlertRuleTemplateResult } from '../types/generated'

// Re-exported so existing import sites keep working from the api barrel.
export type { AlertRule, AlertRulePayload } from '../types/alert'

export const alertsApi = {
  getAgentAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/agent'),
  getProxmoxAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/proxmox'),
  getSyntheticAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/synthetic'),
  getDockerAlertCapabilities: () => api.get('/v1/alert-rules/capabilities/docker'),
  getHostCapabilities: (hostId: string) => api.get(`/v1/hosts/${hostId}/capabilities`),
  getAlertRules: () => api.get<AlertRule[]>('/v1/alert-rules'),
  createAlertRule: (payload: AlertRulePayload) => api.post('/v1/alert-rules', payload),
  updateAlertRule: (id: number, payload: AlertRulePayload) => api.patch(`/v1/alert-rules/${id}`, payload),
  deleteAlertRule: (id: number) => api.delete(`/v1/alert-rules/${id}`),
  resolveAlertIncident: (id: number | string) => api.post(`/v1/alerts/incidents/${id}/resolve`),
  acknowledgeAlertIncident: (id: number | string) => api.post(`/v1/alerts/incidents/${id}/ack`),
  testAlertRule: (payload: AlertRulePayload) => api.post('/v1/alert-rules/test', payload),
  downloadAlertRuleTestLogs: (payload: AlertRulePayload) =>
    api.post('/v1/alert-rules/test/logs', payload, { responseType: 'blob' }),
  getAlertRuleTemplates: () => api.get<AlertRuleTemplate[]>('/v1/alert-rule-templates'),
  createAlertRuleTemplate: (payload: AlertRuleTemplateRequest) => api.post<AlertRuleTemplate>('/v1/alert-rule-templates', payload),
  updateAlertRuleTemplate: (id: number, payload: AlertRuleTemplateRequest) =>
    api.patch<AlertRuleTemplate>(`/v1/alert-rule-templates/${id}`, payload),
  deleteAlertRuleTemplate: (id: number) => api.delete(`/v1/alert-rule-templates/${id}`),
  applyAlertRuleTemplate: (id: number, payload: ApplyAlertRuleTemplateRequest) =>
    api.post<ApplyAlertRuleTemplateResult>(`/v1/alert-rule-templates/${id}/apply`, payload),
}
